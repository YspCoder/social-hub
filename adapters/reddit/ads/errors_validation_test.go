package ads

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

func TestHTTPErrorMappingRateLimitAndRedaction(t *testing.T) {
	header := make(http.Header)
	header.Set("x-request-id", "request-123")
	header.Set("Retry-After", "5")
	header.Set("RateLimit", `"ads-campaign-management-read";r=0;t=10, "ads-reporting";r=0;t=30`)
	err := decodeHTTPError(http.StatusTooManyRequests, header, []byte(`{
		"error":{"code":429,"message":"authorization: access_token=secret-token","fields":[{"field":"starts_at","message":"client_secret=secret-value"}]}
	}`), testNow)
	hub := hubError(t, err)
	if !errors.Is(err, socialhub.ErrRateLimited) || !hub.Retryable() || hub.RetryAfter != 30*time.Second ||
		hub.PlatformCode != "429" || hub.RequestID != "request-123" || strings.Contains(hub.PlatformMessage, "secret-value") ||
		!strings.Contains(hub.PlatformMessage, "[REDACTED]") {
		t.Fatalf("error=%#v", hub)
	}

	tests := []struct {
		status int
		want   socialhub.ErrorCode
		class  socialhub.ErrorClass
	}{
		{http.StatusBadRequest, socialhub.CodeInvalidArgument, socialhub.ClassPermanent},
		{http.StatusUnauthorized, socialhub.CodeUnauthenticated, socialhub.ClassUserAction},
		{http.StatusForbidden, socialhub.CodePermissionDenied, socialhub.ClassUserAction},
		{http.StatusNotFound, socialhub.CodeNotFound, socialhub.ClassPermanent},
		{http.StatusConflict, socialhub.CodeConflict, socialhub.ClassPermanent},
		{http.StatusInternalServerError, socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable},
		{http.StatusTeapot, socialhub.CodePlatformError, socialhub.ClassPermanent},
	}
	for _, test := range tests {
		hub := hubError(t, decodeHTTPError(test.status, nil, []byte(`{"error":{"message":"message"}}`), testNow))
		if hub.Code != test.want || hub.Class != test.class || hub.HTTPStatus != test.status {
			t.Errorf("status=%d error=%#v", test.status, hub)
		}
	}
}

func TestRateLimitParsingAndRetryFallbacks(t *testing.T) {
	header := make(http.Header)
	header.Set("RateLimit-Policy", `"ads-campaign-management-read";q=400;w=60, "ads-reporting";q=60;w=60`)
	header.Set("RateLimit", `"ads-campaign-management-read";r=20;t=10, "ads-reporting";r=2;t=30`)
	limit, ok := parseRateLimit(header)
	if !ok || limit.Policy != "ads-reporting" || limit.Quota != 60 || limit.Window != time.Minute || limit.Remaining != 2 || limit.Reset != 30*time.Second {
		t.Fatalf("limit=%#v ok=%v", limit, ok)
	}
	if _, ok := parseRateLimit(http.Header{"RateLimit": {"invalid"}}); ok {
		t.Fatal("invalid RateLimit accepted")
	}
	header = make(http.Header)
	header.Set("Retry-After", "2.5")
	if delay := retryDelay(header, testNow); delay != 2500*time.Millisecond {
		t.Fatalf("decimal delay=%s", delay)
	}
	header.Set("Retry-After", testNow.Add(5*time.Minute).Format(http.TimeFormat))
	if delay := retryDelay(header, testNow); delay != 5*time.Minute {
		t.Fatalf("date delay=%s", delay)
	}
	header.Set("Retry-After", "invalid")
	if delay := retryDelay(header, testNow); delay != 0 {
		t.Fatalf("invalid delay=%s", delay)
	}
	if !moreRestrictive(RateLimit{Remaining: 1}, RateLimit{Remaining: 2}) ||
		!moreRestrictive(RateLimit{Quota: 100, Remaining: 1}, RateLimit{Quota: 10, Remaining: 1}) {
		t.Fatal("restriction ordering mismatch")
	}
}

func TestErrorHelpersAndValidationDomains(t *testing.T) {
	long := strings.Repeat("界", 300)
	if value := boundedMessage(long, 10); utf8.RuneCountInString(value) != 10 {
		t.Fatalf("bounded=%q", value)
	}
	redacted := redactSensitive("client_secret='one' access_token=two refresh_token:three authorization=Bearer-four")
	for _, secret := range []string{"one", "two", "three", "Bearer-four"} {
		if strings.Contains(redacted, secret) {
			t.Fatalf("redacted=%q", redacted)
		}
	}
	if firstNonEmpty("", " value ", "later") != " value " || firstNonEmpty("", " ") != "" {
		t.Fatal("firstNonEmpty mismatch")
	}

	if !validAdAccountID(testAdAccountID) || validAdAccountID("p2_123") || !validPixelID(testPixelID) || validPixelID("x2_123") ||
		!validPostID(testPostID) || validPostID("t1_abc") || !validResourceID(testCampaignID) || validResourceID("01") || validResourceID("abc") {
		t.Fatal("ID validation mismatch")
	}
	if validOpaque("has\ncontrol", 100) || validOpaque(" spaced ", 100) || validOpaque("too-long", 2) ||
		validText("name\n", 500) || validText("", 500) || validClickURL("ftp://example.test") {
		t.Fatal("text validation mismatch")
	}
	for _, objective := range []CampaignObjective{
		ObjectiveAppInstalls, ObjectiveCatalogSales, ObjectiveClicks, ObjectiveConversions,
		ObjectiveImpressions, ObjectiveLeadGeneration, ObjectiveVideoViewableImpressions,
	} {
		if !validObjective(objective) {
			t.Errorf("objective=%s", objective)
		}
	}
	if validObjective("OTHER") || !validMutationStatus(StatusActive) || validMutationStatus(StatusArchived) ||
		!validCampaignBidStrategy(BidStrategyTargetCPX) || validCampaignBidStrategy(BidStrategyManual) ||
		!validReportBreakdown("COMMUNITY") || validReportBreakdown("bad-value") || !validReportField("NEW_METRIC") || validReportField("bad") {
		t.Fatal("enum validation mismatch")
	}
}

func TestTransportRejectsRedirectsAndMapsErrors(t *testing.T) {
	var redirected atomic.Int32
	sink := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { redirected.Add(1) }))
	defer sink.Close()
	origin := httptest.NewServer(http.RedirectHandler(sink.URL, http.StatusFound))
	defer origin.Close()
	_, client := newTestAdapter(t, origin)
	_, err := client.GetAdAccount(context.Background())
	if err == nil || redirected.Load() != 0 || hubError(t, err).Code != socialhub.CodePlatformError {
		t.Fatalf("redirect error=%v calls=%d", err, redirected.Load())
	}

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("RateLimit", `"ads-reporting";r=0;t=45`)
		writeJSON(t, writer, http.StatusTooManyRequests, map[string]any{"error": map[string]any{"code": 429, "message": "slow down"}})
	}))
	defer server.Close()
	_, client = newTestAdapter(t, server)
	_, err = client.GetAdAccount(context.Background())
	if hub := hubError(t, err); hub.Code != socialhub.CodeRateLimited || hub.RetryAfter != 45*time.Second {
		t.Fatalf("transport error=%#v", hub)
	}
}
