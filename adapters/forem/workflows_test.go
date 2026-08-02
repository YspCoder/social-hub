package forem

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

func TestFetchArticlesCommentsAndReactions(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("api-key") != "api-key" || request.Header.Get("Accept") != foremAccept || request.Header.Get("User-Agent") != userAgent {
			writer.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch request.Method + " " + request.URL.Path {
		case "GET /api/users/me", "GET /api/users/alice":
			writeJSON(writer, http.StatusOK, userFixture())
		case "GET /api/articles/100":
			writeJSON(writer, http.StatusOK, articleFixture(100, true, "Original"))
		case "GET /api/articles/me":
			if request.URL.Query().Get("page") != "2" || request.URL.Query().Get("per_page") != "2" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			writeJSON(writer, http.StatusOK, `[`+articleFixture(100, true, "First")+`,`+articleFixture(99, true, "Second")+`]`)
		case "GET /api/articles":
			if request.URL.Query().Get("username") != "alice" || request.URL.Query().Get("page") != "1" || request.URL.Query().Get("per_page") != "30" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			writeJSON(writer, http.StatusOK, `[`+articleFixture(100, true, "First")+`]`)
		case "GET /api/comments":
			if request.URL.Query().Get("a_id") != "100" || request.URL.Query().Get("page") != "2" || request.URL.Query().Get("per_page") != "1" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			writeJSON(writer, http.StatusOK, `[{"type_of":"comment","id_code":"c1","created_at":"2026-08-02T01:00:00Z","body_html":"<p>Top</p>","user":{"user_id":7,"username":"alice","name":"Alice"},"children":[{"type_of":"comment","id_code":"c2","created_at":"2026-08-02T02:00:00Z","body_html":"<p>Child</p>","user":{"user_id":8,"username":"bob","name":"Bob"},"children":[]}]}]`)
		case "POST /api/articles":
			var input articleEnvelope
			if err := json.NewDecoder(request.Body).Decode(&input); err != nil || input.Article.Title == nil || *input.Article.Title != "New Article" || input.Article.Tags == nil || *input.Article.Tags != "go,sdk" || input.Article.Published == nil || !*input.Article.Published {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			writeJSON(writer, http.StatusCreated, articleFixture(101, true, "New Article"))
		case "PUT /api/articles/100":
			var input articleEnvelope
			if err := json.NewDecoder(request.Body).Decode(&input); err != nil || input.Article.Title == nil || *input.Article.Title != "Updated" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			writeJSON(writer, http.StatusOK, articleFixture(100, true, "Updated"))
		case "PUT /api/articles/100/unpublish":
			if request.URL.Query().Get("note") != "Needs work" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			writer.WriteHeader(http.StatusNoContent)
		case "GET /api/articles/me/all", "GET /api/articles/me/published", "GET /api/articles/me/unpublished":
			if request.URL.Query().Get("page") != "1" || request.URL.Query().Get("per_page") != "1" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			published := request.URL.Path != "/api/articles/me/unpublished"
			writeJSON(writer, http.StatusOK, `[`+articleFixture(100, published, "State")+`]`)
		case "POST /api/reactions", "POST /api/reactions/toggle":
			query := request.URL.Query()
			if query.Get("reactable_id") == "100" && query.Get("reactable_type") == "Article" && query.Get("category") == "like" ||
				query.Get("reactable_id") == "10" && query.Get("reactable_type") == "Comment" && query.Get("category") == "fire" {
				writer.WriteHeader(http.StatusOK)
				return
			}
			writer.WriteHeader(http.StatusBadRequest)
		default:
			writer.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	_, client := newTestClient(t, server)

	user, err := client.GetUser(context.Background(), "")
	if err != nil || user.ID != "7" || user.Username == nil || *user.Username != "alice" || user.DisplayName == nil || *user.DisplayName != "Alice Author" || user.AvatarURL == nil || *user.AvatarURL != "https://cdn.example/alice.png" || user.ProfileURL == nil || *user.ProfileURL != server.URL+"/alice" || len(user.Extensions["forem.user"]) == 0 {
		t.Fatalf("user=%#v err=%v", user, err)
	}
	if _, err := client.GetUser(context.Background(), "alice"); err != nil {
		t.Fatal(err)
	}
	post, err := client.GetPost(context.Background(), "100")
	if err != nil || post.ID != "100" || post.AuthorID == nil || *post.AuthorID != "7" || post.Text == nil || *post.Text != "# Original" || post.Visibility == nil || *post.Visibility != "public" || post.Status.State != socialhub.PublishStatePublished || len(post.Media) != 1 || post.Metrics[0].Value != 3 || post.Metrics[1].Value != 8 || len(post.Extensions["forem.article"]) == 0 {
		t.Fatalf("post=%#v err=%v", post, err)
	}
	posts, err := client.ListPosts(context.Background(), socialhub.ListPostsRequest{Cursor: "2", MaxResults: 2})
	if err != nil || len(posts.Items) != 2 || posts.NextCursor == nil || *posts.NextCursor != "3" || posts.PrevCursor == nil || *posts.PrevCursor != "1" || !posts.HasMore {
		t.Fatalf("posts=%#v err=%v", posts, err)
	}
	byUser, err := client.ListPosts(context.Background(), socialhub.ListPostsRequest{UserID: "alice"})
	if err != nil || len(byUser.Items) != 1 || byUser.Items[0].ID != "100" || byUser.HasMore {
		t.Fatalf("by user=%#v err=%v", byUser, err)
	}
	comments, err := client.ListComments(context.Background(), socialhub.ListCommentsRequest{PostID: "100", Cursor: "2", MaxResults: 1})
	if err != nil || len(comments.Items) != 2 || comments.Items[0].ID != "c1" || comments.Items[0].ParentID != nil || comments.Items[1].ParentID == nil || *comments.Items[1].ParentID != "c1" || comments.Items[1].AuthorID == nil || *comments.Items[1].AuthorID != "8" || comments.NextCursor == nil || *comments.NextCursor != "3" {
		t.Fatalf("comments=%#v err=%v", comments, err)
	}

	created, err := client.CreateArticle(context.Background(), CreateArticleRequest{
		Title: "New Article", BodyMarkdown: "# New", Published: true, Tags: []string{"go", "sdk"},
		MainImageURL: "https://cdn.example/new.png", OrganizationID: "9",
	})
	if err != nil || created.Post.ID != "101" || created.Title != "New Article" || len(created.Tags) != 2 || created.CanonicalURL != "https://example.com/original" || !created.Published || len(created.Raw) == 0 {
		t.Fatalf("created=%#v err=%v", created, err)
	}
	typed, err := client.GetArticle(context.Background(), "100")
	if err != nil || typed.Title != "Original" || typed.BodyHTML != "<h1>Original</h1>" || typed.ReadingTimeMinutes != 4 || typed.PageViewsCount != 42 {
		t.Fatalf("typed=%#v err=%v", typed, err)
	}
	updatedTitle := "Updated"
	updated, err := client.UpdateArticle(context.Background(), "100", UpdateArticleRequest{Title: &updatedTitle})
	if err != nil || updated.Title != "Updated" || updated.Post.ID != "100" {
		t.Fatalf("updated=%#v err=%v", updated, err)
	}
	if err := client.UnpublishArticle(context.Background(), "100", "Needs work"); err != nil {
		t.Fatal(err)
	}
	for _, state := range []ArticleState{ArticleStateAll, ArticleStatePublished, ArticleStateUnpublished} {
		page, err := client.ListMyArticles(context.Background(), state, "", 1)
		if err != nil || len(page.Items) != 1 || page.Items[0].Post.ID != "100" || page.NextCursor == nil {
			t.Fatalf("state %s page=%#v err=%v", state, page, err)
		}
	}

	if err := client.React(context.Background(), socialhub.ReactionRequest{ActorID: "7", TargetID: "100", Kind: socialhub.ReactionLike}); err != nil {
		t.Fatal(err)
	}
	if err := client.CreateForemReaction(context.Background(), ForemReactionRequest{Category: ReactionFire, TargetID: "10", Type: ReactableComment}); err != nil {
		t.Fatal(err)
	}
	if err := client.ToggleForemReaction(context.Background(), ForemReactionRequest{Category: ReactionFire, TargetID: "10", Type: ReactableComment}); err != nil {
		t.Fatal(err)
	}
	if err := client.RemoveReaction(context.Background(), socialhub.ReactionRequest{}); !errors.Is(err, socialhub.ErrUnsupported) {
		t.Fatalf("remove=%v", err)
	}
	if _, err := client.Comment(context.Background(), socialhub.CreateCommentRequest{}); !errors.Is(err, socialhub.ErrUnsupported) {
		t.Fatalf("comment=%v", err)
	}
	if err := client.DeleteComment(context.Background(), "c1"); !errors.Is(err, socialhub.ErrUnsupported) {
		t.Fatalf("delete comment=%v", err)
	}
}

func TestWorkflowValidation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/api/users/me" {
			writeJSON(writer, http.StatusOK, userFixture())
			return
		}
		writer.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()
	_, client := newTestClient(t, server)
	now := time.Now()
	invalidCalls := []func() error{
		func() error { _, err := client.GetUser(context.Background(), "bad/user"); return err },
		func() error { _, err := client.GetPost(context.Background(), "bad"); return err },
		func() error {
			_, err := client.ListPosts(context.Background(), socialhub.ListPostsRequest{StartTime: &now})
			return err
		},
		func() error {
			_, err := client.ListPosts(context.Background(), socialhub.ListPostsRequest{UserID: "bad/user"})
			return err
		},
		func() error {
			_, err := client.ListComments(context.Background(), socialhub.ListCommentsRequest{PostID: "bad"})
			return err
		},
		func() error { _, err := client.CreateArticle(context.Background(), CreateArticleRequest{}); return err },
		func() error {
			_, err := client.CreateArticle(context.Background(), CreateArticleRequest{Title: "x", BodyMarkdown: "x", Tags: []string{"a", "b", "c", "d", "e"}})
			return err
		},
		func() error {
			_, err := client.CreateArticle(context.Background(), CreateArticleRequest{Title: "x", BodyMarkdown: "x", Tags: []string{"a", "a"}})
			return err
		},
		func() error {
			_, err := client.CreateArticle(context.Background(), CreateArticleRequest{Title: "x", BodyMarkdown: "x", MainImageURL: "not-url"})
			return err
		},
		func() error {
			_, err := client.CreateArticle(context.Background(), CreateArticleRequest{Title: "x", BodyMarkdown: "x", OrganizationID: "bad"})
			return err
		},
		func() error {
			_, err := client.UpdateArticle(context.Background(), "bad", UpdateArticleRequest{})
			return err
		},
		func() error {
			_, err := client.UpdateArticle(context.Background(), "1", UpdateArticleRequest{})
			return err
		},
		func() error {
			blank := " "
			_, err := client.UpdateArticle(context.Background(), "1", UpdateArticleRequest{Title: &blank})
			return err
		},
		func() error {
			bad := "not-url"
			_, err := client.UpdateArticle(context.Background(), "1", UpdateArticleRequest{CanonicalURL: &bad})
			return err
		},
		func() error {
			bad := "0"
			_, err := client.UpdateArticle(context.Background(), "1", UpdateArticleRequest{OrganizationID: &bad})
			return err
		},
		func() error { return client.UnpublishArticle(context.Background(), "bad", "") },
		func() error { return client.UnpublishArticle(context.Background(), "1", string(make([]byte, 1025))) },
		func() error { _, err := client.ListMyArticles(context.Background(), "other", "", 1); return err },
		func() error {
			return client.React(context.Background(), socialhub.ReactionRequest{TargetID: "1", Kind: socialhub.ReactionRepost})
		},
		func() error {
			return client.React(context.Background(), socialhub.ReactionRequest{ActorID: "8", TargetID: "1", Kind: socialhub.ReactionLike})
		},
		func() error {
			return client.CreateForemReaction(context.Background(), ForemReactionRequest{Category: "bad", TargetID: "1", Type: ReactableArticle})
		},
		func() error {
			return client.ToggleForemReaction(context.Background(), ForemReactionRequest{Category: ReactionLike, TargetID: "bad", Type: ReactableArticle})
		},
	}
	for index, call := range invalidCalls {
		err := call()
		if !errors.Is(err, socialhub.ErrInvalidArgument) && !errors.Is(err, socialhub.ErrUnsupported) {
			t.Fatalf("invalid call %d error=%v", index, err)
		}
	}
}

func userFixture() string {
	return `{"type_of":"user","id":7,"username":"alice","name":"Alice Author","summary":"Writes Go","website_url":"https://alice.example","joined_at":"Jan 1, 2020","profile_image":"https://cdn.example/alice.png","followers_count":10}`
}

func articleFixture(id int64, published bool, title string) string {
	publishedAt := `"2026-08-01T10:00:00Z"`
	if !published {
		publishedAt = "null"
	}
	return `{"type_of":"article","id":` + strconv.FormatInt(id, 10) + `,"title":` + strconv.Quote(title) + `,"description":"An article","cover_image":"https://cdn.example/cover.png","social_image":"https://cdn.example/social.png","tag_list":["go","sdk"],"tags":"go,sdk","slug":"article","path":"/alice/article","url":"https://dev.to/alice/article","canonical_url":"https://example.com/original","comments_count":3,"positive_reactions_count":8,"public_reactions_count":9,"page_views_count":42,"created_at":"2026-07-31T10:00:00Z","edited_at":"2026-08-02T10:00:00Z","published_at":` + publishedAt + `,"published":` + strconv.FormatBool(published) + `,"body_markdown":` + strconv.Quote("# "+title) + `,"body_html":` + strconv.Quote("<h1>"+title+"</h1>") + `,"reading_time_minutes":4,"user":{"user_id":7,"username":"alice","name":"Alice Author","profile_image":"https://cdn.example/alice.png"}}`
}
