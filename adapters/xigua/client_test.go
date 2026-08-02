package xigua

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"social-hub/pkg/socialhub"
)

const xiguaVideoFixture = `{"item_id":"item-1","video_id":"video-1","title":"demo","cover":"https://img.example/cover.jpg","create_time":1785629045,"statistics":{"digg_count":5,"play_count":100,"share_count":3,"forward_count":2,"comment_count":7}}`

func TestUserVideoPublishAndDomainContracts(t *testing.T) {
	t.Parallel()
	var pendingLookups atomic.Int32
	var createCalls atomic.Int32
	apiServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		assertXiguaAuth(t, writer, request)
		if request.URL.Query().Get("open_id") != "open-id-1" {
			writer.WriteHeader(http.StatusBadRequest)
			return
		}
		switch request.URL.Path {
		case "/xigua/video/list/":
			if request.Method != http.MethodGet || request.URL.Query().Get("cursor") != "0" || request.URL.Query().Get("count") != "10" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			_, _ = writer.Write([]byte(`{"data":{"list":[` + xiguaVideoFixture + `],"cursor":"100","has_more":true,"error_code":0}}`))
		case "/xigua/video/data/":
			if request.Method != http.MethodPost {
				writer.WriteHeader(http.StatusMethodNotAllowed)
				return
			}
			var body struct {
				ItemIDs []string `json:"item_ids"`
			}
			if json.NewDecoder(request.Body).Decode(&body) != nil || len(body.ItemIDs) != 1 {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			if body.ItemIDs[0] == "item-pending" && pendingLookups.Add(1) == 1 {
				writer.WriteHeader(http.StatusNotFound)
				return
			}
			item := xiguaVideoFixture
			if body.ItemIDs[0] == "item-pending" {
				item = `{"item_id":"item-pending","video_id":"video-1","title":"pending demo","create_time":1785629045,"statistics":{}}`
			}
			_, _ = writer.Write([]byte(`{"data":{"list":[` + item + `],"error_code":0}}`))
		case "/xigua/video/create/":
			var body map[string]string
			if json.NewDecoder(request.Body).Decode(&body) != nil {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			if createCalls.Add(1) == 1 {
				if body["video_id"] != "video-1" || body["text"] != "pending demo" || body["abstract"] != "" {
					writer.WriteHeader(http.StatusBadRequest)
					return
				}
				_, _ = writer.Write([]byte(`{"data":{"item_id":"item-pending","error_code":0}}`))
				return
			}
			if body["video_id"] != "video-2" || body["text"] != "typed title" || body["abstract"] != "typed summary" || body["cover_tsp"] != "1200" || body["claim_origin"] != "true" || body["praise"] != "false" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			_, _ = writer.Write([]byte(`{"data":{"item_id":"item-typed","error_code":0}}`))
		default:
			writer.WriteHeader(http.StatusNotFound)
		}
	}))
	defer apiServer.Close()
	oauthServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		assertXiguaAuth(t, writer, request)
		if request.URL.Path != "/oauth/userinfo/" || request.Method != http.MethodPost || request.URL.Query().Get("open_id") != "open-id-1" {
			writer.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = writer.Write([]byte(`{"data":{"open_id":"open-id-1","union_id":"union-1","nickname":"Creator","avatar":"https://img.example/avatar.jpg","error_code":0}}`))
	}))
	defer oauthServer.Close()
	_, client := newTestAdapter(t, apiServer, oauthServer)

	user, err := client.GetUser(context.Background(), "open-id-1")
	if err != nil || user.ID != "open-id-1" || user.DisplayName == nil || *user.DisplayName != "Creator" || len(user.Extensions["xigua.user"]) == 0 {
		t.Fatalf("user=%#v err=%v", user, err)
	}
	page, err := client.ListPosts(context.Background(), socialhub.ListPostsRequest{UserID: "open-id-1"})
	if err != nil || len(page.Items) != 1 || page.NextCursor == nil || *page.NextCursor != "100" || !page.HasMore {
		t.Fatalf("page=%#v err=%v", page, err)
	}
	if page.Items[0].ID != "item-1" || len(page.Items[0].Metrics) != 5 || len(page.Items[0].Media) != 1 || page.Items[0].Media[0].ID != "video-1" {
		t.Fatalf("mapped post=%#v", page.Items[0])
	}
	post, err := client.GetPost(context.Background(), "item-1")
	if err != nil || post.Status == nil || post.Status.State != socialhub.PublishStatePublished {
		t.Fatalf("post=%#v err=%v", post, err)
	}
	text := "pending demo"
	published, err := client.Publish(context.Background(), socialhub.CreatePostRequest{Text: &text, MediaIDs: []string{"video-1"}})
	if err != nil || published.ID != "item-pending" || published.Status == nil || published.Status.State != socialhub.PublishStatePending {
		t.Fatalf("published=%#v err=%v", published, err)
	}
	status, err := client.PublishStatus(context.Background(), "item-pending")
	if err != nil || status.State != socialhub.PublishStatePending {
		t.Fatalf("pending status=%#v err=%v", status, err)
	}
	status, err = client.PublishStatus(context.Background(), "item-pending")
	if err != nil || status.State != socialhub.PublishStatePublished || status.UpdatedAt == nil {
		t.Fatalf("published status=%#v err=%v", status, err)
	}
	coverTimestamp := int64(1200)
	declareOriginal, enableReward := true, false
	typed, err := client.PublishVideo(context.Background(), PublishVideoRequest{
		VideoID: "video-2", Title: "typed title", Summary: "typed summary",
		CoverTimestampMillis: &coverTimestamp, DeclareOriginal: &declareOriginal, EnableReward: &enableReward,
	})
	if err != nil || typed.ID != "item-typed" || typed.Text == nil || *typed.Text != "typed title" || len(typed.Extensions["xigua.publish"]) == 0 {
		t.Fatalf("typed publish=%#v err=%v", typed, err)
	}
}

func assertXiguaAuth(t *testing.T, writer http.ResponseWriter, request *http.Request) {
	t.Helper()
	if request.Header.Get("access-token") != "user-token" || request.Header.Get("Authorization") != "" {
		writer.WriteHeader(http.StatusUnauthorized)
	}
}

func TestClientValidationUnsupportedAndApproval(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	_, client := newTestAdapter(t, server, server)

	tests := []struct {
		name string
		call func() error
		want error
	}{
		{"wrong user", func() error { _, err := client.GetUser(context.Background(), "other"); return err }, socialhub.ErrInvalidArgument},
		{"empty post", func() error { _, err := client.GetPost(context.Background(), ""); return err }, socialhub.ErrInvalidArgument},
		{"bad list user", func() error {
			_, err := client.ListPosts(context.Background(), socialhub.ListPostsRequest{UserID: "other"})
			return err
		}, socialhub.ErrInvalidArgument},
		{"time filter", func() error {
			_, err := client.ListPosts(context.Background(), socialhub.ListPostsRequest{StartTime: &testNow})
			return err
		}, socialhub.ErrUnsupported},
		{"bad cursor", func() error {
			_, err := client.ListPosts(context.Background(), socialhub.ListPostsRequest{Cursor: "01"})
			return err
		}, socialhub.ErrInvalidArgument},
		{"bad page size", func() error {
			_, err := client.ListPosts(context.Background(), socialhub.ListPostsRequest{MaxResults: 21})
			return err
		}, socialhub.ErrInvalidArgument},
		{"no media", func() error {
			_, err := client.Publish(context.Background(), socialhub.CreatePostRequest{})
			return err
		}, socialhub.ErrInvalidArgument},
		{"short title", func() error {
			_, err := client.PublishVideo(context.Background(), PublishVideoRequest{VideoID: "video", Title: "four"})
			return err
		}, socialhub.ErrInvalidArgument},
		{"long summary", func() error {
			_, err := client.PublishVideo(context.Background(), PublishVideoRequest{VideoID: "video", Title: "valid title", Summary: strings.Repeat("x", 401)})
			return err
		}, socialhub.ErrInvalidArgument},
		{"negative cover timestamp", func() error {
			value := int64(-1)
			_, err := client.PublishVideo(context.Background(), PublishVideoRequest{VideoID: "video", Title: "valid title", CoverTimestampMillis: &value})
			return err
		}, socialhub.ErrInvalidArgument},
		{"reply unsupported", func() error {
			id := "item"
			_, err := client.Publish(context.Background(), socialhub.CreatePostRequest{MediaIDs: []string{"video"}, ReplyToID: &id})
			return err
		}, socialhub.ErrUnsupported},
		{"delete unsupported", func() error { return client.DeletePost(context.Background(), "item") }, socialhub.ErrUnsupported},
		{"comments unsupported", func() error {
			_, err := client.ListComments(context.Background(), socialhub.ListCommentsRequest{PostID: "item"})
			return err
		}, socialhub.ErrUnsupported},
		{"bad comment post", func() error {
			_, err := client.ListComments(context.Background(), socialhub.ListCommentsRequest{})
			return err
		}, socialhub.ErrInvalidArgument},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.call(); !errors.Is(err, test.want) {
				t.Fatalf("error=%v want=%v", err, test.want)
			}
		})
	}

	client.scopes = []string{"user_info"}
	if _, err := client.Publish(context.Background(), socialhub.CreatePostRequest{MediaIDs: []string{"video"}}); !errors.Is(err, socialhub.ErrApprovalRequired) {
		t.Fatalf("scope error=%v", err)
	} else {
		var platformErr *socialhub.Error
		if !errors.As(err, &platformErr) || len(platformErr.RequiredScopes) != 1 || platformErr.ApprovalURL == "" {
			t.Fatalf("scope detail=%#v", err)
		}
	}
	capabilities, err := client.Capabilities(context.Background())
	if err != nil || capabilities[socialhub.CapPublish].Approval != socialhub.ApprovalRequired || capabilities[socialhub.CapFetch].Approval != socialhub.ApprovalRequired {
		t.Fatalf("capabilities=%#v err=%v", capabilities, err)
	}
	client.scopes = nil
	capabilities, _ = client.Capabilities(context.Background())
	if capabilities[socialhub.CapPublish].Approval != socialhub.ApprovalUnknown {
		t.Fatalf("unknown approval=%#v", capabilities[socialhub.CapPublish])
	}
}

func TestClientRejectsMismatchedPlatformIdentity(t *testing.T) {
	t.Parallel()
	apiServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/xigua/video/data/":
			_, _ = writer.Write([]byte(`{"data":{"list":[{"item_id":"other","video_id":"video-1"}],"error_code":0}}`))
		case "/xigua/video/list/":
			_, _ = writer.Write([]byte(`{"data":{"list":[{"item_id":""}],"error_code":0}}`))
		default:
			writer.WriteHeader(http.StatusNotFound)
		}
	}))
	defer apiServer.Close()
	oauthServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte(`{"data":{"open_id":"other","error_code":0}}`))
	}))
	defer oauthServer.Close()
	_, client := newTestAdapter(t, apiServer, oauthServer)
	if _, err := client.GetUser(context.Background(), "open-id-1"); !hasCode(err, socialhub.CodePlatformError) {
		t.Fatalf("user identity error=%v", err)
	}
	if _, err := client.GetPost(context.Background(), "item-1"); !hasCode(err, socialhub.CodePlatformError) {
		t.Fatalf("post identity error=%v", err)
	}
	if _, err := client.ListPosts(context.Background(), socialhub.ListPostsRequest{}); !hasCode(err, socialhub.CodePlatformError) {
		t.Fatalf("list identity error=%v", err)
	}
}

func TestClientRejectsNegativePlatformCursor(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte(`{"data":{"list":[],"cursor":-1,"has_more":true,"error_code":0}}`))
	}))
	defer server.Close()
	_, client := newTestAdapter(t, server, server)
	if _, err := client.ListPosts(context.Background(), socialhub.ListPostsRequest{}); !hasCode(err, socialhub.CodePlatformError) {
		t.Fatalf("cursor error=%v", err)
	}
}

func TestClientAcceptsVideoDataWithoutVideoID(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/xigua/video/data/" {
			writer.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = writer.Write([]byte(`{"data":{"list":[{"item_id":"item-1","title":"official sample","cover":"https://img.example/cover.jpg"}],"error_code":0}}`))
	}))
	defer server.Close()
	_, client := newTestAdapter(t, server, server)
	post, err := client.GetPost(context.Background(), "item-1")
	if err != nil || post.ID != "item-1" || len(post.Media) != 0 || len(post.Extensions["xigua.video"]) == 0 {
		t.Fatalf("post=%#v err=%v", post, err)
	}
}

func hasCode(err error, code socialhub.ErrorCode) bool {
	var platformErr *socialhub.Error
	return errors.As(err, &platformErr) && platformErr.Code == code
}
