package criteo

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

func TestHTTPAndBusinessErrorClassification(t *testing.T) {
	tests := []struct {
		status      int
		problemType string
		want        error
		class       socialhub.ErrorClass
	}{
		{http.StatusBadRequest, "validation", socialhub.ErrInvalidArgument, socialhub.ClassPermanent},
		{http.StatusUnauthorized, "authentication", socialhub.ErrUnauthenticated, socialhub.ClassUserAction},
		{http.StatusForbidden, "authorization", socialhub.ErrPermissionDenied, socialhub.ClassUserAction},
		{http.StatusTooManyRequests, "quota", socialhub.ErrRateLimited, socialhub.ClassRetryable},
		{http.StatusServiceUnavailable, "availability", socialhub.ErrUnavailable, socialhub.ClassRetryable},
		{http.StatusGone, "", socialhub.ErrNotFound, socialhub.ClassPermanent},
		{http.StatusConflict, "", socialhub.ErrConflict, socialhub.ClassPermanent},
	}
	for _, test := range tests {
		body := []byte(`{"errors":[{"traceId":"trace-1","type":"` + test.problemType + `","code":"platform-code","title":"title","detail":"detail"}]}`)
		err := decodeHTTPError(test.status, http.Header{"X-Request-Id": {"request-1"}}, body)
		if !errors.Is(err, test.want) {
			t.Errorf("status=%d type=%s error=%v", test.status, test.problemType, err)
			continue
		}
		var api *APIError
		if !errors.As(err, &api) || api.Hub.Class != test.class || api.Hub.PlatformCode != "platform-code" || api.Hub.RequestID != "trace-1" {
			t.Errorf("API error=%#v", api)
		}
	}
	business := businessError("bulk", []Problem{{Type: "access-control", Code: "denied"}})
	if !errors.Is(business, socialhub.ErrPermissionDenied) || requireHubError(t, business).Op != "bulk" {
		t.Fatalf("business error=%v", business)
	}
	unknownServer := decodeHTTPError(http.StatusServiceUnavailable, nil, []byte(`{"errors":[{"type":"unknown"}]}`))
	if !errors.Is(unknownServer, socialhub.ErrUnavailable) {
		t.Fatalf("unknown 503 error=%v", unknownServer)
	}
}

func TestRateLimitHeadersAndSanitization(t *testing.T) {
	reset := time.Now().Add(10 * time.Minute).Unix()
	header := http.Header{
		"Retry-After": {"2"}, "X-Ratelimit-Limit": {"250"}, "X-Ratelimit-Remaining": {"0"},
		"X-Ratelimit-Reset": {strconv.FormatInt(reset, 10)},
	}
	err := newAPIError(http.StatusTooManyRequests, header, []Problem{{
		Type: "quota", Code: "access_token=secret-value", Detail: "Authorization: Bearer secret-token",
	}})
	var api *APIError
	if !errors.As(err, &api) {
		t.Fatal(err)
	}
	if api.Hub.RetryAfter != 2*time.Second || api.RateLimit.Limit != 250 || api.RateLimit.Remaining != 0 || api.RateLimit.ResetAt.Unix() != reset {
		t.Fatalf("API error=%#v", api)
	}
	serialized := api.Hub.PlatformCode + api.Hub.PlatformMessage + api.Error()
	if strings.Contains(serialized, "secret-value") || strings.Contains(serialized, "secret-token") {
		t.Fatalf("secret escaped: %q", serialized)
	}
	if boundedMessage(strings.Repeat("界", 10), 3) != "界界界" {
		t.Fatal("boundedMessage failed")
	}
}

func TestValidationHelpers(t *testing.T) {
	if validEndpoint("https://example.test/") || validEndpoint("https://user:pass@example.test") || !validEndpoint("https://example.test") {
		t.Fatal("endpoint validation failed")
	}
	if validID("0") || validID("12/3") || !validID("123") || validText(" bad ", 10) || !validText("good", 10) {
		t.Fatal("basic validation failed")
	}
	if validSpendLimit(CreateCampaignSpendLimit{Type: SpendLimitCapped}) ||
		!validSpendLimit(CreateCampaignSpendLimit{Type: SpendLimitUncapped}) {
		t.Fatal("spend limit validation failed")
	}
	badAdSet := validCreateAdSetRequest("Ad Set")
	badAdSet.Schedule.EndDate = stringPointer("2026-08-01T00:00:00Z")
	if validCreateAdSet(badAdSet) {
		t.Fatal("date window accepted")
	}
	badAdSet = validCreateAdSetRequest("Ad Set")
	badAdSet.Targeting.FrequencyCapping.MaximumImpressions = 0
	if validCreateAdSet(badAdSet) {
		t.Fatal("invalid frequency accepted")
	}
	if validStatisticsReport(StatisticsReportRequest{Currency: "USD", Dimensions: []Dimension{DimensionDay}, Metrics: []Metric{MetricClicks}, StartDate: "2026-08-09", EndDate: "2026-08-01"}) {
		t.Fatal("backwards report window accepted")
	}
}
