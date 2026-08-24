package youtubeanalytics

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

func TestHTTPErrorMappingSanitizationAndRetry(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Retry-After", "2.5")
		writer.Header().Set("x-goog-request-id", "request-123")
		writeJSON(t, writer, http.StatusForbidden, map[string]any{
			"error": map[string]any{
				"code": 403, "message": "quota failed access_token=secret", "status": "PERMISSION_DENIED",
				"errors": []any{map[string]any{"reason": "quotaExceeded", "domain": "youtubeAnalytics", "message": "retry"}},
			},
		})
	}))
	defer server.Close()
	_, client := newStaticClient(t, server, staticConfig(server.URL))
	_, err := client.QueryReport(context.Background(), reportRequest())
	if !errors.Is(err, socialhub.ErrRateLimited) {
		t.Fatalf("error=%v", err)
	}
	var api *APIError
	if !errors.As(err, &api) || api.Hub.Op != "report_query" || api.Hub.RequestID != "request-123" ||
		api.Hub.RetryAfter != 2500*time.Millisecond || !api.Retryable() || strings.Contains(api.Google.Message, "secret") {
		t.Fatalf("api error=%#v", api)
	}
	if api.Error() == "" || (&APIError{}).Error() == "" || (*APIError)(nil).Unwrap() != nil {
		t.Fatal("APIError methods failed")
	}
}

func TestErrorClassification(t *testing.T) {
	tests := []struct {
		status int
		state  string
		reason string
		code   socialhub.ErrorCode
		class  socialhub.ErrorClass
	}{
		{403, "", "dailyLimitExceeded", socialhub.CodeRateLimited, socialhub.ClassUserAction},
		{401, "", "authError", socialhub.CodeUnauthenticated, socialhub.ClassUserAction},
		{403, "", "insufficientPermissions", socialhub.CodePermissionDenied, socialhub.ClassUserAction},
		{404, "", "groupItemNotFound", socialhub.CodeNotFound, socialhub.ClassPermanent},
		{403, "", "groupContainsMaximumNumberOfItems", socialhub.CodeConflict, socialhub.ClassUserAction},
		{503, "", "backendError", socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable},
		{400, "", "invalidArgument", socialhub.CodeInvalidArgument, socialhub.ClassPermanent},
		{429, "RESOURCE_EXHAUSTED", "", socialhub.CodeRateLimited, socialhub.ClassRetryable},
		{401, "UNAUTHENTICATED", "", socialhub.CodeUnauthenticated, socialhub.ClassUserAction},
		{403, "PERMISSION_DENIED", "", socialhub.CodePermissionDenied, socialhub.ClassUserAction},
		{404, "NOT_FOUND", "", socialhub.CodeNotFound, socialhub.ClassPermanent},
		{409, "ALREADY_EXISTS", "", socialhub.CodeConflict, socialhub.ClassPermanent},
		{503, "UNAVAILABLE", "", socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable},
		{400, "FAILED_PRECONDITION", "", socialhub.CodeInvalidArgument, socialhub.ClassPermanent},
		{422, "", "", socialhub.CodeInvalidArgument, socialhub.ClassPermanent},
		{410, "", "", socialhub.CodeNotFound, socialhub.ClassPermanent},
		{418, "", "", socialhub.CodePlatformError, socialhub.ClassPermanent},
	}
	for _, test := range tests {
		code, class := classifyError(test.status, test.state, test.reason)
		if code != test.code || class != test.class {
			t.Errorf("status=%d state=%q reason=%q got=%s/%s", test.status, test.state, test.reason, code, class)
		}
	}
	if parseRetryAfter("bad") != 0 || parseRetryAfter("-1") != 0 || parseRetryAfter("999999") != 0 {
		t.Fatal("invalid Retry-After accepted")
	}
	if boundedMessage("abcdef", 3) != "abc" || firstNonEmpty("", " value ") != " value " || firstNonEmpty("") != "" {
		t.Fatal("bounded helpers failed")
	}
	redacted := redactSensitive("client_secret=abc bearer xyz refresh_token: def")
	if strings.Contains(redacted, "abc") || strings.Contains(redacted, "xyz") || strings.Contains(redacted, "def") {
		t.Fatalf("redacted=%q", redacted)
	}
}

func TestValidationHelpers(t *testing.T) {
	if !validEndpoint("https://example.com") || validEndpoint("https://user@example.com") || validEndpoint("https://example.com/") ||
		!validCallbackURL("https://app.example/callback?tenant=1") || validCallbackURL("javascript:alert(1)") ||
		!validOpaque("value", 10) || validOpaque(" bad ", 10) || !validAccountBinding(AccountSettings{ChannelID: "MINE"}) ||
		validAccountBinding(AccountSettings{ChannelID: "MINE", ContentOwnerID: "owner"}) || validIdentifier("bad/id", 100) ||
		!validOpaqueID("group/id=opaque") || validOpaqueID("group,id") {
		t.Fatal("endpoint, callback, opaque, or binding validation failed")
	}
	if !validOAuthScopes([]string{youtubeReadOnlyScope, analyticsReadScope}) || validOAuthScopes(nil) ||
		validOAuthScopes([]string{youtubeScope, youtubeScope}) || validOAuthScopes([]string{"openid"}) {
		t.Fatal("scope validation failed")
	}
	if date, ok := validDate("2026-08-10"); !ok || date.Day() != 10 {
		t.Fatal("date validation failed")
	}
	if _, ok := validDate("2026-8-10"); ok || !validCurrency("") || !validCurrency("CNY") || validCurrency("cny") {
		t.Fatal("date or currency validation failed")
	}
	if !validFilterValue("UC_test-123") || validFilterValue("US,CN") || validFilterValue("country==US") ||
		!validResourceKind(ResourceAsset, true) || validResourceKind(ResourceAsset, false) || validResourceKind(ResourceChannel, false) || validResourceKind("unknown", true) {
		t.Fatal("filter or resource-kind validation failed")
	}
	policy := DefaultQuotaPolicy()
	if policy.MaximumGroupItems != 500 || policy.MaximumFilterIDs != 500 || policy.MaximumTrafficSourceCost != 50_000 {
		t.Fatalf("quota=%#v", policy)
	}
}

func TestReportResponseCellAndLimitValidation(t *testing.T) {
	input := ReportQuery{
		StartDate: "2026-08-01", EndDate: "2026-08-02",
		Dimensions: []Dimension{"flag"}, Metrics: []Metric{"count", "ratio"}, MaxResults: 1,
	}
	data := []byte(`{"kind":"youtubeAnalytics#resultTable","columnHeaders":[{"name":"flag","columnType":"DIMENSION","dataType":"BOOLEAN"},{"name":"count","columnType":"METRIC","dataType":"INTEGER"},{"name":"ratio","columnType":"METRIC","dataType":"FLOAT"}],"rows":[[true,10,1.5]]}`)
	var report Report
	if err := json.Unmarshal(data, &report); err != nil || !validReportResponse(&report, input) {
		t.Fatalf("report=%#v err=%v", report, err)
	}
	report.Rows[0][1] = json.Number("1.5")
	if validReportResponse(&report, input) {
		t.Fatal("fractional integer accepted")
	}
	report.Rows[0][1] = json.Number("10")
	report.Rows = append(report.Rows, append([]any(nil), report.Rows[0]...))
	if validReportResponse(&report, input) {
		t.Fatal("row limit exceeded")
	}
	report.Rows = report.Rows[:1]
	report.ColumnHeaders[2].DataType = "DECIMAL"
	if validReportResponse(&report, input) || validCell(DataString, 1) || validCell(DataBoolean, "true") || validCell("bad", "x") {
		t.Fatal("invalid data type accepted")
	}
}

func TestGroupResponseValidation(t *testing.T) {
	decodeGroup := func(value any) Group {
		data, _ := json.Marshal(value)
		var output Group
		if err := json.Unmarshal(data, &output); err != nil {
			t.Fatal(err)
		}
		return output
	}
	valid := decodeGroup(groupFixture("group1", ResourceVideo))
	if !validGroup(&valid, false) || len(valid.Raw) == 0 {
		t.Fatalf("valid group rejected=%#v", valid)
	}
	invalidFixtures := []any{
		map[string]any{"id": "group1", "kind": "wrong"},
		func() any {
			value := groupFixture("group1", ResourceVideo)
			value["snippet"].(map[string]any)["publishedAt"] = "bad"
			return value
		}(),
		func() any {
			value := groupFixture("group1", ResourceVideo)
			value["contentDetails"].(map[string]any)["itemCount"] = "501"
			return value
		}(),
		func() any { value := groupFixture("group1", ResourceAsset); return value }(),
		func() any {
			value := groupFixture("group1", ResourceVideo)
			value["errors"] = map[string]any{"code": "BAD_REQUEST"}
			return value
		}(),
	}
	for index, fixture := range invalidFixtures {
		value := decodeGroup(fixture)
		if validGroup(&value, false) {
			t.Errorf("case %d accepted", index)
		}
	}

	groupsJSON, _ := json.Marshal(map[string]any{
		"kind": "youtube#groupListResponse", "items": []any{groupFixture("group1", ResourceVideo), groupFixture("group1", ResourceVideo)},
	})
	var groups ListGroupsResponse
	if json.Unmarshal(groupsJSON, &groups) != nil || validGroupsResponse(&groups, false) {
		t.Fatal("duplicate groups accepted")
	}
	itemsJSON, _ := json.Marshal(map[string]any{
		"kind": "youtube#groupItemListResponse", "items": []any{
			groupItemFixture("item1", "group1", "video1", ResourceVideo),
			groupItemFixture("item1", "group1", "video2", ResourceVideo),
		},
	})
	var items ListGroupItemsResponse
	if json.Unmarshal(itemsJSON, &items) != nil || validGroupItemsResponse(&items, false) {
		t.Fatal("duplicate items accepted")
	}
}

func TestSuccessfulMalformedJSONAndEmbeddedGoogleError(t *testing.T) {
	responses := []string{
		`not-json`,
		`{"kind":"youtubeAnalytics#resultTable","errors":{"code":"BAD_REQUEST","requestId":"r","error":[{"domain":"youtube","code":"bad"}]}}`,
	}
	for _, response := range responses {
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
			writer.WriteHeader(http.StatusOK)
			_, _ = writer.Write([]byte(response))
		}))
		_, client := newStaticClient(t, server, staticConfig(server.URL))
		_, err := client.QueryReport(context.Background(), reportRequest())
		if requireHubError(t, err).Code != socialhub.CodePlatformError {
			t.Errorf("response=%q error=%v", response, err)
		}
		server.Close()
	}
}
