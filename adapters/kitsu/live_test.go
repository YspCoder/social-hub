package kitsu

import (
	"context"
	"os"
	"testing"

	"social-hub/pkg/socialhub"
)

// TestLivePublicReads is opt-in because it depends on Kitsu's unversioned edge
// service. It uses no credentials and never mutates remote state.
func TestLivePublicReads(t *testing.T) {
	if os.Getenv("KITSU_LIVE_TEST") != "1" {
		t.Skip("set KITSU_LIVE_TEST=1 to exercise public edge reads")
	}
	adapter := &Adapter{}
	if err := adapter.Init(context.Background(), socialhub.AdapterConfig{
		Adapter: adapterName, Accounts: []socialhub.AccountConfig{{ID: "public"}},
	}); err != nil {
		t.Fatal(err)
	}
	common, err := adapter.Client(context.Background(), "public")
	if err != nil {
		t.Fatal(err)
	}
	client := common.(*Client)
	ctx := context.Background()
	if page, err := client.SearchAnime(ctx, SearchRequest{Query: "Cowboy Bebop", Limit: 1}); err != nil || len(page.Items) == 0 {
		t.Fatalf("anime smoke failed: %v", err)
	}
	if page, err := client.SearchManga(ctx, SearchRequest{Query: "Berserk", Limit: 1}); err != nil || len(page.Items) == 0 {
		t.Fatalf("manga smoke failed: %v", err)
	}
	if _, err := client.FindUserBySlug(ctx, "Josh"); err != nil {
		t.Fatalf("user smoke failed: %v", err)
	}
	if page, err := client.ListLibraryEntries(ctx, LibraryEntriesRequest{UserID: "1", Limit: 1}); err != nil || len(page.Items) == 0 {
		t.Fatalf("library smoke failed: %v", err)
	}
	posts, err := client.ListPosts(ctx, PostsRequest{Limit: 1})
	if err != nil || len(posts.Items) == 0 {
		t.Fatalf("post smoke failed: %v", err)
	}
	if _, err := client.ListComments(ctx, CommentsRequest{PostID: posts.Items[0].ID, Limit: 1}); err != nil {
		t.Fatalf("comment smoke failed: %v", err)
	}
}
