package marketing

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"social-hub/pkg/socialhub"
)

func TestAdAccountAndCampaignGroupWireContracts(t *testing.T) {
	status, name := StatusDraft, "Launch group"
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		assertAPIRequest(t, request)
		switch request.Method + " " + request.URL.Path {
		case http.MethodGet + " /adAccounts/" + testAdAccountID:
			writeValue(t, writer, http.StatusOK, AdAccount{ID: NumericID(testAdAccountID), Name: "B2B Demand", Currency: "USD"})
		case http.MethodGet + " /adAccounts/" + testAdAccountID + "/adCampaignGroups":
			if strings.Contains(request.URL.RawQuery, "%28") || !strings.Contains(request.URL.RawQuery, "q=search&search=(status:(values:List(ACTIVE,DRAFT)))") ||
				!strings.Contains(request.URL.RawQuery, "pageSize=25") || !strings.Contains(request.URL.RawQuery, "pageToken=next+token") {
				t.Fatalf("campaign group query=%q", request.URL.RawQuery)
			}
			writeValue(t, writer, http.StatusOK, map[string]any{
				"elements": []CampaignGroup{{ID: NumericID(testCampaignGroupID), Account: accountURNPrefix + testAdAccountID, Name: name, Status: status}},
				"metadata": map[string]string{"nextPageToken": "following"},
			})
		case http.MethodPost + " /adAccounts/" + testAdAccountID + "/adCampaignGroups":
			payload := decodeJSONMap(t, request)
			if request.Header.Get("X-RestLi-Method") != "" || payload["status"] != string(StatusDraft) || payload["account"] != accountURNPrefix+testAdAccountID {
				t.Fatalf("create headers=%v payload=%v", request.Header, payload)
			}
			name = payload["name"].(string)
			status = StatusDraft
			writer.Header().Set("X-RestLi-Id", url.QueryEscape(campaignGroupURNPrefix+testCampaignGroupID))
			writeValue(t, writer, http.StatusCreated, nil)
		case http.MethodPost + " /adAccounts/" + testAdAccountID + "/adCampaignGroups/" + testCampaignGroupID:
			payload := decodeJSONMap(t, request)
			if request.Header.Get("X-RestLi-Method") != "PARTIAL_UPDATE" {
				t.Fatalf("partial update headers=%v", request.Header)
			}
			set := payload["patch"].(map[string]any)["$set"].(map[string]any)
			if value, found := set["name"]; found {
				name = value.(string)
			}
			if value, found := set["status"]; found {
				status = Status(value.(string))
			}
			writeValue(t, writer, http.StatusNoContent, nil)
		case http.MethodGet + " /adAccounts/" + testAdAccountID + "/adCampaignGroups/" + testCampaignGroupID:
			writeValue(t, writer, http.StatusOK, CampaignGroup{
				ID: NumericID(testCampaignGroupID), Account: accountURNPrefix + testAdAccountID,
				Name: name, Status: status, RunSchedule: RunSchedule{Start: 1786276800000, End: 1788955200000},
			})
		default:
			t.Fatalf("unexpected request: %s %s", request.Method, request.URL)
		}
	}))
	defer server.Close()

	_, client := newTestAdapter(t, server)
	account, err := client.GetAdAccount(context.Background())
	if err != nil || string(account.ID) != testAdAccountID {
		t.Fatalf("account=%#v err=%v", account, err)
	}
	page, err := client.ListCampaignGroups(context.Background(), ListRequest{
		Statuses: []Status{StatusActive, StatusDraft}, Cursor: "next token", MaxResults: 25,
	})
	if err != nil || len(page.Items) != 1 || !page.HasMore || *page.NextCursor != "following" {
		t.Fatalf("page=%#v err=%v", page, err)
	}
	created, err := client.CreateCampaignGroup(context.Background(), CreateCampaignGroupRequest{
		Name: "Created group", RunSchedule: RunSchedule{Start: 1786276800000, End: 1788955200000},
		TotalBudget: &Money{Amount: "5000.00", CurrencyCode: "USD"},
	})
	if err != nil || created.Status != StatusDraft || created.Name != "Created group" {
		t.Fatalf("created=%#v err=%v", created, err)
	}
	updatedName := "Updated group"
	updated, err := client.UpdateCampaignGroup(context.Background(), testCampaignGroupID, UpdateCampaignGroupRequest{Name: &updatedName})
	if err != nil || updated.Name != updatedName {
		t.Fatalf("updated=%#v err=%v", updated, err)
	}
	active, err := client.SetCampaignGroupStatus(context.Background(), testCampaignGroupID, StatusActive)
	if err != nil || active.Status != StatusActive {
		t.Fatalf("active=%#v err=%v", active, err)
	}
	if err := client.ArchiveCampaignGroup(context.Background(), testCampaignGroupID); err != nil || status != StatusArchived {
		t.Fatalf("archive status=%q err=%v", status, err)
	}
}

func TestCampaignWireContracts(t *testing.T) {
	status, name := StatusDraft, "Launch campaign"
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		assertAPIRequest(t, request)
		path := "/adAccounts/" + testAdAccountID + "/adCampaigns"
		switch request.Method + " " + request.URL.Path {
		case http.MethodGet + " " + path:
			if !strings.Contains(request.URL.RawQuery, "search=(status:(values:List(ACTIVE,PAUSED,DRAFT,ARCHIVED,COMPLETED,CANCELED,PENDING_DELETION,REMOVED)))") {
				t.Fatalf("campaign query=%q", request.URL.RawQuery)
			}
			writeValue(t, writer, http.StatusOK, map[string]any{
				"elements": []Campaign{campaignFixture(name, status)}, "metadata": map[string]string{"nextPageToken": "campaign-next"},
			})
		case http.MethodPost + " " + path:
			payload := decodeJSONMap(t, request)
			if payload["status"] != string(StatusDraft) || payload["type"] != "SPONSORED_UPDATES" || payload["creativeSelection"] != "OPTIMIZED" ||
				payload["account"] != accountURNPrefix+testAdAccountID || payload["campaignGroup"] != campaignGroupURNPrefix+testCampaignGroupID {
				t.Fatalf("create payload=%v", payload)
			}
			name, status = payload["name"].(string), StatusDraft
			writer.Header().Set("X-RestLi-Id", testCampaignID)
			writeValue(t, writer, http.StatusCreated, nil)
		case http.MethodPost + " " + path + "/" + testCampaignID:
			payload := decodeJSONMap(t, request)
			if request.Header.Get("X-RestLi-Method") != "PARTIAL_UPDATE" {
				t.Fatalf("partial update headers=%v", request.Header)
			}
			set := payload["patch"].(map[string]any)["$set"].(map[string]any)
			if value, found := set["name"]; found {
				name = value.(string)
			}
			if value, found := set["status"]; found {
				status = Status(value.(string))
			}
			writeValue(t, writer, http.StatusNoContent, nil)
		case http.MethodGet + " " + path + "/" + testCampaignID:
			writeValue(t, writer, http.StatusOK, campaignFixture(name, status))
		default:
			t.Fatalf("unexpected request: %s %s", request.Method, request.URL)
		}
	}))
	defer server.Close()
	_, client := newTestAdapter(t, server)

	page, err := client.ListCampaigns(context.Background(), ListRequest{})
	if err != nil || len(page.Items) != 1 || *page.NextCursor != "campaign-next" {
		t.Fatalf("page=%#v err=%v", page, err)
	}
	created, err := client.CreateCampaign(context.Background(), validCampaignRequest())
	if err != nil || created.Status != StatusDraft {
		t.Fatalf("created=%#v err=%v", created, err)
	}
	updatedName := "Retargeted campaign"
	end := int64(1789041600000)
	updated, err := client.UpdateCampaign(context.Background(), testCampaignID, UpdateCampaignRequest{Name: &updatedName, EndTime: &end})
	if err != nil || updated.Name != updatedName {
		t.Fatalf("updated=%#v err=%v", updated, err)
	}
	active, err := client.SetCampaignStatus(context.Background(), testCampaignID, StatusActive)
	if err != nil || active.Status != StatusActive {
		t.Fatalf("active=%#v err=%v", active, err)
	}
	if err := client.ArchiveCampaign(context.Background(), testCampaignID); err != nil || status != StatusArchived {
		t.Fatalf("archive status=%q err=%v", status, err)
	}
}

func TestCreativeBatchAndFinderWireContracts(t *testing.T) {
	status := StatusDraft
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		assertAPIRequest(t, request)
		path := "/adAccounts/" + testAdAccountID + "/creatives"
		switch request.Method + " " + request.URL.Path {
		case http.MethodGet + " " + path:
			if request.Header.Get("X-RestLi-Method") != "FINDER" || request.URL.RawQuery != "q=criteria&sortOrder=ASCENDING&pageSize=100&pageToken=creative+next" {
				t.Fatalf("finder headers=%v query=%q", request.Header, request.URL.RawQuery)
			}
			writeValue(t, writer, http.StatusOK, map[string]any{
				"elements": []Creative{creativeFixture(status)}, "metadata": map[string]string{"nextPageToken": "more-creatives"},
			})
		case http.MethodPost + " " + path:
			if request.Header.Get("X-RestLi-Method") != "BATCH_CREATE" {
				t.Fatalf("batch headers=%v", request.Header)
			}
			payload := decodeJSONMap(t, request)
			elements := payload["elements"].([]any)
			element := elements[0].(map[string]any)
			if len(elements) != 1 || element["intendedStatus"] != string(StatusDraft) || element["campaign"] != campaignURNPrefix+testCampaignID ||
				element["content"].(map[string]any)["reference"] != "urn:li:ugcPost:6778045555198214144" {
				t.Fatalf("batch payload=%v", payload)
			}
			status = StatusDraft
			writeValue(t, writer, http.StatusCreated, map[string]any{"elements": []any{map[string]any{"status": 201, "id": testCreativeID}}})
		case http.MethodPost + " " + path + "/" + testCreativeID:
			if request.Header.Get("X-RestLi-Method") != "PARTIAL_UPDATE" {
				t.Fatalf("partial update headers=%v", request.Header)
			}
			payload := decodeJSONMap(t, request)
			set := payload["patch"].(map[string]any)["$set"].(map[string]any)
			status = Status(set["intendedStatus"].(string))
			writeValue(t, writer, http.StatusNoContent, nil)
		case http.MethodGet + " " + path + "/" + testCreativeID:
			writeValue(t, writer, http.StatusOK, creativeFixture(status))
		default:
			t.Fatalf("unexpected request: %s %s", request.Method, request.URL)
		}
	}))
	defer server.Close()
	_, client := newTestAdapter(t, server)

	page, err := client.ListCreatives(context.Background(), ListCreativesRequest{Cursor: "creative next", MaxResults: 100})
	if err != nil || len(page.Items) != 1 || *page.NextCursor != "more-creatives" {
		t.Fatalf("page=%#v err=%v", page, err)
	}
	created, err := client.CreateCreative(context.Background(), CreateCreativeRequest{
		CampaignID: testCampaignID, ContentURN: "urn:li:ugcPost:6778045555198214144", Name: "Launch creative",
	})
	if err != nil || created.IntendedStatus != StatusDraft {
		t.Fatalf("created=%#v err=%v", created, err)
	}
	active, err := client.SetCreativeStatus(context.Background(), testCreativeID, StatusActive)
	if err != nil || active.IntendedStatus != StatusActive {
		t.Fatalf("active=%#v err=%v", active, err)
	}
	if err := client.ArchiveCreative(context.Background(), testCreativeID); err != nil || status != StatusArchived {
		t.Fatalf("archive status=%q err=%v", status, err)
	}
}

func TestAnalyticsRawRestliQueryAndDynamicMetrics(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		assertAPIRequest(t, request)
		if request.Method != http.MethodGet || request.URL.Path != "/adAnalytics" || strings.Contains(request.URL.RawQuery, "%28") ||
			!strings.Contains(request.URL.RawQuery, "accounts=List(urn%3Ali%3AsponsoredAccount%3A"+testAdAccountID+")") ||
			!strings.Contains(request.URL.RawQuery, "dateRange=(start:(year:2026,month:8,day:1),end:(year:2026,month:8,day:7))") ||
			!strings.Contains(request.URL.RawQuery, "fields=impressions,clicks") {
			t.Fatalf("analytics request=%s %s", request.Method, request.URL)
		}
		writeValue(t, writer, http.StatusOK, map[string]any{"elements": []any{map[string]any{
			"dateRange": map[string]any{
				"start": map[string]int{"year": 2026, "month": 8, "day": 1},
				"end":   map[string]int{"year": 2026, "month": 8, "day": 7},
			},
			"pivotValues": []string{accountURNPrefix + testAdAccountID}, "impressions": 1200, "clicks": 42,
		}}})
	}))
	defer server.Close()
	_, client := newTestAdapter(t, server)
	rows, err := client.GetAdAnalytics(context.Background(), AnalyticsRequest{
		StartDate: "2026-08-01", EndDate: "2026-08-07", Pivot: PivotAccount,
		Granularity: GranularityDaily, Fields: []string{"impressions", "clicks"},
	})
	if err != nil || len(rows) != 1 || string(rows[0].Metrics["clicks"]) != "42" || len(rows[0].Metrics) != 2 {
		t.Fatalf("rows=%#v err=%v", rows, err)
	}
}

func TestOwnershipAndContractRejections(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		assertAPIRequest(t, request)
		switch {
		case strings.Contains(request.URL.Path, "adCampaignGroups"):
			writeValue(t, writer, http.StatusOK, CampaignGroup{ID: NumericID(testCampaignGroupID), Account: accountURNPrefix + "999"})
		case strings.Contains(request.URL.Path, "adCampaigns"):
			value := campaignFixture("wrong owner", StatusDraft)
			value.Account = accountURNPrefix + "999"
			writeValue(t, writer, http.StatusOK, value)
		case strings.Contains(request.URL.Path, "creatives"):
			value := creativeFixture(StatusDraft)
			value.Account = accountURNPrefix + "999"
			writeValue(t, writer, http.StatusOK, value)
		default:
			writeValue(t, writer, http.StatusOK, AdAccount{ID: NumericID("999")})
		}
	}))
	defer server.Close()
	_, client := newTestAdapter(t, server)
	calls := []func() error{
		func() error { _, err := client.GetAdAccount(context.Background()); return err },
		func() error { _, err := client.GetCampaignGroup(context.Background(), testCampaignGroupID); return err },
		func() error { _, err := client.GetCampaign(context.Background(), testCampaignID); return err },
		func() error { _, err := client.GetCreative(context.Background(), testCreativeID); return err },
	}
	for index, call := range calls {
		if err := call(); hubError(t, err).Code != socialhub.CodePlatformError {
			t.Errorf("call %d error=%v", index, err)
		}
	}
}

func campaignFixture(name string, status Status) Campaign {
	return Campaign{
		ID: NumericID(testCampaignID), Account: accountURNPrefix + testAdAccountID,
		CampaignGroup: campaignGroupURNPrefix + testCampaignGroupID, AssociatedEntity: "urn:li:organization:2414183",
		Name: name, Status: status, Type: "SPONSORED_UPDATES", Objective: ObjectiveBrandAwareness,
		RunSchedule: RunSchedule{Start: 1786276800000, End: 1788955200000},
	}
}

func creativeFixture(status Status) Creative {
	return Creative{
		ID: testCreativeID, Account: accountURNPrefix + testAdAccountID,
		Campaign: campaignURNPrefix + testCampaignID, Content: CreativeContent{Reference: "urn:li:ugcPost:6778045555198214144"},
		Name: "Launch creative", IntendedStatus: status,
	}
}

func TestAnalyticsRowRejectsMalformedJSON(t *testing.T) {
	var row AnalyticsRow
	if err := json.Unmarshal([]byte(`{"dateRange":false}`), &row); err == nil {
		t.Fatal("expected malformed dateRange error")
	}
	if err := json.Unmarshal([]byte(`{"pivotValues":false}`), &row); err == nil {
		t.Fatal("expected malformed pivotValues error")
	}
}
