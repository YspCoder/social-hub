package listenbrainz

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

// TestLivePublicReads is opt-in because it calls ListenBrainz's shared public
// service. It uses no credentials and never mutates remote state.
func TestLivePublicReads(t *testing.T) {
	if os.Getenv("LISTENBRAINZ_LIVE_TEST") != "1" {
		t.Skip("set LISTENBRAINZ_LIVE_TEST=1 to exercise public API v1 reads")
	}
	adapter := &Adapter{}
	if err := adapter.Init(context.Background(), socialhub.AdapterConfig{
		Adapter:  adapterName,
		Accounts: []socialhub.AccountConfig{{ID: "public", Settings: map[string]any{"username": "rob"}}},
	}); err != nil {
		t.Fatal(err)
	}
	common, err := adapter.Client(context.Background(), "public")
	if err != nil {
		t.Fatal(err)
	}
	client := common.(*Client)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	users, err := liveCall(ctx, func() ([]User, error) { return client.SearchUsers(ctx, "rob") })
	if err != nil || len(users) == 0 {
		t.Fatalf("user search failed: %#v, %v", users, err)
	}
	listens, err := liveCall(ctx, func() (*ListenPage, error) {
		return client.ListListens(ctx, ListensRequest{Count: 1})
	})
	if err != nil || listens.Count != 1 || len(listens.Listens) != 1 {
		t.Fatalf("listens failed: %#v, %v", listens, err)
	}
	if count, err := liveCall(ctx, func() (int64, error) { return client.GetListenCount(ctx, "") }); err != nil || count <= 0 {
		t.Fatalf("listen count failed: %d, %v", count, err)
	}
	if _, err := liveCall(ctx, func() (*Listen, error) { return client.GetPlayingNow(ctx, "") }); err != nil {
		t.Fatalf("playing now failed: %v", err)
	}
	feedback, err := liveCall(ctx, func() (socialhub.Page[Feedback], error) {
		return client.ListFeedback(ctx, FeedbackListRequest{MaxResults: 1, Metadata: true})
	})
	if err != nil || len(feedback.Items) > 1 {
		t.Fatalf("feedback failed: %#v, %v", feedback, err)
	}
	playlists, err := liveCall(ctx, func() (socialhub.Page[Playlist], error) {
		return client.ListUserPlaylists(ctx, "", PlaylistPageRequest{MaxResults: 1})
	})
	if err != nil || len(playlists.Items) != 1 {
		t.Fatalf("user playlists failed: %#v, %v", playlists, err)
	}
	playlist, err := liveCall(ctx, func() (*Playlist, error) {
		return client.GetPlaylist(ctx, playlistMBID, false)
	})
	if err != nil || playlist.Title == "" || len(playlist.Track) == 0 {
		t.Fatalf("playlist failed: %#v, %v", playlist, err)
	}
}

func liveCall[T any](ctx context.Context, call func() (T, error)) (T, error) {
	for attempt := 0; ; attempt++ {
		result, err := call()
		if !errors.Is(err, socialhub.ErrRateLimited) || attempt == 2 {
			return result, err
		}
		delay := time.Duration(1<<attempt) * 2 * time.Second
		var platformErr *socialhub.Error
		if errors.As(err, &platformErr) && platformErr.RetryAfter > delay {
			delay = platformErr.RetryAfter
		}
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			var zero T
			return zero, ctx.Err()
		case <-timer.C:
		}
	}
}
