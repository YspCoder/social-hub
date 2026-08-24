package appleads

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

func TestHTTPAndBusinessErrorClassification(t *testing.T) {
	tests := []struct {
		status    int
		code      socialhub.ErrorCode
		sentinel  error
		retryable bool
	}{
		{http.StatusBadRequest, socialhub.CodeInvalidArgument, socialhub.ErrInvalidArgument, false},
		{http.StatusUnauthorized, socialhub.CodeUnauthenticated, socialhub.ErrUnauthenticated, false},
		{http.StatusForbidden, socialhub.CodePermissionDenied, socialhub.ErrPermissionDenied, false},
		{http.StatusNotFound, socialhub.CodeNotFound, socialhub.ErrNotFound, false},
		{http.StatusConflict, socialhub.CodeConflict, socialhub.ErrConflict, false},
		{http.StatusTooManyRequests, socialhub.CodeRateLimited, socialhub.ErrRateLimited, true},
		{http.StatusServiceUnavailable, socialhub.CodeTemporarilyUnavailable, socialhub.ErrUnavailable, true},
		{http.StatusTeapot, socialhub.CodePlatformError, nil, false},
	}
	for _, test := range tests {
		t.Run(fmt.Sprint(test.status), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				writer.Header().Set("Retry-After", "2.5")
				writer.Header().Set("x-apple-request-uuid", "apple-request")
				writeJSON(t, writer, test.status, map[string]any{"error": map[string]any{"errors": []ErrorItem{{
					MessageCode: "AUTH", Message: "access_token secret-value", Field: "authorization",
				}}}})
			}))
			defer server.Close()
			_, client := newStaticClient(t, server)
			_, err := client.ListACL(context.Background(), Pagination{Limit: 1})
			var api *APIError
			if !errors.As(err, &api) || api.Hub.Code != test.code || api.Hub.Op != "acl_list" || api.Hub.RequestID != "apple-request" ||
				api.Retryable() != test.retryable || strings.Contains(api.Errors[0].Message, "secret-value") || strings.Contains(api.Error(), "secret-value") {
				t.Fatalf("API error=%#v err=%v", api, err)
			}
			if test.sentinel != nil && !errors.Is(err, test.sentinel) {
				t.Fatalf("error %v does not match %v", err, test.sentinel)
			}
			if test.status == http.StatusTooManyRequests && api.Hub.RetryAfter != 2500*time.Millisecond {
				t.Fatalf("RetryAfter=%v", api.Hub.RetryAfter)
			}
		})
	}

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writeJSON(t, writer, http.StatusOK, map[string]any{
			"data":  []UserACL(nil),
			"error": map[string]any{"errors": []ErrorItem{{MessageCode: "DUPLICATE", Message: "duplicate", Field: "name"}}},
		})
	}))
	defer server.Close()
	_, client := newStaticClient(t, server)
	_, err := client.ListACL(context.Background(), Pagination{Limit: 1})
	var api *APIError
	if !errors.As(err, &api) || api.Hub.Op != "acl_list" || api.Hub.Code != socialhub.CodeInvalidArgument {
		t.Fatalf("business error=%#v err=%v", api, err)
	}
}

func TestErrorHelpersAndRedaction(t *testing.T) {
	var nilAPI *APIError
	if nilAPI.Error() == "" || nilAPI.Unwrap() != nil || nilAPI.Retryable() {
		t.Fatal("nil APIError contract failed")
	}
	api := &APIError{Hub: &socialhub.Error{Code: socialhub.CodeRateLimited, Class: socialhub.ClassRetryable}}
	if api.Unwrap() == nil || !api.Retryable() || api.Error() == "" {
		t.Fatal("APIError contract failed")
	}
	if parseRetryAfter("-1") != 0 || parseRetryAfter("999999") != 0 || parseRetryAfter("bad") != 0 {
		t.Fatal("invalid Retry-After was accepted")
	}
	future := time.Now().Add(2 * time.Minute).UTC().Format(http.TimeFormat)
	if delay := parseRetryAfter(future); delay < time.Minute || delay > 3*time.Minute {
		t.Fatalf("date Retry-After=%v", delay)
	}
	past := time.Now().Add(-time.Hour).UTC().Format(http.TimeFormat)
	if parseRetryAfter(past) != 0 {
		t.Fatal("past Retry-After was accepted")
	}
	message := `client_secret="topsecret" private_key abc access_token=token authorization: Bearer hidden`
	redacted := redactSensitive(message)
	for _, secret := range []string{"topsecret", "abc", "token", "hidden"} {
		if strings.Contains(redacted, secret) {
			t.Fatalf("redacted=%q still contains %q", redacted, secret)
		}
	}
	if got := boundedMessage(strings.Repeat("界", 20), 7); got != strings.Repeat("界", 7) {
		t.Fatalf("bounded=%q", got)
	}
	inner := errors.New("dial failed")
	if sanitizeCause(&url.Error{Op: "Get", URL: "https://user:pass@example.com", Err: inner}) != inner || sanitizeCause(inner) != inner {
		t.Fatal("transport cause sanitization failed")
	}
	items := sanitizeErrorItems([]ErrorItem{{MessageCode: strings.Repeat("x", 300), Message: message, Field: strings.Repeat("f", 300)}})
	if len(items[0].MessageCode) != 256 || len(items[0].Field) != 256 || strings.Contains(items[0].Message, "topsecret") {
		t.Fatalf("items=%#v", items)
	}
	if firstNonEmpty("", "  ", "value") != "value" || firstNonEmpty("", "") != "" {
		t.Fatal("firstNonEmpty failed")
	}
}

func TestValidationBoundaries(t *testing.T) {
	if !validMoney(Money{Amount: "1.25", Currency: "USD"}) || validMoney(Money{Amount: "-1", Currency: "USD"}) ||
		validMoney(Money{Amount: " 1", Currency: "USD"}) || validMoney(Money{Amount: "1", Currency: "usd"}) ||
		validMoney(Money{Amount: "not-number", Currency: "USD"}) {
		t.Fatal("Money validation failed")
	}
	if !validPositiveMoney(nil) || validPositiveMoney(&Money{Amount: "0", Currency: "USD"}) {
		t.Fatal("positive Money validation failed")
	}
	valid := Selector{
		Conditions: []Condition{{Field: "status", Operator: "IN", Values: []any{"PAUSED"}}},
		Fields:     []string{"id"}, OrderBy: []Sorting{{Field: "id", SortOrder: SortAscending}},
		Pagination: Pagination{Limit: 20},
	}
	if !validSelector(valid) {
		t.Fatal("valid Selector rejected")
	}
	selectorTests := []Selector{
		{Pagination: Pagination{}},
		{Pagination: Pagination{Limit: 20}, Conditions: []Condition{{Field: "", Operator: "IN", Values: []any{1}}}},
		{Pagination: Pagination{Limit: 20}, Conditions: []Condition{{Field: "id", Operator: "", Values: []any{1}}}},
		{Pagination: Pagination{Limit: 20}, Conditions: []Condition{{Field: "id", Operator: "IN"}}},
		{Pagination: Pagination{Limit: 20}, Fields: []string{""}},
		{Pagination: Pagination{Limit: 20}, OrderBy: []Sorting{{Field: "id", SortOrder: "SIDEWAYS"}}},
		{Pagination: Pagination{Limit: 20}, OrderBy: []Sorting{{Field: "id", SortOrder: SortAscending}, {Field: "name", SortOrder: SortDescending}}},
	}
	for _, selector := range selectorTests {
		if validSelector(selector) {
			t.Fatalf("invalid Selector accepted: %#v", selector)
		}
	}
	if !validDate("2026-08-09") || validDate("2026-13-40") || !validDate("") ||
		!validDateTime("2026-08-09T01:02:03Z") || !validDateTime("2026-08-09T01:02:03.000") || validDateTime("2026-08-09") ||
		!validRawObject([]byte(`{"age":null}`)) || validRawObject([]byte(`[]`)) || validRawObject([]byte(`bad`)) {
		t.Fatal("date or raw object validation failed")
	}
	if !validCountries([]string{"US", "CN"}) || validCountries(nil) || validCountries([]string{"us"}) || validCountries([]string{"US", "US"}) {
		t.Fatal("country validation failed")
	}

	report := ReportingRequest{
		StartTime: "2026-08-01", EndTime: "2026-08-02", Selector: Selector{Pagination: Pagination{Limit: 20}}, ReturnRowTotals: true,
	}
	if !validReportRequest(report) {
		t.Fatal("valid report rejected")
	}
	reportTests := []ReportingRequest{
		{},
		{StartTime: "2026-08-03", EndTime: "2026-08-02", Selector: Selector{Pagination: Pagination{Limit: 20}}, ReturnRowTotals: true},
		{StartTime: "2026-08-01", EndTime: "2026-08-02", Selector: Selector{Pagination: Pagination{Limit: 20}}, Granularity: "YEARLY"},
		{StartTime: "2026-08-01", EndTime: "2026-08-02", Selector: Selector{Pagination: Pagination{Limit: 20}}, TimeZone: "LOCAL", ReturnRowTotals: true},
		{StartTime: "2026-08-01", EndTime: "2026-08-02", Selector: Selector{Pagination: Pagination{Limit: 20}}, Granularity: GranularityDaily, ReturnRowTotals: true},
		{StartTime: "2026-08-01", EndTime: "2026-08-02", Selector: Selector{Pagination: Pagination{Limit: 20}}},
		{StartTime: "2026-08-01", EndTime: "2026-08-02", Selector: Selector{Pagination: Pagination{Limit: 20}}, ReturnRowTotals: true, GroupBy: []string{"unknown"}},
		{StartTime: "2026-08-01", EndTime: "2026-08-02", Selector: Selector{Pagination: Pagination{Limit: 20}}, ReturnRowTotals: true, GroupBy: []string{"gender", "gender"}},
	}
	for _, input := range reportTests {
		if validReportRequest(input) {
			t.Fatalf("invalid report accepted: %#v", input)
		}
	}
}

func TestPaidMediaInputValidation(t *testing.T) {
	validCampaign := CreateCampaignRequest{
		Name: "Search", AdamID: testAdamID, DailyBudgetAmount: Money{Amount: "10", Currency: "USD"},
		BudgetAmount: &Money{Amount: "100", Currency: "USD"}, BillingEvent: "TAPS",
		SupplySources: []string{"APPSTORE_SEARCH_RESULTS"}, CountriesOrRegions: []string{"US"},
		AdChannelType: "SEARCH", BiddingStrategy: "MANUAL_CPT",
	}
	if err := validateCreateCampaign(validCampaign); err != nil {
		t.Fatal(err)
	}
	invalidCampaigns := []CreateCampaignRequest{
		{},
		func() CreateCampaignRequest { value := validCampaign; value.BillingEvent = "CLICKS"; return value }(),
		func() CreateCampaignRequest { value := validCampaign; value.AdChannelType = "VIDEO"; return value }(),
		func() CreateCampaignRequest {
			value := validCampaign
			value.SupplySources = []string{"UNKNOWN"}
			return value
		}(),
		func() CreateCampaignRequest {
			value := validCampaign
			value.TargetCPA = &Money{Amount: "1", Currency: "USD"}
			return value
		}(),
		func() CreateCampaignRequest {
			value := validCampaign
			value.BiddingStrategy = "MAX_CONVERSIONS"
			return value
		}(),
		func() CreateCampaignRequest { value := validCampaign; value.BiddingStrategy = "UNKNOWN"; return value }(),
		func() CreateCampaignRequest {
			value := validCampaign
			value.BudgetAmount = &Money{Amount: "5", Currency: "USD"}
			return value
		}(),
		func() CreateCampaignRequest {
			value := validCampaign
			value.BudgetAmount = &Money{Amount: "100", Currency: "EUR"}
			return value
		}(),
	}
	for _, input := range invalidCampaigns {
		if err := validateCreateCampaign(input); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Fatalf("Campaign=%#v error=%v", input, err)
		}
	}
	maxConversions := validCampaign
	maxConversions.BiddingStrategy = "MAX_CONVERSIONS"
	maxConversions.TargetCPA = &Money{Amount: "2", Currency: "USD"}
	if err := validateCreateCampaign(maxConversions); err != nil {
		t.Fatalf("MAX_CONVERSIONS error=%v", err)
	}
	strategy := "MANUAL_CPT"
	if !validUpdateCampaign(UpdateCampaignRequest{Name: stringPointer("new")}) || validUpdateCampaign(UpdateCampaignRequest{}) ||
		validUpdateCampaign(UpdateCampaignRequest{Name: stringPointer("")}) || !validUpdateCampaign(UpdateCampaignRequest{BiddingStrategy: &strategy}) {
		t.Fatal("Campaign update validation failed")
	}

	validGroup := CreateAdGroupRequest{Name: "Group", PricingModel: "CPC", DefaultBidAmount: &Money{Amount: "1", Currency: "USD"}}
	if !validCreateAdGroup(validGroup) || validCreateAdGroup(CreateAdGroupRequest{}) ||
		validCreateAdGroup(CreateAdGroupRequest{Name: "Group", PricingModel: "CPM", DefaultBidAmount: validGroup.DefaultBidAmount}) ||
		validCreateAdGroup(CreateAdGroupRequest{Name: "Group", PricingModel: "CPC", AutomatedKeywordsRequired: true}) {
		t.Fatal("Ad Group create validation failed")
	}
	optIn := true
	if !validUpdateAdGroup(UpdateAdGroupRequest{AutomatedKeywordsOptIn: &optIn}) || validUpdateAdGroup(UpdateAdGroupRequest{}) ||
		validUpdateAdGroup(UpdateAdGroupRequest{TargetingDimensions: []byte(`[]`)}) {
		t.Fatal("Ad Group update validation failed")
	}

	if !validCreateKeywords([]CreateKeywordRequest{{Text: "travel", MatchType: MatchBroad}}) || validCreateKeywords(nil) ||
		validCreateKeywords([]CreateKeywordRequest{{Text: "", MatchType: MatchBroad}}) ||
		validCreateKeywords([]CreateKeywordRequest{{Text: "travel", MatchType: "FUZZY"}}) {
		t.Fatal("Keyword create validation failed")
	}
	paused := KeywordPaused
	if !validUpdateKeywords([]UpdateKeywordRequest{{ID: 1, Status: &paused}}) || validUpdateKeywords(nil) ||
		validUpdateKeywords([]UpdateKeywordRequest{{ID: 1}}) ||
		validUpdateKeywords([]UpdateKeywordRequest{{ID: 1, Status: &paused}, {ID: 1, Status: &paused}}) {
		t.Fatal("Keyword update validation failed")
	}
	if !validCreateCreative(CreateCreativeRequest{AdamID: 1, Name: "Default", Type: CreativeDefaultProductPage}) ||
		!validCreateCreative(CreateCreativeRequest{AdamID: 1, Name: "Custom", Type: CreativeCustomProductPage, ProductPageID: "page"}) ||
		validCreateCreative(CreateCreativeRequest{AdamID: 1, Name: "Custom", Type: CreativeCustomProductPage}) ||
		validCreateCreative(CreateCreativeRequest{AdamID: 1, Name: "Unknown", Type: "UNKNOWN"}) {
		t.Fatal("Creative validation failed")
	}
}

func TestResponseOwnershipValidators(t *testing.T) {
	client := &Client{orgID: testOrgID}
	checks := []error{
		client.validateCampaign("test", nil, 0),
		client.validateCampaign("test", &Campaign{ID: 1, OrgID: 999}, 0),
		client.validateCampaign("test", &Campaign{ID: 1, OrgID: testOrgID}, 2),
		client.validateAdGroup("test", &AdGroup{ID: 1, OrgID: 999, CampaignID: 2}, 2, 0),
		client.validateAdGroup("test", &AdGroup{ID: 1, OrgID: testOrgID, CampaignID: 2}, 2, 3),
		client.validateKeyword("test", &Keyword{ID: 1, CampaignID: 2, AdGroupID: 3}, 9, 3, 0),
		client.validateKeyword("test", &Keyword{ID: 1, CampaignID: 2, AdGroupID: 3}, 2, 3, 4),
		client.validateCreative("test", &Creative{ID: 1, OrgID: 999, AdamID: 2}, 0),
		client.validateCreative("test", &Creative{ID: 1, OrgID: testOrgID, AdamID: 2}, 3),
		client.validateAd("test", &Ad{ID: 1, OrgID: 999, CampaignID: 2, AdGroupID: 3, CreativeID: 4}, 2, 3, 0),
		client.validateAd("test", &Ad{ID: 1, OrgID: testOrgID, CampaignID: 2, AdGroupID: 3, CreativeID: 4}, 2, 3, 9),
	}
	for _, err := range checks {
		if requireHubError(t, err).Code != socialhub.CodePlatformError {
			t.Fatalf("error=%v", err)
		}
	}
}

func TestCampaignEnableAuditRefusals(t *testing.T) {
	tests := []struct {
		name   string
		groups []AdGroup
		total  int64
	}{
		{"empty", nil, 0},
		{"incomplete", []AdGroup{testAdGroup(AdGroupPaused)}, 1001},
		{"enabled child", []AdGroup{testAdGroup(AdGroupEnabled)}, 1},
		{"deleted child", []AdGroup{func() AdGroup { group := testAdGroup(AdGroupPaused); group.Deleted = true; return group }()}, 1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var writes atomic.Int64
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				if request.Method != http.MethodGet {
					writes.Add(1)
				}
				switch request.URL.Path {
				case "/api/v5/campaigns/2001":
					writeJSON(t, writer, http.StatusOK, envelope(testCampaign(CampaignPaused)))
				case "/api/v5/campaigns/2001/adgroups":
					writeJSON(t, writer, http.StatusOK, pagedEnvelope(test.groups, test.total))
				default:
					http.NotFound(writer, request)
				}
			}))
			defer server.Close()
			_, client := newStaticClient(t, server)
			if _, err := client.SetCampaignEnabled(context.Background(), testCampaignID, true); !errors.Is(err, socialhub.ErrInvalidArgument) || writes.Load() != 0 {
				t.Fatalf("error=%v writes=%d", err, writes.Load())
			}
		})
	}
}

func TestParentStateAndDeleteGatesIssueNoWrites(t *testing.T) {
	campaign := testCampaign(CampaignEnabled)
	group := testAdGroup(AdGroupEnabled)
	keyword := testKeyword(KeywordActive)
	creative := testCreative("INVALID")
	ad := testAd(AdEnabled)
	var writes atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet {
			writes.Add(1)
			http.Error(writer, "unexpected write", http.StatusInternalServerError)
			return
		}
		switch request.URL.Path {
		case "/api/v5/campaigns/2001":
			writeJSON(t, writer, http.StatusOK, envelope(campaign))
		case "/api/v5/campaigns/2001/adgroups/3001":
			writeJSON(t, writer, http.StatusOK, envelope(group))
		case "/api/v5/campaigns/2001/adgroups/3001/targetingkeywords/4001":
			writeJSON(t, writer, http.StatusOK, envelope(keyword))
		case "/api/v5/creatives/5001":
			writeJSON(t, writer, http.StatusOK, envelope(creative))
		case "/api/v5/campaigns/2001/adgroups/3001/ads/6001":
			writeJSON(t, writer, http.StatusOK, envelope(ad))
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	_, client := newStaticClient(t, server)
	ctx := context.Background()
	assertInvalid := func(err error) {
		t.Helper()
		if !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Fatalf("error=%v", err)
		}
	}

	_, err := client.CreateAdGroup(ctx, testCampaignID, CreateAdGroupRequest{
		Name: "Group", PricingModel: "CPC", DefaultBidAmount: &Money{Amount: "1", Currency: "USD"},
	})
	assertInvalid(err)
	campaign.Status = CampaignPaused
	_, err = client.SetAdGroupEnabled(ctx, testCampaignID, testAdGroupID, true)
	assertInvalid(err)
	campaign.Status, group.Status = CampaignEnabled, AdGroupEnabled
	_, err = client.CreateKeywords(ctx, testCampaignID, testAdGroupID, []CreateKeywordRequest{{Text: "travel", MatchType: MatchBroad}})
	assertInvalid(err)
	campaign.Status = CampaignPaused
	activeKeyword := KeywordActive
	_, err = client.UpdateKeywords(ctx, testCampaignID, testAdGroupID, []UpdateKeywordRequest{{ID: testKeywordID, Status: &activeKeyword}})
	assertInvalid(err)
	campaign.Status = CampaignEnabled
	assertInvalid(client.DeleteKeyword(ctx, testCampaignID, testAdGroupID, testKeywordID))
	_, err = client.CreateAd(ctx, testCampaignID, testAdGroupID, CreateAdRequest{CreativeID: testCreativeID, Name: "Ad"})
	assertInvalid(err)
	group.Status = AdGroupPaused
	_, err = client.CreateAd(ctx, testCampaignID, testAdGroupID, CreateAdRequest{CreativeID: testCreativeID, Name: "Ad"})
	assertInvalid(err)
	creative.State, creative.AdamID = "VALID", testAdamID+1
	_, err = client.CreateAd(ctx, testCampaignID, testAdGroupID, CreateAdRequest{CreativeID: testCreativeID, Name: "Ad"})
	assertInvalid(err)
	campaign.Status = CampaignPaused
	_, err = client.SetAdEnabled(ctx, testCampaignID, testAdGroupID, testAdID, true)
	assertInvalid(err)
	assertInvalid(client.DeleteAd(ctx, testCampaignID, testAdGroupID, testAdID))
	campaign.Status = CampaignEnabled
	assertInvalid(client.DeleteCampaign(ctx, testCampaignID))
	group.Status = AdGroupEnabled
	assertInvalid(client.DeleteAdGroup(ctx, testCampaignID, testAdGroupID))
	if writes.Load() != 0 {
		t.Fatalf("writes=%d", writes.Load())
	}
}

func TestMalformedAndInvalidSuccessResponses(t *testing.T) {
	tests := []struct {
		name string
		body string
		code socialhub.ErrorCode
	}{
		{"malformed", `not-json`, socialhub.CodePlatformError},
		{"invalid ACL", `{"data":[{"orgId":0,"orgName":""}],"error":null}`, socialhub.CodePlatformError},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				writer.Header().Set("Content-Type", "application/json")
				_, _ = writer.Write([]byte(test.body))
			}))
			defer server.Close()
			_, client := newStaticClient(t, server)
			_, err := client.ListACL(context.Background(), Pagination{Limit: 1})
			if requireHubError(t, err).Code != test.code {
				t.Fatalf("error=%v", err)
			}
		})
	}
}
