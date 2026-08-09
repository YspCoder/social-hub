package outbrain

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"social-hub/pkg/socialhub"
)

func TestTypedWorkflowsAndWireContracts(t *testing.T) {
	var campaignEnabled atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		assertAPIRequest(t, request)
		switch request.Method + " " + request.URL.Path {
		case "GET /marketers":
			if request.URL.Query().Get("extraFields") != "Account" {
				t.Fatalf("marketer query=%v", request.URL.Query())
			}
			writeJSON(t, writer, http.StatusOK, map[string]any{"marketers": []Marketer{marketerFixture()}, "count": 1})
		case "GET /marketers/" + testMarketerID:
			writeJSON(t, writer, http.StatusOK, marketerFixture())
		case "GET /marketers/" + testMarketerID + "/budgets":
			writeJSON(t, writer, http.StatusOK, map[string]any{"budgets": []Budget{budgetFixture()}, "count": 1})
		case "GET /budgets/" + testBudgetID:
			writeJSON(t, writer, http.StatusOK, budgetFixture())
		case "POST /marketers/" + testMarketerID + "/budgets":
			var payload CreateBudgetRequest
			decodeJSONBody(t, request, &payload)
			if payload.Name != "New Budget" || payload.Amount != 75 || payload.Type != BudgetCampaign || payload.Pacing != PacingSpendASAP {
				t.Fatalf("create Budget payload=%#v", payload)
			}
			value := budgetFixture()
			value.Name, value.Amount = payload.Name, payload.Amount
			writeJSON(t, writer, http.StatusOK, value)
		case "PUT /budgets/" + testBudgetID:
			var payload UpdateBudgetRequest
			decodeJSONBody(t, request, &payload)
			value := budgetFixture()
			if payload.Amount != nil {
				value.Amount = *payload.Amount
			}
			writeJSON(t, writer, http.StatusOK, value)
		case "GET /marketers/" + testMarketerID + "/campaigns":
			query := request.URL.Query()
			if query.Get("fetch") != "all" || query.Get("limit") != "10" || query.Get("offset") != "3" || query.Get("includeArchived") != "true" ||
				query.Get("fromBudgetStartDate") != "2026-08-01" || query.Get("toBudgetEndDate") != "2026-08-31" || query.Get("daysToLookBackForChanges") != "5" {
				t.Fatalf("Campaign query=%v", query)
			}
			writeJSON(t, writer, http.StatusOK, map[string]any{"campaigns": []Campaign{campaignFixture(campaignEnabled.Load())}, "count": 1})
		case "GET /campaigns/" + testCampaignID:
			writeJSON(t, writer, http.StatusOK, campaignFixture(campaignEnabled.Load()))
		case "POST /campaigns":
			var payload map[string]any
			decodeJSONBody(t, request, &payload)
			if payload["enabled"] != false || payload["name"] != "Created Campaign" || payload["budgetId"] != testBudgetID {
				t.Fatalf("create Campaign payload=%#v", payload)
			}
			value := campaignFixture(false)
			value.Name = payload["name"].(string)
			writeJSON(t, writer, http.StatusOK, value)
		case "PUT /campaigns/" + testCampaignID:
			var payload map[string]any
			decodeJSONBody(t, request, &payload)
			value := campaignFixture(campaignEnabled.Load())
			if name, found := payload["name"]; found {
				value.Name = name.(string)
			}
			if enabled, found := payload["enabled"]; found {
				campaignEnabled.Store(enabled.(bool))
				value.Enabled = enabled.(bool)
			}
			writeJSON(t, writer, http.StatusOK, value)
		case "GET /campaigns/" + testCampaignID + "/promotedLinks":
			query := request.URL.Query()
			if query.Get("statuses") != "" && query.Get("statuses") != "APPROVED,PENDING" {
				t.Fatalf("PromotedLink statuses=%v", query)
			}
			writeJSON(t, writer, http.StatusOK, map[string]any{"promotedLinks": []PromotedLink{promotedLinkFixture(false, true)}, "count": 1, "totalCount": 1})
		case "GET /promotedLinks/" + testPromotedLinkID:
			writeJSON(t, writer, http.StatusOK, promotedLinkFixture(false, true))
		case "POST /campaigns/" + testCampaignID + "/promotedLinks":
			var payload map[string]any
			decodeJSONBody(t, request, &payload)
			if payload["enabled"] != false || payload["text"] != "Created headline" || payload["url"] != "https://example.test/new" {
				t.Fatalf("create PromotedLink payload=%#v", payload)
			}
			value := promotedLinkFixture(false, false)
			value.Text, value.URL = payload["text"].(string), payload["url"].(string)
			writeJSON(t, writer, http.StatusOK, value)
		case "PUT /promotedLinks/" + testPromotedLinkID:
			var payload map[string]any
			decodeJSONBody(t, request, &payload)
			if payload["enabled"] != true {
				t.Fatalf("status payload=%#v", payload)
			}
			writer.WriteHeader(http.StatusOK)
		case "PUT /campaigns/" + testCampaignID + "/promotedLinks":
			var payload []PromotedLinkCPCUpdate
			decodeJSONBody(t, request, &payload)
			if len(payload) != 1 || payload[0].ID != testPromotedLinkID || payload[0].CPC != 0.7 {
				t.Fatalf("CPC payload=%#v", payload)
			}
			value := promotedLinkFixture(false, true)
			value.CPC = payload[0].CPC
			writeJSON(t, writer, http.StatusOK, []PromotedLinkUpdateResult{{OperationStatus: OperationStatus{Status: "Success"}, ID: testPromotedLinkID, PromotedLink: value}})
		case "GET /reports/marketers/" + testMarketerID + "/campaigns":
			query := request.URL.Query()
			if query.Get("from") != "2026-08-01" || query.Get("to") != "2026-08-09" || query.Get("limit") != "10" || query.Get("offset") != "3" ||
				query.Get("sort") != "-ctr" || query.Get("campaignId") != testCampaignID || query.Get("includeViewedImpressions") != "true" || query.Get("timezone") != "GMT+12:00" {
				t.Fatalf("Campaign report query=%v", query)
			}
			writeJSON(t, writer, http.StatusOK, map[string]any{
				"results":      []any{map[string]any{"metadata": map[string]any{"id": testCampaignID, "name": "Campaign", "budget": budgetFixture()}, "metrics": map[string]any{"clicks": 4, "unknown": 7}}},
				"totalResults": 1, "summary": map[string]any{"clicks": 4},
			})
		case "GET /reports/marketers/" + testMarketerID + "/promotedContent":
			query := request.URL.Query()
			if query.Get("promotedLinkId") != testPromotedLinkID || query.Get("campaignId") != testCampaignID || query.Get("sort") != "creationTime" {
				t.Fatalf("Promoted Content query=%v", query)
			}
			writeJSON(t, writer, http.StatusOK, map[string]any{
				"results":      []any{map[string]any{"metadata": map[string]any{"id": testPromotedLinkID, "campaignId": testCampaignID, "campaignName": "Campaign", "title": "Headline"}, "metrics": map[string]any{"impressions": 100}}},
				"totalResults": 1, "summary": map[string]any{"impressions": 100},
			})
		case "GET /reports/marketers/" + testMarketerID + "/campaigns/periodic":
			query := request.URL.Query()
			if query.Get("breakdown") != "dayOfWeek" || query.Get("campaignId") != testCampaignID || query.Get("includeConversionDetails") != "true" {
				t.Fatalf("periodic Campaign query=%v", query)
			}
			writeJSON(t, writer, http.StatusOK, map[string]any{
				"campaignResults": []any{map[string]any{"campaignId": testCampaignID, "results": []any{periodicWireRow()}, "totalResults": 1}}, "totalCampaigns": 1,
			})
		case "GET /reports/marketers/" + testMarketerID + "/campaigns/" + testCampaignID + "/periodicContent":
			query := request.URL.Query()
			if query.Get("breakdown") != "daily" || query.Get("enabledCampaignsOnly") != "true" {
				t.Fatalf("periodic content query=%v", query)
			}
			writeJSON(t, writer, http.StatusOK, map[string]any{
				"promotedLinkResults": []any{map[string]any{"promotedLinkId": testPromotedLinkID, "results": []any{periodicWireRow()}, "totalResults": 1}}, "totalPromotedLinks": 1,
			})
		default:
			t.Fatalf("unexpected request: %s %s", request.Method, request.URL)
		}
	}))
	defer server.Close()
	_, client := newTestAdapter(t, server)
	ctx := context.Background()

	if marketers, err := client.ListMarketers(ctx); err != nil || len(marketers) != 1 || marketers[0].ID != testMarketerID {
		t.Fatalf("marketers=%#v err=%v", marketers, err)
	}
	if marketer, err := client.GetMarketer(ctx); err != nil || marketer.ID != testMarketerID {
		t.Fatalf("marketer=%#v err=%v", marketer, err)
	}
	if marketer, err := client.ValidateConfiguredMarketer(ctx); err != nil || marketer.ID != testMarketerID {
		t.Fatalf("validated marketer=%#v err=%v", marketer, err)
	}
	if budgets, err := client.ListBudgets(ctx, false); err != nil || len(budgets) != 1 {
		t.Fatalf("budgets=%#v err=%v", budgets, err)
	}
	if budget, err := client.GetBudget(ctx, testBudgetID); err != nil || budget.ID != testBudgetID {
		t.Fatalf("budget=%#v err=%v", budget, err)
	}
	createdBudget, err := client.CreateBudget(ctx, CreateBudgetRequest{Name: "New Budget", Amount: 75, StartDate: "2026-08-01", EndDate: "2026-08-03", Pacing: PacingSpendASAP, Type: BudgetCampaign})
	if err != nil || createdBudget.Name != "New Budget" {
		t.Fatalf("created Budget=%#v err=%v", createdBudget, err)
	}
	if updated, err := client.UpdateBudget(ctx, testBudgetID, UpdateBudgetRequest{Amount: floatPointer(125)}); err != nil || updated.Amount != 125 {
		t.Fatalf("updated Budget=%#v err=%v", updated, err)
	}
	page, err := client.ListCampaigns(ctx, ListCampaignsRequest{IncludeArchived: true, FromBudgetStartDate: "2026-08-01", ToBudgetEndDate: "2026-08-31", Limit: 10, Offset: 3, DaysToLookBack: 5})
	if err != nil || page.Count != 1 || len(page.Items) != 1 {
		t.Fatalf("Campaign page=%#v err=%v", page, err)
	}
	if campaign, err := client.GetCampaign(ctx, testCampaignID); err != nil || campaign.ID != testCampaignID {
		t.Fatalf("Campaign=%#v err=%v", campaign, err)
	}
	createdCampaign, err := client.CreateCampaign(ctx, CreateCampaignRequest{Name: "Created Campaign", CPC: 0.25, BudgetID: testBudgetID, Targeting: CampaignTargeting{Platform: []string{"DESKTOP"}}})
	if err != nil || createdCampaign.Enabled || createdCampaign.Name != "Created Campaign" {
		t.Fatalf("created Campaign=%#v err=%v", createdCampaign, err)
	}
	if updated, err := client.UpdateCampaign(ctx, testCampaignID, UpdateCampaignRequest{Name: stringPointer("Updated Campaign")}); err != nil || updated.Name != "Updated Campaign" {
		t.Fatalf("updated Campaign=%#v err=%v", updated, err)
	}
	links, err := client.ListPromotedLinks(ctx, testCampaignID, ListPromotedLinksRequest{Enabled: boolPointer(false), Statuses: []string{"APPROVED", "PENDING"}, Limit: 500, Sort: "-creationTime", ImageWidth: 100, ImageHeight: 100})
	if err != nil || links.TotalCount != 1 || len(links.Items) != 1 {
		t.Fatalf("PromotedLinks=%#v err=%v", links, err)
	}
	if link, err := client.GetPromotedLink(ctx, testCampaignID, testPromotedLinkID); err != nil || link.ID != testPromotedLinkID {
		t.Fatalf("PromotedLink=%#v err=%v", link, err)
	}
	createdLink, err := client.CreatePromotedLink(ctx, testCampaignID, CreatePromotedLinkRequest{Text: "Created headline", URL: "https://example.test/new", ImageURL: "https://example.test/new.jpg"})
	if err != nil || createdLink.Enabled || createdLink.Text != "Created headline" {
		t.Fatalf("created PromotedLink=%#v err=%v", createdLink, err)
	}
	results, err := client.UpdatePromotedLinkCPCs(ctx, testCampaignID, []PromotedLinkCPCUpdate{{ID: testPromotedLinkID, CPC: 0.7}})
	if err != nil || len(results) != 1 || results[0].PromotedLink.CPC != 0.7 {
		t.Fatalf("CPC results=%#v err=%v", results, err)
	}
	if enabled, err := client.SetCampaignEnabled(ctx, testCampaignID, true); err != nil || !enabled.Enabled {
		t.Fatalf("enabled Campaign=%#v err=%v", enabled, err)
	}
	if err := client.SetPromotedLinkEnabled(ctx, testCampaignID, testPromotedLinkID, true); err != nil {
		t.Fatalf("enable PromotedLink error=%v", err)
	}

	campaignReport, err := client.CampaignPerformance(ctx, CampaignReportRequest{From: "2026-08-01", To: "2026-08-09", Limit: 10, Offset: 3, Sort: "-ctr", CampaignIDs: []string{testCampaignID}, IncludeViewedImpressions: true, Timezone: "GMT+12:00"})
	if err != nil || len(campaignReport.Results) != 1 || campaignReport.Results[0].Metrics.Clicks != 4 || !strings.Contains(string(campaignReport.Results[0].Metrics.Raw), "unknown") {
		t.Fatalf("Campaign report=%#v err=%v", campaignReport, err)
	}
	contentReport, err := client.PromotedContentPerformance(ctx, PromotedContentReportRequest{From: "2026-08-01", To: "2026-08-09", Sort: "creationTime", CampaignIDs: []string{testCampaignID}, PromotedLinkID: testPromotedLinkID})
	if err != nil || len(contentReport.Results) != 1 || contentReport.Results[0].Metrics.Impressions != 100 {
		t.Fatalf("Content report=%#v err=%v", contentReport, err)
	}
	periodic, err := client.CampaignPeriodicPerformance(ctx, CampaignPeriodicReportRequest{From: "2026-08-01", To: "2026-08-09", CampaignIDs: []string{testCampaignID}, Breakdown: "dayOfWeek", IncludeConversionDetails: true})
	if err != nil || len(periodic.CampaignResults) != 1 || periodic.CampaignResults[0].Results[0].Metrics.Clicks != 2 {
		t.Fatalf("periodic Campaign report=%#v err=%v", periodic, err)
	}
	contentPeriodic, err := client.PromotedContentPeriodicPerformance(ctx, PromotedContentPeriodicReportRequest{CampaignID: testCampaignID, From: "2026-08-01", To: "2026-08-09", Breakdown: "daily", EnabledCampaignsOnly: true})
	if err != nil || len(contentPeriodic.PromotedLinkResults) != 1 || contentPeriodic.PromotedLinkResults[0].Results[0].Metrics.Clicks != 2 {
		t.Fatalf("periodic Content report=%#v err=%v", contentPeriodic, err)
	}
}

func TestPausedFirstSafetyContracts(t *testing.T) {
	t.Run("Campaign create rejects enabled response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			assertAPIRequest(t, request)
			if request.Method == http.MethodGet {
				writeJSON(t, writer, http.StatusOK, map[string]any{"budgets": []Budget{budgetFixture()}, "count": 1})
				return
			}
			writeJSON(t, writer, http.StatusOK, campaignFixture(true))
		}))
		defer server.Close()
		_, client := newTestAdapter(t, server)
		_, err := client.CreateCampaign(context.Background(), validCreateCampaignRequest())
		if hubError(t, err).Code != socialhub.CodePlatformError {
			t.Fatalf("error=%v", err)
		}
	})

	t.Run("PromotedLink create requires disabled Campaign", func(t *testing.T) {
		var writes atomic.Int32
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			if request.Method != http.MethodGet {
				writes.Add(1)
				t.Fatal("unsafe write reached network")
			}
			writeJSON(t, writer, http.StatusOK, campaignFixture(true))
		}))
		defer server.Close()
		_, client := newTestAdapter(t, server)
		_, err := client.CreatePromotedLink(context.Background(), testCampaignID, validCreatePromotedLinkRequest())
		if !errors.Is(err, socialhub.ErrInvalidArgument) || writes.Load() != 0 {
			t.Fatalf("error=%v writes=%d", err, writes.Load())
		}
	})

	unsafePages := []PromotedLinkPage{
		{},
		{Items: []PromotedLink{promotedLinkFixture(true, true)}, Count: 1, TotalCount: 1},
		{Items: []PromotedLink{promotedLinkFixture(false, false)}, Count: 1, TotalCount: 1},
		func() PromotedLinkPage {
			link := promotedLinkFixture(false, true)
			link.Archived = true
			return PromotedLinkPage{Items: []PromotedLink{link}, Count: 1, TotalCount: 1}
		}(),
		{Items: []PromotedLink{promotedLinkFixture(false, true)}, Count: 1, TotalCount: 501},
	}
	for index, page := range unsafePages {
		page := page
		t.Run("Campaign enable gate "+string(rune('0'+index)), func(t *testing.T) {
			var writes atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				if request.Method != http.MethodGet {
					writes.Add(1)
					t.Fatal("unsafe write reached network")
				}
				if strings.HasSuffix(request.URL.Path, "/promotedLinks") {
					writeJSON(t, writer, http.StatusOK, map[string]any{"promotedLinks": page.Items, "count": page.Count, "totalCount": page.TotalCount})
					return
				}
				writeJSON(t, writer, http.StatusOK, campaignFixture(false))
			}))
			defer server.Close()
			_, client := newTestAdapter(t, server)
			if _, err := client.SetCampaignEnabled(context.Background(), testCampaignID, true); err == nil || writes.Load() != 0 {
				t.Fatalf("error=%v writes=%d", err, writes.Load())
			}
		})
	}

	t.Run("PromotedLink enable requires enabled Campaign and approval", func(t *testing.T) {
		for _, fixture := range []struct {
			campaign Campaign
			link     PromotedLink
		}{
			{campaignFixture(false), promotedLinkFixture(false, true)},
			{campaignFixture(true), promotedLinkFixture(false, false)},
			func() struct {
				campaign Campaign
				link     PromotedLink
			} {
				link := promotedLinkFixture(false, true)
				link.Archived = true
				return struct {
					campaign Campaign
					link     PromotedLink
				}{campaignFixture(true), link}
			}(),
		} {
			fixture := fixture
			var writes atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				if request.Method != http.MethodGet {
					writes.Add(1)
					t.Fatal("unsafe PromotedLink write reached network")
				}
				if strings.HasPrefix(request.URL.Path, "/campaigns/") {
					writeJSON(t, writer, http.StatusOK, fixture.campaign)
					return
				}
				writeJSON(t, writer, http.StatusOK, fixture.link)
			}))
			_, client := newTestAdapter(t, server)
			err := client.SetPromotedLinkEnabled(context.Background(), testCampaignID, testPromotedLinkID, true)
			server.Close()
			if !errors.Is(err, socialhub.ErrInvalidArgument) || writes.Load() != 0 {
				t.Fatalf("error=%v writes=%d", err, writes.Load())
			}
		}
	})

	t.Run("Budget update cannot undercut minimum", func(t *testing.T) {
		var writes atomic.Int32
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			if request.Method != http.MethodGet {
				writes.Add(1)
				t.Fatal("unsafe Budget write reached network")
			}
			writeJSON(t, writer, http.StatusOK, map[string]any{"budgets": []Budget{budgetFixture()}, "count": 1})
		}))
		defer server.Close()
		_, client := newTestAdapter(t, server)
		_, err := client.UpdateBudget(context.Background(), testBudgetID, UpdateBudgetRequest{Amount: floatPointer(50)})
		if !errors.Is(err, socialhub.ErrInvalidArgument) || writes.Load() != 0 {
			t.Fatalf("error=%v writes=%d", err, writes.Load())
		}
	})
}

func TestHTTPErrorKeepsLogicalOperationAndRateDelay(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("AMPLIFY-REQUEST-ID", "request-429")
		writer.Header().Set("rate-limit-msec-left", "1750")
		writeJSON(t, writer, http.StatusTooManyRequests, map[string]string{"moreInfo": "list-marketers", "errorMessage": "slow down"})
	}))
	defer server.Close()
	_, client := newTestAdapter(t, server)
	_, err := client.ListMarketers(context.Background())
	hub := hubError(t, err)
	if hub.Op != "list_marketers" || hub.Code != socialhub.CodeRateLimited || hub.RetryAfter.String() != "1.75s" || hub.RequestID != "request-429" {
		t.Fatalf("hub=%#v", hub)
	}
}

func TestOwnershipChecksStopWrites(t *testing.T) {
	t.Run("Campaign owner", func(t *testing.T) {
		value := campaignFixture(false)
		value.MarketerID = "another-marketer"
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) { writeJSON(t, writer, http.StatusOK, value) }))
		defer server.Close()
		_, client := newTestAdapter(t, server)
		if _, err := client.GetCampaign(context.Background(), testCampaignID); hubError(t, err).Code != socialhub.CodePlatformError {
			t.Fatalf("error=%v", err)
		}
	})

	t.Run("Budget owner", func(t *testing.T) {
		var writes atomic.Int32
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			if request.Method != http.MethodGet {
				writes.Add(1)
			}
			writeJSON(t, writer, http.StatusOK, map[string]any{"budgets": []Budget{}, "count": 0})
		}))
		defer server.Close()
		_, client := newTestAdapter(t, server)
		_, err := client.UpdateBudget(context.Background(), testBudgetID, UpdateBudgetRequest{Amount: floatPointer(100)})
		if !errors.Is(err, socialhub.ErrPermissionDenied) || writes.Load() != 0 {
			t.Fatalf("error=%v writes=%d", err, writes.Load())
		}
	})

	t.Run("PromotedLink owner", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			if strings.HasPrefix(request.URL.Path, "/campaigns/") {
				writeJSON(t, writer, http.StatusOK, campaignFixture(false))
				return
			}
			value := promotedLinkFixture(false, true)
			value.CampaignID = "another-campaign"
			writeJSON(t, writer, http.StatusOK, value)
		}))
		defer server.Close()
		_, client := newTestAdapter(t, server)
		if _, err := client.GetPromotedLink(context.Background(), testCampaignID, testPromotedLinkID); hubError(t, err).Code != socialhub.CodePlatformError {
			t.Fatalf("error=%v", err)
		}
	})
}

func TestLocalValidationAvoidsNetwork(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Fatal("invalid input reached network") }))
	defer server.Close()
	_, client := newTestAdapter(t, server)
	calls := []func() error{
		func() error { _, err := client.GetBudget(context.Background(), "bad/id"); return err },
		func() error { _, err := client.CreateBudget(context.Background(), CreateBudgetRequest{}); return err },
		func() error {
			_, err := client.UpdateBudget(context.Background(), testBudgetID, UpdateBudgetRequest{})
			return err
		},
		func() error {
			_, err := client.ListCampaigns(context.Background(), ListCampaignsRequest{Limit: 51})
			return err
		},
		func() error { _, err := client.GetCampaign(context.Background(), "bad/id"); return err },
		func() error {
			_, err := client.CreateCampaign(context.Background(), CreateCampaignRequest{})
			return err
		},
		func() error {
			_, err := client.UpdateCampaign(context.Background(), testCampaignID, UpdateCampaignRequest{})
			return err
		},
		func() error {
			_, err := client.ListPromotedLinks(context.Background(), "bad/id", ListPromotedLinksRequest{})
			return err
		},
		func() error {
			_, err := client.GetPromotedLink(context.Background(), "bad/id", testPromotedLinkID)
			return err
		},
		func() error {
			_, err := client.CreatePromotedLink(context.Background(), testCampaignID, CreatePromotedLinkRequest{})
			return err
		},
		func() error {
			_, err := client.UpdatePromotedLinkCPCs(context.Background(), testCampaignID, nil)
			return err
		},
		func() error {
			_, err := client.CampaignPerformance(context.Background(), CampaignReportRequest{From: "bad", To: "bad"})
			return err
		},
		func() error {
			_, err := client.PromotedContentPerformance(context.Background(), PromotedContentReportRequest{From: "2026-08-09", To: "2026-08-01"})
			return err
		},
		func() error {
			_, err := client.CampaignPeriodicPerformance(context.Background(), CampaignPeriodicReportRequest{From: "2026-08-01", To: "2026-08-09", Breakdown: "bad"})
			return err
		},
		func() error {
			_, err := client.PromotedContentPeriodicPerformance(context.Background(), PromotedContentPeriodicReportRequest{})
			return err
		},
	}
	for index, call := range calls {
		if err := call(); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Errorf("call %d error=%v", index, err)
		}
	}
}

func TestResponseContractValidation(t *testing.T) {
	tests := []struct {
		name string
		path string
		body any
		call func(*Client) error
	}{
		{"marketer count", "/marketers", map[string]any{"marketers": []Marketer{marketerFixture()}, "count": 2}, func(client *Client) error { _, err := client.ListMarketers(context.Background()); return err }},
		{"budget count", "/marketers/" + testMarketerID + "/budgets", map[string]any{"budgets": []Budget{budgetFixture()}, "count": 2}, func(client *Client) error { _, err := client.ListBudgets(context.Background(), false); return err }},
		{"campaign count", "/marketers/" + testMarketerID + "/campaigns", map[string]any{"campaigns": []Campaign{campaignFixture(false)}, "count": 2}, func(client *Client) error {
			_, err := client.ListCampaigns(context.Background(), ListCampaignsRequest{})
			return err
		}},
		{"report total", "/reports/marketers/" + testMarketerID + "/campaigns", map[string]any{"results": []any{map[string]any{"metadata": map[string]any{"id": testCampaignID}, "metrics": map[string]any{}}}, "totalResults": 0}, func(client *Client) error {
			_, err := client.CampaignPerformance(context.Background(), CampaignReportRequest{From: "2026-08-01", To: "2026-08-09"})
			return err
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				if request.URL.Path != test.path {
					t.Fatalf("path=%s", request.URL.Path)
				}
				writeJSON(t, writer, http.StatusOK, test.body)
			}))
			defer server.Close()
			_, client := newTestAdapter(t, server)
			if err := test.call(client); hubError(t, err).Code != socialhub.CodePlatformError {
				t.Fatalf("error=%v", err)
			}
		})
	}
}

func periodicWireRow() map[string]any {
	return map[string]any{
		"metadata": map[string]any{"id": "2026-08-09", "fromDate": "2026-08-09", "toDate": "2026-08-09"},
		"metrics":  map[string]any{"clicks": 2, "spend": 1.25},
	}
}

func validCreateCampaignRequest() CreateCampaignRequest {
	return CreateCampaignRequest{Name: "Campaign", CPC: 0.25, BudgetID: testBudgetID}
}

func validCreatePromotedLinkRequest() CreatePromotedLinkRequest {
	return CreatePromotedLinkRequest{Text: "Headline", URL: "https://example.test/article", ImageURL: "https://example.test/image.jpg"}
}

func TestMalformedJSONResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"marketers":`))
	}))
	defer server.Close()
	_, client := newTestAdapter(t, server)
	_, err := client.ListMarketers(context.Background())
	if hubError(t, err).Code != socialhub.CodePlatformError {
		t.Fatalf("error=%v", err)
	}
}

func TestPromotedLinkUpdateResponseUnknownID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.Method + " " + request.URL.Path {
		case "GET /campaigns/" + testCampaignID:
			writeJSON(t, writer, http.StatusOK, campaignFixture(false))
		case "GET /campaigns/" + testCampaignID + "/promotedLinks":
			writeJSON(t, writer, http.StatusOK, map[string]any{"promotedLinks": []PromotedLink{promotedLinkFixture(false, true)}, "count": 1, "totalCount": 1})
		default:
			value := promotedLinkFixture(false, true)
			value.ID = "unexpected-link"
			writeJSON(t, writer, http.StatusOK, []PromotedLinkUpdateResult{{OperationStatus: OperationStatus{Status: "Success"}, ID: value.ID, PromotedLink: value}})
		}
	}))
	defer server.Close()
	_, client := newTestAdapter(t, server)
	_, err := client.UpdatePromotedLinkCPCs(context.Background(), testCampaignID, []PromotedLinkCPCUpdate{{ID: testPromotedLinkID, CPC: 0.7}})
	if hubError(t, err).Code != socialhub.CodePlatformError {
		t.Fatalf("error=%v", err)
	}
}

func TestWireJSONUsesExpectedFieldNames(t *testing.T) {
	encoded, err := json.Marshal(CreateBudgetRequest{Name: "Budget", Amount: 75, StartDate: "2026-08-01", EndDate: "2026-08-03", Pacing: PacingSpendASAP, Type: BudgetCampaign})
	if err != nil || !strings.Contains(string(encoded), `"startDate"`) || !strings.Contains(string(encoded), `"endDate"`) {
		t.Fatalf("JSON=%s err=%v", encoded, err)
	}
}
