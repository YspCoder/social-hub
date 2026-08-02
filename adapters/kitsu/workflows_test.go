package kitsu

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

const userResource = `{"type":"users","id":"42","attributes":{"name":"Ada","slug":"ada"}}`

func jsonAPIHandler(t *testing.T) http.HandlerFunc {
	t.Helper()
	return func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Accept") != jsonAPIContentType {
			t.Errorf("unexpected Accept: %q", request.Header.Get("Accept"))
		}
		if request.Header.Get("User-Agent") != "social-hub-kitsu-tests/1.0" {
			t.Errorf("unexpected User-Agent: %q", request.Header.Get("User-Agent"))
		}
		if request.Method != http.MethodGet && request.Method != http.MethodDelete {
			if request.Header.Get("Content-Type") != jsonAPIContentType {
				t.Errorf("unexpected Content-Type: %q", request.Header.Get("Content-Type"))
			}
			if request.Header.Get("Authorization") != "Bearer "+testAccessToken {
				t.Errorf("unexpected Authorization header")
			}
		}
		writer.Header().Set("Content-Type", jsonAPIContentType)
		switch request.Method + " " + request.URL.Path {
		case "GET /api/edge/anime":
			if request.URL.Query().Get("filter[text]") != "Bebop" || request.URL.Query().Get("page[limit]") != "1" {
				t.Errorf("unexpected anime query: %s", request.URL.RawQuery)
			}
			_, _ = writer.Write([]byte(`{"data":[{"type":"anime","id":"1","attributes":{"canonicalTitle":"Cowboy Bebop","episodeCount":26}}],"links":{"next":"https://other.invalid/api/edge/anime?page%5Boffset%5D=1"}}`))
		case "GET /api/edge/anime/1":
			_, _ = writer.Write([]byte(`{"data":{"type":"anime","id":"1","attributes":{"canonicalTitle":"Cowboy Bebop","episodeCount":26}}}`))
		case "GET /api/edge/manga":
			_, _ = writer.Write([]byte(`{"data":[{"type":"manga","id":"8","attributes":{"canonicalTitle":"Berserk","chapterCount":380}}],"links":{}}`))
		case "GET /api/edge/manga/8":
			_, _ = writer.Write([]byte(`{"data":{"type":"manga","id":"8","attributes":{"canonicalTitle":"Berserk","chapterCount":380}}}`))
		case "GET /api/edge/users":
			if request.URL.Query().Get("filter[slug]") != "ada" && request.URL.Query().Get("filter[self]") != "true" {
				t.Errorf("unexpected user filter: %s", request.URL.RawQuery)
			}
			_, _ = writer.Write([]byte(`{"data":[` + userResource + `],"links":{}}`))
		case "GET /api/edge/users/42":
			_, _ = writer.Write([]byte(`{"data":` + userResource + `}`))
		case "GET /api/edge/library-entries":
			if request.URL.Query().Get("filter[userId]") != "42" || request.URL.Query().Get("include") != "anime,manga" {
				t.Errorf("unexpected library query: %s", request.URL.RawQuery)
			}
			_, _ = writer.Write([]byte(`{"data":[{"type":"libraryEntries","id":"90","attributes":{"status":"current","progress":3,"private":false,"reactionSkipped":"unskipped"},"relationships":{"user":{"data":{"type":"users","id":"42"}},"media":{"data":{"type":"anime","id":"1"}}}}],"included":[{"type":"anime","id":"1","attributes":{"canonicalTitle":"Cowboy Bebop"}}],"links":{}}`))
		case "GET /api/edge/library-entries/90":
			_, _ = writer.Write([]byte(`{"data":{"type":"libraryEntries","id":"90","attributes":{"status":"current","progress":3},"relationships":{"user":{"data":{"type":"users","id":"42"}},"media":{"data":{"type":"anime","id":"1"}}}},"included":[{"type":"anime","id":"1","attributes":{"canonicalTitle":"Cowboy Bebop"}}]}`))
		case "GET /api/edge/posts":
			if request.URL.Query().Get("sort") != "-createdAt" {
				t.Errorf("unexpected post sort")
			}
			_, _ = writer.Write([]byte(`{"data":[{"type":"posts","id":"100","attributes":{"content":"hello","commentsCount":1},"relationships":{"user":{"data":{"type":"users","id":"42"}}}}],"included":[` + userResource + `],"links":{}}`))
		case "GET /api/edge/posts/100":
			_, _ = writer.Write([]byte(`{"data":{"type":"posts","id":"100","attributes":{"content":"hello"},"relationships":{"user":{"data":{"type":"users","id":"42"}}}},"included":[` + userResource + `]}`))
		case "GET /api/edge/comments":
			if request.URL.Query().Get("filter[postId]") != "100" {
				t.Errorf("unexpected comment filter")
			}
			_, _ = writer.Write([]byte(`{"data":[{"type":"comments","id":"200","attributes":{"content":"reply","likesCount":2},"relationships":{"post":{"data":{"type":"posts","id":"100"}},"user":{"data":{"type":"users","id":"42"}},"parent":{"data":null}}}],"included":[` + userResource + `],"links":{}}`))
		case "GET /api/edge/comments/200":
			_, _ = writer.Write([]byte(`{"data":{"type":"comments","id":"200","attributes":{"content":"reply"},"relationships":{"post":{"data":{"type":"posts","id":"100"}},"user":{"data":{"type":"users","id":"42"}}}},"included":[` + userResource + `]}`))
		case "POST /api/edge/library-entries":
			var body map[string]any
			if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
				t.Error(err)
			}
			data := body["data"].(map[string]any)
			attrs := data["attributes"].(map[string]any)
			if attrs["progress"] != float64(0) || attrs["private"] != false || attrs["notes"] != "" {
				t.Errorf("explicit zero values were lost: %#v", attrs)
			}
			_, _ = writer.Write([]byte(`{"data":{"type":"libraryEntries","id":"91","attributes":{"status":"planned","progress":0,"notes":"","private":false},"relationships":{"user":{"data":{"type":"users","id":"42"}},"anime":{"data":{"type":"anime","id":"1"}}}}}`))
		case "PATCH /api/edge/library-entries/90":
			_, _ = writer.Write([]byte(`{"data":{"type":"libraryEntries","id":"90","attributes":{"status":"completed","progress":26},"relationships":{"user":{"data":{"type":"users","id":"42"}},"anime":{"data":{"type":"anime","id":"1"}}}}}`))
		case "DELETE /api/edge/library-entries/90":
			writer.WriteHeader(http.StatusNoContent)
		case "POST /api/edge/posts":
			_, _ = writer.Write([]byte(`{"data":{"type":"posts","id":"101","attributes":{"content":"new post"},"relationships":{"user":{"data":{"type":"users","id":"42"}}}}}`))
		case "PATCH /api/edge/posts/100":
			_, _ = writer.Write([]byte(`{"data":{"type":"posts","id":"100","attributes":{"content":"edited","spoiler":false,"nsfw":false},"relationships":{"user":{"data":{"type":"users","id":"42"}}}}}`))
		case "DELETE /api/edge/posts/100":
			writer.WriteHeader(http.StatusNoContent)
		case "POST /api/edge/comments":
			_, _ = writer.Write([]byte(`{"data":{"type":"comments","id":"201","attributes":{"content":"nested"},"relationships":{"post":{"data":{"type":"posts","id":"100"}},"user":{"data":{"type":"users","id":"42"}},"parent":{"data":{"type":"comments","id":"200"}}}}}`))
		case "PATCH /api/edge/comments/200":
			_, _ = writer.Write([]byte(`{"data":{"type":"comments","id":"200","attributes":{"content":"updated"},"relationships":{"post":{"data":{"type":"posts","id":"100"}},"user":{"data":{"type":"users","id":"42"}}}}}`))
		case "DELETE /api/edge/comments/201":
			writer.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(writer, request)
		}
	}
}

func TestReadWorkflows(t *testing.T) {
	server := httptest.NewServer(jsonAPIHandler(t))
	defer server.Close()
	_, client := newTestClient(t, server, true, true)
	ctx := context.Background()
	anime, err := client.SearchAnime(ctx, SearchRequest{Query: "Bebop", Limit: 1})
	if err != nil || len(anime.Items) != 1 || anime.Items[0].Kind != MediaAnime || !anime.HasMore || anime.NextCursor == nil || *anime.NextCursor != "1" {
		t.Fatalf("unexpected anime result: %#v, %v", anime, err)
	}
	searchedManga, err := client.SearchManga(ctx, SearchRequest{Query: "Berserk"})
	if err != nil || len(searchedManga.Items) != 1 || searchedManga.Items[0].Kind != MediaManga {
		t.Fatalf("unexpected manga search: %#v, %v", searchedManga, err)
	}
	detail, err := client.GetAnime(ctx, "1")
	if err != nil || detail.EpisodeCount != 26 {
		t.Fatalf("unexpected anime: %#v, %v", detail, err)
	}
	manga, err := client.GetManga(ctx, "8")
	if err != nil || manga.CanonicalTitle != "Berserk" || manga.Kind != MediaManga {
		t.Fatalf("unexpected manga: %#v, %v", manga, err)
	}
	user, err := client.FindUserBySlug(ctx, "ada")
	if err != nil || user.ID != "42" {
		t.Fatalf("unexpected user: %#v, %v", user, err)
	}
	user, err = client.GetUser(ctx, "42")
	if err != nil || user.Name != "Ada" {
		t.Fatalf("unexpected user detail: %#v, %v", user, err)
	}
	current, err := client.GetCurrentUser(ctx)
	if err != nil || current.Slug != "ada" {
		t.Fatalf("unexpected current user: %#v, %v", current, err)
	}
	library, err := client.ListLibraryEntries(ctx, LibraryEntriesRequest{})
	if err != nil || len(library.Items) != 1 || library.Items[0].Media == nil || library.Items[0].Media.Kind != MediaAnime || library.Items[0].ReactionSkipped != "unskipped" {
		t.Fatalf("unexpected library: %#v, %v", library, err)
	}
	entry, err := client.GetLibraryEntry(ctx, "90")
	if err != nil || entry.Media == nil || entry.Media.ID != "1" {
		t.Fatalf("unexpected entry: %#v, %v", entry, err)
	}
	posts, err := client.ListPosts(ctx, PostsRequest{})
	if err != nil || len(posts.Items) != 1 || posts.Items[0].User == nil {
		t.Fatalf("unexpected posts: %#v, %v", posts, err)
	}
	post, err := client.GetPost(ctx, "100")
	if err != nil || post.User == nil || post.Content != "hello" {
		t.Fatalf("unexpected post detail: %#v, %v", post, err)
	}
	comments, err := client.ListComments(ctx, CommentsRequest{PostID: "100"})
	if err != nil || len(comments.Items) != 1 || comments.Items[0].PostID != "100" || comments.Items[0].User == nil {
		t.Fatalf("unexpected comments: %#v, %v", comments, err)
	}
	comment, err := client.GetComment(ctx, "200")
	if err != nil || comment.User == nil || comment.Content != "reply" {
		t.Fatalf("unexpected comment detail: %#v, %v", comment, err)
	}
}

func TestMutationWorkflowsPreservePointers(t *testing.T) {
	server := httptest.NewServer(jsonAPIHandler(t))
	defer server.Close()
	_, client := newTestClient(t, server, true, true)
	ctx := context.Background()
	zero, no, empty := 0, false, ""
	entry, err := client.CreateLibraryEntry(ctx, CreateLibraryEntryRequest{
		MediaID: "1", MediaKind: MediaAnime, Status: LibraryPlanned,
		Progress: &zero, Private: &no, Notes: &empty,
	})
	if err != nil || entry.ID != "91" || entry.Progress != 0 {
		t.Fatalf("unexpected entry: %#v, %v", entry, err)
	}
	completed := LibraryCompleted
	entry, err = client.UpdateLibraryEntry(ctx, UpdateLibraryEntryRequest{ID: "90", Status: &completed, Progress: &zero})
	if err != nil || entry.Status != LibraryCompleted {
		t.Fatalf("unexpected updated entry: %#v, %v", entry, err)
	}
	if err := client.DeleteLibraryEntry(ctx, "90"); err != nil {
		t.Fatal(err)
	}
	created, err := client.CreatePost(ctx, CreatePostRequest{Content: "new post", MediaID: "1", MediaKind: MediaAnime})
	if err != nil || created.ID != "101" {
		t.Fatalf("unexpected created post: %#v, %v", created, err)
	}
	content := "edited"
	post, err := client.UpdatePost(ctx, UpdatePostRequest{ID: "100", Content: &content, Spoiler: &no})
	if err != nil || post.Content != content {
		t.Fatalf("unexpected post: %#v, %v", post, err)
	}
	if err := client.DeletePost(ctx, "100"); err != nil {
		t.Fatal(err)
	}
	comment, err := client.CreateComment(ctx, CreateCommentRequest{PostID: "100", ParentID: "200", Content: "nested"})
	if err != nil || comment.ParentID != "200" {
		t.Fatalf("unexpected comment: %#v, %v", comment, err)
	}
	comment, err = client.UpdateComment(ctx, UpdateCommentRequest{ID: "200", Content: "updated"})
	if err != nil || comment.Content != "updated" {
		t.Fatalf("unexpected updated comment: %#v, %v", comment, err)
	}
	if err := client.DeleteComment(ctx, "201"); err != nil {
		t.Fatal(err)
	}
}

func TestWorkflowValidation(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	_, client := newTestClient(t, server, true, true)
	ctx := context.Background()
	tests := []func() error{
		func() error { _, err := client.SearchAnime(ctx, SearchRequest{Query: ""}); return err },
		func() error { _, err := client.GetAnime(ctx, "01"); return err },
		func() error { _, err := client.ListPosts(ctx, PostsRequest{Limit: 21}); return err },
		func() error {
			_, err := client.ListLibraryEntries(ctx, LibraryEntriesRequest{UserID: "bad"})
			return err
		},
		func() error {
			_, err := client.CreatePost(ctx, CreatePostRequest{Content: strings.Repeat("x", maxTextLength+1)})
			return err
		},
		func() error { _, err := client.UpdatePost(ctx, UpdatePostRequest{ID: "1"}); return err },
		func() error { _, err := client.CreateComment(ctx, CreateCommentRequest{PostID: "1"}); return err },
		func() error { _, err := client.UpdateLibraryEntry(ctx, UpdateLibraryEntryRequest{ID: "1"}); return err },
		func() error { _, err := client.ListPosts(ctx, PostsRequest{}, socialhub.WithFields("id")); return err },
	}
	for index, test := range tests {
		if err := test(); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Fatalf("case %d: expected invalid argument, got %v", index, err)
		}
	}
}
