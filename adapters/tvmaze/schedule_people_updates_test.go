package tvmaze

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestSchedulePeopleAndUpdatesWorkflows(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/api/schedule":
			if request.URL.Query().Get("country") == "US" && request.URL.Query().Get("date") != "2026-08-02" {
				http.Error(writer, "bad date", http.StatusBadRequest)
				return
			}
			writeJSON(writer, http.StatusOK, `[{"id":10,"name":"Episode","season":1,"number":1,"_embedded":{"show":`+showFixture+`}}]`)
		case "/api/schedule/web":
			writeJSON(writer, http.StatusOK, `[{"id":11,"name":"Web Episode","season":1,"number":2,"_embedded":{"show":`+showFixture+`}}]`)
		case "/api/search/people":
			if request.URL.Query().Get("q") != "Adam Scott" {
				http.Error(writer, "bad query", http.StatusBadRequest)
				return
			}
			writeJSON(writer, http.StatusOK, `[{"score":0.95,"person":{"id":30,"name":"Adam Scott","country":{"name":"United States","code":"US","timezone":"America/New_York"},"birthday":"1973-04-03","deathday":null,"gender":"Male","updated":1700000000}}]`)
		case "/api/people/30":
			writeJSON(writer, http.StatusOK, `{"id":30,"name":"Adam Scott","birthday":"1973-04-03","deathday":null,"gender":"Male","updated":1700000000}`)
		case "/api/people/30/castcredits":
			if request.URL.Query().Get("embed") != "show" {
				http.Error(writer, "missing show embed", http.StatusBadRequest)
				return
			}
			writeJSON(writer, http.StatusOK, `[{"self":false,"voice":false,"_links":{"show":{"href":"show"},"character":{"href":"character"}},"_embedded":{"show":`+showFixture+`}}]`)
		case "/api/people/30/crewcredits":
			if request.URL.Query().Get("embed") != "show" {
				http.Error(writer, "missing show embed", http.StatusBadRequest)
				return
			}
			writeJSON(writer, http.StatusOK, `[{"type":"Producer","_links":{"show":{"href":"show"}},"_embedded":{"show":`+showFixture+`}}]`)
		case "/api/updates/shows":
			if request.URL.Query().Get("since") != "week" {
				http.Error(writer, "bad period", http.StatusBadRequest)
				return
			}
			writeJSON(writer, http.StatusOK, `{"20":1700000020,"3":1700000003}`)
		case "/api/updates/people":
			if request.URL.Query().Has("since") {
				http.Error(writer, "unexpected period", http.StatusBadRequest)
				return
			}
			writeJSON(writer, http.StatusOK, `{"30":1700000030}`)
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	_, client := newTestClient(t, server)
	ctx := context.Background()
	date := time.Date(2026, time.August, 2, 10, 0, 0, 0, time.UTC)

	defaultSchedule, err := client.ListSchedule(ctx, ScheduleRequest{})
	if err != nil || len(defaultSchedule) != 1 || defaultSchedule[0].Embedded.Show == nil {
		t.Fatalf("default schedule=%#v err=%v", defaultSchedule, err)
	}
	schedule, err := client.ListSchedule(ctx, ScheduleRequest{Country: "US", Date: &date})
	if err != nil || len(schedule) != 1 || schedule[0].Embedded.Show.Name != "Severance" {
		t.Fatalf("schedule=%#v err=%v", schedule, err)
	}
	web, err := client.ListWebSchedule(ctx, WebScheduleRequest{})
	if err != nil || len(web) != 1 {
		t.Fatalf("web schedule=%#v err=%v", web, err)
	}
	global := ""
	if _, err := client.ListWebSchedule(ctx, WebScheduleRequest{Country: &global, Date: &date}); err != nil {
		t.Fatal(err)
	}
	us := "US"
	if _, err := client.ListWebSchedule(ctx, WebScheduleRequest{Country: &us}); err != nil {
		t.Fatal(err)
	}

	people, err := client.SearchPeople(ctx, "Adam Scott")
	if err != nil || len(people) != 1 || people[0].Person.Country == nil || people[0].Person.Country.Code != "US" {
		t.Fatalf("people=%#v err=%v", people, err)
	}
	person, err := client.GetPerson(ctx, 30)
	if err != nil || person.Name != "Adam Scott" || person.Deathday != nil {
		t.Fatalf("person=%#v err=%v", person, err)
	}
	cast, err := client.ListCastCredits(ctx, 30)
	if err != nil || len(cast) != 1 || cast[0].Embedded.Show == nil || cast[0].Links.Character == nil {
		t.Fatalf("cast credits=%#v err=%v", cast, err)
	}
	crew, err := client.ListCrewCredits(ctx, 30)
	if err != nil || len(crew) != 1 || crew[0].Type != "Producer" || crew[0].Embedded.Show == nil {
		t.Fatalf("crew credits=%#v err=%v", crew, err)
	}
	updates, err := client.ListShowUpdates(ctx, UpdateWeek)
	if err != nil || len(updates) != 2 || updates[0].ID != 3 || updates[1].ID != 20 || !updates[0].UpdatedAt.Equal(time.Unix(1700000003, 0).UTC()) {
		t.Fatalf("show updates=%#v err=%v", updates, err)
	}
	updates, err = client.ListPeopleUpdates(ctx, "")
	if err != nil || len(updates) != 1 || updates[0].ID != 30 {
		t.Fatalf("people updates=%#v err=%v", updates, err)
	}
}

func TestUpdatesRejectMalformedPlatformPayload(t *testing.T) {
	responses := []string{`{"01":1700000000}`, `{"1":0}`}
	call := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writeJSON(writer, http.StatusOK, responses[call])
		call++
	}))
	defer server.Close()
	_, client := newTestClient(t, server)
	for range responses {
		if _, err := client.ListShowUpdates(context.Background(), UpdateDay); err == nil {
			t.Fatal("expected malformed update payload error")
		}
	}
}
