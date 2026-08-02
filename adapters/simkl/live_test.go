package simkl

import (
	"context"
	"os"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

// TestLivePublicTrending is opt-in because it depends on Simkl's public CDN.
// It uses no credentials and never mutates remote state.
func TestLivePublicTrending(t *testing.T) {
	if os.Getenv("SIMKL_LIVE_TEST") != "1" {
		t.Skip("set SIMKL_LIVE_TEST=1 to exercise public Simkl CDN reads")
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
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	for _, mediaType := range []MediaType{MediaMovie, MediaTV, MediaAnime} {
		items, err := client.ListTrending(ctx, TrendingRequest{Type: mediaType, Period: TrendingToday, Limit: 100})
		if err != nil || len(items) == 0 || items[0].IDs.Simkl <= 0 || items[0].Title == "" {
			t.Fatalf("%s trending smoke failed: first=%#v err=%v", mediaType, firstTrending(items), err)
		}
	}
}

func firstTrending(items []TrendingItem) *TrendingItem {
	if len(items) == 0 {
		return nil
	}
	return &items[0]
}
