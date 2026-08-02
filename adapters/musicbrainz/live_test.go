package musicbrainz

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

// TestLivePublicReads is opt-in because it calls MusicBrainz's shared public
// service. The adapter's conservative public-service gate remains enabled.
func TestLivePublicReads(t *testing.T) {
	if os.Getenv("MUSICBRAINZ_LIVE_TEST") != "1" {
		t.Skip("set MUSICBRAINZ_LIVE_TEST=1 to exercise public WS/2 reads")
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
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	artists, err := liveCall(ctx, func() (socialhub.Page[Artist], error) {
		return client.SearchArtists(ctx, SearchRequest{Query: `artist:"The Beatles"`, Limit: 1})
	})
	if err != nil || len(artists.Items) == 0 {
		t.Fatalf("artist search failed: %v", err)
	}
	if artist, err := liveCall(ctx, func() (*Artist, error) { return client.GetArtist(ctx, artistMBID) }); err != nil || artist.Name != "The Beatles" {
		t.Fatalf("artist lookup failed: %#v, %v", artist, err)
	}
	releaseGroups, err := liveCall(ctx, func() (socialhub.Page[ReleaseGroup], error) {
		return client.SearchReleaseGroups(ctx, SearchRequest{Query: `releasegroup:"Abbey Road" AND artist:"The Beatles"`, Limit: 1})
	})
	if err != nil || len(releaseGroups.Items) == 0 {
		t.Fatalf("release-group search failed: %v", err)
	}
	if releaseGroup, err := liveCall(ctx, func() (*ReleaseGroup, error) {
		return client.GetReleaseGroup(ctx, releaseGroupMBID)
	}); err != nil || releaseGroup.Title != "Abbey Road" {
		t.Fatalf("release-group lookup failed: %#v, %v", releaseGroup, err)
	}
	releases, err := liveCall(ctx, func() (socialhub.Page[Release], error) {
		return client.SearchReleases(ctx, SearchRequest{Query: `release:"Abbey Road" AND artist:"The Beatles"`, Limit: 1})
	})
	if err != nil || len(releases.Items) == 0 {
		t.Fatalf("release search failed: %v", err)
	}
	if release, err := liveCall(ctx, func() (*Release, error) { return client.GetRelease(ctx, releaseMBID) }); err != nil || release.Title != "Abbey Road" {
		t.Fatalf("release lookup failed: %#v, %v", release, err)
	}
	recordings, err := liveCall(ctx, func() (socialhub.Page[Recording], error) {
		return client.SearchRecordings(ctx, SearchRequest{Query: `recording:"Come Together" AND artist:"The Beatles"`, Limit: 1})
	})
	if err != nil || len(recordings.Items) == 0 {
		t.Fatalf("recording search failed: %v", err)
	}
	if recording, err := liveCall(ctx, func() (*Recording, error) {
		return client.GetRecording(ctx, recordingMBID)
	}); err != nil || recording.Title != "Come Together" {
		t.Fatalf("recording lookup failed: %#v, %v", recording, err)
	}
	works, err := liveCall(ctx, func() (socialhub.Page[Work], error) {
		return client.SearchWorks(ctx, SearchRequest{Query: `work:"Come Together"`, Limit: 1})
	})
	if err != nil || len(works.Items) == 0 {
		t.Fatalf("work search failed: %v", err)
	}
	if work, err := liveCall(ctx, func() (*Work, error) { return client.GetWork(ctx, workMBID) }); err != nil || work.Title != "Come Together" {
		t.Fatalf("work lookup failed: %#v, %v", work, err)
	}
	if page, err := liveCall(ctx, func() (socialhub.Page[ReleaseGroup], error) {
		return client.ListArtistReleaseGroups(ctx, artistMBID, BrowseRequest{Limit: 1})
	}); err != nil || len(page.Items) == 0 {
		t.Fatalf("artist release-group browse failed: %v", err)
	}
	if page, err := liveCall(ctx, func() (socialhub.Page[Recording], error) {
		return client.ListArtistRecordings(ctx, artistMBID, BrowseRequest{Limit: 1})
	}); err != nil || len(page.Items) == 0 {
		t.Fatalf("artist recording browse failed: %v", err)
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
