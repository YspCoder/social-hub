package lemmy

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

func TestPostWorkflow(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer jwt-token" {
			writer.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch request.Method + " " + request.URL.Path {
		case "POST /api/v3/post":
			var payload createPostPayload
			if err := json.NewDecoder(request.Body).Decode(&payload); err != nil || payload.Name != "Uploaded image" ||
				payload.CommunityID != 5 || payload.URL == "" || payload.LanguageID == nil || *payload.LanguageID != 2 {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			writeJSON(writer, http.StatusCreated, `{"post_view":`+postViewFixture(41, 7, 5, payload.Name)+`}`)
		case "GET /api/v3/post":
			writeJSON(writer, http.StatusOK, `{"post_view":`+postViewFixture(41, 7, 5, "Uploaded image")+`}`)
		case "PUT /api/v3/post":
			var payload updatePostPayload
			if err := json.NewDecoder(request.Body).Decode(&payload); err != nil || payload.PostID != 41 || payload.Name == nil || *payload.Name != "Updated title" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			writeJSON(writer, http.StatusOK, `{"post_view":`+postViewFixture(41, 7, 5, *payload.Name)+`}`)
		case "POST /api/v3/post/delete":
			var payload struct {
				PostID  int64 `json:"post_id"`
				Deleted bool  `json:"deleted"`
			}
			if err := json.NewDecoder(request.Body).Decode(&payload); err != nil || payload.PostID != 41 || !payload.Deleted {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			fixture := strings.Replace(postViewFixture(41, 7, 5, "Updated title"), `"local":true`, `"local":true,"deleted":true`, 1)
			writeJSON(writer, http.StatusOK, `{"post_view":`+fixture+`}`)
		case "GET /api/v3/post/list":
			query := request.URL.Query()
			if query.Get("type_") != "Local" || query.Get("sort") != "New" || query.Get("limit") != "1" ||
				query.Get("page_cursor") != "opaque" || query.Get("community_id") != "5" || query.Get("show_nsfw") != "true" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			writeJSON(writer, http.StatusOK, `{"posts":[`+postViewFixture(41, 7, 5, "Uploaded image")+`],"next_page":"next opaque"}`)
		default:
			writer.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	_, client := newTestClient(t, server)
	size := int64(3)
	client.media["image.png"] = socialhub.Media{
		ID: "image.png", URL: server.URL + "/pictrs/image/image.png", MIME: "image/png",
		Type: socialhub.MediaTypeImage, Size: &size, State: socialhub.MediaStateReady,
	}

	created, err := client.CreatePost(context.Background(), CreatePostRequest{
		Title: "Uploaded image", CommunityID: "5", MediaID: "image.png", Body: "A body", AltText: "Alt", LanguageID: "2",
	})
	if err != nil || created.Common.ID != "41" || created.Title != "Uploaded image" || created.CommunityID != "5" ||
		created.LanguageID != "2" || created.Score != 7 || len(created.Raw) == 0 {
		t.Fatalf("created=%#v err=%v", created, err)
	}
	got, err := client.GetLemmyPost(context.Background(), "41")
	if err != nil || got.Common.ID != "41" || got.Title != "Uploaded image" {
		t.Fatalf("got=%#v err=%v", got, err)
	}
	title := "Updated title"
	nsfw := true
	updated, err := client.UpdatePost(context.Background(), "41", UpdatePostRequest{Title: &title, NSFW: &nsfw})
	if err != nil || updated.Title != title {
		t.Fatalf("updated=%#v err=%v", updated, err)
	}
	feed, err := client.ListFeed(context.Background(), FeedRequest{
		Listing: ListingLocal, Sort: SortNew, Cursor: "opaque", MaxResults: 1, CommunityID: "5", ShowNSFW: true,
	})
	if err != nil || len(feed.Items) != 1 || feed.NextCursor == nil || *feed.NextCursor != "next opaque" || !feed.HasMore {
		t.Fatalf("feed=%#v err=%v", feed, err)
	}
	if err := client.DeletePost(context.Background(), "41"); err != nil {
		t.Fatal(err)
	}
}

func TestPostWorkflowValidation(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	_, client := newTestClient(t, server)
	invalidCreates := []CreatePostRequest{
		{},
		{Title: "ok", CommunityID: "5"},
		{Title: "Valid title", CommunityID: "0"},
		{Title: "Valid title", CommunityID: "5", URL: "https://example.test", MediaID: "image"},
		{Title: "Valid title", CommunityID: "5", MediaID: "missing"},
		{Title: "Valid title", CommunityID: "5", URL: "ftp://example.test"},
		{Title: "Valid title", CommunityID: "5", CustomThumbnailURL: "bad"},
		{Title: "Valid title", CommunityID: "5", LanguageID: "bad"},
		{Title: "Valid title", CommunityID: "5", Body: "bad\x00body"},
	}
	for index, input := range invalidCreates {
		if _, err := client.CreatePost(context.Background(), input); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Fatalf("invalid create %d error=%v", index, err)
		}
	}
	blank := "x"
	badURL := "ftp://example.test"
	badLanguage := "0"
	badBody := "bad\x00body"
	badThumbnail := "not-url"
	invalidUpdates := []struct {
		id    string
		input UpdatePostRequest
	}{
		{"bad", UpdatePostRequest{Title: stringPointer("Valid title")}},
		{"1", UpdatePostRequest{}},
		{"1", UpdatePostRequest{Title: &blank}},
		{"1", UpdatePostRequest{URL: &badURL}},
		{"1", UpdatePostRequest{LanguageID: &badLanguage}},
		{"1", UpdatePostRequest{Body: &badBody}},
		{"1", UpdatePostRequest{CustomThumbnailURL: &badThumbnail}},
	}
	for index, test := range invalidUpdates {
		if _, err := client.UpdatePost(context.Background(), test.id, test.input); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Fatalf("invalid update %d error=%v", index, err)
		}
	}
	if err := client.DeletePost(context.Background(), "bad"); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("bad delete=%v", err)
	}
	invalidFeeds := []FeedRequest{
		{MaxResults: 51}, {Cursor: "bad\ncursor"}, {CommunityID: "5", CommunityName: "go"},
		{CommunityID: "bad"}, {CommunityName: "bad/name"}, {Listing: "Bad"}, {Sort: "Bad"},
	}
	for index, input := range invalidFeeds {
		if _, err := client.ListFeed(context.Background(), input); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Fatalf("invalid feed %d error=%v", index, err)
		}
	}
	query, err := feedQuery(FeedRequest{CommunityName: "go", SavedOnly: true, LikedOnly: true, DislikedOnly: true, ShowHidden: true, ShowRead: true})
	if err != nil || query.Get("type_") != "All" || query.Get("sort") != "Hot" || query.Get("community_name") != "go" ||
		query.Get("saved_only") != "true" || query.Get("liked_only") != "true" || query.Get("disliked_only") != "true" ||
		query.Get("show_hidden") != "true" || query.Get("show_read") != "true" {
		t.Fatalf("default feed query=%v err=%v", query, err)
	}
	if id, err := optionalID("", "test"); err != nil || id != nil {
		t.Fatalf("empty optional ID=%v err=%v", id, err)
	}
}

func TestReactionsVotesAndComments(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.Method + " " + request.URL.Path {
		case "GET /api/v3/person":
			writeJSON(writer, http.StatusOK, personResponseFixture(7, "alice", nil))
		case "POST /api/v3/post/like":
			var payload struct {
				PostID int64 `json:"post_id"`
				Score  int   `json:"score"`
			}
			if err := json.NewDecoder(request.Body).Decode(&payload); err != nil || payload.PostID != 41 || !validVote(payload.Score) {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			writeJSON(writer, http.StatusOK, `{"post_view":`+postViewFixture(41, 7, 5, "Voted post")+`}`)
		case "POST /api/v3/comment/like":
			var payload struct {
				CommentID int64 `json:"comment_id"`
				Score     int   `json:"score"`
			}
			if err := json.NewDecoder(request.Body).Decode(&payload); err != nil || payload.CommentID != 31 || !validVote(payload.Score) {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			writeJSON(writer, http.StatusOK, `{"comment_view":`+commentViewFixture(31, 41, 8, "0.30.31", "Comment")+`}`)
		case "POST /api/v3/comment":
			var payload struct {
				Content  string `json:"content"`
				PostID   int64  `json:"post_id"`
				ParentID int64  `json:"parent_id"`
			}
			if err := json.NewDecoder(request.Body).Decode(&payload); err != nil || payload.Content != "A reply" || payload.PostID != 41 || payload.ParentID != 30 {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			writeJSON(writer, http.StatusCreated, `{"comment_view":`+commentViewFixture(31, 41, 7, "0.30.31", payload.Content)+`}`)
		case "POST /api/v3/comment/delete":
			fixture := strings.Replace(commentViewFixture(31, 41, 7, "0.30.31", "A reply"), `"published":`, `"deleted":true,"published":`, 1)
			writeJSON(writer, http.StatusOK, `{"comment_view":`+fixture+`}`)
		default:
			writer.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	_, client := newTestClient(t, server)

	if err := client.React(context.Background(), socialhub.ReactionRequest{ActorID: "7", TargetID: "41", Kind: socialhub.ReactionLike}); err != nil {
		t.Fatal(err)
	}
	if err := client.RemoveReaction(context.Background(), socialhub.ReactionRequest{TargetID: "41", Kind: socialhub.ReactionLike}); err != nil {
		t.Fatal(err)
	}
	if err := client.VotePost(context.Background(), "41", -1); err != nil {
		t.Fatal(err)
	}
	if err := client.VoteComment(context.Background(), "31", 1); err != nil {
		t.Fatal(err)
	}
	parent := "30"
	comment, err := client.Comment(context.Background(), socialhub.CreateCommentRequest{PostID: "41", ParentID: &parent, Text: "A reply"})
	if err != nil || comment.ID != "31" || comment.ParentID == nil || *comment.ParentID != "30" || comment.Text != "A reply" {
		t.Fatalf("comment=%#v err=%v", comment, err)
	}
	if err := client.DeleteComment(context.Background(), "31"); err != nil {
		t.Fatal(err)
	}
}

func TestReactionAndCommentValidation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/api/v3/person" {
			writeJSON(writer, http.StatusOK, personResponseFixture(7, "alice", nil))
			return
		}
		writer.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()
	_, client := newTestClient(t, server)
	invalid := []func() error{
		func() error {
			return client.React(context.Background(), socialhub.ReactionRequest{TargetID: "1", Kind: socialhub.ReactionRepost})
		},
		func() error {
			return client.RemoveReaction(context.Background(), socialhub.ReactionRequest{TargetID: "1", Kind: socialhub.ReactionRepost})
		},
		func() error {
			return client.React(context.Background(), socialhub.ReactionRequest{ActorID: "bad", TargetID: "1", Kind: socialhub.ReactionLike})
		},
		func() error {
			return client.React(context.Background(), socialhub.ReactionRequest{ActorID: "8", TargetID: "1", Kind: socialhub.ReactionLike})
		},
		func() error { return client.VotePost(context.Background(), "bad", 1) },
		func() error { return client.VotePost(context.Background(), "1", 2) },
		func() error { return client.VoteComment(context.Background(), "bad", 0) },
		func() error {
			_, err := client.Comment(context.Background(), socialhub.CreateCommentRequest{})
			return err
		},
		func() error {
			bad := "0"
			_, err := client.Comment(context.Background(), socialhub.CreateCommentRequest{PostID: "1", ParentID: &bad, Text: "text"})
			return err
		},
		func() error { return client.DeleteComment(context.Background(), "bad") },
	}
	for index, call := range invalid {
		err := call()
		if !errors.Is(err, socialhub.ErrInvalidArgument) && !errors.Is(err, socialhub.ErrUnsupported) {
			t.Fatalf("invalid reaction %d error=%v", index, err)
		}
	}
}

func TestMutationResponseValidation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.Method + " " + request.URL.Path {
		case "POST /api/v3/post":
			writeJSON(writer, http.StatusOK, `{"post_view":{}}`)
		case "PUT /api/v3/post", "POST /api/v3/post/like":
			writeJSON(writer, http.StatusOK, `{"post_view":`+postViewFixture(99, 7, 5, "Wrong")+`}`)
		case "POST /api/v3/post/delete":
			writeJSON(writer, http.StatusOK, `{"post_view":`+postViewFixture(41, 7, 5, "Not deleted")+`}`)
		case "POST /api/v3/comment", "POST /api/v3/comment/like", "POST /api/v3/comment/delete":
			writeJSON(writer, http.StatusOK, `{"comment_view":`+commentViewFixture(99, 99, 8, "0.99", "Wrong")+`}`)
		case "GET /api/v3/post/list":
			writeJSON(writer, http.StatusOK, `{"posts":[],"next_page":"bad\ncursor"}`)
		default:
			writer.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	_, client := newTestClient(t, server)
	title := "Updated"
	badCalls := []func() error{
		func() error {
			_, err := client.CreatePost(context.Background(), CreatePostRequest{Title: "Valid", CommunityID: "5"})
			return err
		},
		func() error {
			_, err := client.UpdatePost(context.Background(), "41", UpdatePostRequest{Title: &title})
			return err
		},
		func() error { return client.DeletePost(context.Background(), "41") },
		func() error { return client.VotePost(context.Background(), "41", 1) },
		func() error { return client.VoteComment(context.Background(), "31", 1) },
		func() error {
			_, err := client.Comment(context.Background(), socialhub.CreateCommentRequest{PostID: "41", Text: "text"})
			return err
		},
		func() error { return client.DeleteComment(context.Background(), "31") },
		func() error { _, err := client.ListFeed(context.Background(), FeedRequest{}); return err },
	}
	for index, call := range badCalls {
		var platformErr *socialhub.Error
		if err := call(); !errors.As(err, &platformErr) || platformErr.Code != socialhub.CodePlatformError {
			t.Fatalf("bad mutation %d error=%v", index, err)
		}
	}
}
