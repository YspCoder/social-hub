package soundcloud

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"social-hub/pkg/socialhub"
)

func TestFetchReactionsAndActivityFeed(t *testing.T) {
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "OAuth access-token" {
			writer.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch request.Method + " " + request.URL.Path {
		case "GET /api/me":
			writeJSON(writer, `{"urn":"`+testUserURN+`","username":"artist","full_name":"Artist Name","avatar_url":"https://cdn.example/avatar.jpg","permalink_url":"https://soundcloud.com/artist","plan":"pro","followers_count":10,"track_count":2}`)
		case "GET /api/users/soundcloud:users:789":
			writeJSON(writer, `{"urn":"soundcloud:users:789","username":"listener"}`)
		case "GET /api/tracks/soundcloud:tracks:456":
			writeJSON(writer, trackJSON(testTrackURN))
		case "GET /api/me/tracks":
			if request.URL.Query().Get("linked_partitioning") != "true" || request.URL.Query().Get("limit") != "200" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			writeJSON(writer, `{"collection":[`+trackJSON(testTrackURN)+`],"next_href":"`+server.URL+`/api/me/tracks?cursor=next-token"}`)
		case "GET /api/users/soundcloud:users:789/tracks":
			writeJSON(writer, `{"collection":[]}`)
		case "GET /api/tracks/soundcloud:tracks:456/comments":
			writeJSON(writer, `{"collection":[{"urn":"soundcloud:comments:10","track_urn":"`+testTrackURN+`","user_urn":"soundcloud:users:789","body":"Nice","timestamp":"1234","created_at":"2026-08-01T01:00:00Z","uri":"https://api.soundcloud.com/comments/10"}],"next_href":"`+server.URL+`/api/comments?cursor=comment-next"}`)
		case "POST /api/likes/tracks/soundcloud:tracks:456", "DELETE /api/likes/tracks/soundcloud:tracks:456", "POST /api/reposts/tracks/soundcloud:tracks:456":
			writer.WriteHeader(http.StatusCreated)
		case "POST /api/tracks/soundcloud:tracks:456/comments":
			var body struct {
				Comment struct {
					Body string `json:"body"`
				} `json:"comment"`
			}
			if json.NewDecoder(request.Body).Decode(&body) != nil || body.Comment.Body != "Great track" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			writeJSON(writer, `{"urn":"soundcloud:comments:11","track_urn":"`+testTrackURN+`","user_urn":"`+testUserURN+`","body":"Great track","created_at":"2026-08-01T02:00:00Z"}`)
		case "GET /api/me/feed":
			writeJSON(writer, `{"collection":[`+
				`{"type":"track:repost","created_at":"2026-08-01T03:00:00Z","reposter":"soundcloud:users:789","origin":`+trackJSON(testTrackURN)+`},`+
				`{"type":"playlist","created_at":"2026-08-01T04:00:00Z","origin":{"urn":"soundcloud:playlists:20","title":"Set","sharing":"public","user_urn":"`+testUserURN+`","tracks":[{"urn":"`+testTrackURN+`"}]}},`+
				`{"type":"future:type","created_at":"2026-08-01T05:00:00Z","origin":{"value":1}}],`+
				`"next_href":"`+server.URL+`/api/me/feed?cursor=feed-next","future_href":"`+server.URL+`/api/me/feed?cursor=feed-future"}`)
		default:
			writer.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	_, client := newTestAdapter(t, server)

	user, err := client.GetUser(context.Background(), "")
	if err != nil || user.ID != testUserURN || user.DisplayName == nil || *user.DisplayName != "Artist Name" || user.AccountType == nil || *user.AccountType != "pro" {
		t.Fatalf("user=%#v err=%v", user, err)
	}
	other, err := client.GetUser(context.Background(), "soundcloud:users:789")
	if err != nil || other.Username == nil || *other.Username != "listener" {
		t.Fatalf("other=%#v err=%v", other, err)
	}
	post, err := client.GetPost(context.Background(), testTrackURN)
	if err != nil || post.ID != testTrackURN || len(post.Media) != 1 || post.Media[0].Type != socialhub.MediaTypeAudio || post.Media[0].Duration == nil || len(post.Metrics) != 5 {
		t.Fatalf("post=%#v err=%v", post, err)
	}
	page, err := client.ListPosts(context.Background(), socialhub.ListPostsRequest{MaxResults: 250})
	if err != nil || len(page.Items) != 1 || page.NextCursor == nil || *page.NextCursor != "next-token" || !page.HasMore {
		t.Fatalf("posts=%#v err=%v", page, err)
	}
	otherPage, err := client.ListPosts(context.Background(), socialhub.ListPostsRequest{UserID: "soundcloud:users:789", Cursor: "cursor-1"})
	if err != nil || len(otherPage.Items) != 0 {
		t.Fatalf("other posts=%#v err=%v", otherPage, err)
	}
	comments, err := client.ListComments(context.Background(), socialhub.ListCommentsRequest{PostID: testTrackURN, MaxResults: 10})
	if err != nil || len(comments.Items) != 1 || comments.Items[0].ID != "soundcloud:comments:10" || comments.NextCursor == nil {
		t.Fatalf("comments=%#v err=%v", comments, err)
	}

	like := socialhub.ReactionRequest{ActorID: testUserURN, TargetID: testTrackURN, Kind: socialhub.ReactionLike}
	if err := client.React(context.Background(), like); err != nil {
		t.Fatal(err)
	}
	if err := client.RemoveReaction(context.Background(), like); err != nil {
		t.Fatal(err)
	}
	repost := socialhub.ReactionRequest{TargetID: testTrackURN, Kind: socialhub.ReactionRepost}
	if err := client.React(context.Background(), repost); err != nil {
		t.Fatal(err)
	}
	if err := client.RemoveReaction(context.Background(), repost); !errors.Is(err, socialhub.ErrUnsupported) {
		t.Fatalf("unrepost error=%v", err)
	}
	comment, err := client.Comment(context.Background(), socialhub.CreateCommentRequest{PostID: testTrackURN, Text: "Great track"})
	if err != nil || comment.ID != "soundcloud:comments:11" {
		t.Fatalf("comment=%#v err=%v", comment, err)
	}
	feed, err := client.Feed(context.Background(), "", 50)
	if err != nil || len(feed.Items) != 3 || feed.Items[0].Track == nil || feed.Items[1].Playlist == nil || len(feed.Items[1].Playlist.TrackURNs) != 1 || len(feed.Items[2].RawOrigin) == 0 || feed.NextCursor == nil || feed.FutureCursor == nil {
		t.Fatalf("feed=%#v err=%v", feed, err)
	}
}

func TestFetchReactionValidationAndPlatformErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/api/me":
			writeJSON(writer, `{"urn":"soundcloud:users:999"}`)
		case "/api/tracks/soundcloud:tracks:456":
			writeJSON(writer, trackJSON("soundcloud:tracks:999"))
		case "/api/me/tracks":
			writeJSON(writer, `{"collection":[],"next_href":"https://evil.example/items?cursor=bad"}`)
		default:
			writer.Header().Set("Retry-After", "12")
			writer.Header().Set("X-Request-Id", "request-1")
			writer.WriteHeader(http.StatusTooManyRequests)
			writeJSON(writer, `{"code":429,"message":"slow down"}`)
		}
	}))
	defer server.Close()
	_, client := newTestAdapter(t, server)

	if _, err := client.GetUser(context.Background(), ""); errorCode(err) != socialhub.CodePlatformError {
		t.Fatalf("configured user mismatch=%v", err)
	}
	if _, err := client.GetPost(context.Background(), testTrackURN); errorCode(err) != socialhub.CodePlatformError {
		t.Fatalf("track mismatch=%v", err)
	}
	if _, err := client.ListPosts(context.Background(), socialhub.ListPostsRequest{}); errorCode(err) != socialhub.CodePlatformError {
		t.Fatalf("pagination origin error=%v", err)
	}
	invalidCalls := []func() error{
		func() error { _, err := client.GetUser(context.Background(), "123"); return err },
		func() error { _, err := client.GetPost(context.Background(), "456"); return err },
		func() error {
			_, err := client.ListPosts(context.Background(), socialhub.ListPostsRequest{MaxResults: -1})
			return err
		},
		func() error {
			_, err := client.ListComments(context.Background(), socialhub.ListCommentsRequest{PostID: "bad"})
			return err
		},
		func() error {
			return client.React(context.Background(), socialhub.ReactionRequest{TargetID: "bad", Kind: socialhub.ReactionLike})
		},
		func() error {
			return client.React(context.Background(), socialhub.ReactionRequest{ActorID: "soundcloud:users:999", TargetID: testTrackURN, Kind: socialhub.ReactionLike})
		},
	}
	for index, call := range invalidCalls {
		if err := call(); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Fatalf("invalid call %d error=%v", index, err)
		}
	}
	start := testNow
	if _, err := client.ListPosts(context.Background(), socialhub.ListPostsRequest{StartTime: &start}); !errors.Is(err, socialhub.ErrUnsupported) {
		t.Fatalf("time filter error=%v", err)
	}
	parent := "soundcloud:comments:1"
	if _, err := client.Comment(context.Background(), socialhub.CreateCommentRequest{PostID: testTrackURN, ParentID: &parent, Text: "reply"}); !errors.Is(err, socialhub.ErrUnsupported) {
		t.Fatalf("reply error=%v", err)
	}
	if err := client.DeleteComment(context.Background(), "soundcloud:comments:1"); !errors.Is(err, socialhub.ErrUnsupported) {
		t.Fatalf("delete comment error=%v", err)
	}
	if err := client.React(context.Background(), socialhub.ReactionRequest{TargetID: testTrackURN, Kind: "clap"}); !errors.Is(err, socialhub.ErrUnsupported) {
		t.Fatalf("reaction kind error=%v", err)
	}
	err := decodeHTTPError(http.StatusTooManyRequests, http.Header{"Retry-After": {"12"}, "X-Request-Id": {"request-1"}}, []byte(`{"code":429,"message":"slow down"}`))
	var hubError *socialhub.Error
	if !errors.As(err, &hubError) || hubError.Code != socialhub.CodeRateLimited || hubError.PlatformCode != "429" || hubError.RequestID != "request-1" || hubError.RetryAfter.Seconds() != 12 || !strings.Contains(hubError.PlatformMessage, "slow") {
		t.Fatalf("HTTP error=%#v", err)
	}
}
