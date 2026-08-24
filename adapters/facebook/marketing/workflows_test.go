package marketing

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestManagementAndInsightsWorkflow(t *testing.T) {
	t.Parallel()
	type capture struct {
		forms   map[string]url.Values
		queries map[string]url.Values
	}
	seen := capture{forms: make(map[string]url.Values), queries: make(map[string]url.Values)}
	var mu sync.Mutex
	recordForm := func(name string, request *http.Request) {
		_ = request.ParseForm()
		mu.Lock()
		seen.forms[name] = cloneValues(request.PostForm)
		mu.Unlock()
	}
	recordQuery := func(name string, request *http.Request) {
		mu.Lock()
		seen.queries[name] = request.URL.Query()
		mu.Unlock()
	}

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer access-token" || request.URL.Query().Get("appsecret_proof") == "" {
			writeJSON(writer, http.StatusUnauthorized, `{"error":{"code":190,"message":"bad auth"}}`)
			return
		}
		switch {
		case request.Method == http.MethodPost && request.URL.Path == "/v25.0/act_123/campaigns":
			recordForm("campaign_create", request)
			writeJSON(writer, http.StatusOK, `{"id":"111"}`)
		case request.Method == http.MethodGet && request.URL.Path == "/v25.0/act_123/campaigns":
			recordQuery("campaign_list", request)
			writeJSON(writer, http.StatusOK, `{"data":[{"id":"111","account_id":"123","name":"Launch","objective":"OUTCOME_TRAFFIC","status":"PAUSED","effective_status":"PAUSED","daily_budget":"5000","special_ad_categories":[],"created_time":"2026-08-09T01:00:00+0000"}],"paging":{"cursors":{"before":"prev-campaign","after":"next-campaign"},"next":"https://graph.example/next"}}`)
		case request.Method == http.MethodGet && request.URL.Path == "/v25.0/111":
			writeJSON(writer, http.StatusOK, `{"id":"111","account_id":"123","name":"Launch","objective":"OUTCOME_TRAFFIC","configured_status":"PAUSED","effective_status":"PAUSED","created_time":"2026-08-09T01:00:00+0000","updated_time":"2026-08-09T02:00:00Z"}`)
		case request.Method == http.MethodPost && request.URL.Path == "/v25.0/111":
			recordForm("campaign_update", request)
			writeJSON(writer, http.StatusOK, `{"success":true}`)
		case request.Method == http.MethodPost && request.URL.Path == "/v25.0/act_123/adsets":
			recordForm("adset_create", request)
			writeJSON(writer, http.StatusOK, `{"id":"222"}`)
		case request.Method == http.MethodGet && request.URL.Path == "/v25.0/111/adsets":
			recordQuery("adset_list", request)
			writeJSON(writer, http.StatusOK, `{"data":[{"id":"222","account_id":"123","campaign_id":"111","name":"US Adults","status":"PAUSED","optimization_goal":"LANDING_PAGE_VIEWS","billing_event":"IMPRESSIONS","targeting":{"age_min":21,"age_max":55,"geo_locations":{"countries":["US"]}}}],"paging":{"cursors":{"after":"next-adset"},"next":"https://graph.example/next"}}`)
		case request.Method == http.MethodGet && request.URL.Path == "/v25.0/222":
			writeJSON(writer, http.StatusOK, `{"id":"222","account_id":"123","campaign_id":"111","name":"US Adults","effective_status":"PAUSED","start_time":"2026-08-10T00:00:00+0000","end_time":"2026-08-20T00:00:00+0000","targeting":{"geo_locations":{"countries":["US"]}}}`)
		case request.Method == http.MethodPost && request.URL.Path == "/v25.0/222":
			recordForm("adset_update", request)
			writeJSON(writer, http.StatusOK, `{"id":"222"}`)
		case request.Method == http.MethodPost && request.URL.Path == "/v25.0/act_123/adcreatives":
			recordForm("creative_create", request)
			writeJSON(writer, http.StatusOK, `{"id":"333","effective_object_story_id":"123_999"}`)
		case request.Method == http.MethodGet && request.URL.Path == "/v25.0/act_123/adcreatives":
			recordQuery("creative_list", request)
			writeJSON(writer, http.StatusOK, `{"data":[{"id":"333","account_id":"123","name":"Launch Creative","object_story_id":"123_999","thumbnail_url":"https://cdn.example/creative.jpg"}]}`)
		case request.Method == http.MethodGet && request.URL.Path == "/v25.0/333":
			writeJSON(writer, http.StatusOK, `{"id":"333","account_id":"123","name":"Launch Creative","effective_object_story_id":"123_999"}`)
		case request.Method == http.MethodPost && request.URL.Path == "/v25.0/act_123/ads":
			recordForm("ad_create", request)
			writeJSON(writer, http.StatusOK, `{"id":"444"}`)
		case request.Method == http.MethodGet && request.URL.Path == "/v25.0/222/ads":
			recordQuery("ad_list", request)
			writeJSON(writer, http.StatusOK, `{"data":[{"id":"444","account_id":"123","campaign_id":"111","adset_id":"222","name":"Launch Ad","status":"PAUSED","creative":{"id":"333"}}],"paging":{"cursors":{"after":"next-ad"},"next":"https://graph.example/next"}}`)
		case request.Method == http.MethodGet && request.URL.Path == "/v25.0/444":
			writeJSON(writer, http.StatusOK, `{"id":"444","account_id":"123","campaign_id":"111","adset_id":"222","name":"Launch Ad","effective_status":"PAUSED","creative":{"id":"333","name":"Launch Creative"},"created_time":"2026-08-09T03:00:00+0000"}`)
		case request.Method == http.MethodPost && request.URL.Path == "/v25.0/444":
			recordForm("ad_update", request)
			writeJSON(writer, http.StatusOK, `{"success":true}`)
		case request.Method == http.MethodGet && request.URL.Path == "/v25.0/111/insights":
			recordQuery("insights", request)
			writeJSON(writer, http.StatusOK, `{"data":[{"account_id":"123","campaign_id":"111","campaign_name":"Launch","date_start":"2026-08-01","date_stop":"2026-08-08","impressions":"1000","reach":"800","clicks":"30","spend":"42.50","ctr":"3.0","actions":[{"action_type":"link_click","value":"30"}]}],"paging":{"cursors":{"before":"prev-insight","after":"next-insight"},"next":"https://graph.example/next"},"summary":{"impressions":"1000","spend":"42.50"}}`)
		default:
			writeJSON(writer, http.StatusNotFound, `{"error":{"code":100,"message":"unexpected route"}}`)
		}
	}))
	defer server.Close()
	_, client := newTestAdapter(t, server, []string{managementScope})
	ctx := context.Background()

	campaign, err := client.CreateCampaign(ctx, CreateCampaignRequest{
		Name: "Launch", Objective: ObjectiveTraffic, DailyBudget: 5000,
	})
	if err != nil || campaign.ID != testCampaignID || campaign.Status != StatusPaused || campaign.DailyBudget != "5000" {
		t.Fatalf("created campaign=%#v err=%v", campaign, err)
	}
	campaign, err = client.GetCampaign(ctx, testCampaignID)
	if err != nil || campaign.CreatedTime == nil || campaign.CreatedTime.Time.Hour() != 1 || len(campaign.Raw) == 0 {
		t.Fatalf("campaign=%#v err=%v", campaign, err)
	}
	campaigns, err := client.ListCampaigns(ctx, ListCampaignsRequest{Cursor: "campaign-cursor", MaxResults: 20, EffectiveStatuses: []string{"PAUSED"}})
	if err != nil || len(campaigns.Items) != 1 || !campaigns.HasMore || campaigns.NextCursor == nil || *campaigns.NextCursor != "next-campaign" {
		t.Fatalf("campaigns=%#v err=%v", campaigns, err)
	}
	active := StatusActive
	if err := client.UpdateCampaign(ctx, testCampaignID, UpdateCampaignRequest{Status: &active}); err != nil {
		t.Fatal(err)
	}

	start := time.Date(2026, time.August, 10, 0, 0, 0, 0, time.UTC)
	end := start.Add(10 * 24 * time.Hour)
	targeting := TargetingSpec{AgeMin: 21, AgeMax: 55, GeoLocations: &GeoLocations{Countries: []string{"US"}}, PublisherPlatforms: []string{"facebook", "instagram"}}
	adSet, err := client.CreateAdSet(ctx, CreateAdSetRequest{
		Name: "US Adults", CampaignID: testCampaignID, OptimizationGoal: "LANDING_PAGE_VIEWS",
		BillingEvent: BillingEventImpressions, DailyBudget: 3000, StartTime: &start, EndTime: &end,
		Targeting: targeting, PromotedObject: &PromotedObject{PageID: "123"},
	})
	if err != nil || adSet.ID != testAdSetID || adSet.Status != StatusPaused || adSet.Targeting.AgeMin != 21 {
		t.Fatalf("created ad set=%#v err=%v", adSet, err)
	}
	adSet, err = client.GetAdSet(ctx, testAdSetID)
	if err != nil || adSet.StartTime == nil || adSet.EndTime == nil || len(adSet.Raw) == 0 {
		t.Fatalf("ad set=%#v err=%v", adSet, err)
	}
	adSets, err := client.ListAdSets(ctx, ListAdSetsRequest{CampaignID: testCampaignID, Cursor: "adset-cursor", EffectiveStatuses: []string{"PAUSED"}})
	if err != nil || len(adSets.Items) != 1 || !adSets.HasMore {
		t.Fatalf("ad sets=%#v err=%v", adSets, err)
	}
	newBudget := int64(3500)
	if err := client.UpdateAdSet(ctx, testAdSetID, UpdateAdSetRequest{DailyBudget: &newBudget}); err != nil {
		t.Fatal(err)
	}

	creative, err := client.CreateAdCreative(ctx, CreateAdCreativeRequest{Name: "Launch Creative", ObjectStoryID: "123_999", URLTags: "utm_source=meta"})
	if err != nil || creative.ID != testCreativeID || creative.EffectiveObjectStoryID != "123_999" {
		t.Fatalf("created creative=%#v err=%v", creative, err)
	}
	creative, err = client.GetAdCreative(ctx, testCreativeID)
	if err != nil || creative.Name != "Launch Creative" || len(creative.Raw) == 0 {
		t.Fatalf("creative=%#v err=%v", creative, err)
	}
	creatives, err := client.ListAdCreatives(ctx, ListAdCreativesRequest{Cursor: "creative-cursor", MaxResults: 10})
	if err != nil || len(creatives.Items) != 1 || creatives.Items[0].ThumbnailURL == "" {
		t.Fatalf("creatives=%#v err=%v", creatives, err)
	}

	ad, err := client.CreateAd(ctx, CreateAdRequest{Name: "Launch Ad", AdSetID: testAdSetID, CreativeID: testCreativeID})
	if err != nil || ad.ID != testAdID || ad.Status != StatusPaused || ad.Creative == nil || ad.Creative.ID != testCreativeID {
		t.Fatalf("created ad=%#v err=%v", ad, err)
	}
	ad, err = client.GetAd(ctx, testAdID)
	if err != nil || ad.Creative == nil || ad.Creative.Name != "Launch Creative" || ad.CreatedTime == nil || len(ad.Raw) == 0 {
		t.Fatalf("ad=%#v err=%v", ad, err)
	}
	ads, err := client.ListAds(ctx, ListAdsRequest{AdSetID: testAdSetID, EffectiveStatuses: []string{"PAUSED"}})
	if err != nil || len(ads.Items) != 1 || !ads.HasMore {
		t.Fatalf("ads=%#v err=%v", ads, err)
	}
	if err := client.UpdateAd(ctx, testAdID, UpdateAdRequest{Status: &active}); err != nil {
		t.Fatal(err)
	}

	insights, err := client.GetInsights(ctx, InsightsRequest{
		EntityID: testCampaignID, Level: InsightLevelCampaign,
		TimeRange:  &TimeRange{Since: "2026-08-01", Until: "2026-08-08"},
		Breakdowns: []string{"country"}, TimeIncrement: 1, Cursor: "insight-cursor", MaxResults: 25,
	})
	if err != nil || len(insights.Items) != 1 || insights.Items[0].Spend != "42.50" || len(insights.Items[0].Raw) == 0 ||
		len(insights.Items[0].Actions) != 1 || insights.Summary == nil || insights.Summary.Impressions != "1000" || !insights.HasMore {
		t.Fatalf("insights=%#v err=%v", insights, err)
	}

	mu.Lock()
	defer mu.Unlock()
	if seen.forms["campaign_create"].Get("status") != "PAUSED" || seen.forms["campaign_create"].Get("special_ad_categories") != "[]" {
		t.Fatalf("campaign create form=%v", seen.forms["campaign_create"])
	}
	if seen.forms["campaign_update"].Get("status") != "ACTIVE" {
		t.Fatalf("campaign update form=%v", seen.forms["campaign_update"])
	}
	var sentTargeting TargetingSpec
	if err := json.Unmarshal([]byte(seen.forms["adset_create"].Get("targeting")), &sentTargeting); err != nil || sentTargeting.AgeMin != 21 || sentTargeting.GeoLocations == nil || sentTargeting.GeoLocations.Countries[0] != "US" {
		t.Fatalf("targeting=%#v err=%v", sentTargeting, err)
	}
	if seen.forms["adset_create"].Get("status") != "PAUSED" || seen.forms["adset_update"].Get("daily_budget") != "3500" {
		t.Fatalf("ad set forms=%v / %v", seen.forms["adset_create"], seen.forms["adset_update"])
	}
	if seen.forms["creative_create"].Get("object_story_id") != "123_999" || seen.forms["ad_create"].Get("status") != "PAUSED" || !strings.Contains(seen.forms["ad_create"].Get("creative"), testCreativeID) {
		t.Fatalf("creative/ad forms=%v / %v", seen.forms["creative_create"], seen.forms["ad_create"])
	}
	if seen.forms["ad_update"].Get("status") != "ACTIVE" {
		t.Fatalf("ad update form=%v", seen.forms["ad_update"])
	}
	if seen.queries["campaign_list"].Get("after") != "campaign-cursor" || seen.queries["campaign_list"].Get("effective_status") != `["PAUSED"]` {
		t.Fatalf("campaign list query=%v", seen.queries["campaign_list"])
	}
	if seen.queries["insights"].Get("level") != "campaign" || seen.queries["insights"].Get("breakdowns") != "country" || seen.queries["insights"].Get("time_increment") != "1" || !strings.Contains(seen.queries["insights"].Get("fields"), "impressions") {
		t.Fatalf("insights query=%v", seen.queries["insights"])
	}
}

func cloneValues(input url.Values) url.Values {
	output := make(url.Values, len(input))
	for key, values := range input {
		output[key] = append([]string(nil), values...)
	}
	return output
}
