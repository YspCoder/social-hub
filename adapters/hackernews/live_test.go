package hackernews

import (
	"context"
	"os"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

func TestPublicLiveSmoke(t *testing.T) {
	if os.Getenv("HACKERNEWS_LIVE_TEST") != "1" {
		t.Skip("set HACKERNEWS_LIVE_TEST=1 to exercise public Hacker News API v0 reads")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	adapter, err := socialhub.Open(ctx, adapterName, socialhub.AdapterConfig{
		Adapter: adapterName, Accounts: []socialhub.AccountConfig{{ID: "public"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer adapter.Close()
	common, err := adapter.Client(ctx, "public")
	if err != nil {
		t.Fatal(err)
	}
	client := common.(*Client)
	maxID, err := client.MaxItemID(ctx)
	if err != nil || maxID <= 0 {
		t.Fatalf("max ID=%d err=%v", maxID, err)
	}
	feed, err := client.ListFeed(ctx, FeedRequest{Feed: FeedTop, MaxResults: 1})
	if err != nil || len(feed.Items) != 1 || feed.Items[0].ID <= 0 {
		t.Fatalf("feed=%#v err=%v", feed, err)
	}
}
