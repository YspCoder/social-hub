package mixcloud

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

const userFixture = `{
  "key":"/sample-dj/","url":"https://www.mixcloud.com/sample-dj/","name":"Sample DJ","username":"sample-dj",
  "pictures":{"large":"https://images.example/avatar.jpg"},"biog":"Radio host","created_time":"2020-01-02T03:04:05Z",
  "follower_count":12,"following_count":3,"cloudcast_count":4,"favorite_count":5,"listen_count":6,"is_pro":true,
  "city":"Shanghai","country":"China"
}`

const cloudcastFixture = `{
  "key":"/sample-dj/episode-1/","url":"https://www.mixcloud.com/sample-dj/episode-1/","name":"Episode 1",
  "description":"Deep session","slug":"episode-1","created_time":"2026-08-01T01:02:03Z","updated_time":"2026-08-01T02:03:04Z",
  "play_count":100,"favorite_count":20,"comment_count":2,"listener_count":80,"repost_count":4,
  "pictures":{"extra_large":"https://images.example/cover.jpg"},
  "user":{"key":"/sample-dj/","url":"https://www.mixcloud.com/sample-dj/","name":"Sample DJ","username":"sample-dj","pictures":{}},
  "hosts":[],"audio_length":3600,"sections":[{"chapter":"Intro","start_time":0}]
}`

const commentFixture = `{
  "key":"/comments/cr/64/c123/","url":"https://www.mixcloud.com/sample-dj/episode-1/?comment=1",
  "user":{"key":"/listener/","url":"https://www.mixcloud.com/listener/","name":"Listener","username":"listener","pictures":{}},
  "submit_date":"2026-08-01T04:05:06Z","comment":"Great show"
}`

func TestFetchAndCommonMapping(t *testing.T) {
	start := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 8, 2, 0, 0, 0, 0, time.UTC)
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Query().Get("access_token") != "access-token" || request.UserAgent() != "social-hub-tests/1.0" {
			t.Errorf("auth/headers URL=%s headers=%v", request.URL, request.Header)
		}
		switch request.URL.Path {
		case "/me/", "/sample-dj/":
			writeJSON(writer, http.StatusOK, userFixture)
		case "/sample-dj/episode-1/":
			writeJSON(writer, http.StatusOK, cloudcastFixture)
		case "/sample-dj/супервсё2/":
			body := strings.ReplaceAll(cloudcastFixture, "episode-1", "супервсё2")
			writeJSON(writer, http.StatusOK, body)
		case "/sample-dj/cloudcasts/":
			query := request.URL.Query()
			if query.Get("limit") != "2" || query.Get("offset") != "20" || query.Get("since") != fmt.Sprint(start.Unix()) || query.Get("until") != fmt.Sprint(end.Unix()) {
				t.Errorf("cloudcast query=%v", query)
			}
			body := fmt.Sprintf(`{"data":[%s],"paging":{"next":%q,"previous":%q},"name":"uploads"}`,
				cloudcastFixture, server.URL+"/sample-dj/cloudcasts/?limit=2&offset=22", server.URL+"/sample-dj/cloudcasts/?limit=2&offset=18")
			writeJSON(writer, http.StatusOK, body)
		case "/sample-dj/episode-1/comments/":
			query := request.URL.Query()
			if query.Get("limit") != "2" || query.Get("offset") != "2" {
				t.Errorf("comment query=%v", query)
			}
			body := fmt.Sprintf(`{"data":[%s],"paging":{"next":%q,"previous":%q}}`,
				commentFixture, server.URL+"/sample-dj/episode-1/comments/?limit=2&offset=4", server.URL+"/sample-dj/episode-1/comments/?limit=2&offset=0")
			writeJSON(writer, http.StatusOK, body)
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	_, client := newTestClient(t, server, "pro")

	current, err := client.CurrentUser(context.Background())
	if err != nil || current.Username != "sample-dj" {
		t.Fatalf("current=%#v err=%v", current, err)
	}
	user, err := client.GetUser(context.Background(), "/sample-dj/")
	if err != nil || user.ID != "/sample-dj/" || pointerValue(user.DisplayName) != "Sample DJ" ||
		pointerValue(user.AvatarURL) != "https://images.example/avatar.jpg" || pointerValue(user.AccountType) != "pro" {
		t.Fatalf("user=%#v err=%v", user, err)
	}
	me, err := client.GetUser(context.Background(), "me")
	if err != nil || me.ID != user.ID {
		t.Fatalf("me=%#v err=%v", me, err)
	}
	post, err := client.GetPost(context.Background(), "/sample-dj/episode-1/")
	if err != nil || post.ID != "/sample-dj/episode-1/" || pointerValue(post.AuthorID) != "/sample-dj/" ||
		pointerValue(post.Text) != "Deep session" || len(post.Media) != 2 || post.Media[0].Type != socialhub.MediaTypeAudio ||
		post.Media[0].Duration == nil || *post.Media[0].Duration != time.Hour || post.Media[1].URL != "https://images.example/cover.jpg" ||
		len(post.Metrics) != 5 {
		t.Fatalf("post=%#v err=%v", post, err)
	}
	unicodePost, err := client.GetPost(context.Background(), "/sample-dj/супервсё2/")
	if err != nil || unicodePost.ID != "/sample-dj/супервсё2/" {
		t.Fatalf("Unicode post=%#v err=%v", unicodePost, err)
	}
	posts, err := client.ListPosts(context.Background(), socialhub.ListPostsRequest{
		UserID: "/sample-dj/", Cursor: "20", MaxResults: 2, StartTime: &start, EndTime: &end,
	})
	if err != nil || len(posts.Items) != 1 || pointerValue(posts.NextCursor) != "22" || pointerValue(posts.PrevCursor) != "18" || !posts.HasMore {
		t.Fatalf("posts=%#v err=%v", posts, err)
	}
	comments, err := client.ListComments(context.Background(), socialhub.ListCommentsRequest{
		PostID: "/sample-dj/episode-1/", Cursor: "2", MaxResults: 2,
	})
	if err != nil || len(comments.Items) != 1 || comments.Items[0].Text != "Great show" ||
		comments.Items[0].PostID != "/sample-dj/episode-1/" || pointerValue(comments.Items[0].AuthorID) != "/listener/" ||
		pointerValue(comments.NextCursor) != "4" || pointerValue(comments.PrevCursor) != "0" {
		t.Fatalf("comments=%#v err=%v", comments, err)
	}
}

func TestDiscoveryWorkflows(t *testing.T) {
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/search/" || request.URL.Query().Get("q") != "deep radio" ||
			request.URL.Query().Get("limit") != "3" || request.URL.Query().Get("offset") != "6" {
			t.Errorf("request=%s", request.URL)
		}
		next := server.URL + "/search/?q=deep+radio&type=" + request.URL.Query().Get("type") + "&limit=3&offset=9&access_token=sensitive"
		switch request.URL.Query().Get("type") {
		case "cloudcast":
			writeJSON(writer, http.StatusOK, fmt.Sprintf(`{"data":[%s],"paging":{"next":%q}}`, cloudcastFixture, next))
		case "user":
			writeJSON(writer, http.StatusOK, fmt.Sprintf(`{"data":[%s],"paging":{"next":%q}}`, userFixture, next))
		case "tag":
			writeJSON(writer, http.StatusOK, fmt.Sprintf(`{"data":[{"key":"/genres/deep-house/","url":"https://www.mixcloud.com/genres/deep-house/","name":"Deep House"}],"paging":{"next":%q}}`, next))
		default:
			http.Error(writer, "bad type", http.StatusBadRequest)
		}
	}))
	defer server.Close()
	_, client := newTestClient(t, server, "")
	input := SearchRequest{Query: "deep radio", Cursor: "6", MaxResults: 3}
	cloudcasts, err := client.SearchCloudcasts(context.Background(), input)
	if err != nil || len(cloudcasts.Data) != 1 || strings.Contains(cloudcasts.Paging.Next, "access_token") {
		t.Fatalf("cloudcasts=%#v err=%v", cloudcasts, err)
	}
	users, err := client.SearchUsers(context.Background(), input)
	if err != nil || len(users.Data) != 1 {
		t.Fatalf("users=%#v err=%v", users, err)
	}
	tags, err := client.SearchTags(context.Background(), input)
	if err != nil || len(tags.Data) != 1 || tags.Data[0].Name != "Deep House" {
		t.Fatalf("tags=%#v err=%v", tags, err)
	}
}

func TestFetchValidationAndResponseGuards(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/other/":
			writeJSON(writer, http.StatusOK, userFixture)
		case "/sample-dj/wrong/":
			writeJSON(writer, http.StatusOK, cloudcastFixture)
		case "/sample-dj/cloudcasts/":
			writeJSON(writer, http.StatusOK, `{"data":[],"paging":{"next":"https://evil.example/?offset=2"}}`)
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	_, client := newTestClient(t, server, "")
	if _, err := client.GetMixcloudUser(context.Background(), "bad/name"); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("invalid user=%v", err)
	}
	if _, err := client.GetMixcloudUser(context.Background(), "other"); err == nil {
		t.Fatal("mismatched user succeeded")
	}
	if _, err := client.GetCloudcast(context.Background(), "/sample-dj/wrong/"); err == nil {
		t.Fatal("mismatched Cloudcast succeeded")
	}
	if _, err := client.ListPosts(context.Background(), socialhub.ListPostsRequest{Cursor: "-1"}); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("invalid cursor=%v", err)
	}
	if _, err := client.ListPosts(context.Background(), socialhub.ListPostsRequest{MaxResults: 101}); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("invalid limit=%v", err)
	}
	start, end := testNow, testNow.Add(-time.Hour)
	if _, err := client.ListPosts(context.Background(), socialhub.ListPostsRequest{StartTime: &start, EndTime: &end}); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("invalid time=%v", err)
	}
	if _, err := client.ListCloudcastComments(context.Background(), "/sample-dj/episode-1/", PageRequest{StartTime: &start}); !errors.Is(err, socialhub.ErrUnsupported) {
		t.Fatalf("comment time=%v", err)
	}
	if _, err := client.ListPosts(context.Background(), socialhub.ListPostsRequest{}); err == nil {
		t.Fatal("foreign paging URL succeeded")
	}
	if _, err := client.SearchTags(context.Background(), SearchRequest{}); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("empty search=%v", err)
	}
	if _, err := client.ListComments(context.Background(), socialhub.ListCommentsRequest{PostID: "bad"}); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("invalid comment target=%v", err)
	}
}

func pointerValue[T any](value *T) T {
	if value == nil {
		var zero T
		return zero
	}
	return *value
}
