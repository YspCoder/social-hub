package imgur

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const imageFixture = `{"id":"img1","account_id":7,"account_url":"alice","title":"Title","description":"Description","datetime":1700000000,"type":"image/png","width":640,"height":480,"size":123,"views":9,"link":"https://i.imgur.com/img1.png","in_gallery":true,"comment_count":2,"favorite_count":3,"ups":5,"downs":1,"score":4}`

func TestCommonAndTypedWorkflows(t *testing.T) {
	shareCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		expectedAuth := "Bearer access-token"
		if request.Method == http.MethodGet && (request.URL.Path == "/3/account/alice" || request.URL.Path == "/3/image/img1" || request.URL.Path == "/3/gallery/img1/comments/best" || strings.HasPrefix(request.URL.Path, "/3/album/")) {
			expectedAuth = "Client-ID client-id"
		}
		if request.Header.Get("Authorization") != expectedAuth {
			t.Errorf("%s %s Authorization=%q want %q", request.Method, request.URL.Path, request.Header.Get("Authorization"), expectedAuth)
			writeJSON(writer, http.StatusUnauthorized, `{"data":{"error":"bad auth"},"success":false,"status":401}`)
			return
		}
		switch request.Method + " " + request.URL.Path {
		case "GET /3/account/alice":
			writeEnvelope(writer, http.StatusOK, `{"id":7,"url":"alice","bio":"bio","avatar":"https://i.imgur.com/avatar.png","cover":"https://i.imgur.com/cover.png","reputation":10,"reputation_name":"Trusted","created":1600000000}`)
		case "GET /3/image/img1":
			writeEnvelope(writer, http.StatusOK, imageFixture)
		case "GET /3/account/alice/images/0":
			if request.URL.Query().Get("perPage") != "1" {
				t.Errorf("perPage=%q", request.URL.Query().Get("perPage"))
			}
			writeEnvelope(writer, http.StatusOK, `[`+imageFixture+`]`)
		case "GET /3/gallery/img1/comments/best":
			writeEnvelope(writer, http.StatusOK, `[{"id":10,"image_id":"img1","comment":"root","author":"alice","author_id":7,"datetime":1700000001,"parent_id":0,"children":[{"id":"11","image_id":"img1","comment":"reply","author":"bob","author_id":"8","datetime":1700000002,"parent_id":10}]}]`)
		case "POST /3/gallery/image/img1":
			if request.ParseForm() != nil || request.Form.Get("title") == "" || request.Form.Get("terms") != "1" {
				t.Errorf("share form=%v", request.Form)
			}
			shareCalls++
			writeEnvelope(writer, http.StatusOK, `true`)
		case "DELETE /3/gallery/img1", "POST /3/gallery/img1/vote/up", "POST /3/gallery/img1/vote/veto":
			writeEnvelope(writer, http.StatusOK, `true`)
		case "POST /3/comment":
			if request.ParseForm() != nil || request.Form.Get("image_id") != "img1" || request.Form.Get("comment") != "hello" || request.Form.Get("parent_id") != "10" {
				t.Errorf("comment form=%v", request.Form)
			}
			writeEnvelope(writer, http.StatusOK, `{"id":42}`)
		case "DELETE /3/comment/42":
			writeEnvelope(writer, http.StatusOK, `true`)
		case "POST /3/image/img1":
			if request.ParseForm() != nil || request.Form.Get("title") != "updated" {
				t.Errorf("image update form=%v", request.Form)
			}
			writeEnvelope(writer, http.StatusOK, `true`)
		case "DELETE /3/image/img1":
			writeEnvelope(writer, http.StatusOK, `true`)
		case "POST /3/image/img1/favorite":
			writeEnvelope(writer, http.StatusOK, `"favorited"`)
		case "GET /3/album/album1":
			writeEnvelope(writer, http.StatusOK, `{"id":"album1","title":"album","images_count":1,"images":[`+imageFixture+`]}`)
		case "GET /3/album/album1/images":
			writeEnvelope(writer, http.StatusOK, `[`+imageFixture+`]`)
		case "POST /3/album":
			if request.ParseForm() != nil || request.Form.Get("ids[]") != "img1" || request.Form.Get("title") != "album" {
				t.Errorf("create album form=%v", request.Form)
			}
			writeEnvelope(writer, http.StatusOK, `{"id":"album1","deletehash":"album-delete"}`)
		case "PUT /3/album/album1":
			if request.ParseForm() != nil || request.Form.Get("description") != "changed" {
				t.Errorf("update album form=%v", request.Form)
			}
			writeEnvelope(writer, http.StatusOK, `true`)
		case "DELETE /3/album/album1":
			writeEnvelope(writer, http.StatusOK, `true`)
		case "GET /3/credits":
			writeEnvelope(writer, http.StatusOK, `{"UserLimit":500,"UserRemaining":490,"UserReset":1700003600,"ClientLimit":12500,"ClientRemaining":12400}`)
		default:
			t.Errorf("unexpected request %s %s", request.Method, request.URL.String())
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	_, client := newTestClient(t, server, true)
	ctx := context.Background()

	user, err := client.GetUser(ctx, "")
	if err != nil || user.ID != "7" || user.Username == nil || *user.Username != "alice" || user.AvatarURL == nil {
		t.Fatalf("user=%#v err=%v", user, err)
	}
	post, err := client.GetPost(ctx, "img1")
	if err != nil || post.ID != "img1" || len(post.Media) != 1 || post.Media[0].Width == nil || post.Visibility == nil || *post.Visibility != "public_gallery" || len(post.Metrics) != 6 {
		t.Fatalf("post=%#v err=%v", post, err)
	}
	posts, err := client.ListPosts(ctx, socialhub.ListPostsRequest{MaxResults: 1})
	if err != nil || len(posts.Items) != 1 || !posts.HasMore || posts.NextCursor == nil || *posts.NextCursor != "1" {
		t.Fatalf("posts=%#v err=%v", posts, err)
	}
	comments, err := client.ListComments(ctx, socialhub.ListCommentsRequest{PostID: "img1", MaxResults: 2})
	if err != nil || len(comments.Items) != 2 || comments.Items[1].ParentID == nil || *comments.Items[1].ParentID != "10" {
		t.Fatalf("comments=%#v err=%v", comments, err)
	}

	title := "Gallery title"
	published, err := client.Publish(ctx, socialhub.CreatePostRequest{Text: &title, MediaIDs: []string{"img1"}})
	if err != nil || published.Status == nil || published.Status.State != socialhub.PublishStatePublished || published.CreatedAt == nil || !published.CreatedAt.Equal(testNow) {
		t.Fatalf("published=%#v err=%v", published, err)
	}
	if err := client.ShareImage(ctx, GalleryShareRequest{ImageID: "img1", Title: "Title", Topic: "funny", Mature: true, Tags: []string{"tag1", "tag2"}}); err != nil {
		t.Fatal(err)
	}
	status, err := client.PublishStatus(ctx, "img1")
	if err != nil || status.State != socialhub.PublishStatePublished {
		t.Fatalf("status=%#v err=%v", status, err)
	}
	if err := client.DeletePost(ctx, "img1"); err != nil {
		t.Fatal(err)
	}
	reaction := socialhub.ReactionRequest{ActorID: "alice", TargetID: "img1", Kind: socialhub.ReactionLike}
	if err := client.React(ctx, reaction); err != nil {
		t.Fatal(err)
	}
	if err := client.RemoveReaction(ctx, reaction); err != nil {
		t.Fatal(err)
	}
	parent := "10"
	comment, err := client.Comment(ctx, socialhub.CreateCommentRequest{PostID: "img1", ParentID: &parent, Text: "hello"})
	if err != nil || comment.ID != "42" || comment.ParentID == nil || *comment.ParentID != "10" || comment.CreatedAt == nil || !comment.CreatedAt.Equal(testNow) {
		t.Fatalf("comment=%#v err=%v", comment, err)
	}
	if err := client.DeleteComment(ctx, "42"); err != nil {
		t.Fatal(err)
	}

	updated := "updated"
	if err := client.UpdateImage(ctx, "img1", ImageUpdateRequest{Title: &updated}); err != nil {
		t.Fatal(err)
	}
	state, err := client.ToggleFavorite(ctx, "img1")
	if err != nil || state != "favorited" {
		t.Fatalf("favorite state=%q err=%v", state, err)
	}
	if err := client.DeleteImage(ctx, "img1"); err != nil {
		t.Fatal(err)
	}
	album, err := client.GetAlbum(ctx, "album1")
	if err != nil || album.ImagesCount != 1 {
		t.Fatalf("album=%#v err=%v", album, err)
	}
	images, err := client.ListAlbumImages(ctx, "album1")
	if err != nil || len(images) != 1 {
		t.Fatalf("images=%#v err=%v", images, err)
	}
	reference, err := client.CreateAlbum(ctx, AlbumRequest{ImageIDs: []string{"img1"}, Title: stringPointer("album")})
	if err != nil || reference.ID != "album1" || reference.DeleteHash != "album-delete" {
		t.Fatalf("reference=%#v err=%v", reference, err)
	}
	if err := client.UpdateAlbum(ctx, "album1", AlbumRequest{Description: stringPointer("changed")}); err != nil {
		t.Fatal(err)
	}
	if err := client.DeleteAlbum(ctx, "album1"); err != nil {
		t.Fatal(err)
	}
	credits, err := client.Credits(ctx)
	if err != nil || credits.ClientRemaining != 12400 || credits.UserRemaining != 490 {
		t.Fatalf("credits=%#v err=%v", credits, err)
	}
	if shareCalls != 2 {
		t.Fatalf("share calls=%d", shareCalls)
	}
}

func TestAnonymousImageAndAlbumManagement(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Client-ID client-id" {
			t.Errorf("Authorization=%q", request.Header.Get("Authorization"))
		}
		switch request.Method + " " + request.URL.Path {
		case "POST /3/image/deletehash", "DELETE /3/image/deletehash", "PUT /3/album/album-delete", "DELETE /3/album/album-delete":
			writeEnvelope(writer, http.StatusOK, `true`)
		case "POST /3/album":
			writeEnvelope(writer, http.StatusOK, `{"id":"album1","deletehash":"album-delete"}`)
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	_, client := newTestClient(t, server, false)
	ctx := context.Background()
	title := "anonymous"
	if err := client.UpdateImage(ctx, "deletehash", ImageUpdateRequest{Title: &title}); err != nil {
		t.Fatal(err)
	}
	if err := client.DeleteImage(ctx, "deletehash"); err != nil {
		t.Fatal(err)
	}
	reference, err := client.CreateAlbum(ctx, AlbumRequest{DeleteHashes: []string{"deletehash"}})
	if err != nil || reference.DeleteHash != "album-delete" {
		t.Fatalf("reference=%#v err=%v", reference, err)
	}
	if err := client.UpdateAlbum(ctx, "album-delete", AlbumRequest{Title: &title}); err != nil {
		t.Fatal(err)
	}
	if err := client.DeleteAlbum(ctx, "album-delete"); err != nil {
		t.Fatal(err)
	}
}

func TestWorkflowValidation(t *testing.T) {
	client := &Client{accountID: "main", username: "alice", clock: fixedClock{testNow}}
	text := "text"
	badText := string([]byte{0})
	parent := "bad/id"
	start := time.Unix(1, 0)
	tests := []struct {
		name string
		run  func() error
	}{
		{"get user", func() error { _, err := client.GetUser(context.Background(), "bad/name"); return err }},
		{"get image", func() error { _, err := client.GetImage(context.Background(), ""); return err }},
		{"list auth", func() error { _, err := client.ListAccountImages(context.Background(), "alice", "", 1); return err }},
		{"list time", func() error {
			_, err := client.ListPosts(context.Background(), socialhub.ListPostsRequest{StartTime: &start})
			return err
		}},
		{"update empty", func() error { return client.UpdateImage(context.Background(), "img1", ImageUpdateRequest{}) }},
		{"update text", func() error {
			return client.UpdateImage(context.Background(), "img1", ImageUpdateRequest{Title: &badText})
		}},
		{"delete image", func() error { return client.DeleteImage(context.Background(), "bad/id") }},
		{"favorite auth", func() error { _, err := client.ToggleFavorite(context.Background(), "img1"); return err }},
		{"album get", func() error { _, err := client.GetAlbum(context.Background(), "bad/id"); return err }},
		{"album images", func() error { _, err := client.ListAlbumImages(context.Background(), "bad/id"); return err }},
		{"album create image", func() error {
			_, err := client.CreateAlbum(context.Background(), AlbumRequest{ImageIDs: []string{"bad/id"}})
			return err
		}},
		{"album create hash", func() error {
			_, err := client.CreateAlbum(context.Background(), AlbumRequest{DeleteHashes: []string{"bad/hash"}})
			return err
		}},
		{"album update empty", func() error { return client.UpdateAlbum(context.Background(), "album", AlbumRequest{}) }},
		{"album update id", func() error { return client.UpdateAlbum(context.Background(), "bad/id", AlbumRequest{Title: &text}) }},
		{"album delete", func() error { return client.DeleteAlbum(context.Background(), "bad/id") }},
		{"share auth", func() error {
			return client.ShareImage(context.Background(), GalleryShareRequest{ImageID: "img1", Title: "title"})
		}},
		{"vote auth", func() error { return client.Vote(context.Background(), "img1", GalleryVoteUp) }},
		{"publish media", func() error {
			_, err := client.Publish(context.Background(), socialhub.CreatePostRequest{Text: &text})
			return err
		}},
		{"publish reply", func() error {
			_, err := client.Publish(context.Background(), socialhub.CreatePostRequest{Text: &text, MediaIDs: []string{"img1"}, ReplyToID: &text})
			return err
		}},
		{"publish visibility", func() error {
			visibility := "private"
			_, err := client.Publish(context.Background(), socialhub.CreatePostRequest{Text: &text, MediaIDs: []string{"img1"}, Visibility: &visibility})
			return err
		}},
		{"react kind", func() error {
			return client.React(context.Background(), socialhub.ReactionRequest{TargetID: "img1", Kind: socialhub.ReactionRepost})
		}},
		{"remove actor", func() error {
			return client.RemoveReaction(context.Background(), socialhub.ReactionRequest{ActorID: "bob", TargetID: "img1", Kind: socialhub.ReactionLike})
		}},
		{"comments cursor", func() error {
			_, err := client.ListComments(context.Background(), socialhub.ListCommentsRequest{PostID: "img1", Cursor: "1"})
			return err
		}},
		{"comment auth", func() error {
			_, err := client.Comment(context.Background(), socialhub.CreateCommentRequest{PostID: "img1", Text: "text"})
			return err
		}},
		{"comment parent", func() error {
			bearer := testBearerClient()
			_, err := bearer.Comment(context.Background(), socialhub.CreateCommentRequest{PostID: "img1", ParentID: &parent, Text: "text"})
			return err
		}},
		{"delete comment auth", func() error { return client.DeleteComment(context.Background(), "42") }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			code := errorCode(test.run())
			if code != socialhub.CodeInvalidArgument && code != socialhub.CodeUnauthenticated && code != socialhub.CodeUnsupported {
				t.Fatalf("code=%q", code)
			}
		})
	}
}

func TestHTTPEnvelopeErrorsAndHelpers(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch strings.TrimPrefix(request.URL.Path, "/3/image/") {
		case "limited":
			writer.Header().Set("Retry-After", "7")
			writer.Header().Set("X-Request-ID", "imgur-request")
			writeJSON(writer, http.StatusTooManyRequests, `{"data":{"error":"slow down","request":"/3/image/limited","method":"GET"},"success":false,"status":429}`)
		case "missing":
			writeJSON(writer, http.StatusNotFound, `{"data":{"error":"not found"},"success":false,"status":404}`)
		case "bad-json":
			writeJSON(writer, http.StatusOK, `{`)
		case "bad-envelope":
			writeJSON(writer, http.StatusOK, `{"data":{"error":"bad"},"success":false,"status":400}`)
		case "null":
			writeJSON(writer, http.StatusOK, `{"data":null,"success":true,"status":200}`)
		case "mismatch":
			writeEnvelope(writer, http.StatusOK, `{"id":"other","link":"https://i.imgur.com/other.png"}`)
		default:
			writeJSON(writer, http.StatusInternalServerError, `{"data":"down","success":false,"status":500}`)
		}
	}))
	defer server.Close()
	_, client := newTestClient(t, server, false)
	tests := []struct {
		id   string
		code socialhub.ErrorCode
	}{
		{"missing", socialhub.CodeNotFound}, {"bad-json", socialhub.CodePlatformError}, {"bad-envelope", socialhub.CodeInvalidArgument},
		{"null", socialhub.CodePlatformError}, {"mismatch", socialhub.CodePlatformError}, {"server", socialhub.CodeTemporarilyUnavailable},
	}
	for _, test := range tests {
		t.Run(test.id, func(t *testing.T) {
			_, err := client.GetImage(context.Background(), test.id)
			if errorCode(err) != test.code {
				t.Fatalf("error=%v", err)
			}
		})
	}
	_, err := client.GetImage(context.Background(), "limited")
	var platformErr *socialhub.Error
	if !errors.As(err, &platformErr) || platformErr.Code != socialhub.CodeRateLimited || platformErr.RetryAfter != 7*time.Second || platformErr.RequestID != "imgur-request" || platformErr.PlatformMessage != "slow down" {
		t.Fatalf("limited error=%#v", err)
	}

	classifications := map[int]socialhub.ErrorCode{
		http.StatusBadRequest: socialhub.CodeInvalidArgument, http.StatusUnauthorized: socialhub.CodeUnauthenticated,
		http.StatusForbidden: socialhub.CodePermissionDenied, http.StatusGone: socialhub.CodeNotFound,
		http.StatusConflict: socialhub.CodeConflict, http.StatusTooManyRequests: socialhub.CodeRateLimited,
		http.StatusServiceUnavailable: socialhub.CodeTemporarilyUnavailable, http.StatusTeapot: socialhub.CodePlatformError,
	}
	for status, want := range classifications {
		if got, _ := classifyError(status); got != want {
			t.Fatalf("status=%d got=%q want=%q", status, got, want)
		}
	}
	if parseRetryAfter("7") != 7*time.Second || parseRetryAfter("bad") != 0 || parseRetryAfter("86401") != 0 {
		t.Fatal("Retry-After parsing failed")
	}
	if boundedMessage(strings.Repeat("界", 4), 2) != "界界" || firstNonEmpty("", "value") != "value" {
		t.Fatal("message helpers failed")
	}
	if page, err := parsePage("42"); err != nil || page != 42 {
		t.Fatalf("page=%d err=%v", page, err)
	}
	for _, cursor := range []string{"-1", "1000001", "bad"} {
		if _, err := parsePage(cursor); errorCode(err) != socialhub.CodeInvalidArgument {
			t.Fatalf("cursor=%q err=%v", cursor, err)
		}
	}
	var identifier ID
	for input, want := range map[string]ID{`"abc"`: "abc", `123`: "123", `null`: ""} {
		if err := identifier.UnmarshalJSON([]byte(input)); err != nil || identifier != want {
			t.Fatalf("input=%s id=%q err=%v", input, identifier, err)
		}
	}
	if err := identifier.UnmarshalJSON([]byte(`{}`)); err == nil {
		t.Fatal("object ID must fail")
	}
}

func writeEnvelope(writer http.ResponseWriter, status int, data string) {
	writeJSON(writer, status, `{"data":`+data+`,"success":true,"status":`+strconv.Itoa(status)+`}`)
}

func stringPointer(value string) *string { return &value }

func testBearerClient() *Client {
	api := &transport.Client{}
	return &Client{accountID: "main", username: "alice", user: api, public: api, clock: fixedClock{testNow}}
}
