package unityadvertising

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"social-hub/pkg/socialhub"
)

func TestAllAdvertisingManagementWorkflows(t *testing.T) {
	organizationPath := "/advertise/v1/organizations/" + testOrganizationID
	appsPath := organizationPath + "/apps"
	appPath := appsPath + "/" + testCampaignSetID
	creativesPath := appPath + "/creatives"
	creativePath := creativesPath + "/" + testCreativeID
	packsPath := appPath + "/creative-packs"
	packPath := packsPath + "/" + testCreativePackID
	campaignsPath := appPath + "/campaigns"
	campaignPath := campaignsPath + "/" + testCampaignID
	assignedPath := campaignPath + "/assigned-creative-packs"
	seen := make(map[string]int)

	cpiPage := Page[CPIBid]{Total: 1, Results: []CPIBid{{Country: "US", Bid: "1.250"}}}
	sourcePage := Page[SourceBid]{Total: 1, Results: []SourceBid{{Country: "US", SourceAppID: testSourceAppID, Bid: "1.250"}}}
	roasPage := Page[ROASBid]{Total: 1, Results: []ROASBid{{Country: "US", Goal: "25.50", MaxBid: "500.000"}}}
	retentionPage := Page[RetentionBid]{Total: 1, Results: []RetentionBid{{Country: "US", BaseBid: "1.250", MaxBid: "500.000"}}}
	eventPage := Page[EventOptimizationBid]{Total: 1, Results: []EventOptimizationBid{{Country: "US", Bid: "1.250"}}}

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		assertBearerRequest(t, request)
		key := request.Method + " " + request.URL.Path
		seen[key]++
		switch key {
		case http.MethodGet + " " + appsPath:
			query := request.URL.Query()
			if query.Get("offset") != "2" || query.Get("limit") != "10" || query.Get("filter[store]") != "apple" || query.Get("filter[storeId]") != "1358236" {
				t.Errorf("app list query=%v", query)
			}
			writeJSON(t, writer, http.StatusOK, Page[App]{Total: 1, Offset: 2, Limit: 10, Results: []App{appFixture()}})
		case http.MethodPost + " " + appsPath:
			var payload map[string]any
			decodeJSONBody(t, request, &payload)
			if payload["store"] != "apple" || payload["storeId"] != "1358236" {
				t.Errorf("app create payload=%v", payload)
			}
			writeJSON(t, writer, http.StatusCreated, appFixture())
		case http.MethodGet + " " + appPath:
			if request.Header.Get("X-Request-ID") != "caller-request" {
				t.Errorf("X-Request-ID=%q", request.Header.Get("X-Request-ID"))
			}
			writeJSON(t, writer, http.StatusOK, appFixture())
		case http.MethodPatch + " " + appPath:
			var payload map[string]any
			decodeJSONBody(t, request, &payload)
			if value, exists := payload["adomain"]; !exists || value != nil || payload["appAttributionClickUrl"] != "https://example.test/new-click" {
				t.Errorf("app update payload=%v", payload)
			}
			writeJSON(t, writer, http.StatusOK, appFixture())
		case http.MethodDelete + " " + appPath:
			writer.WriteHeader(http.StatusNoContent)
		case http.MethodGet + " " + creativesPath:
			if request.URL.Query().Get("offset") != "3" || request.URL.Query().Get("limit") != "20" {
				t.Errorf("creative list query=%v", request.URL.Query())
			}
			writeJSON(t, writer, http.StatusOK, Page[Creative]{Total: 1, Offset: 3, Limit: 20, Results: []Creative{creativeFixture()}})
		case http.MethodPost + " " + creativesPath:
			if err := request.ParseMultipartForm(1 << 20); err != nil {
				t.Fatal(err)
			}
			var metadata map[string]any
			if err := json.Unmarshal([]byte(request.FormValue("creativeInfo")), &metadata); err != nil {
				t.Fatal(err)
			}
			file, header, err := request.FormFile("videoFile")
			if err != nil {
				t.Fatal(err)
			}
			defer file.Close()
			body, _ := io.ReadAll(file)
			if metadata["name"] != "Launch video" || header.Filename != "launch.mp4" || header.Header.Get("Content-Type") != "video/mp4" || string(body) != "video" {
				t.Errorf("creative metadata=%v header=%v body=%q", metadata, header.Header, body)
			}
			writeJSON(t, writer, http.StatusCreated, creativeFixture())
		case http.MethodGet + " " + creativePath:
			writeJSON(t, writer, http.StatusOK, creativeFixture())
		case http.MethodGet + " " + packsPath:
			if request.URL.Query().Get("offset") != "4" || request.URL.Query().Get("limit") != "30" || request.URL.Query().Get("filter[name]") != "Launch*" {
				t.Errorf("creative pack query=%v", request.URL.Query())
			}
			writeJSON(t, writer, http.StatusOK, Page[CreativePack]{Total: 1, Offset: 4, Limit: 30, Results: []CreativePack{creativePackFixture()}})
		case http.MethodPost + " " + packsPath:
			var payload CreateCreativePackRequest
			decodeJSONBody(t, request, &payload)
			if payload.Name != "Launch pack" || !slices.Equal(payload.CreativeIDs, []string{testCreativeID}) {
				t.Errorf("creative pack create payload=%#v", payload)
			}
			writeJSON(t, writer, http.StatusCreated, creativePackFixture())
		case http.MethodGet + " " + packPath:
			writeJSON(t, writer, http.StatusOK, creativePackFixture())
		case http.MethodPatch + " " + packPath:
			var payload map[string]any
			decodeJSONBody(t, request, &payload)
			if payload["name"] != "Renamed pack" {
				t.Errorf("creative pack update payload=%v", payload)
			}
			writeJSON(t, writer, http.StatusOK, creativePackFixture())
		case http.MethodDelete + " " + packPath:
			writer.WriteHeader(http.StatusNoContent)
		case http.MethodGet + " " + campaignsPath:
			if request.URL.Query().Get("filter[enabled]") != "true" {
				t.Errorf("campaign query=%v", request.URL.Query())
			}
			writeJSON(t, writer, http.StatusOK, Page[Campaign]{Total: 1, Results: []Campaign{campaignFixture()}})
		case http.MethodPost + " " + campaignsPath:
			var payload map[string]any
			decodeJSONBody(t, request, &payload)
			if payload["goal"] != "installs" || payload["name"] != "Launch campaign" {
				t.Errorf("campaign create payload=%v", payload)
			}
			writeJSON(t, writer, http.StatusCreated, campaignFixture())
		case http.MethodGet + " " + campaignPath:
			if !slices.Equal(request.URL.Query()["includeFields"], []string{"cpiBids", "budget"}) {
				t.Errorf("campaign include query=%v", request.URL.Query())
			}
			writeJSON(t, writer, http.StatusOK, campaignFixture())
		case http.MethodPatch + " " + campaignPath:
			var payload map[string]any
			decodeJSONBody(t, request, &payload)
			if payload["name"] != "Updated campaign" || payload["enabled"] != true {
				t.Errorf("campaign update payload=%v", payload)
			}
			writeJSON(t, writer, http.StatusOK, campaignFixture())
		case http.MethodDelete + " " + campaignPath:
			writer.WriteHeader(http.StatusNoContent)
		case http.MethodGet + " " + assignedPath:
			writeJSON(t, writer, http.StatusOK, Page[AssignedCreativePack]{Total: 1, Results: []AssignedCreativePack{{ID: testCreativePackID}}})
		case http.MethodPost + " " + assignedPath:
			var payload map[string]string
			decodeJSONBody(t, request, &payload)
			if payload["id"] != testCreativePackID {
				t.Errorf("assignment payload=%v", payload)
			}
			writeJSON(t, writer, http.StatusCreated, AssignedCreativePack{ID: testCreativePackID})
		case http.MethodDelete + " " + assignedPath + "/" + testCreativePackID:
			writer.WriteHeader(http.StatusNoContent)
		case http.MethodGet + " " + campaignPath + "/targeting":
			writeJSON(t, writer, http.StatusOK, targetingFixture())
		case http.MethodPatch + " " + campaignPath + "/targeting":
			var payload Targeting
			decodeJSONBody(t, request, &payload)
			if payload.AppTargeting == nil || payload.AppTargeting.AllowList == nil {
				t.Errorf("targeting payload=%#v", payload)
			}
			writeJSON(t, writer, http.StatusOK, targetingFixture())
		case http.MethodGet + " " + campaignPath + "/budget":
			writeJSON(t, writer, http.StatusOK, budgetFixture())
		case http.MethodPatch + " " + campaignPath + "/budget":
			var payload map[string]any
			decodeJSONBody(t, request, &payload)
			if payload["total"] != "3000.00" || payload["daily"] != "600.00" {
				t.Errorf("budget payload=%v", payload)
			}
			writeJSON(t, writer, http.StatusOK, budgetFixture())
		case http.MethodGet + " " + appPath + "/audience-pinpointer/sdk-event-names":
			writeJSON(t, writer, http.StatusOK, Page[SDKEventNamesInfo]{Total: 1, Results: []SDKEventNamesInfo{{EventOptimizationType: "purchase", SDKEventNames: []SDKEventName{"purchase_complete"}}}})
		case http.MethodGet + " " + appPath + "/audience-pinpointer/event-optimization-info":
			writeJSON(t, writer, http.StatusOK, Page[EventOptimizationInfo]{Total: 1, Results: []EventOptimizationInfo{{EventOptimizationType: "purchase", Countries: []CountryCode{"US"}}}})
		case http.MethodGet + " " + campaignPath + "/cpi-bids", http.MethodPut + " " + campaignPath + "/cpi-bids", http.MethodPatch + " " + campaignPath + "/cpi-bids":
			writeJSON(t, writer, http.StatusOK, cpiPage)
		case http.MethodGet + " " + campaignPath + "/source-bids", http.MethodPut + " " + campaignPath + "/source-bids", http.MethodPatch + " " + campaignPath + "/source-bids":
			writeJSON(t, writer, http.StatusOK, sourcePage)
		case http.MethodGet + " " + campaignPath + "/roas-bids", http.MethodPut + " " + campaignPath + "/roas-bids", http.MethodPatch + " " + campaignPath + "/roas-bids":
			writeJSON(t, writer, http.StatusOK, roasPage)
		case http.MethodGet + " " + campaignPath + "/retention-bids", http.MethodPut + " " + campaignPath + "/retention-bids", http.MethodPatch + " " + campaignPath + "/retention-bids":
			writeJSON(t, writer, http.StatusOK, retentionPage)
		case http.MethodGet + " " + campaignPath + "/event-optimization-bids", http.MethodPut + " " + campaignPath + "/event-optimization-bids", http.MethodPatch + " " + campaignPath + "/event-optimization-bids":
			writeJSON(t, writer, http.StatusOK, eventPage)
		default:
			t.Fatalf("unexpected request %s", key)
		}
	}))
	defer server.Close()
	_, client := newTestAdapter(t, server)
	ctx := context.Background()

	if page, err := client.ListApps(ctx, ListAppsRequest{Offset: 2, Limit: 10, Store: StoreApple, StoreID: "1358236"}); err != nil || len(page.Results) != 1 || len(page.Results[0].Raw) == 0 {
		t.Fatalf("apps=%#v err=%v", page, err)
	}
	createApp := CreateAppleAppRequest{Store: StoreApple, StoreID: "1358236", AttributionClickURL: NewNullableString("https://example.test/click")}
	if value, err := client.CreateApp(ctx, createApp); err != nil || value.ID != testCampaignSetID {
		t.Fatalf("created app=%#v err=%v", value, err)
	}
	if value, err := client.GetApp(ctx, testCampaignSetID, socialhub.WithRequestID("caller-request")); err != nil || value.ID != testCampaignSetID {
		t.Fatalf("app=%#v err=%v", value, err)
	}
	if value, err := client.UpdateApp(ctx, testCampaignSetID, UpdateAppRequest{ADomain: NewNullString(), AttributionClickURL: NewNullableString("https://example.test/new-click")}); err != nil || value.ID != testCampaignSetID {
		t.Fatalf("updated app=%#v err=%v", value, err)
	}
	if err := client.DeleteApp(ctx, testCampaignSetID); err != nil {
		t.Fatal(err)
	}

	if page, err := client.ListCreatives(ctx, testCampaignSetID, ListCreativesRequest{Offset: 3, Limit: 20}); err != nil || len(page.Results) != 1 {
		t.Fatalf("creatives=%#v err=%v", page, err)
	}
	video := []byte("video")
	if value, err := client.CreateCreative(ctx, testCampaignSetID, VideoUpload{Name: "Launch video", Language: LanguageEnglish, Filename: "launch.mp4", Size: int64(len(video)), File: bytes.NewReader(video)}); err != nil || value.ID != testCreativeID {
		t.Fatalf("created creative=%#v err=%v", value, err)
	}
	if value, err := client.GetCreative(ctx, testCampaignSetID, testCreativeID); err != nil || value.ID != testCreativeID {
		t.Fatalf("creative=%#v err=%v", value, err)
	}

	if page, err := client.ListCreativePacks(ctx, testCampaignSetID, ListCreativePacksRequest{Offset: 4, Limit: 30, Name: "Launch*"}); err != nil || len(page.Results) != 1 {
		t.Fatalf("creative packs=%#v err=%v", page, err)
	}
	createPack := CreateCreativePackRequest{Name: "Launch pack", CreativeIDs: []string{testCreativeID}, Type: CreativePackVideo}
	if value, err := client.CreateCreativePack(ctx, testCampaignSetID, createPack); err != nil || value.ID != testCreativePackID {
		t.Fatalf("created pack=%#v err=%v", value, err)
	}
	if value, err := client.GetCreativePack(ctx, testCampaignSetID, testCreativePackID); err != nil || value.ID != testCreativePackID {
		t.Fatalf("pack=%#v err=%v", value, err)
	}
	if value, err := client.UpdateCreativePack(ctx, testCampaignSetID, testCreativePackID, UpdateCreativePackRequest{Name: stringPointer("Renamed pack")}); err != nil || value.ID != testCreativePackID {
		t.Fatalf("updated pack=%#v err=%v", value, err)
	}
	if err := client.DeleteCreativePack(ctx, testCampaignSetID, testCreativePackID); err != nil {
		t.Fatal(err)
	}

	enabled := true
	if page, err := client.ListCampaigns(ctx, testCampaignSetID, ListCampaignsRequest{Enabled: &enabled}); err != nil || len(page.Results) != 1 {
		t.Fatalf("campaigns=%#v err=%v", page, err)
	}
	createCampaign := CreateInstallsCampaignRequest{CampaignCreateBase: CampaignCreateBase{Name: "Launch campaign", BillingType: BillingCPI, BiddingStrategy: BiddingManual}, Goal: CampaignGoalInstalls}
	if value, err := client.CreateCampaign(ctx, testCampaignSetID, createCampaign); err != nil || value.ID != testCampaignID {
		t.Fatalf("created campaign=%#v err=%v", value, err)
	}
	if value, err := client.GetCampaign(ctx, testCampaignSetID, testCampaignID, GetCampaignRequest{IncludeFields: []CampaignIncludeField{IncludeCPIBids, IncludeBudget, IncludeCPIBids}}); err != nil || value.ID != testCampaignID {
		t.Fatalf("campaign=%#v err=%v", value, err)
	}
	if value, err := client.UpdateCampaign(ctx, testCampaignSetID, testCampaignID, UpdateCampaignRequest{Name: stringPointer("Updated campaign"), Enabled: boolPointer(true)}); err != nil || value.ID != testCampaignID {
		t.Fatalf("updated campaign=%#v err=%v", value, err)
	}
	if err := client.DeleteCampaign(ctx, testCampaignSetID, testCampaignID); err != nil {
		t.Fatal(err)
	}
	if page, err := client.ListAssignedCreativePacks(ctx, testCampaignSetID, testCampaignID); err != nil || len(page.Results) != 1 {
		t.Fatalf("assignments=%#v err=%v", page, err)
	}
	if value, err := client.AssignCreativePack(ctx, testCampaignSetID, testCampaignID, testCreativePackID); err != nil || value.ID != testCreativePackID {
		t.Fatalf("assignment=%#v err=%v", value, err)
	}
	if err := client.UnassignCreativePack(ctx, testCampaignSetID, testCampaignID, testCreativePackID); err != nil {
		t.Fatal(err)
	}
	if value, err := client.GetTargeting(ctx, testCampaignSetID, testCampaignID); err != nil || value.AppTargeting == nil {
		t.Fatalf("targeting=%#v err=%v", value, err)
	}
	if value, err := client.UpdateTargeting(ctx, testCampaignSetID, testCampaignID, targetingFixture()); err != nil || value.DeviceTargeting == nil {
		t.Fatalf("updated targeting=%#v err=%v", value, err)
	}
	if value, err := client.GetCampaignBudget(ctx, testCampaignSetID, testCampaignID); err != nil || value.Daily == nil {
		t.Fatalf("budget=%#v err=%v", value, err)
	}
	total, daily := Money("3000.00"), Money("600.00")
	if value, err := client.UpdateCampaignBudget(ctx, testCampaignSetID, testCampaignID, DailyBudgetUpdate{Total: &total, Daily: &daily}); err != nil || value.Daily == nil {
		t.Fatalf("updated budget=%#v err=%v", value, err)
	}
	if page, err := client.ListSDKEventNames(ctx, testCampaignSetID); err != nil || len(page.Results) != 1 {
		t.Fatalf("SDK event names=%#v err=%v", page, err)
	}

	assertBidWorkflows(t, ctx, client)
	if len(seen) != 42 {
		t.Fatalf("covered %d unique operations, want 42: %v", len(seen), seen)
	}
}

func assertBidWorkflows(t *testing.T, ctx context.Context, client *Client) {
	t.Helper()
	if _, err := client.ListCPIBids(ctx, testCampaignSetID, testCampaignID); err != nil {
		t.Fatal(err)
	}
	if _, err := client.ReplaceCPIBids(ctx, testCampaignSetID, testCampaignID, []CPIBid{{Country: "US", Bid: "1.250"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := client.PatchCPIBids(ctx, testCampaignSetID, testCampaignID, []CPIBidPatch{{Country: "US", Bid: nil}}); err != nil {
		t.Fatal(err)
	}
	if _, err := client.ListSourceBids(ctx, testCampaignSetID, testCampaignID); err != nil {
		t.Fatal(err)
	}
	if _, err := client.ReplaceSourceBids(ctx, testCampaignSetID, testCampaignID, []SourceBid{{Country: "US", SourceAppID: testSourceAppID, Bid: "1.250"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := client.PatchSourceBids(ctx, testCampaignSetID, testCampaignID, []SourceBidPatch{{Country: "US", SourceAppID: testSourceAppID, Bid: nil}}); err != nil {
		t.Fatal(err)
	}
	if _, err := client.ListROASBids(ctx, testCampaignSetID, testCampaignID); err != nil {
		t.Fatal(err)
	}
	if _, err := client.ReplaceROASBids(ctx, testCampaignSetID, testCampaignID, []ROASBidReplace{{Country: "US", Goal: "25.50", MaxBid: bidPointer("2.000")}}); err != nil {
		t.Fatal(err)
	}
	if _, err := client.PatchROASBids(ctx, testCampaignSetID, testCampaignID, []ROASBidPatch{{Country: "US", Goal: nil, MaxBid: bidPointer("2.000")}}); err != nil {
		t.Fatal(err)
	}
	if _, err := client.ListRetentionBids(ctx, testCampaignSetID, testCampaignID); err != nil {
		t.Fatal(err)
	}
	if _, err := client.ReplaceRetentionBids(ctx, testCampaignSetID, testCampaignID, []RetentionBid{{Country: "US", BaseBid: "1.000", MaxBid: "2.000"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := client.PatchRetentionBids(ctx, testCampaignSetID, testCampaignID, []RetentionBidPatch{{Country: "US", BaseBid: nil, MaxBid: "2.000"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := client.ListEventOptimizationInfo(ctx, testCampaignSetID); err != nil {
		t.Fatal(err)
	}
	if _, err := client.ListEventOptimizationBids(ctx, testCampaignSetID, testCampaignID); err != nil {
		t.Fatal(err)
	}
	if _, err := client.ReplaceEventOptimizationBids(ctx, testCampaignSetID, testCampaignID, []EventOptimizationBid{{Country: "US", Bid: "1.250"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := client.PatchEventOptimizationBids(ctx, testCampaignSetID, testCampaignID, []EventOptimizationBidPatch{{Country: "US", Bid: nil}}); err != nil {
		t.Fatal(err)
	}
}
