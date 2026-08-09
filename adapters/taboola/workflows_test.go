package taboola

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
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		assertAPIRequest(t, request)
		switch request.Method + " " + request.URL.Path {
		case "GET /backstage/api/1.0/users/current/account":
			writeJSON(t, writer, http.StatusOK, Account{ID: 1, Name: "Network", AccountID: "demo-network", PartnerTypes: []string{"ADVERTISER"}, CampaignTypes: []string{"PAID"}, Currency: "USD"})
		case "GET /backstage/api/1.0/users/current/allowed-accounts":
			writeJSON(t, writer, http.StatusOK, map[string]any{"results": []Account{{ID: 2, Name: "Advertiser", AccountID: testAdvertiserID, PartnerTypes: []string{"ADVERTISER"}, CampaignTypes: []string{"PAID"}, Currency: "USD"}}})
		case "GET /backstage/api/1.0/" + testAdvertiserID + "/campaigns/":
			if request.URL.Query().Get("page") != "2" || request.URL.Query().Get("page_size") != "10" || request.URL.Query().Get("fetch_level") != FetchRecentAndPaused || request.URL.Query().Get("sort") != "name,asc" {
				t.Fatalf("campaign query=%v", request.URL.Query())
			}
			writeJSON(t, writer, http.StatusOK, map[string]any{"results": []Campaign{campaignFixture(false, CampaignPaused)}, "metadata": map[string]any{"total": 25, "count": 1}})
		case "GET /backstage/api/1.0/" + testAdvertiserID + "/campaigns/" + testCampaignID + "/":
			writeJSON(t, writer, http.StatusOK, campaignFixture(false, CampaignPaused))
		case "POST /backstage/api/1.0/" + testAdvertiserID + "/campaigns/":
			payload := decodeJSONBody(t, request)
			if payload["is_active"] != false || payload["name"] != "Created Campaign" || payload["status"] != nil {
				t.Fatalf("create Campaign payload=%#v", payload)
			}
			value := campaignFixture(false, CampaignPaused)
			value.Name = "Created Campaign"
			writeJSON(t, writer, http.StatusOK, value)
		case "POST /backstage/api/1.0/" + testAdvertiserID + "/campaigns/" + testCampaignID:
			payload := decodeJSONBody(t, request)
			value := campaignFixture(false, CampaignPaused)
			if active, found := payload["is_active"]; found {
				value.IsActive = boolPointer(active.(bool))
				if active.(bool) {
					value.Status = CampaignRunning
				}
			} else {
				if payload["name"] != "Updated Campaign" {
					t.Fatalf("update Campaign payload=%#v", payload)
				}
				value.Name = "Updated Campaign"
			}
			writeJSON(t, writer, http.StatusOK, value)
		case "GET /backstage/api/1.0/" + testAdvertiserID + "/campaigns/" + testCampaignID + "/items/":
			writeJSON(t, writer, http.StatusOK, map[string]any{"results": []CampaignItem{itemFixture(false, ItemPaused)}})
		case "GET /backstage/api/1.0/" + testAdvertiserID + "/campaigns/" + testCampaignID + "/items/" + testItemID + "/":
			writeJSON(t, writer, http.StatusOK, itemFixture(true, ItemRunning))
		case "POST /backstage/api/1.0/" + testAdvertiserID + "/campaigns/" + testCampaignID + "/items/":
			payload := decodeJSONBody(t, request)
			if len(payload) != 1 || payload["url"] != "https://example.test/new-article" {
				t.Fatalf("create Item payload=%#v", payload)
			}
			value := itemFixture(true, ItemCrawling)
			value.URL = payload["url"].(string)
			writeJSON(t, writer, http.StatusOK, value)
		case "POST /backstage/api/1.0/" + testAdvertiserID + "/campaigns/" + testCampaignID + "/items/" + testItemID + "/":
			payload := decodeJSONBody(t, request)
			value := itemFixture(true, ItemRunning)
			if active, found := payload["is_active"]; found {
				value.IsActive = boolPointer(active.(bool))
				if !active.(bool) {
					value.Status = ItemPaused
				}
			} else {
				if payload["title"] != "Updated Item" {
					t.Fatalf("update Item payload=%#v", payload)
				}
				value.Title = "Updated Item"
			}
			writeJSON(t, writer, http.StatusOK, value)
		case "GET /backstage/api/1.0/" + testAdvertiserID + "/reports/campaign-summary/dimensions/day":
			query := request.URL.Query()
			if query.Get("start_date") != "2026-08-01" || query.Get("end_date") != "2026-08-09" || query.Get("campaign") != testCampaignID || query.Get("country") != "US" || query.Get("include_multi_conversions") != "true" {
				t.Fatalf("summary query=%v", query)
			}
			writeJSON(t, writer, http.StatusOK, map[string]any{
				"results":     []map[string]any{{"date": "2026-08-09 00:00:00.0", "clicks": 4, "impressions": 100, "spent": 2.5, "custom_metric": 7}},
				"recordCount": 1, "metadata": map[string]any{"total": 1, "count": 1}, "timezone": "UTC",
			})
		case "GET /backstage/api/1.0/" + testAdvertiserID + "/reports/realtime-campaign-summary/dimensions/by_hour":
			query := request.URL.Query()
			if query.Get("start_date") != "2026-08-09T00:00:00" || query.Get("end_date") != "2026-08-09T23:59:59" || query.Get("campaign") != testCampaignID || query.Get("site_id") != "3001" || query.Get("fetch_config") != "true" {
				t.Fatalf("realtime query=%v", query)
			}
			writeJSON(t, writer, http.StatusOK, map[string]any{
				"results":     []map[string]any{{"date": "2026-08-09 12:00:00.0", "clicks": 1, "visible_impressions": 20, "spent": 0.5}},
				"recordCount": 1, "metadata": map[string]any{"total": 1, "count": 1}, "timezone": "UTC",
			})
		default:
			t.Fatalf("unexpected request: %s %s", request.Method, request.URL)
		}
	}))
	defer server.Close()
	_, client := newTestAdapter(t, server)
	ctx := context.Background()

	current, err := client.CurrentAccount(ctx)
	if err != nil || current.AccountID != "demo-network" {
		t.Fatalf("current=%#v err=%v", current, err)
	}
	allowed, err := client.AllowedAccounts(ctx)
	if err != nil || len(allowed) != 1 || allowed[0].AccountID != testAdvertiserID {
		t.Fatalf("allowed=%#v err=%v", allowed, err)
	}
	validated, err := client.ValidateConfiguredAccount(ctx)
	if err != nil || validated.AccountID != testAdvertiserID {
		t.Fatalf("validated=%#v err=%v", validated, err)
	}

	page, err := client.ListCampaigns(ctx, ListCampaignsRequest{FetchLevel: FetchRecentAndPaused, Page: 2, PageSize: 10, Sort: "name,asc"})
	if err != nil || len(page.Items) != 1 || page.Total != 25 || page.Count != 1 || !page.HasMore {
		t.Fatalf("page=%#v err=%v", page, err)
	}
	if value, err := client.GetCampaign(ctx, testCampaignID); err != nil || value.ID != testCampaignID {
		t.Fatalf("Campaign=%#v err=%v", value, err)
	}
	created, err := client.CreateCampaign(ctx, CreateCampaignRequest{
		Name: "Created Campaign", BrandingText: "Brand", BidStrategy: BidStrategyFixed,
		MarketingObjective: "DRIVE_WEBSITE_TRAFFIC", CPC: floatPointer(0.25),
		SpendingLimit: floatPointer(100), SpendingLimitModel: SpendingMonthly,
	})
	if err != nil || created.IsActive == nil || *created.IsActive {
		t.Fatalf("created Campaign=%#v err=%v", created, err)
	}
	updated, err := client.UpdateCampaign(ctx, testCampaignID, UpdateCampaignRequest{Name: stringPointer("Updated Campaign")})
	if err != nil || updated.Name != "Updated Campaign" {
		t.Fatalf("updated Campaign=%#v err=%v", updated, err)
	}
	if paused, err := client.SetCampaignActive(ctx, testCampaignID, false); err != nil || paused.IsActive == nil || *paused.IsActive {
		t.Fatalf("paused Campaign=%#v err=%v", paused, err)
	}
	if enabled, err := client.SetCampaignActive(ctx, testCampaignID, true); err != nil || enabled.IsActive == nil || !*enabled.IsActive {
		t.Fatalf("enabled Campaign=%#v err=%v", enabled, err)
	}

	items, err := client.ListItems(ctx, testCampaignID)
	if err != nil || len(items) != 1 || items[0].CampaignID != testCampaignID {
		t.Fatalf("items=%#v err=%v", items, err)
	}
	if value, err := client.GetItem(ctx, testCampaignID, testItemID); err != nil || value.ID != testItemID {
		t.Fatalf("Item=%#v err=%v", value, err)
	}
	createdItem, err := client.CreateItem(ctx, testCampaignID, CreateItemRequest{URL: "https://example.test/new-article"})
	if err != nil || createdItem.Status != ItemCrawling {
		t.Fatalf("created Item=%#v err=%v", createdItem, err)
	}
	updatedItem, err := client.UpdateItem(ctx, testCampaignID, testItemID, UpdateItemRequest{Title: stringPointer("Updated Item")})
	if err != nil || updatedItem.Title != "Updated Item" {
		t.Fatalf("updated Item=%#v err=%v", updatedItem, err)
	}
	pausedItem, err := client.SetItemActive(ctx, testCampaignID, testItemID, false)
	if err != nil || pausedItem.IsActive == nil || *pausedItem.IsActive {
		t.Fatalf("paused Item=%#v err=%v", pausedItem, err)
	}
	enabledItem, err := client.SetItemActive(ctx, testCampaignID, testItemID, true)
	if err != nil || enabledItem.IsActive == nil || !*enabledItem.IsActive {
		t.Fatalf("enabled Item=%#v err=%v", enabledItem, err)
	}

	summary, err := client.CampaignSummaryReport(ctx, ReportRequest{
		Dimension: "day", StartDate: "2026-08-01", EndDate: "2026-08-09", CampaignIDs: []string{testCampaignID}, Country: "US", IncludeMultiConversions: true,
	})
	if err != nil || len(summary.Rows) != 1 || summary.Rows[0].Clicks != 4 || summary.Total != 1 || summary.Timezone != "UTC" || !strings.Contains(string(summary.Rows[0].Raw), "custom_metric") {
		t.Fatalf("summary=%#v err=%v", summary, err)
	}
	realtime, err := client.RealtimeCampaignReport(ctx, RealtimeReportRequest{
		Dimension: "by_hour", StartDate: "2026-08-09T00:00:00", EndDate: "2026-08-09T23:59:59", CampaignIDs: []string{testCampaignID}, SiteID: "3001", FetchConfig: true,
	})
	if err != nil || len(realtime.Rows) != 1 || realtime.Rows[0].VisibleImpressions != 20 {
		t.Fatalf("realtime=%#v err=%v", realtime, err)
	}
}

func TestPausedFirstSafetyContracts(t *testing.T) {
	t.Run("create Campaign rejects active response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			assertAPIRequest(t, request)
			writeJSON(t, writer, http.StatusOK, campaignFixture(true, CampaignRunning))
		}))
		defer server.Close()
		_, client := newTestAdapter(t, server)
		_, err := client.CreateCampaign(context.Background(), validCreateCampaignRequest())
		if hubError(t, err).Code != socialhub.CodePlatformError {
			t.Fatalf("error=%v", err)
		}
	})

	t.Run("create Item requires paused Campaign", func(t *testing.T) {
		var writes atomic.Int32
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			assertAPIRequest(t, request)
			if request.Method != http.MethodGet {
				writes.Add(1)
				t.Fatal("unsafe Item write reached network")
			}
			writeJSON(t, writer, http.StatusOK, campaignFixture(true, CampaignRunning))
		}))
		defer server.Close()
		_, client := newTestAdapter(t, server)
		_, err := client.CreateItem(context.Background(), testCampaignID, CreateItemRequest{URL: "https://example.test"})
		if !errors.Is(err, socialhub.ErrInvalidArgument) || writes.Load() != 0 {
			t.Fatalf("error=%v writes=%d", err, writes.Load())
		}
	})

	for _, item := range []CampaignItem{
		itemFixture(true, ItemPaused), itemFixture(false, ItemCrawling), itemFixture(false, ItemPendingApproval),
		func() CampaignItem { value := itemFixture(false, ItemPaused); value.IsActive = nil; return value }(),
	} {
		item := item
		t.Run("enable Campaign rejects unsafe Item "+string(item.Status), func(t *testing.T) {
			var writes atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				assertAPIRequest(t, request)
				if request.Method != http.MethodGet {
					writes.Add(1)
					t.Fatal("unsafe Campaign write reached network")
				}
				if strings.HasSuffix(request.URL.Path, "/items/") {
					writeJSON(t, writer, http.StatusOK, map[string]any{"results": []CampaignItem{item}})
					return
				}
				writeJSON(t, writer, http.StatusOK, campaignFixture(false, CampaignPaused))
			}))
			defer server.Close()
			_, client := newTestAdapter(t, server)
			_, err := client.SetCampaignActive(context.Background(), testCampaignID, true)
			if err == nil || writes.Load() != 0 {
				t.Fatalf("error=%v writes=%d", err, writes.Load())
			}
		})
	}
}

func TestOwnershipAndItemStateStopWrites(t *testing.T) {
	var writes atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		assertAPIRequest(t, request)
		if request.Method != http.MethodGet {
			writes.Add(1)
			t.Fatal("unsafe write reached network")
		}
		switch {
		case strings.HasSuffix(request.URL.Path, "/items/"+testItemID+"/"):
			writeJSON(t, writer, http.StatusOK, itemFixture(true, ItemCrawling))
		default:
			writeJSON(t, writer, http.StatusOK, campaignFixture(false, CampaignPaused))
		}
	}))
	defer server.Close()
	_, client := newTestAdapter(t, server)
	if _, err := client.UpdateItem(context.Background(), testCampaignID, testItemID, UpdateItemRequest{Title: stringPointer("new")}); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("CRAWLING update error=%v", err)
	}
	if _, err := client.SetItemActive(context.Background(), testCampaignID, testItemID, false); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("CRAWLING status error=%v", err)
	}
	if writes.Load() != 0 {
		t.Fatalf("writes=%d", writes.Load())
	}

	wrongOwner := campaignFixture(false, CampaignPaused)
	wrongOwner.AdvertiserID = "another-advertiser"
	ownerServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writeJSON(t, writer, http.StatusOK, wrongOwner)
	}))
	defer ownerServer.Close()
	_, ownerClient := newTestAdapter(t, ownerServer)
	if _, err := ownerClient.GetCampaign(context.Background(), testCampaignID); hubError(t, err).Code != socialhub.CodePlatformError {
		t.Fatalf("ownership error=%v", err)
	}
}

func TestLocalValidationAvoidsNetwork(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Fatal("invalid input reached network") }))
	defer server.Close()
	_, client := newTestAdapter(t, server)
	calls := []func() error{
		func() error {
			_, err := client.ListCampaigns(context.Background(), ListCampaignsRequest{Page: 1})
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
		func() error { _, err := client.SetCampaignActive(context.Background(), "bad", true); return err },
		func() error { _, err := client.ListItems(context.Background(), "bad"); return err },
		func() error { _, err := client.GetItem(context.Background(), testCampaignID, "bad"); return err },
		func() error {
			_, err := client.CreateItem(context.Background(), testCampaignID, CreateItemRequest{URL: "file:///etc/passwd"})
			return err
		},
		func() error {
			_, err := client.UpdateItem(context.Background(), testCampaignID, testItemID, UpdateItemRequest{})
			return err
		},
		func() error {
			_, err := client.SetItemActive(context.Background(), testCampaignID, "bad", false)
			return err
		},
		func() error {
			_, err := client.CampaignSummaryReport(context.Background(), ReportRequest{Dimension: "BAD", StartDate: "2026-08-09", EndDate: "2026-08-01"})
			return err
		},
		func() error {
			_, err := client.CampaignSummaryReport(context.Background(), ReportRequest{Dimension: "day", StartDate: "2026-08-01", EndDate: "2026-08-09", Country: "US", Platform: "DESK"})
			return err
		},
		func() error {
			_, err := client.RealtimeCampaignReport(context.Background(), RealtimeReportRequest{Dimension: "by_hour", StartDate: "bad", EndDate: "bad"})
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
	t.Run("Item Campaign mismatch", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			if strings.HasSuffix(request.URL.Path, "/items/"+testItemID+"/") {
				value := itemFixture(true, ItemRunning)
				value.CampaignID = "9999"
				writeJSON(t, writer, http.StatusOK, value)
				return
			}
			writeJSON(t, writer, http.StatusOK, campaignFixture(false, CampaignPaused))
		}))
		defer server.Close()
		_, client := newTestAdapter(t, server)
		if _, err := client.GetItem(context.Background(), testCampaignID, testItemID); hubError(t, err).Code != socialhub.CodePlatformError {
			t.Fatalf("error=%v", err)
		}
	})

	t.Run("report metadata mismatch", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			writeJSON(t, writer, http.StatusOK, map[string]any{"results": []any{}, "recordCount": 0, "metadata": map[string]any{"total": 1, "count": 1}})
		}))
		defer server.Close()
		_, client := newTestAdapter(t, server)
		_, err := client.CampaignSummaryReport(context.Background(), ReportRequest{Dimension: "day", StartDate: "2026-08-01", EndDate: "2026-08-09"})
		if hubError(t, err).Code != socialhub.CodePlatformError {
			t.Fatalf("error=%v", err)
		}
	})

	var row ReportRow
	if err := json.Unmarshal([]byte(`{"clicks":3,"unknown":"kept"}`), &row); err != nil || row.Clicks != 3 || !strings.Contains(string(row.Raw), "unknown") {
		t.Fatalf("row=%#v err=%v", row, err)
	}
}

func validCreateCampaignRequest() CreateCampaignRequest {
	return CreateCampaignRequest{
		Name: "Campaign", BrandingText: "Brand", BidStrategy: BidStrategyFixed,
		MarketingObjective: "DRIVE_WEBSITE_TRAFFIC", CPC: floatPointer(0.25),
		SpendingLimit: floatPointer(100), SpendingLimitModel: SpendingMonthly,
	}
}
