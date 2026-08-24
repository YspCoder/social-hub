package criteo

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"social-hub/pkg/socialhub"
)

func TestAdvertiserCampaignAdSetAndStatisticsWorkflows(t *testing.T) {
	adSetName := "Prospecting"
	activation, delivery := ActivationOff, DeliveryDraft
	var campaignPatchCalls, adSetPatchCalls, startCalls, stopCalls int
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		assertAPIRequest(t, request)
		switch request.Method + " " + request.URL.Path {
		case "GET " + advertisersMePath:
			writeJSON(t, writer, http.StatusOK, successEnvelope([]any{map[string]any{
				"type": "advertiser", "id": testAdvertiserID,
				"attributes": map[string]any{"advertiserName": "Example Advertiser"},
			}}))
		case "GET " + campaignsPath + "/" + testCampaignID:
			writeJSON(t, writer, http.StatusOK, successEnvelope(campaignResource(testCampaignID, testAdvertiserID, "Acquisition")))
		case "POST " + campaignsPath + "/search":
			var payload campaignSearchEnvelope
			decodeJSONBody(t, request, &payload)
			if len(payload.Filters.AdvertiserIDs) != 1 || payload.Filters.AdvertiserIDs[0] != testAdvertiserID ||
				len(payload.Filters.CampaignIDs) != 1 || payload.Filters.CampaignIDs[0] != testCampaignID {
				t.Errorf("campaign filters=%#v", payload.Filters)
			}
			writeJSON(t, writer, http.StatusOK, successEnvelope([]any{campaignResource(testCampaignID, testAdvertiserID, "Acquisition")}))
		case "POST " + campaignsPath:
			var payload map[string]any
			decodeJSONBody(t, request, &payload)
			data := payload["data"].(map[string]any)
			attributes := data["attributes"].(map[string]any)
			spend := attributes["spendLimit"].(map[string]any)
			if data["type"] != "Campaign" || attributes["advertiserId"] != testAdvertiserID ||
				attributes["goal"] != "Acquisition" || spend["spendLimitType"] != "capped" || spend["spendLimitRenewal"] != "monthly" {
				t.Errorf("campaign create payload=%v", payload)
			}
			writeJSON(t, writer, http.StatusCreated, successEnvelope(campaignResource("2002", testAdvertiserID, "New Acquisition")))
		case "PATCH " + campaignsPath:
			campaignPatchCalls++
			var payload map[string]any
			decodeJSONBody(t, request, &payload)
			data := payload["data"].([]any)
			resource := data[0].(map[string]any)
			attributes := resource["attributes"].(map[string]any)
			if resource["type"] != "Campaign" || resource["id"] != testCampaignID || attributes["name"] != nil || attributes["goal"] != nil {
				t.Errorf("campaign patch payload=%v", payload)
			}
			writeJSON(t, writer, http.StatusOK, successEnvelope([]any{map[string]any{"type": "Campaign", "id": testCampaignID}}))
		case "GET " + adSetsPath + "/" + testAdSetID:
			writeJSON(t, writer, http.StatusOK, successEnvelope(adSetResource(testAdSetID, testAdvertiserID, testCampaignID, adSetName, activation, delivery)))
		case "POST " + adSetsPath + "/search":
			var payload adSetSearchEnvelope
			decodeJSONBody(t, request, &payload)
			if len(payload.Filters.AdvertiserIDs) != 1 || payload.Filters.AdvertiserIDs[0] != testAdvertiserID ||
				len(payload.Filters.AdSetIDs) != 1 || payload.Filters.AdSetIDs[0] != testAdSetID {
				t.Errorf("Ad Set filters=%#v", payload.Filters)
			}
			writeJSON(t, writer, http.StatusOK, successEnvelope([]any{adSetResource(testAdSetID, testAdvertiserID, testCampaignID, adSetName, activation, delivery)}))
		case "POST " + adSetsPath:
			var payload map[string]any
			decodeJSONBody(t, request, &payload)
			data := payload["data"].(map[string]any)
			attributes := data["attributes"].(map[string]any)
			if data["type"] != "AdSet" || attributes["campaignId"] != testCampaignID || attributes["datasetId"] != testDatasetID ||
				attributes["trackingCode"] != "utm_source=criteo" || attributes["budget"] == nil || attributes["targeting"] == nil {
				t.Errorf("Ad Set create payload=%v", payload)
			}
			writeJSON(t, writer, http.StatusCreated, successEnvelope(adSetResource("3002", testAdvertiserID, testCampaignID, "New Ad Set", ActivationOff, DeliveryDraft)))
		case "PATCH " + adSetsPath:
			adSetPatchCalls++
			var payload map[string]any
			decodeJSONBody(t, request, &payload)
			data := payload["data"].([]any)
			resource := data[0].(map[string]any)
			attributes := resource["attributes"].(map[string]any)
			if resource["type"] != "PatchAdSetV24Q3" || resource["id"] != testAdSetID || attributes["name"] != "Renamed" {
				t.Errorf("Ad Set patch payload=%v", payload)
			}
			adSetName = "Renamed"
			writeJSON(t, writer, http.StatusOK, successEnvelope([]any{map[string]any{"type": "AdSetIdV24Q3", "id": testAdSetID, "attributes": map[string]any{}}}))
		case "POST " + adSetsPath + "/start":
			startCalls++
			assertAdSetIDBody(t, request, testAdSetID)
			activation, delivery = ActivationOn, DeliveryLive
			writeJSON(t, writer, http.StatusOK, successEnvelope([]any{map[string]any{"type": "AdSetId", "id": testAdSetID}}))
		case "POST " + adSetsPath + "/stop":
			stopCalls++
			assertAdSetIDBody(t, request, testAdSetID)
			activation, delivery = ActivationOff, DeliveryPaused
			writeJSON(t, writer, http.StatusOK, successEnvelope([]any{map[string]any{"type": "AdSetId", "id": testAdSetID}}))
		case "POST " + statisticsReportPath:
			var payload statisticsReportWire
			decodeJSONBody(t, request, &payload)
			if payload.AdvertiserIDs != testAdvertiserID || payload.Format != "json" || payload.Timezone != "UTC" ||
				len(payload.Dimensions) != 1 || payload.Dimensions[0] != DimensionCampaignID || len(payload.Metrics) != 2 {
				t.Errorf("report payload=%#v", payload)
			}
			writeJSON(t, writer, http.StatusOK, []map[string]any{{"CampaignId": testCampaignID, "Clicks": 12, "AdvertiserCost": 4.5}})
		default:
			t.Errorf("unexpected request: %s %s", request.Method, request.URL.Path)
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	_, client := newStaticClient(t, server)
	ctx := context.Background()

	advertisers, err := client.ListAdvertisers(ctx)
	if err != nil || len(advertisers) != 1 || advertisers[0].Name != "Example Advertiser" {
		t.Fatalf("advertisers=%#v err=%v", advertisers, err)
	}
	if advertiser, err := client.ValidateConfiguredAdvertiser(ctx); err != nil || advertiser.ID != testAdvertiserID {
		t.Fatalf("advertiser=%#v err=%v", advertiser, err)
	}
	campaigns, err := client.SearchCampaigns(ctx, CampaignSearchRequest{CampaignIDs: []string{testCampaignID}})
	if err != nil || len(campaigns) != 1 || campaigns[0].BudgetAutomation == nil {
		t.Fatalf("campaigns=%#v err=%v", campaigns, err)
	}
	createdCampaign, err := client.CreateCampaign(ctx, CreateCampaignRequest{
		Name: "New Acquisition", Goal: GoalAcquisition,
		SpendLimit:       CreateCampaignSpendLimit{Type: SpendLimitCapped, Amount: floatPointer(100), Renewal: RenewalMonthly},
		BudgetAutomation: &CreateBudgetAutomation{Enabled: true, Objective: AutomationConversions},
	})
	if err != nil || createdCampaign.ID != "2002" {
		t.Fatalf("created Campaign=%#v err=%v", createdCampaign, err)
	}
	amount := 125.0
	updatedCampaign, err := client.UpdateCampaign(ctx, testCampaignID, UpdateCampaignRequest{
		SpendLimit: &PatchCampaignSpendLimit{Amount: &NullableFloat{Value: &amount}},
	})
	if err != nil || updatedCampaign.ID != testCampaignID || campaignPatchCalls != 1 {
		t.Fatalf("updated Campaign=%#v calls=%d err=%v", updatedCampaign, campaignPatchCalls, err)
	}
	adSets, err := client.SearchAdSets(ctx, AdSetSearchRequest{AdSetIDs: []string{testAdSetID}})
	if err != nil || len(adSets) != 1 || adSets[0].Schedule.ActivationStatus != ActivationOff {
		t.Fatalf("Ad Sets=%#v err=%v", adSets, err)
	}
	createdAdSet, err := client.CreateAdSet(ctx, testCampaignID, validCreateAdSetRequest("New Ad Set"))
	if err != nil || createdAdSet.ID != "3002" || createdAdSet.Schedule.DeliveryStatus != DeliveryDraft {
		t.Fatalf("created Ad Set=%#v err=%v", createdAdSet, err)
	}
	updatedAdSet, err := client.UpdateAdSet(ctx, testAdSetID, UpdateAdSetRequest{Name: stringPointer("Renamed")})
	if err != nil || updatedAdSet.Name != "Renamed" || adSetPatchCalls != 1 {
		t.Fatalf("updated Ad Set=%#v calls=%d err=%v", updatedAdSet, adSetPatchCalls, err)
	}
	started, err := client.StartAdSet(ctx, testAdSetID)
	if err != nil || started.Schedule.ActivationStatus != ActivationOn || startCalls != 1 {
		t.Fatalf("started=%#v calls=%d err=%v", started, startCalls, err)
	}
	startedAgain, err := client.StartAdSet(ctx, testAdSetID)
	if err != nil || startedAgain.Schedule.ActivationStatus != ActivationOn || startCalls != 1 {
		t.Fatalf("idempotent start=%#v calls=%d err=%v", startedAgain, startCalls, err)
	}
	stopped, err := client.StopAdSet(ctx, testAdSetID)
	if err != nil || stopped.Schedule.ActivationStatus != ActivationOff || stopCalls != 1 {
		t.Fatalf("stopped=%#v calls=%d err=%v", stopped, stopCalls, err)
	}
	stoppedAgain, err := client.StopAdSet(ctx, testAdSetID)
	if err != nil || stoppedAgain.Schedule.ActivationStatus != ActivationOff || stopCalls != 1 {
		t.Fatalf("idempotent stop=%#v calls=%d err=%v", stoppedAgain, stopCalls, err)
	}
	report, err := client.Report(ctx, StatisticsReportRequest{
		Currency: "USD", Dimensions: []Dimension{DimensionCampaignID}, Metrics: []Metric{MetricClicks, MetricAdvertiserCost},
		StartDate: "2026-08-01", EndDate: "2026-08-09",
	})
	if err != nil || report.ContentType != "application/json" || !json.Valid(report.Data) || !strings.Contains(string(report.Data), `"Clicks":12`) {
		t.Fatalf("report=%#v err=%v", report, err)
	}
}

func TestUncappedCampaignWireOmitsAmountAndRenewal(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		var payload map[string]any
		decodeJSONBody(t, request, &payload)
		attributes := payload["data"].(map[string]any)["attributes"].(map[string]any)
		spend := attributes["spendLimit"].(map[string]any)
		if spend["spendLimitType"] != "uncapped" || spend["spendLimitAmount"] != nil || spend["spendLimitRenewal"] != nil {
			t.Errorf("spendLimit=%v", spend)
		}
		resource := campaignResource("2002", testAdvertiserID, "Uncapped")
		resource["attributes"].(map[string]any)["spendLimit"] = map[string]any{
			"spendLimitType": "uncapped", "spendLimitAmount": map[string]any{"value": nil}, "spendLimitRenewal": "undefined",
		}
		writeJSON(t, writer, http.StatusCreated, successEnvelope(resource))
	}))
	defer server.Close()
	_, client := newStaticClient(t, server)
	_, err := client.CreateCampaign(context.Background(), CreateCampaignRequest{
		Name: "Uncapped", Goal: GoalAcquisition, SpendLimit: CreateCampaignSpendLimit{Type: SpendLimitUncapped},
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestWorkflowContractAndOwnershipErrors(t *testing.T) {
	t.Run("partial error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
			writeJSON(t, writer, http.StatusOK, map[string]any{
				"data":   []any{campaignResource(testCampaignID, testAdvertiserID, "Campaign")},
				"errors": []Problem{{Type: "validation", Code: "invalid", Detail: "one item failed"}},
			})
		}))
		defer server.Close()
		_, client := newStaticClient(t, server)
		_, err := client.SearchCampaigns(context.Background(), CampaignSearchRequest{})
		if !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Fatalf("error=%v", err)
		}
		var api *APIError
		if !errors.As(err, &api) || len(api.Problems) != 1 || api.Problems[0].Code != "invalid" {
			t.Fatalf("API error=%#v", api)
		}
	})

	t.Run("campaign ownership", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
			writeJSON(t, writer, http.StatusOK, successEnvelope(campaignResource(testCampaignID, "99999", "Other")))
		}))
		defer server.Close()
		_, client := newStaticClient(t, server)
		_, err := client.GetCampaign(context.Background(), testCampaignID)
		if !errors.Is(err, socialhub.ErrPermissionDenied) {
			t.Fatalf("error=%v", err)
		}
	})

	t.Run("new Ad Set active", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			if request.Method == http.MethodGet {
				writeJSON(t, writer, http.StatusOK, successEnvelope(campaignResource(testCampaignID, testAdvertiserID, "Campaign")))
				return
			}
			writeJSON(t, writer, http.StatusCreated, successEnvelope(adSetResource("3002", testAdvertiserID, testCampaignID, "Unsafe", ActivationOn, DeliveryLive)))
		}))
		defer server.Close()
		_, client := newStaticClient(t, server)
		_, err := client.CreateAdSet(context.Background(), testCampaignID, validCreateAdSetRequest("Unsafe"))
		if requireHubError(t, err).Code != socialhub.CodePlatformError {
			t.Fatalf("error=%v", err)
		}
	})

	t.Run("archived cannot start", func(t *testing.T) {
		var posts int
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			if request.Method == http.MethodPost {
				posts++
			}
			writeJSON(t, writer, http.StatusOK, successEnvelope(adSetResource(testAdSetID, testAdvertiserID, testCampaignID, "Archived", ActivationOff, DeliveryArchived)))
		}))
		defer server.Close()
		_, client := newStaticClient(t, server)
		_, err := client.StartAdSet(context.Background(), testAdSetID)
		if !errors.Is(err, socialhub.ErrConflict) || posts != 0 {
			t.Fatalf("posts=%d error=%v", posts, err)
		}
	})

	t.Run("mutation result mismatch", func(t *testing.T) {
		var gets int
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			if request.Method == http.MethodGet {
				gets++
				writeJSON(t, writer, http.StatusOK, successEnvelope(adSetResource(testAdSetID, testAdvertiserID, testCampaignID, "Ad Set", ActivationOff, DeliveryDraft)))
				return
			}
			writeJSON(t, writer, http.StatusOK, successEnvelope([]any{map[string]any{"type": "AdSetId", "id": "9999"}}))
		}))
		defer server.Close()
		_, client := newStaticClient(t, server)
		_, err := client.StartAdSet(context.Background(), testAdSetID)
		if requireHubError(t, err).Code != socialhub.CodePlatformError || gets != 1 {
			t.Fatalf("gets=%d error=%v", gets, err)
		}
	})

	t.Run("non JSON report", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
			writer.Header().Set("Content-Type", "text/csv")
			_, _ = writer.Write([]byte(`{"Clicks":1}`))
		}))
		defer server.Close()
		_, client := newStaticClient(t, server)
		_, err := client.Report(context.Background(), validReportRequest())
		if requireHubError(t, err).Code != socialhub.CodePlatformError {
			t.Fatalf("error=%v", err)
		}
	})
}

func TestWorkflowValidation(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	_, client := newStaticClient(t, server)
	calls := []func() error{
		func() error { _, err := client.GetCampaign(context.Background(), "../1"); return err },
		func() error {
			_, err := client.SearchCampaigns(context.Background(), CampaignSearchRequest{CampaignIDs: []string{"1", "1"}})
			return err
		},
		func() error {
			_, err := client.CreateCampaign(context.Background(), CreateCampaignRequest{})
			return err
		},
		func() error {
			_, err := client.UpdateCampaign(context.Background(), testCampaignID, UpdateCampaignRequest{})
			return err
		},
		func() error { _, err := client.GetAdSet(context.Background(), "bad"); return err },
		func() error {
			_, err := client.SearchAdSets(context.Background(), AdSetSearchRequest{AdSetIDs: []string{"1", "1"}})
			return err
		},
		func() error {
			_, err := client.CreateAdSet(context.Background(), testCampaignID, CreateAdSetRequest{})
			return err
		},
		func() error {
			_, err := client.UpdateAdSet(context.Background(), testAdSetID, UpdateAdSetRequest{})
			return err
		},
		func() error {
			_, err := client.Report(context.Background(), StatisticsReportRequest{Currency: "usd"})
			return err
		},
	}
	for index, call := range calls {
		if err := call(); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Errorf("call %d error=%v", index, err)
		}
	}
}

func assertAdSetIDBody(t *testing.T, request *http.Request, id string) {
	t.Helper()
	var payload map[string]any
	decodeJSONBody(t, request, &payload)
	data := payload["data"].([]any)
	resource := data[0].(map[string]any)
	if resource["type"] != "AdSetId" || resource["id"] != id || resource["attributes"] != nil || len(resource) != 2 {
		t.Errorf("activation payload=%v", payload)
	}
}

func validCreateAdSetRequest(name string) CreateAdSetRequest {
	return CreateAdSetRequest{
		Name: name, DatasetID: testDatasetID, Objective: ObjectiveVisits, MediaType: MediaDisplay,
		Schedule: CreateAdSetSchedule{StartDate: "2026-08-10T00:00:00Z"},
		Bidding:  CreateAdSetBidding{CostController: CostMaxCPC, BidAmount: floatPointer(1.25)},
		Budget: CreateAdSetBudget{
			Strategy: BudgetCapped, Amount: floatPointer(50), Renewal: BudgetDaily, DeliverySmoothing: DeliveryStandard,
		},
		Targeting: CreateAdSetTargeting{
			DeliveryLimitations: &DeliveryLimitations{Devices: []Device{DeviceDesktop}},
			FrequencyCapping:    FrequencyCapping{Frequency: FrequencyDaily, MaximumImpressions: 3},
		},
		TrackingCode: "utm_source=criteo",
	}
}

func validReportRequest() StatisticsReportRequest {
	return StatisticsReportRequest{
		Currency: "USD", Dimensions: []Dimension{DimensionCampaignID}, Metrics: []Metric{MetricClicks},
		StartDate: "2026-08-01", EndDate: "2026-08-09",
	}
}
