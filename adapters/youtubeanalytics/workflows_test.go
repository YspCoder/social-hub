package youtubeanalytics

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"sync/atomic"
	"testing"

	"social-hub/pkg/socialhub"
)

func TestChannelReportAndGroupWorkflows(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		calls.Add(1)
		query := request.URL.Query()
		switch request.Method + " " + request.URL.Path {
		case "GET /v2/reports":
			assertAPIRequest(t, request, http.MethodGet, "/v2/reports")
			expected := map[string]string{
				"ids": "channel==" + testChannelID, "startDate": "2026-08-01", "endDate": "2026-08-09",
				"metrics": "views", "dimensions": "day", "filters": "video==video1,video2", "sort": "-views",
				"currency": "USD", "maxResults": "100", "startIndex": "1",
			}
			for key, value := range expected {
				if query.Get(key) != value {
					t.Errorf("%s=%q", key, query.Get(key))
				}
			}
			if query.Get("onBehalfOfContentOwner") != "" || query.Get("includeHistoricalChannelData") != "" {
				t.Errorf("unexpected owner query=%v", query)
			}
			writeJSON(t, writer, http.StatusOK, reportFixture())
		case "GET /v2/groups":
			assertAPIRequest(t, request, http.MethodGet, "/v2/groups")
			if query.Get("mine") != "true" || query.Get("pageToken") != "page-1" || query.Get("id") != "" {
				t.Errorf("group list query=%v", query)
			}
			writeJSON(t, writer, http.StatusOK, map[string]any{
				"kind": "youtube#groupListResponse", "etag": "list-etag", "nextPageToken": "page-2",
				"items": []any{groupFixture("group1", ResourceVideo)},
			})
		case "POST /v2/groups":
			assertAPIRequest(t, request, http.MethodPost, "/v2/groups")
			var body map[string]any
			if json.NewDecoder(request.Body).Decode(&body) != nil || body["id"] != nil ||
				body["snippet"].(map[string]any)["title"] != "Top videos" ||
				body["contentDetails"].(map[string]any)["itemType"] != string(ResourceVideo) {
				t.Errorf("create body=%v", body)
			}
			writeJSON(t, writer, http.StatusOK, groupFixture("group1", ResourceVideo))
		case "PUT /v2/groups":
			assertAPIRequest(t, request, http.MethodPut, "/v2/groups")
			var body map[string]any
			if json.NewDecoder(request.Body).Decode(&body) != nil || body["id"] != "group1" || body["snippet"].(map[string]any)["title"] != "Renamed" {
				t.Errorf("rename body=%v", body)
			}
			fixture := groupFixture("group1", ResourceVideo)
			fixture["snippet"].(map[string]any)["title"] = "Renamed"
			writeJSON(t, writer, http.StatusOK, fixture)
		case "DELETE /v2/groups":
			assertAPIRequest(t, request, http.MethodDelete, "/v2/groups")
			if query.Get("id") != "group1" {
				t.Errorf("delete group query=%v", query)
			}
			writer.WriteHeader(http.StatusNoContent)
		case "GET /v2/groupItems":
			assertAPIRequest(t, request, http.MethodGet, "/v2/groupItems")
			if query.Get("groupId") != "group1" {
				t.Errorf("group items query=%v", query)
			}
			writeJSON(t, writer, http.StatusOK, map[string]any{
				"kind": "youtube#groupItemListResponse", "etag": "items-etag",
				"items": []any{groupItemFixture("item1", "group1", "video1", ResourceVideo)},
			})
		case "POST /v2/groupItems":
			assertAPIRequest(t, request, http.MethodPost, "/v2/groupItems")
			var body map[string]any
			if json.NewDecoder(request.Body).Decode(&body) != nil || body["groupId"] != "group1" ||
				body["resource"].(map[string]any)["id"] != "video1" || body["resource"].(map[string]any)["kind"] != string(ResourceVideo) {
				t.Errorf("add item body=%v", body)
			}
			writeJSON(t, writer, http.StatusOK, groupItemFixture("item1", "group1", "video1", ResourceVideo))
		case "DELETE /v2/groupItems":
			assertAPIRequest(t, request, http.MethodDelete, "/v2/groupItems")
			if query.Get("id") != "item1" {
				t.Errorf("remove item query=%v", query)
			}
			writer.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	_, client := newStaticClient(t, server, staticConfig(server.URL))

	report, err := client.QueryReport(context.Background(), reportRequest())
	if err != nil || len(report.Raw) == 0 || len(report.Rows) != 1 || report.Rows[0][1].(json.Number).String() != "10" {
		t.Fatalf("report=%#v err=%v", report, err)
	}
	groups, err := client.ListGroups(context.Background(), ListGroupsRequest{Mine: true, PageToken: "page-1"})
	if err != nil || groups.NextPageToken != "page-2" || len(groups.Items) != 1 || len(groups.Items[0].Raw) == 0 {
		t.Fatalf("groups=%#v err=%v", groups, err)
	}
	created, err := client.CreateGroup(context.Background(), CreateGroupInput{Title: "Top videos", ItemType: ResourceVideo})
	if err != nil || created.ID != "group1" {
		t.Fatalf("created=%#v err=%v", created, err)
	}
	renamed, err := client.RenameGroup(context.Background(), "group1", "Renamed")
	if err != nil || renamed.Snippet.Title != "Renamed" {
		t.Fatalf("renamed=%#v err=%v", renamed, err)
	}
	if err := client.DeleteGroup(context.Background(), "group1"); err != nil {
		t.Fatal(err)
	}
	items, err := client.ListGroupItems(context.Background(), "group1")
	if err != nil || len(items.Items) != 1 || len(items.Items[0].Raw) == 0 {
		t.Fatalf("items=%#v err=%v", items, err)
	}
	added, err := client.AddGroupItem(context.Background(), AddGroupItemInput{GroupID: "group1", ResourceID: "video1", Kind: ResourceVideo})
	if err != nil || added.AlreadyPresent || added.Item == nil || added.Item.ID != "item1" {
		t.Fatalf("added=%#v err=%v", added, err)
	}
	if err := client.RemoveGroupItem(context.Background(), "item1"); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 8 {
		t.Fatalf("calls=%d", calls.Load())
	}
}

func TestContentOwnerBindingAndAssetWorkflow(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		calls.Add(1)
		query := request.URL.Query()
		switch request.Method + " " + request.URL.Path {
		case "GET /v2/reports":
			if query.Get("ids") != "contentOwner=="+testOwnerID || query.Get("includeHistoricalChannelData") != "true" ||
				query.Get("onBehalfOfContentOwner") != "" {
				t.Errorf("owner report query=%v", query)
			}
			writeJSON(t, writer, http.StatusOK, map[string]any{
				"kind":          "youtubeAnalytics#resultTable",
				"columnHeaders": []any{map[string]any{"name": "estimatedRevenue", "columnType": "METRIC", "dataType": "FLOAT"}},
				"rows":          []any{[]any{12.5}},
			})
		case "POST /v2/groups":
			if query.Get("onBehalfOfContentOwner") != testOwnerID {
				t.Errorf("owner group query=%v", query)
			}
			var body map[string]any
			_ = json.NewDecoder(request.Body).Decode(&body)
			if body["contentDetails"].(map[string]any)["itemType"] != string(ResourceAsset) {
				t.Errorf("owner group body=%v", body)
			}
			fixture := groupFixture("assetgroup", ResourceAsset)
			fixture["snippet"].(map[string]any)["title"] = "Assets"
			writeJSON(t, writer, http.StatusOK, fixture)
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	_, client := newStaticClient(t, server, ownerConfig(server.URL))
	input := ReportQuery{
		StartDate: "2026-08-01", EndDate: "2026-08-09", Metrics: []Metric{MetricEstimatedRevenue},
		Currency: "CNY", IncludeHistoricalChannelData: true,
	}
	result, err := client.QueryReport(context.Background(), input)
	if err != nil || result.Rows[0][0].(json.Number).String() != "12.5" || !requiresMonetaryScope(input) {
		t.Fatalf("owner report=%#v err=%v", result, err)
	}
	group, err := client.CreateGroup(context.Background(), CreateGroupInput{Title: "Assets", ItemType: ResourceAsset})
	if err != nil || group.ID != "assetgroup" {
		t.Fatalf("asset group=%#v err=%v", group, err)
	}
	if calls.Load() != 2 {
		t.Fatalf("calls=%d", calls.Load())
	}
}

func TestGroupItemAlreadyPresent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		assertAPIRequest(t, request, http.MethodPost, "/v2/groupItems")
		writer.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	_, client := newStaticClient(t, server, staticConfig(server.URL))
	result, err := client.AddGroupItem(context.Background(), AddGroupItemInput{GroupID: "group1", ResourceID: "video1", Kind: ResourceVideo})
	if err != nil || !result.AlreadyPresent || result.Item != nil {
		t.Fatalf("result=%#v err=%v", result, err)
	}
}

func TestWorkflowInputValidation(t *testing.T) {
	requestSeen := false
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { requestSeen = true }))
	defer server.Close()
	_, channel := newStaticClient(t, server, staticConfig(server.URL))
	_, owner := newStaticClient(t, server, ownerConfig(server.URL))

	invalidReports := []ReportQuery{
		{},
		{StartDate: "2026-02-30", EndDate: "2026-03-01", Metrics: []Metric{MetricViews}},
		{StartDate: "2026-08-02", EndDate: "2026-08-01", Metrics: []Metric{MetricViews}},
		{StartDate: "2026-08-01", EndDate: "2026-08-02", Metrics: []Metric{"bad-name"}},
		{StartDate: "2026-08-01", EndDate: "2026-08-02", Metrics: []Metric{MetricViews, MetricViews}},
		{StartDate: "2026-08-01", EndDate: "2026-08-02", Metrics: []Metric{MetricViews}, Dimensions: []Dimension{DimensionDay}, Sort: []Sort{{Name: "likes"}}},
		{StartDate: "2026-08-01", EndDate: "2026-08-02", Metrics: []Metric{MetricViews}, Filters: []Filter{{Dimension: DimensionCountry, Values: []string{"US", "CN"}}}},
		{StartDate: "2026-08-01", EndDate: "2026-08-02", Metrics: []Metric{MetricViews}, Currency: "usd"},
		{StartDate: "2026-08-01", EndDate: "2026-08-02", Metrics: []Metric{MetricViews}, MaxResults: -1},
		{StartDate: "2026-08-01", EndDate: "2026-08-02", Metrics: []Metric{MetricViews}, IncludeHistoricalChannelData: true},
	}
	traffic := ReportQuery{
		StartDate: "2026-01-01", EndDate: "2026-04-11", Metrics: []Metric{MetricViews},
		Dimensions: []Dimension{DimensionTrafficSourceType}, Filters: []Filter{{Dimension: DimensionVideo}},
	}
	for index := 0; index < 500; index++ {
		traffic.Filters[0].Values = append(traffic.Filters[0].Values, "video"+strconv.Itoa(index))
	}
	invalidReports = append(invalidReports, traffic)
	for index, input := range invalidReports {
		if _, err := channel.QueryReport(context.Background(), input); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Errorf("report case %d error=%v", index, err)
		}
	}

	invocations := []func() error{
		func() error { _, err := channel.ListGroups(context.Background(), ListGroupsRequest{}); return err },
		func() error {
			_, err := channel.ListGroups(context.Background(), ListGroupsRequest{Mine: true, IDs: []string{"g"}})
			return err
		},
		func() error { _, err := channel.CreateGroup(context.Background(), CreateGroupInput{}); return err },
		func() error {
			_, err := channel.CreateGroup(context.Background(), CreateGroupInput{Title: "Assets", ItemType: ResourceAsset})
			return err
		},
		func() error { _, err := channel.RenameGroup(context.Background(), "bad,id", "Title"); return err },
		func() error { return channel.DeleteGroup(context.Background(), "") },
		func() error { _, err := channel.ListGroupItems(context.Background(), ""); return err },
		func() error { _, err := channel.AddGroupItem(context.Background(), AddGroupItemInput{}); return err },
		func() error { return channel.RemoveGroupItem(context.Background(), "bad,id") },
		func() error {
			_, err := owner.CreateGroup(context.Background(), CreateGroupInput{Title: "Bad", ItemType: ResourceKind("bad")})
			return err
		},
	}
	for index, invoke := range invocations {
		if err := invoke(); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Errorf("workflow case %d error=%v", index, err)
		}
	}
	if requestSeen {
		t.Fatal("invalid workflow reached HTTP server")
	}
}

func TestWorkflowContractValidation(t *testing.T) {
	tests := []struct {
		name     string
		response any
		invoke   func(*Client) error
	}{
		{"report kind", map[string]any{"kind": "wrong"}, func(client *Client) error {
			_, err := client.QueryReport(context.Background(), reportRequest())
			return err
		}},
		{"report headers", func() any { value := reportFixture(); value["columnHeaders"] = []any{}; return value }(), func(client *Client) error {
			_, err := client.QueryReport(context.Background(), reportRequest())
			return err
		}},
		{"report row width", func() any { value := reportFixture(); value["rows"] = []any{[]any{"day"}}; return value }(), func(client *Client) error {
			_, err := client.QueryReport(context.Background(), reportRequest())
			return err
		}},
		{"report type", func() any { value := reportFixture(); value["rows"] = []any{[]any{"day", "10"}}; return value }(), func(client *Client) error {
			_, err := client.QueryReport(context.Background(), reportRequest())
			return err
		}},
		{"report embedded errors", func() any {
			value := reportFixture()
			value["errors"] = map[string]any{"code": "BAD_REQUEST"}
			return value
		}(), func(client *Client) error {
			_, err := client.QueryReport(context.Background(), reportRequest())
			return err
		}},
		{"groups kind", map[string]any{"kind": "wrong"}, func(client *Client) error {
			_, err := client.ListGroups(context.Background(), ListGroupsRequest{Mine: true})
			return err
		}},
		{"group malformed", map[string]any{"kind": "youtube#groupListResponse", "items": []any{map[string]any{"kind": "youtube#group"}}}, func(client *Client) error {
			_, err := client.ListGroups(context.Background(), ListGroupsRequest{Mine: true})
			return err
		}},
		{"group outside requested set", map[string]any{"kind": "youtube#groupListResponse", "items": []any{groupFixture("group2", ResourceVideo)}}, func(client *Client) error {
			_, err := client.ListGroups(context.Background(), ListGroupsRequest{IDs: []string{"group1"}})
			return err
		}},
		{"created group mismatch", func() any {
			value := groupFixture("group1", ResourceVideo)
			value["snippet"].(map[string]any)["title"] = "Other"
			return value
		}(), func(client *Client) error {
			_, err := client.CreateGroup(context.Background(), CreateGroupInput{Title: "Expected", ItemType: ResourceVideo})
			return err
		}},
		{"items wrong group", map[string]any{"kind": "youtube#groupItemListResponse", "items": []any{groupItemFixture("item1", "other", "video1", ResourceVideo)}}, func(client *Client) error {
			_, err := client.ListGroupItems(context.Background(), "group1")
			return err
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
				writeJSON(t, writer, http.StatusOK, test.response)
			}))
			defer server.Close()
			_, client := newStaticClient(t, server, staticConfig(server.URL))
			err := test.invoke(client)
			if requireHubError(t, err).Code != socialhub.CodePlatformError {
				t.Fatalf("error=%v", err)
			}
		})
	}
}

func TestListGroupsByIDs(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Query().Get("id") != "group1,group2" || request.URL.Query().Get("mine") != "" {
			t.Errorf("query=%v", request.URL.Query())
		}
		writeJSON(t, writer, http.StatusOK, map[string]any{
			"kind":  "youtube#groupListResponse",
			"items": []any{groupFixture("group1", ResourceVideo), groupFixture("group2", ResourceVideo)},
		})
	}))
	defer server.Close()
	_, client := newStaticClient(t, server, staticConfig(server.URL))
	response, err := client.ListGroups(context.Background(), ListGroupsRequest{IDs: []string{"group1", "group2"}})
	if err != nil || len(response.Items) != 2 || reflect.DeepEqual(response.Items[0].Raw, response.Items[1].Raw) {
		t.Fatalf("response=%#v err=%v", response, err)
	}
}
