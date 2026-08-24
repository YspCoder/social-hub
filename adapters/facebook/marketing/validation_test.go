package marketing

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

func TestWorkflowInputValidation(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	_, client := newTestAdapter(t, server, []string{managementScope})
	ctx := context.Background()
	targeting := TargetingSpec{GeoLocations: &GeoLocations{Countries: []string{"US"}}}
	active := StatusActive
	badStatus := Status("RUNNING")
	negative := int64(-1)
	daily, lifetime := int64(100), int64(200)
	name := "name"
	badCreative := "x"
	start := testNow.Add(time.Hour)
	end := testNow

	tests := []struct {
		name string
		call func() error
	}{
		{"get campaign ID", func() error { _, err := client.GetCampaign(ctx, "x/1"); return err }},
		{"list campaign page", func() error { _, err := client.ListCampaigns(ctx, ListCampaignsRequest{MaxResults: 101}); return err }},
		{"list campaign status", func() error {
			_, err := client.ListCampaigns(ctx, ListCampaignsRequest{EffectiveStatuses: []string{"paused"}})
			return err
		}},
		{"create campaign name", func() error {
			_, err := client.CreateCampaign(ctx, CreateCampaignRequest{Objective: ObjectiveTraffic})
			return err
		}},
		{"create campaign budget", func() error {
			_, err := client.CreateCampaign(ctx, CreateCampaignRequest{Name: "x", Objective: ObjectiveTraffic, DailyBudget: 1, LifetimeBudget: 2})
			return err
		}},
		{"create campaign category", func() error {
			_, err := client.CreateCampaign(ctx, CreateCampaignRequest{Name: "x", Objective: ObjectiveTraffic, SpecialAdCategories: []SpecialAdCategory{"bad"}})
			return err
		}},
		{"update campaign ID", func() error { return client.UpdateCampaign(ctx, "bad", UpdateCampaignRequest{Name: &name}) }},
		{"update campaign empty", func() error { return client.UpdateCampaign(ctx, testCampaignID, UpdateCampaignRequest{}) }},
		{"update campaign status", func() error {
			return client.UpdateCampaign(ctx, testCampaignID, UpdateCampaignRequest{Status: &badStatus})
		}},
		{"update campaign budget", func() error {
			return client.UpdateCampaign(ctx, testCampaignID, UpdateCampaignRequest{DailyBudget: &daily, LifetimeBudget: &lifetime})
		}},
		{"get adset ID", func() error { _, err := client.GetAdSet(ctx, "bad"); return err }},
		{"list adset parent", func() error { _, err := client.ListAdSets(ctx, ListAdSetsRequest{CampaignID: "bad"}); return err }},
		{"create adset targeting", func() error {
			_, err := client.CreateAdSet(ctx, CreateAdSetRequest{Name: "x", CampaignID: testCampaignID, OptimizationGoal: "REACH", BillingEvent: BillingEventImpressions})
			return err
		}},
		{"create adset schedule", func() error {
			_, err := client.CreateAdSet(ctx, CreateAdSetRequest{Name: "x", CampaignID: testCampaignID, OptimizationGoal: "REACH", BillingEvent: BillingEventImpressions, Targeting: targeting, StartTime: &start, EndTime: &end})
			return err
		}},
		{"update adset empty", func() error { return client.UpdateAdSet(ctx, testAdSetID, UpdateAdSetRequest{}) }},
		{"update adset bid", func() error { return client.UpdateAdSet(ctx, testAdSetID, UpdateAdSetRequest{BidAmount: &negative}) }},
		{"get creative ID", func() error { _, err := client.GetAdCreative(ctx, "bad"); return err }},
		{"create creative sources", func() error {
			_, err := client.CreateAdCreative(ctx, CreateAdCreativeRequest{Name: "x", ObjectStoryID: "123_1", ObjectStorySpec: &ObjectStorySpec{}})
			return err
		}},
		{"create creative link", func() error {
			_, err := client.CreateAdCreative(ctx, CreateAdCreativeRequest{Name: "x", ObjectStorySpec: &ObjectStorySpec{PageID: "123", LinkData: &LinkData{Link: "javascript:alert(1)"}}})
			return err
		}},
		{"get ad ID", func() error { _, err := client.GetAd(ctx, "bad"); return err }},
		{"list ad parent", func() error { _, err := client.ListAds(ctx, ListAdsRequest{AdSetID: "bad"}); return err }},
		{"create ad creative", func() error {
			_, err := client.CreateAd(ctx, CreateAdRequest{Name: "x", AdSetID: testAdSetID, CreativeID: badCreative})
			return err
		}},
		{"update ad empty", func() error { return client.UpdateAd(ctx, testAdID, UpdateAdRequest{}) }},
		{"update ad status", func() error { return client.UpdateAd(ctx, testAdID, UpdateAdRequest{Status: &badStatus}) }},
		{"insights entity", func() error { _, err := client.GetInsights(ctx, InsightsRequest{EntityID: "bad"}); return err }},
		{"insights level", func() error { _, err := client.GetInsights(ctx, InsightsRequest{Level: "creative"}); return err }},
		{"insights fields", func() error {
			_, err := client.GetInsights(ctx, InsightsRequest{Fields: []string{"bad(field)"}})
			return err
		}},
		{"insights date conflict", func() error {
			_, err := client.GetInsights(ctx, InsightsRequest{DatePreset: "last_7d", TimeRange: &TimeRange{Since: "2026-08-01", Until: "2026-08-02"}})
			return err
		}},
		{"insights date order", func() error {
			_, err := client.GetInsights(ctx, InsightsRequest{TimeRange: &TimeRange{Since: "2026-08-03", Until: "2026-08-02"}})
			return err
		}},
		{"insights increment", func() error { _, err := client.GetInsights(ctx, InsightsRequest{TimeIncrement: 91}); return err }},
		{"active is valid control", func() error {
			if !validMutationStatus(active) {
				return errors.New("active rejected")
			}
			return socialhub.ErrInvalidArgument
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.call(); !errors.Is(err, socialhub.ErrInvalidArgument) {
				t.Fatalf("error=%v", err)
			}
		})
	}
}

func TestLinkCreativeRequest(t *testing.T) {
	t.Parallel()
	var objectStorySpec string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_ = request.ParseForm()
		objectStorySpec = request.PostForm.Get("object_story_spec")
		writeJSON(writer, http.StatusOK, `{"id":"333"}`)
	}))
	defer server.Close()
	_, client := newTestAdapter(t, server, []string{managementScope})
	creative, err := client.CreateAdCreative(context.Background(), CreateAdCreativeRequest{
		Name: "Link Creative",
		ObjectStorySpec: &ObjectStorySpec{PageID: "123", LinkData: &LinkData{
			Link: "https://example.com/product", Message: "New product", ImageHash: "abc123",
			CallToAction: &CallToAction{Type: "SHOP_NOW", Value: CallToActionValue{Link: "https://example.com/product"}},
		}},
	})
	if err != nil || creative.ID != testCreativeID || objectStorySpec == "" {
		t.Fatalf("creative=%#v spec=%q err=%v", creative, objectStorySpec, err)
	}
}

func TestResponseContractsAndRedirectRejection(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch {
		case request.Method == http.MethodGet && request.URL.Path == "/v25.0/111":
			writeJSON(writer, http.StatusOK, `{"id":"999"}`)
		case request.Method == http.MethodGet && request.URL.Path == "/v25.0/act_123/campaigns":
			writeJSON(writer, http.StatusOK, `{"data":[{"name":"missing id"}]}`)
		case request.Method == http.MethodPost && request.URL.Path == "/v25.0/act_123/campaigns":
			writeJSON(writer, http.StatusOK, `{}`)
		case request.Method == http.MethodPost && request.URL.Path == "/v25.0/111":
			writeJSON(writer, http.StatusOK, `{"success":false}`)
		case request.Method == http.MethodGet && request.URL.Path == "/v25.0/222":
			writeJSON(writer, http.StatusOK, `{"id":"222","created_time":"not-a-time"}`)
		default:
			writer.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	_, client := newTestAdapter(t, server, []string{managementScope})
	ctx := context.Background()
	name := "renamed"
	calls := []func() error{
		func() error { _, err := client.GetCampaign(ctx, testCampaignID); return err },
		func() error { _, err := client.ListCampaigns(ctx, ListCampaignsRequest{}); return err },
		func() error {
			_, err := client.CreateCampaign(ctx, CreateCampaignRequest{Name: "x", Objective: ObjectiveTraffic})
			return err
		},
		func() error { return client.UpdateCampaign(ctx, testCampaignID, UpdateCampaignRequest{Name: &name}) },
		func() error { _, err := client.GetAdSet(ctx, testAdSetID); return err },
	}
	for index, call := range calls {
		if err := call(); hubErrorCode(err) != socialhub.CodePlatformError {
			t.Fatalf("call %d error=%v", index, err)
		}
	}

	var redirected bool
	target := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { redirected = true }))
	defer target.Close()
	redirector := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		http.Redirect(writer, request, target.URL, http.StatusFound)
	}))
	defer redirector.Close()
	_, redirectClient := newTestAdapter(t, redirector, []string{readScope})
	if _, err := redirectClient.GetCampaign(ctx, testCampaignID); err == nil || redirected {
		t.Fatalf("redirect error=%v followed=%v", err, redirected)
	}
}

func TestGraphTime(t *testing.T) {
	t.Parallel()
	var value GraphTime
	if err := value.UnmarshalJSON([]byte(`"2026-08-09T01:02:03+0800"`)); err != nil || value.Location() == nil {
		t.Fatalf("value=%v err=%v", value, err)
	}
	encoded, err := value.MarshalJSON()
	if err != nil || len(encoded) == 0 {
		t.Fatalf("encoded=%q err=%v", encoded, err)
	}
	if err := value.UnmarshalJSON([]byte(`"invalid"`)); err == nil {
		t.Fatal("invalid time accepted")
	}
}

func TestDefaultInsightFieldsFollowLevel(t *testing.T) {
	t.Parallel()
	account := defaultFieldsForLevel(InsightLevelAccount)
	campaign := defaultFieldsForLevel(InsightLevelCampaign)
	adSet := defaultFieldsForLevel(InsightLevelAdSet)
	ad := defaultFieldsForLevel(InsightLevelAd)
	if contains(account, "campaign_id") || !contains(campaign, "campaign_id") || contains(campaign, "adset_id") ||
		!contains(adSet, "adset_id") || contains(adSet, "ad_id") || !contains(ad, "ad_id") {
		t.Fatalf("fields account=%v campaign=%v adset=%v ad=%v", account, campaign, adSet, ad)
	}
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
