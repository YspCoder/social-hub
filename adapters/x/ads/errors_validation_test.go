package ads

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

func TestHTTPErrorMappingRateHeadersAndRedaction(t *testing.T) {
	header := make(http.Header)
	header.Set("x-request-id", "request-123")
	header.Set("x-account-rate-limit-reset", strconv.FormatInt(testNow.Add(30*time.Second).Unix(), 10))
	header.Set("x-rate-limit-reset", strconv.FormatInt(testNow.Add(60*time.Second).Unix(), 10))
	err := decodeHTTPError(http.StatusTooManyRequests, header, []byte(`{
		"errors":[{"parameter":"start_time","details":"access_token=secret-token","code":"TOO_MANY_REQUESTS","message":"authorization: Bearer-secret"}]
	}`), testNow)
	hub := hubError(t, err)
	if !errors.Is(err, socialhub.ErrRateLimited) || !hub.Retryable() || hub.RetryAfter != 30*time.Second ||
		hub.PlatformCode != "TOO_MANY_REQUESTS" || hub.RequestID != "request-123" ||
		strings.Contains(hub.PlatformMessage, "Bearer-secret") || !strings.Contains(hub.PlatformMessage, "[REDACTED]") {
		t.Fatalf("error=%#v", hub)
	}

	tests := []struct {
		status int
		code   string
		want   socialhub.ErrorCode
		class  socialhub.ErrorClass
	}{
		{http.StatusUnauthorized, "UNAUTHORIZED_ACCESS", socialhub.CodeUnauthenticated, socialhub.ClassUserAction},
		{http.StatusForbidden, "UNAUTHORIZED_CLIENT_APPLICATION", socialhub.CodeApprovalRequired, socialhub.ClassUserAction},
		{http.StatusNotFound, "CAMPAIGN_NOT_FOUND", socialhub.CodeNotFound, socialhub.ClassPermanent},
		{http.StatusRequestTimeout, "CANCELLED_REQUEST", socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable},
		{http.StatusBadRequest, "INVALID_PARAMETER", socialhub.CodeInvalidArgument, socialhub.ClassPermanent},
		{http.StatusForbidden, "ACTION_NOT_ALLOWED", socialhub.CodePermissionDenied, socialhub.ClassUserAction},
		{http.StatusConflict, "CONFLICT", socialhub.CodeConflict, socialhub.ClassPermanent},
		{http.StatusInternalServerError, "INTERNAL_ERROR", socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable},
		{http.StatusTeapot, "UNKNOWN", socialhub.CodePlatformError, socialhub.ClassPermanent},
	}
	for _, test := range tests {
		body := []byte(`{"errors":[{"code":"` + test.code + `","message":"message"}]}`)
		hub := hubError(t, decodeHTTPError(test.status, nil, body, testNow))
		if hub.Code != test.want || hub.Class != test.class || hub.HTTPStatus != test.status {
			t.Errorf("status=%d code=%s error=%#v", test.status, test.code, hub)
		}
	}
}

func TestRetryDelayFallbacksAndErrorHelpers(t *testing.T) {
	header := make(http.Header)
	header.Set("x-account-rate-limit-reset", "bad")
	header.Set("x-rate-limit-reset", strconv.FormatInt(testNow.Add(time.Minute).Unix(), 10))
	if delay := retryDelay(header, testNow); delay != time.Minute {
		t.Fatalf("global reset delay=%s", delay)
	}
	header = make(http.Header)
	header.Set("Retry-After", "2.5")
	if delay := retryDelay(header, testNow); delay != 2500*time.Millisecond {
		t.Fatalf("decimal Retry-After=%s", delay)
	}
	header.Set("Retry-After", testNow.Add(5*time.Minute).Format(http.TimeFormat))
	if delay := retryDelay(header, testNow); delay != 5*time.Minute {
		t.Fatalf("date Retry-After=%s", delay)
	}
	header.Set("Retry-After", "invalid")
	if delay := retryDelay(header, testNow); delay != 0 {
		t.Fatalf("invalid Retry-After=%s", delay)
	}
	long := strings.Repeat("界", 300)
	if value := boundedMessage(long, 10); utf8.RuneCountInString(value) != 10 {
		t.Fatalf("bounded rune count=%d", utf8.RuneCountInString(value))
	}
	redacted := redactSensitive("oauth_token_secret='one' consumer_secret=two client_secret:three oauth_token=four")
	if strings.Contains(redacted, "one") || strings.Contains(redacted, "two") || strings.Contains(redacted, "three") || strings.Contains(redacted, "four") {
		t.Fatalf("redacted=%q", redacted)
	}
	if firstNonEmpty("", " value ", "later") != " value " || firstNonEmpty("", " ") != "" {
		t.Fatal("firstNonEmpty mismatch")
	}
}

func TestTransportInvokesDecoderAndRejectsRedirects(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		assertAPIRequest(t, request)
		writer.Header().Set("x-account-rate-limit-reset", strconv.FormatInt(testNow.Add(45*time.Second).Unix(), 10))
		writer.WriteHeader(http.StatusTooManyRequests)
		_, _ = writer.Write([]byte(`{"errors":[{"code":"TOO_MANY_REQUESTS","message":"slow down"}]}`))
	}))
	_, client := newTestAdapter(t, server)
	_, err := client.GetAdAccount(context.Background())
	if hub := hubError(t, err); hub.Code != socialhub.CodeRateLimited || hub.RetryAfter != 45*time.Second {
		t.Fatalf("transport error=%#v", hub)
	}
	server.Close()

	var redirected atomic.Int32
	sink := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { redirected.Add(1) }))
	defer sink.Close()
	origin := httptest.NewServer(http.RedirectHandler(sink.URL, http.StatusFound))
	defer origin.Close()
	_, client = newTestAdapter(t, origin)
	if _, err := client.GetAdAccount(context.Background()); err == nil || redirected.Load() != 0 {
		t.Fatalf("redirect error=%v calls=%d", err, redirected.Load())
	}
}

func TestLocalInputValidationAvoidsNetwork(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("invalid input reached network")
	}))
	defer server.Close()
	_, client := newTestAdapter(t, server)
	validStats := StatsRequest{
		Entity: AnalyticsLineItem, EntityIDs: []string{testLineItemID}, StartTime: testNow.Add(-time.Hour), EndTime: testNow,
		Granularity: GranularityHour, Placement: AnalyticsPlacementAllOnTwitter, MetricGroups: []MetricGroup{MetricGroupEngagement},
	}
	calls := []func() error{
		func() error {
			_, err := client.ListCampaigns(context.Background(), ListRequest{Cursor: " bad"})
			return err
		},
		func() error { _, err := client.GetCampaign(context.Background(), "INVALID"); return err },
		func() error {
			_, err := client.CreateCampaign(context.Background(), CreateCampaignRequest{})
			return err
		},
		func() error {
			_, err := client.UpdateCampaign(context.Background(), testCampaignID, UpdateCampaignRequest{})
			return err
		},
		func() error {
			_, err := client.UpdateCampaign(context.Background(), testCampaignID, UpdateCampaignRequest{Status: statusPointer(StatusDraft)})
			return err
		},
		func() error {
			_, err := client.ListLineItems(context.Background(), ListRequest{Count: 1001})
			return err
		},
		func() error { _, err := client.GetLineItem(context.Background(), "bad-id"); return err },
		func() error {
			_, err := client.CreateLineItem(context.Background(), CreateLineItemRequest{})
			return err
		},
		func() error {
			_, err := client.UpdateLineItem(context.Background(), testLineItemID, UpdateLineItemRequest{})
			return err
		},
		func() error {
			_, err := client.ListPromotedTweets(context.Background(), ListPromotedTweetsRequest{LineItemIDs: []string{"same", "same"}})
			return err
		},
		func() error { _, err := client.GetPromotedTweet(context.Background(), "BAD"); return err },
		func() error {
			_, err := client.AssociateTweets(context.Background(), AssociateTweetsRequest{})
			return err
		},
		func() error {
			input := validStats
			input.EndTime = input.EndTime.Add(time.Minute)
			_, err := client.GetStats(context.Background(), input)
			return err
		},
		func() error {
			input := validStats
			input.StartTime = input.EndTime.Add(-8 * 24 * time.Hour)
			_, err := client.GetStats(context.Background(), input)
			return err
		},
		func() error {
			input := validStats
			input.Entity = AnalyticsAccount
			input.EntityIDs = []string{testLineItemID}
			_, err := client.GetStats(context.Background(), input)
			return err
		},
	}
	for index, call := range calls {
		if err := call(); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Errorf("call %d error=%v", index, err)
		}
	}
}

func TestValidationDomains(t *testing.T) {
	if !validAdsID("abc123") || validAdsID("ABC") || validAdsID("") || validAdsID(strings.Repeat("a", 33)) ||
		!validTweetID(testTweetID) || validTweetID("0") || validTweetID("01") || validTweetID("not-a-tweet") ||
		validTweetID("18446744073709551616") {
		t.Fatal("ID validation mismatch")
	}
	if validOpaque("has\ncontrol", 100) || validOpaque(" spaced ", 100) || validOpaque("too-long", 2) ||
		validUniqueAdsIDs(nil, 20) || validUniqueAdsIDs([]string{"a", "a"}, 20) ||
		validUniqueTweetIDs([]string{testTweetID, testTweetID}, 50) {
		t.Fatal("opaque or collection validation mismatch")
	}
	if !validMutationStatus(StatusActive) || !validMutationStatus(StatusPaused) || validMutationStatus(StatusDraft) {
		t.Fatal("status validation mismatch")
	}
	for _, objective := range []Objective{ObjectiveAppEngagements, ObjectiveAppInstalls, ObjectiveReach, ObjectiveFollowers, ObjectiveEngagements, ObjectiveVideoViews, ObjectivePrerollViews, ObjectiveWebsiteClicks} {
		if !validObjective(objective) {
			t.Errorf("invalid objective %s", objective)
		}
	}
	if validObjective("OTHER") || !validProductType(ProductMedia) || validProductType("OTHER") {
		t.Fatal("objective or product validation mismatch")
	}
	placements := []Placement{PlacementAllOnTwitter, PlacementPublisherNetwork, PlacementTapBanner, PlacementTapFull, PlacementTapFullLandscape, PlacementTapNative, PlacementTapMRect, PlacementTwitterProfile, PlacementTwitterReplies, PlacementTwitterSearch, PlacementTwitterTimeline}
	if !validPlacements(placements) || validPlacements([]Placement{PlacementAllOnTwitter, PlacementAllOnTwitter}) || validPlacements([]Placement{"OTHER"}) {
		t.Fatal("placement validation mismatch")
	}
	for _, entity := range []AnalyticsEntity{AnalyticsAccount, AnalyticsCampaign, AnalyticsFundingInstrument, AnalyticsLineItem, AnalyticsPromotedAccount, AnalyticsPromotedTweet} {
		if !validAnalyticsEntity(entity) {
			t.Errorf("invalid entity %s", entity)
		}
	}
	groups := []MetricGroup{MetricGroupBilling, MetricGroupEngagement, MetricGroupLifetimeMobileConversion, MetricGroupMobileConversion, MetricGroupVideo, MetricGroupWebConversion}
	if !validMetricGroups(groups) || validMetricGroups([]MetricGroup{MetricGroupBilling, MetricGroupBilling}) || validMetricGroups([]MetricGroup{"OTHER"}) ||
		validAnalyticsEntity("OTHER") || !validGranularity(GranularityTotal) || validGranularity("OTHER") ||
		!validAnalyticsPlacement(AnalyticsPlacementTrend) || validAnalyticsPlacement("OTHER") {
		t.Fatal("analytics validation mismatch")
	}
}

func TestCreateLineItemValidationVariants(t *testing.T) {
	valid := CreateLineItemRequest{
		CampaignID: testCampaignID, Objective: ObjectiveEngagements, ProductType: ProductPromotedTweets,
		Placements: []Placement{PlacementAllOnTwitter}, BidStrategy: BidStrategyMax,
		BidAmountLocalMicro: int64Pointer(100), DailyBudgetAmountLocalMicro: int64Pointer(1000),
		TotalBudgetAmountLocalMicro: int64Pointer(10000), StartTime: testNow,
	}
	if !validCreateLineItem(valid) {
		t.Fatal("valid Line Item rejected")
	}
	invalid := []CreateLineItemRequest{
		func() CreateLineItemRequest { value := valid; value.BidAmountLocalMicro = nil; return value }(),
		func() CreateLineItemRequest { value := valid; value.BidStrategy = BidStrategyAuto; return value }(),
		func() CreateLineItemRequest { value := valid; value.Placements = []Placement{"OTHER"}; return value }(),
		func() CreateLineItemRequest {
			value := valid
			end := testNow.Add(-time.Hour)
			value.EndTime = &end
			return value
		}(),
		func() CreateLineItemRequest {
			value := valid
			value.DailyBudgetAmountLocalMicro = int64Pointer(20000)
			return value
		}(),
	}
	for index, value := range invalid {
		if validCreateLineItem(value) {
			t.Errorf("invalid Line Item %d accepted", index)
		}
	}
	valid.BidStrategy, valid.BidAmountLocalMicro = BidStrategyAuto, nil
	valid.StartTime = valid.StartTime.Add(30 * time.Second)
	if !validCreateLineItem(valid) {
		t.Fatal("AUTO Line Item with second precision rejected")
	}
}
