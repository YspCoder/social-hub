package deviantart

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"social-hub/pkg/socialhub"
)

func TestTypedAndCommonWorkflows(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer access-token" || request.Header.Get("dA-minor-version") != minorVersion ||
			request.Header.Get("User-Agent") != "social-hub-tests/1.0" || request.URL.Query().Get("access_token") != "" {
			t.Errorf("headers=%v query=%v", request.Header, request.URL.Query())
			writeJSON(writer, http.StatusUnauthorized, `{"error":"unauthorized"}`)
			return
		}
		switch request.Method + " " + request.URL.Path {
		case "GET /api/v1/oauth2/user/whoami":
			writeJSON(writer, http.StatusOK, userJSON())
		case "GET /api/v1/oauth2/user/profile/sample-artist":
			writeJSON(writer, http.StatusOK, profileJSON())
		case "GET /api/v1/oauth2/deviation/" + testDeviationID:
			if request.Header.Get("X-Request-ID") == "post-request" && request.URL.Query().Get("with_session") != "" {
				t.Error("unexpected query")
			}
			writeJSON(writer, http.StatusOK, deviationJSON(testDeviationID, "A &amp; B"))
		case "GET /api/v1/oauth2/gallery/all":
			if request.URL.Query().Get("username") != "sample-artist" || request.URL.Query().Get("offset") != "10" || request.URL.Query().Get("limit") != "2" {
				t.Errorf("gallery query=%v", request.URL.Query())
			}
			writeJSON(writer, http.StatusOK, `{"has_more":true,"next_offset":12,"results":[`+
				deviationJSON(testDeviationID, "First")+`,`+deviationJSON("bbbbbbbb-cccc-dddd-eeee-ffffffffffff", "Second")+`]}`)
		case "GET /api/v1/oauth2/user/profile/posts":
			if request.URL.Query().Get("username") != "sample-artist" || request.URL.Query().Get("cursor") != "cursor-1" {
				t.Errorf("profile posts query=%v", request.URL.Query())
			}
			next := "cursor-2"
			writeJSON(writer, http.StatusOK, `{"has_more":true,"next_cursor":"`+next+`","prev_cursor":null,"results":[`+deviationJSON(testDeviationID, "Post")+`]}`)
		case "GET /api/v1/oauth2/comments/deviation/" + testDeviationID:
			if request.URL.Query().Get("offset") != "0" || request.URL.Query().Get("limit") != "2" {
				t.Errorf("comments query=%v", request.URL.Query())
			}
			writeJSON(writer, http.StatusOK, `{"has_more":true,"next_offset":2,"has_less":false,"prev_offset":null,"total":3,"thread":[`+commentJSON(testCommentID, "")+`]}`)
		case "POST /api/v1/oauth2/user/statuses/post":
			mustForm(t, request)
			if request.Form.Get("body") == "" {
				t.Errorf("status form=%v", request.Form)
			}
			writeJSON(writer, http.StatusOK, `{"statusid":"status-123"}`)
		case "POST /api/v1/oauth2/comments/post/deviation/" + testDeviationID:
			mustForm(t, request)
			if request.Form.Get("body") == "" {
				t.Errorf("comment form=%v", request.Form)
			}
			writeJSON(writer, http.StatusOK, commentJSON(testCommentID, request.Form.Get("commentid")))
		case "POST /api/v1/oauth2/collections/fave", "POST /api/v1/oauth2/collections/unfave":
			mustForm(t, request)
			if request.Form.Get("deviationid") != testDeviationID {
				t.Errorf("favourite form=%v", request.Form)
			}
			writeJSON(writer, http.StatusOK, `{"success":true,"favourites":7}`)
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	scopes := []string{"basic", "user", "browse", "user.manage", "comment.post", "collection"}
	_, client := newTestClient(t, server, scopes)
	ctx := context.Background()

	who, err := client.WhoAmI(ctx)
	if err != nil || who.UserID != testUserID || who.Profile == nil {
		t.Fatalf("who=%#v err=%v", who, err)
	}
	commonUser, err := client.GetUser(ctx, "me")
	if err != nil || commonUser.ID != testUserID || dereference(commonUser.DisplayName) != "Sample Artist" {
		t.Fatalf("common user=%#v err=%v", commonUser, err)
	}
	profile, err := client.Profile(ctx, "sample-artist")
	if err != nil || profile.ProfileURL == "" || profile.Stats.UserDeviations != 12 {
		t.Fatalf("profile=%#v err=%v", profile, err)
	}
	commonProfile, err := client.GetUser(ctx, "sample-artist")
	if err != nil || dereference(commonProfile.ProfileURL) != "https://www.deviantart.com/sample-artist" {
		t.Fatalf("common profile=%#v err=%v", commonProfile, err)
	}

	deviation, err := client.GetDeviation(ctx, testDeviationID)
	if err != nil || deviation.Content == nil || deviation.Stats.Favourites != 4 {
		t.Fatalf("deviation=%#v err=%v", deviation, err)
	}
	post, err := client.GetPost(ctx, testDeviationID, socialhub.WithRequestID("post-request"))
	if err != nil || dereference(post.Text) != "A & B" || len(post.Media) != 1 || post.Media[0].Width == nil || len(post.Metrics) != 2 {
		t.Fatalf("post=%#v err=%v", post, err)
	}
	page, err := client.ListPosts(ctx, socialhub.ListPostsRequest{Cursor: "10", MaxResults: 2})
	if err != nil || len(page.Items) != 2 || dereference(page.NextCursor) != "12" || dereference(page.PrevCursor) != "8" || !page.HasMore {
		t.Fatalf("post page=%#v err=%v", page, err)
	}
	profilePosts, err := client.ListProfilePosts(ctx, ProfilePostsRequest{Username: "sample-artist", Cursor: "cursor-1"})
	if err != nil || len(profilePosts.Results) != 1 || dereference(profilePosts.NextCursor) != "cursor-2" {
		t.Fatalf("profile posts=%#v err=%v", profilePosts, err)
	}

	comments, err := client.ListComments(ctx, socialhub.ListCommentsRequest{PostID: testDeviationID, MaxResults: 2})
	if err != nil || len(comments.Items) != 1 || dereference(comments.NextCursor) != "2" || comments.Items[0].AuthorID == nil {
		t.Fatalf("comments=%#v err=%v", comments, err)
	}
	parent := "parent-1"
	comment, err := client.Comment(ctx, socialhub.CreateCommentRequest{PostID: testDeviationID, ParentID: &parent, Text: "Nice work"})
	if err != nil || comment.ID != testCommentID || dereference(comment.ParentID) != parent {
		t.Fatalf("comment=%#v err=%v", comment, err)
	}

	status, err := client.PostStatus(ctx, StatusPostRequest{Body: "hello", ShareID: testDeviationID, ShareParentID: "parent-1", StashID: "stash-1"})
	if err != nil || status.StatusID != "status-123" {
		t.Fatalf("status=%#v err=%v", status, err)
	}
	text := "common status"
	published, err := client.Publish(ctx, socialhub.CreatePostRequest{Text: &text})
	if err != nil || published.ID != "status-123" || dereference(published.AuthorID) != testUserID || published.Status.State != socialhub.PublishStatePublished {
		t.Fatalf("published=%#v err=%v", published, err)
	}

	favourite, err := client.Favourite(ctx, FavouriteRequest{DeviationID: testDeviationID, FolderIDs: []string{"folder-1"}})
	if err != nil || favourite.Favourites != 7 {
		t.Fatalf("favourite=%#v err=%v", favourite, err)
	}
	if err := client.React(ctx, socialhub.ReactionRequest{ActorID: testUserID, TargetID: testDeviationID, Kind: socialhub.ReactionLike}); err != nil {
		t.Fatal(err)
	}
	if err := client.RemoveReaction(ctx, socialhub.ReactionRequest{TargetID: testDeviationID, Kind: socialhub.ReactionLike}); err != nil {
		t.Fatal(err)
	}
	if _, err := client.Unfavourite(ctx, FavouriteRequest{DeviationID: testDeviationID}); err != nil {
		t.Fatal(err)
	}
	if _, err := client.PublishStatus(ctx, "status-123"); !errors.Is(err, socialhub.ErrUnsupported) {
		t.Fatalf("publish status=%v", err)
	}
	if err := client.DeletePost(ctx, "status-123"); !errors.Is(err, socialhub.ErrUnsupported) {
		t.Fatalf("delete post=%v", err)
	}
	if err := client.DeleteComment(ctx, testCommentID); !errors.Is(err, socialhub.ErrUnsupported) {
		t.Fatalf("delete comment=%v", err)
	}
}

func TestWorkflowValidationAndMalformedResponses(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/api/v1/oauth2/user/whoami":
			writeJSON(writer, http.StatusOK, strings.Replace(userJSON(), testUserID, "different", 1))
		case "/api/v1/oauth2/user/profile/sample-artist":
			writeJSON(writer, http.StatusOK, strings.Replace(profileJSON(), "sample-artist", "different", 1))
		case "/api/v1/oauth2/deviation/" + testDeviationID:
			writeJSON(writer, http.StatusOK, deviationJSON("different", "bad"))
		case "/api/v1/oauth2/gallery/all":
			writeJSON(writer, http.StatusOK, `{"has_more":true,"next_offset":60000,"results":[]}`)
		case "/api/v1/oauth2/comments/deviation/" + testDeviationID:
			writeJSON(writer, http.StatusOK, `{"has_more":false,"next_offset":null,"has_less":true,"prev_offset":null,"thread":[]}`)
		case "/api/v1/oauth2/user/profile/posts":
			writeJSON(writer, http.StatusOK, `{"has_more":true,"next_cursor":null,"prev_cursor":null,"results":[]}`)
		case "/api/v1/oauth2/user/statuses/post":
			writeJSON(writer, http.StatusOK, `{"statusid":"bad id"}`)
		case "/api/v1/oauth2/comments/post/deviation/" + testDeviationID:
			writeJSON(writer, http.StatusOK, `{"commentid":"bad id"}`)
		case "/api/v1/oauth2/collections/fave":
			writeJSON(writer, http.StatusOK, `{"success":false,"favourites":-1}`)
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	_, client := newTestClient(t, server, nil)
	ctx := context.Background()

	for name, call := range map[string]func() error{
		"who":       func() error { _, err := client.WhoAmI(ctx); return err },
		"profile":   func() error { _, err := client.Profile(ctx, "sample-artist"); return err },
		"deviation": func() error { _, err := client.GetDeviation(ctx, testDeviationID); return err },
		"gallery": func() error {
			_, err := client.ListGallery(ctx, GalleryPageRequest{Username: "sample-artist"})
			return err
		},
		"comments": func() error {
			_, err := client.ListDeviationComments(ctx, DeviationCommentsRequest{DeviationID: testDeviationID})
			return err
		},
		"profile posts": func() error {
			_, err := client.ListProfilePosts(ctx, ProfilePostsRequest{Username: "sample-artist"})
			return err
		},
		"status": func() error { _, err := client.PostStatus(ctx, StatusPostRequest{Body: "x"}); return err },
		"comment": func() error {
			_, err := client.PostDeviationComment(ctx, DeviationCommentRequest{DeviationID: testDeviationID, Body: "x"})
			return err
		},
		"favourite": func() error {
			_, err := client.Favourite(ctx, FavouriteRequest{DeviationID: testDeviationID})
			return err
		},
	} {
		t.Run(name, func(t *testing.T) {
			if err := call(); err == nil {
				t.Fatal("expected malformed response error")
			}
		})
	}

	text := "x"
	parent := "bad id"
	invalid := []func() error{
		func() error { _, err := client.Profile(ctx, "bad/name"); return err },
		func() error { _, err := client.GetDeviation(ctx, "bad id"); return err },
		func() error {
			_, err := client.ListGallery(ctx, GalleryPageRequest{Username: "x", Offset: -1})
			return err
		},
		func() error {
			_, err := client.ListProfilePosts(ctx, ProfilePostsRequest{Username: "x", Cursor: "bad\n"})
			return err
		},
		func() error {
			_, err := client.ListDeviationComments(ctx, DeviationCommentsRequest{DeviationID: "bad id"})
			return err
		},
		func() error { _, err := client.PostStatus(ctx, StatusPostRequest{}); return err },
		func() error {
			_, err := client.PostStatus(ctx, StatusPostRequest{ShareParentID: "parent-1"})
			return err
		},
		func() error {
			_, err := client.PostDeviationComment(ctx, DeviationCommentRequest{DeviationID: testDeviationID})
			return err
		},
		func() error {
			_, err := client.Favourite(ctx, FavouriteRequest{DeviationID: testDeviationID, FolderIDs: []string{"same", "same"}})
			return err
		},
		func() error {
			return client.React(ctx, socialhub.ReactionRequest{TargetID: testDeviationID, Kind: socialhub.ReactionRepost})
		},
		func() error {
			return client.RemoveReaction(ctx, socialhub.ReactionRequest{ActorID: "other", TargetID: testDeviationID, Kind: socialhub.ReactionLike})
		},
		func() error {
			_, err := client.Comment(ctx, socialhub.CreateCommentRequest{PostID: testDeviationID, ParentID: &parent})
			return err
		},
		func() error { _, err := client.Publish(ctx, socialhub.CreatePostRequest{}); return err },
		func() error {
			_, err := client.Publish(ctx, socialhub.CreatePostRequest{Text: &text, MediaIDs: []string{"media"}})
			return err
		},
		func() error {
			_, err := client.ListPosts(ctx, socialhub.ListPostsRequest{StartTime: &testNow})
			return err
		},
		func() error {
			_, err := client.ListComments(ctx, socialhub.ListCommentsRequest{PostID: "bad id"})
			return err
		},
	}
	for index, call := range invalid {
		if err := call(); err == nil {
			t.Fatalf("invalid call %d succeeded", index)
		}
	}
	if _, err := (&Client{}).GetPost(ctx, testDeviationID); !errors.Is(err, socialhub.ErrUnauthenticated) {
		t.Fatalf("missing API=%v", err)
	}
	if _, err := client.mapDeviation(Deviation{}); err == nil {
		t.Fatal("invalid deviation mapping succeeded")
	}
	if _, err := client.mapComment(testDeviationID, Comment{}); err == nil {
		t.Fatal("invalid comment mapping succeeded")
	}
}

func mustForm(t *testing.T, request *http.Request) url.Values {
	t.Helper()
	if err := request.ParseForm(); err != nil {
		t.Fatal(err)
	}
	return request.Form
}

func userJSON() string {
	return `{"userid":"` + testUserID + `","username":"sample-artist","usericon":"https://images.example/avatar.png","type":"regular","profile":{"user_is_artist":true,"artist_level":"professional","artist_speciality":"digital","real_name":"Sample Artist","tagline":"Art","website":"https://example.test","cover_photo":"https://images.example/cover.png"},"stats":{"watchers":10,"friends":2}}`
}

func profileJSON() string {
	return `{"user":` + userJSON() + `,"is_watching":false,"profile_url":"https://www.deviantart.com/sample-artist","user_is_artist":true,"artist_level":"professional","artist_specialty":"digital","real_name":"Sample Artist","tagline":"Art","countryid":1,"country":"US","website":"https://example.test","bio":"Bio","cover_photo":null,"last_status":null,"stats":{"user_deviations":12,"user_favourites":4,"user_comments":3,"profile_pageviews":99}}`
}

func deviationJSON(id, title string) string {
	return `{"deviationid":"` + id + `","printid":null,"url":"https://www.deviantart.com/sample-artist/art/work","title":"` + title + `","is_favourited":true,"is_deleted":false,"is_published":true,"author":{"userid":"` + testUserID + `","username":"sample-artist","usericon":"https://images.example/avatar.png","type":"regular"},"stats":{"comments":2,"favourites":4},"published_time":"2026-08-01T12:00:00Z","allows_comments":true,"content":{"src":"https://images.example/work.png","height":600,"width":800,"transparency":false,"filesize":1234},"is_mature":false}`
}

func commentJSON(id, parent string) string {
	parentValue := "null"
	if parent != "" {
		parentValue = `"` + parent + `"`
	}
	return `{"commentid":"` + id + `","parentid":` + parentValue + `,"posted":"2026-08-01T13:00:00Z","replies":0,"hidden":null,"body":"Great","is_liked":false,"is_featured":false,"likes":3,"user":{"userid":"` + testUserID + `","username":"sample-artist","usericon":"https://images.example/avatar.png","type":"regular"}}`
}
