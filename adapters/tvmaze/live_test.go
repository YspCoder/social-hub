package tvmaze

import (
	"context"
	"os"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

// TestLivePublicReads is opt-in because it depends on TVmaze's unversioned
// public service. It uses no credentials and never mutates remote state.
func TestLivePublicReads(t *testing.T) {
	if os.Getenv("TVMAZE_LIVE_TEST") != "1" {
		t.Skip("set TVMAZE_LIVE_TEST=1 to exercise public TVmaze reads")
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

	shows, err := client.SearchShows(ctx, "Severance")
	if err != nil || len(shows) == 0 {
		t.Fatalf("show search failed: %v", err)
	}
	show, err := client.LookupShow(ctx, LookupShowRequest{IMDB: "tt11280740"})
	if err != nil || show.ID != 44933 {
		t.Fatalf("show lookup failed: %#v, %v", show, err)
	}
	if episodes, err := client.ListEpisodes(ctx, show.ID, true); err != nil || len(episodes) == 0 {
		t.Fatalf("episode smoke failed: %v", err)
	}
	if seasons, err := client.ListSeasons(ctx, show.ID); err != nil || len(seasons) == 0 {
		t.Fatalf("season smoke failed: %v", err)
	}
	if cast, err := client.ListCast(ctx, show.ID); err != nil || len(cast) == 0 {
		t.Fatalf("cast smoke failed: %v", err)
	}
	people, err := client.SearchPeople(ctx, "Adam Scott")
	if err != nil || len(people) == 0 {
		t.Fatalf("people search failed: %v", err)
	}
	if _, err := client.GetPerson(ctx, people[0].Person.ID); err != nil {
		t.Fatalf("person smoke failed: %v", err)
	}
	if _, err := client.ListCastCredits(ctx, people[0].Person.ID); err != nil {
		t.Fatalf("credits smoke failed: %v", err)
	}
	if _, err := client.ListSchedule(ctx, ScheduleRequest{}); err != nil {
		t.Fatalf("broadcast schedule smoke failed: %v", err)
	}
	if _, err := client.ListWebSchedule(ctx, WebScheduleRequest{}); err != nil {
		t.Fatalf("web schedule smoke failed: %v", err)
	}
	if updates, err := client.ListShowUpdates(ctx, UpdateDay); err != nil || len(updates) == 0 {
		t.Fatalf("show updates smoke failed: %v", err)
	}
	if updates, err := client.ListPeopleUpdates(ctx, UpdateDay); err != nil || len(updates) == 0 {
		t.Fatalf("people updates smoke failed: %v", err)
	}
}
