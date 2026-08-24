package admob

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

func TestGoogleErrorClassificationRetryAndRedaction(t *testing.T) {
	body := []byte(`{"error":{"code":429,"message":"access_token=secret-value exhausted","status":"RESOURCE_EXHAUSTED","details":[{"@type":"type.googleapis.com/google.rpc.ErrorInfo","reason":"RATE_LIMIT_EXCEEDED","domain":"admob.googleapis.com","metadata":{"token":"secret"}}]}}`)
	header := http.Header{"X-Goog-Request-Id": {"request-1"}, "Retry-After": {"2"}}
	err := decodeHTTPError(http.StatusTooManyRequests, header, body)
	var api *APIError
	if !errors.As(err, &api) || !api.Retryable() || api.Hub.Code != socialhub.CodeRateLimited || api.Hub.RetryAfter != 2*time.Second ||
		api.Hub.RequestID != "request-1" || strings.Contains(api.Hub.PlatformMessage, "secret-value") || api.Hub.PlatformCode != "RATE_LIMIT_EXCEEDED" ||
		len(api.Google.Details) != 1 || api.Google.Details[0].Metadata != nil {
		t.Fatalf("error=%#v", api)
	}
	if !errors.Is(err, socialhub.ErrRateLimited) || api.Error() == "" || api.Unwrap() == nil {
		t.Fatalf("wrapped error=%v", err)
	}
	if (&APIError{}).Error() == "" || (*APIError)(nil).Error() == "" || (*APIError)(nil).Unwrap() != nil || (*APIError)(nil).Retryable() {
		t.Fatal("nil API error contract failed")
	}
}

func TestErrorClassificationQuotaAndHelpers(t *testing.T) {
	tests := []struct {
		status      int
		rpc, reason string
		code        socialhub.ErrorCode
		class       socialhub.ErrorClass
	}{
		{400, "", "", socialhub.CodeInvalidArgument, socialhub.ClassPermanent},
		{401, "", "", socialhub.CodeUnauthenticated, socialhub.ClassUserAction},
		{403, "", "", socialhub.CodePermissionDenied, socialhub.ClassUserAction},
		{404, "", "", socialhub.CodeNotFound, socialhub.ClassPermanent},
		{409, "", "", socialhub.CodeConflict, socialhub.ClassPermanent},
		{429, "", "", socialhub.CodeRateLimited, socialhub.ClassRetryable},
		{500, "", "", socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable},
		{418, "", "", socialhub.CodePlatformError, socialhub.ClassPermanent},
		{403, "", "ACCESS_TOKEN_SCOPE_INSUFFICIENT", socialhub.CodePermissionDenied, socialhub.ClassUserAction},
		{409, "ABORTED", "", socialhub.CodeConflict, socialhub.ClassPermanent},
		{503, "DEADLINE_EXCEEDED", "", socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable},
		{429, "", "QUOTA_EXCEEDED", socialhub.CodeRateLimited, socialhub.ClassRetryable},
	}
	for _, test := range tests {
		code, class := classifyError(test.status, test.rpc, test.reason)
		if code != test.code || class != test.class {
			t.Errorf("status=%d rpc=%q reason=%q got=%s/%s", test.status, test.rpc, test.reason, code, class)
		}
	}
	policy := DefaultQuotaPolicy()
	if policy.AccountReadsPerMinutePerProject != 900 || policy.InventoryReadsPerMinutePerProject != 120 ||
		policy.InventoryReadsPerDayPerProject != 172_800 || policy.ReportingReadsPerMinutePerProject != 900 ||
		policy.DefaultInventoryPageSize != 10_000 || policy.MaximumInventoryPageSize != 20_000 || policy.MaximumReportRows != 100_000 {
		t.Fatalf("policy=%#v", policy)
	}
	if parseRetryAfter("3") != 3*time.Second || parseRetryAfter("bad") != 0 || firstNonEmpty("", "value") != "value" ||
		redactSensitive("client_secret: topsecret") == "client_secret: topsecret" {
		t.Fatal("error helpers failed")
	}
	if !errors.Is(ownershipError("test", "app"), socialhub.ErrPermissionDenied) || requireHubError(t, platformContractError("test", "bad")).Code != socialhub.CodePlatformError {
		t.Fatal("typed errors failed")
	}
}

func TestValidationPrimitivesAndReportSpecs(t *testing.T) {
	if !validPublisherID(testPublisherID) || validPublisherID("pub-x") || !digits("0123", 4) || digits("12x", 4) ||
		!validCurrency("USD", true) || validCurrency("usd", true) || !validLanguageCode("zh-CN", true) || validLanguageCode("bad code", true) ||
		!validOutputTimeZone("Asia/Shanghai") || validOutputTimeZone("../secret") || !validEndpoint("https://example.com") || validEndpoint("https://example.com/") {
		t.Fatal("validation primitives failed")
	}
	validNetwork := validNetworkSpec()
	if !validNetworkReportSpec(validNetwork) || !validMediationReportSpec(validMediationSpec()) {
		t.Fatal("valid report spec rejected")
	}
	invalidNetwork := []NetworkReportSpec{
		{},
		func() NetworkReportSpec { value := validNetwork; value.DateRange.EndDate.Day = 8; return value }(),
		func() NetworkReportSpec {
			value := validNetwork
			value.Dimensions = []Dimension{DimensionDate, DimensionMonth}
			return value
		}(),
		func() NetworkReportSpec {
			value := validNetwork
			value.Dimensions = []Dimension{DimensionDate, DimensionDate}
			return value
		}(),
		func() NetworkReportSpec { value := validNetwork; value.Metrics = []Metric{"UNKNOWN"}; return value }(),
		func() NetworkReportSpec {
			value := validNetwork
			value.Dimensions = []Dimension{DimensionAdType}
			value.Metrics = []Metric{MetricAdRequests}
			return value
		}(),
		func() NetworkReportSpec {
			value := validNetwork
			value.DimensionFilters = []DimensionFilter{{Dimension: DimensionCountry}}
			return value
		}(),
		func() NetworkReportSpec {
			value := validNetwork
			value.SortConditions = []SortCondition{{Dimension: DimensionDate, Metric: MetricClicks, Order: SortAscending}}
			return value
		}(),
		func() NetworkReportSpec {
			value := validNetwork
			value.SortConditions = []SortCondition{{Metric: MetricClicks, Order: "SIDEWAYS"}}
			return value
		}(),
		func() NetworkReportSpec { value := validNetwork; value.TimeZone = "Asia/Shanghai"; return value }(),
		func() NetworkReportSpec { value := validNetwork; value.MaxReportRows = 100_001; return value }(),
		func() NetworkReportSpec {
			value := validNetwork
			value.LocalizationSettings.CurrencyCode = "usd"
			return value
		}(),
	}
	for index, value := range invalidNetwork {
		if validNetworkReportSpec(value) {
			t.Errorf("invalid network spec %d accepted: %#v", index, value)
		}
	}
	invalidMediation := validMediationSpec()
	invalidMediation.Dimensions = []Dimension{DimensionMobileOSVersion}
	invalidMediation.Metrics = []Metric{MetricObservedECPM}
	if validMediationReportSpec(invalidMediation) {
		t.Fatal("incompatible mediation spec accepted")
	}
	transportErr := &url.Error{Op: "Get", URL: "https://example.com?token=secret", Err: errors.New("dial failed")}
	if sanitizeCause(transportErr).Error() != "dial failed" || sanitizeCause(nil) != nil {
		t.Fatal("cause sanitization failed")
	}
}

func TestInvalidCallsDoNotReachNetwork(t *testing.T) {
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { requestCount++ }))
	defer server.Close()
	_, client := newStaticClient(t, server)
	ctx := context.Background()
	for _, invoke := range []func() error{
		func() error { _, err := client.ListAccounts(ctx, ListRequest{PageSize: -1}); return err },
		func() error { _, err := client.ListApps(ctx, ListRequest{PageSize: 20_001}); return err },
		func() error { _, err := client.ListAdUnits(ctx, ListRequest{PageToken: "bad\ntoken"}); return err },
		func() error { _, err := client.GenerateNetworkReport(ctx, NetworkReportSpec{}); return err },
		func() error { _, err := client.GenerateMediationReport(ctx, MediationReportSpec{}); return err },
	} {
		if err := invoke(); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Errorf("validation error=%v", err)
		}
	}
	if requestCount != 0 {
		t.Fatalf("invalid calls made %d requests", requestCount)
	}
}

func TestResourceOwnershipAndPaginationFailures(t *testing.T) {
	tests := []struct {
		name     string
		response any
		invoke   func(*Client) error
		code     socialhub.ErrorCode
	}{
		{"account", PublisherAccount{Name: "accounts/pub-999", PublisherID: "pub-999", ReportingTimeZone: "UTC", CurrencyCode: "USD"}, func(client *Client) error {
			_, err := client.GetAccount(context.Background())
			return err
		}, socialhub.CodePermissionDenied},
		{"apps", map[string]any{"apps": []App{{Name: "accounts/pub-999/apps/1", AppID: "ca-app-pub-999~1", Platform: AppPlatformAndroid, AppApprovalState: AppApprovalApproved}}}, func(client *Client) error {
			_, err := client.ListApps(context.Background(), ListRequest{})
			return err
		}, socialhub.CodePermissionDenied},
		{"ad units", map[string]any{"adUnits": []AdUnit{{Name: "accounts/pub-999/adUnits/1", AdUnitID: "ca-app-pub-999/1", AppID: "ca-app-pub-999~2", DisplayName: "Ad", AdFormat: AdFormatBanner, AdTypes: []AdType{AdTypeRichMedia}}}}, func(client *Client) error {
			_, err := client.ListAdUnits(context.Background(), ListRequest{})
			return err
		}, socialhub.CodePermissionDenied},
		{"duplicate accounts", map[string]any{"account": []PublisherAccount{accountFixture(), accountFixture()}}, func(client *Client) error {
			_, err := client.ListAccounts(context.Background(), ListRequest{})
			return err
		}, socialhub.CodePlatformError},
		{"bad token", map[string]any{"apps": []App{}, "nextPageToken": "bad\ntoken"}, func(client *Client) error {
			_, err := client.ListApps(context.Background(), ListRequest{})
			return err
		}, socialhub.CodePlatformError},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
				writeJSON(t, writer, http.StatusOK, test.response)
			}))
			defer server.Close()
			_, client := newStaticClient(t, server)
			if err := test.invoke(client); requireHubError(t, err).Code != test.code {
				t.Fatalf("error=%v", err)
			}
		})
	}
}

func TestRawCaptureAndMetricJSON(t *testing.T) {
	var account PublisherAccount
	if err := json.Unmarshal([]byte(`{"name":"accounts/pub-1","publisherId":"pub-1"}`), &account); err != nil || len(account.Raw) == 0 {
		t.Fatalf("account=%#v err=%v", account, err)
	}
	var app App
	if err := json.Unmarshal([]byte(`{"name":"accounts/pub-1/apps/1"}`), &app); err != nil || len(app.Raw) == 0 {
		t.Fatalf("app=%#v err=%v", app, err)
	}
	var adUnit AdUnit
	if err := json.Unmarshal([]byte(`{"name":"accounts/pub-1/adUnits/1"}`), &adUnit); err != nil || len(adUnit.Raw) == 0 {
		t.Fatalf("adUnit=%#v err=%v", adUnit, err)
	}
}
