package dv360

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

func TestGoogleRPCErrorClassificationRetryAndRedaction(t *testing.T) {
	body := []byte(`{"error":{"code":429,"message":"access_token=secret-value exhausted","status":"RESOURCE_EXHAUSTED","details":[{"@type":"type.googleapis.com/google.rpc.RetryInfo","retryDelay":"2.5s"}]}}`)
	header := http.Header{"X-Google-Request-Id": {"request-1"}}
	err := decodeHTTPError(http.StatusTooManyRequests, header, body)
	var api *APIError
	if !errors.As(err, &api) || !api.Retryable() || api.Hub.Code != socialhub.CodeRateLimited ||
		api.Hub.RetryAfter != 2500*time.Millisecond || api.Hub.RequestID != "request-1" ||
		strings.Contains(api.Hub.PlatformMessage, "secret-value") || api.RPC.Status != "RESOURCE_EXHAUSTED" {
		t.Fatalf("error=%#v", api)
	}
	if !errors.Is(err, socialhub.ErrRateLimited) {
		t.Fatalf("wrapped error=%v", err)
	}
	if (&APIError{}).Error() == "" || (*APIError)(nil).Error() == "" || (*APIError)(nil).Unwrap() != nil || (*APIError)(nil).Retryable() {
		t.Fatal("nil API error contract failed")
	}

	header.Set("Retry-After", "3")
	err = decodeHTTPError(http.StatusTooManyRequests, header, body)
	if !errors.As(err, &api) || api.Hub.RetryAfter != 3*time.Second {
		t.Fatalf("header retry=%#v", api)
	}
}

func TestErrorClassificationAndQuotaPolicy(t *testing.T) {
	tests := []struct {
		status   int
		platform string
		code     socialhub.ErrorCode
		class    socialhub.ErrorClass
	}{
		{400, "INVALID_ARGUMENT", socialhub.CodeInvalidArgument, socialhub.ClassPermanent},
		{400, "FAILED_PRECONDITION", socialhub.CodeInvalidArgument, socialhub.ClassPermanent},
		{429, "RESOURCE_EXHAUSTED", socialhub.CodeRateLimited, socialhub.ClassRetryable},
		{401, "UNAUTHENTICATED", socialhub.CodeUnauthenticated, socialhub.ClassUserAction},
		{403, "PERMISSION_DENIED", socialhub.CodePermissionDenied, socialhub.ClassUserAction},
		{404, "NOT_FOUND", socialhub.CodeNotFound, socialhub.ClassPermanent},
		{409, "ABORTED", socialhub.CodeConflict, socialhub.ClassPermanent},
		{503, "UNAVAILABLE", socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable},
		{422, "", socialhub.CodeInvalidArgument, socialhub.ClassPermanent},
		{401, "", socialhub.CodeUnauthenticated, socialhub.ClassUserAction},
		{403, "", socialhub.CodePermissionDenied, socialhub.ClassUserAction},
		{410, "", socialhub.CodeNotFound, socialhub.ClassPermanent},
		{409, "", socialhub.CodeConflict, socialhub.ClassPermanent},
		{429, "", socialhub.CodeRateLimited, socialhub.ClassRetryable},
		{500, "", socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable},
		{418, "", socialhub.CodePlatformError, socialhub.ClassPermanent},
	}
	for _, test := range tests {
		code, class := classifyError(test.status, test.platform)
		if code != test.code || class != test.class {
			t.Errorf("status=%d platform=%q got=%s/%s", test.status, test.platform, code, class)
		}
	}
	policy := DefaultQuotaPolicy()
	if policy.ProjectRequestsPerMinute != 1500 || policy.ProjectWriteRequestsPerMinute != 700 ||
		policy.AdvertiserRequestsPerMinute != 300 || policy.AdvertiserWriteRequestsPerMinute != 150 ||
		policy.WriteIntensiveRequestCost != 5 {
		t.Fatalf("policy=%#v", policy)
	}
	if parseRetryAfter("1.5") != 1500*time.Millisecond || parseRetryAfter("bad") != 0 ||
		retryInfoDelay([]json.RawMessage{json.RawMessage(`{"@type":"other","retryDelay":"1s"}`)}) != 0 {
		t.Fatal("retry parser failed")
	}
}

func TestValidationPrimitivesAndPages(t *testing.T) {
	if !validEndpoint("https://example.com") || validEndpoint("https://example.com/") || validEndpoint("https://u:p@example.com") ||
		!validID("123") || validID("0") || validID("01x") || validID(strings.Repeat("1", 21)) ||
		!validDisplayName("品牌 campaign") || validDisplayName(" name ") || validDisplayName(strings.Repeat("x", 241)) ||
		!validOptionalText("", 1) || validOpaque(" value ", 20) {
		t.Fatal("primitive validation failed")
	}
	fields := map[string]struct{}{"displayName": {}}
	if !validPage(ListRequest{PageSize: 10, PageToken: "page", Filter: `entityStatus="ENTITY_STATUS_ACTIVE"`, OrderBy: "displayName desc"}, 100, fields) ||
		validPage(ListRequest{PageSize: 101}, 100, fields) || validPage(ListRequest{Filter: strings.Repeat("x", 501)}, 100, fields) ||
		validPage(ListRequest{OrderBy: "unknown"}, 100, fields) || validPage(ListRequest{OrderBy: "displayName ascending"}, 100, fields) {
		t.Fatal("page validation failed")
	}
	end := Date{Year: 2026, Month: 8, Day: 11}
	if !validDate(Date{Year: 2026, Month: 8, Day: 10}) || validDate(Date{Year: 2037, Month: 1, Day: 1}) ||
		!validDateRange(DateRange{StartDate: Date{Year: 2026, Month: 8, Day: 10}, EndDate: &end}) ||
		validDateRange(DateRange{StartDate: end, EndDate: &Date{Year: 2026, Month: 8, Day: 10}}) ||
		!validPositiveInt64String("1") || validPositiveInt64String("01") || !validNonnegativeInt64String("0") {
		t.Fatal("date or integer validation failed")
	}
	transportErr := &url.Error{Op: "Get", URL: "https://example.com?token=secret", Err: errors.New("dial failed")}
	if sanitizeCause(transportErr).Error() != "dial failed" || sanitizeCause(errors.New("plain")).Error() != "plain" {
		t.Fatal("cause sanitization failed")
	}
}

func TestFrequencyCampaignAndBudgetValidation(t *testing.T) {
	validCaps := []FrequencyCap{
		{Unlimited: true},
		{TimeUnit: TimeUnitMonths, TimeUnitCount: 1, MaxImpressions: 2},
		{TimeUnit: TimeUnitWeeks, TimeUnitCount: 4, MaxImpressions: 2},
		{TimeUnit: TimeUnitDays, TimeUnitCount: 6, MaxImpressions: 2},
		{TimeUnit: TimeUnitHours, TimeUnitCount: 23, MaxImpressions: 2},
		{TimeUnit: TimeUnitMinutes, TimeUnitCount: 59, MaxImpressions: 2},
	}
	for _, cap := range validCaps {
		if !validFrequencyCap(cap) {
			t.Errorf("valid cap=%#v", cap)
		}
	}
	for _, cap := range []FrequencyCap{
		{Unlimited: true, MaxImpressions: 1}, {TimeUnit: TimeUnitMonths, TimeUnitCount: 2, MaxImpressions: 1},
		{TimeUnit: TimeUnitDays, TimeUnitCount: 0, MaxImpressions: 1}, {TimeUnit: "bad", TimeUnitCount: 1, MaxImpressions: 1},
	} {
		if validFrequencyCap(cap) {
			t.Errorf("invalid cap=%#v", cap)
		}
	}
	goal := validTestCampaignGoal()
	if !validCampaignGoal(goal) || validCampaignGoal(CampaignGoal{Type: CampaignGoalBrandAwareness, PerformanceGoal: PerformanceGoal{Type: PerformanceGoalCPV}}) ||
		validPerformanceGoal(PerformanceGoal{Type: PerformanceGoalCPM, AmountMicros: "0"}) ||
		!validPerformanceGoal(PerformanceGoal{Type: PerformanceGoalCTR, PercentageMicros: "70000"}) ||
		!validPerformanceGoal(PerformanceGoal{Type: PerformanceGoalOther, Value: "custom"}) {
		t.Fatal("performance goal validation failed")
	}
	end := Date{Year: 2026, Month: 12, Day: 31}
	budget := CampaignBudget{
		DisplayName: "Campaign budget", BudgetAmountMicros: "1000000",
		DateRange:            DateRange{StartDate: Date{Year: 2026, Month: 8, Day: 10}, EndDate: &end},
		ExternalBudgetSource: "EXTERNAL_BUDGET_SOURCE_NONE", BudgetUnit: BudgetUnitCurrency,
	}
	if !validCampaignBudget(budget) || !validCampaignBudgets([]CampaignBudget{budget}) ||
		validCampaignBudget(CampaignBudget{}) || validCampaignBudgets([]CampaignBudget{{ID: "1"}, {ID: "1"}}) ||
		!validCampaignFlight(CampaignFlight{PlannedSpendAmountMicros: "0", PlannedDates: budget.DateRange}) {
		t.Fatal("campaign budget validation failed")
	}
}

func TestPacingInsertionOrderAndKPIValidation(t *testing.T) {
	flight := Pacing{Type: PacingEven, Period: PacingPeriodFlight}
	dailyCurrency := Pacing{Type: PacingAhead, Period: PacingPeriodDaily, DailyMaxMicros: "1000000"}
	if !validPacing(flight) || !validPacingForBudget(dailyCurrency, BudgetUnitCurrency) ||
		validPacing(Pacing{Type: PacingASAP, Period: PacingPeriodFlight}) ||
		validPacing(Pacing{Type: PacingEven, Period: PacingPeriodDaily}) ||
		validPacingForBudget(dailyCurrency, BudgetUnitImpressions) {
		t.Fatal("pacing validation failed")
	}
	end := Date{Year: 2026, Month: 12, Day: 31}
	budget := InsertionOrderBudget{
		BudgetUnit: BudgetUnitCurrency, AutomationType: InsertionOrderAutomationNone,
		BudgetSegments: []InsertionOrderBudgetSegment{{
			BudgetAmountMicros: "1000000",
			DateRange:          DateRange{StartDate: Date{Year: 2026, Month: 8, Day: 10}, EndDate: &end},
		}},
	}
	if !validInsertionOrderBudget(budget) || validInsertionOrderBudget(InsertionOrderBudget{}) {
		t.Fatal("insertion order budget validation failed")
	}
	validKPIs := []KPI{
		{Type: KPICPM, AmountMicros: "1000000"}, {Type: KPICTR, PercentageMicros: "70000"},
		{Type: KPICustomValueOverCost, AlgorithmID: "123"}, {Type: KPIOther, Value: "quality"},
		{Type: KPICPV}, {Type: KPICPCL}, {Type: KPIMaximizePacing},
	}
	for _, kpi := range validKPIs {
		if !validKPI(kpi) {
			t.Errorf("valid KPI=%#v", kpi)
		}
	}
	if validKPI(KPI{Type: KPICPM, AmountMicros: "bad"}) || validOptimizationObjective("bad") ||
		!validOptimizationObjective(OptimizationClick) || !validInsertionOrderType("") || validInsertionOrderType("bad") {
		t.Fatal("KPI/objective validation failed")
	}
}

func TestBiddingAndLineItemValidation(t *testing.T) {
	fixedLine := BiddingStrategy{FixedBid: &FixedBidStrategy{BidAmountMicros: "1000000"}}
	fixedOrder := BiddingStrategy{FixedBid: &FixedBidStrategy{BidAmountMicros: "0"}}
	performance := BiddingStrategy{PerformanceGoalAuto: &PerformanceGoalBidStrategy{Type: BiddingGoalCPA, AmountMicros: "1000000"}}
	maximize := BiddingStrategy{MaximizeSpendAuto: &MaximizeSpendBidStrategy{Type: BiddingGoalCPC}}
	if !validBiddingStrategy(fixedLine, false) || !validBiddingStrategy(fixedOrder, true) ||
		!validBiddingStrategy(performance, false) || !validBiddingStrategy(maximize, false) ||
		validBiddingStrategy(BiddingStrategy{}, false) || validBiddingStrategy(fixedLine, true) ||
		validBiddingStrategy(BiddingStrategy{FixedBid: &FixedBidStrategy{BidAmountMicros: "1000000001"}}, false) ||
		validBiddingStrategy(BiddingStrategy{MaximizeSpendAuto: &MaximizeSpendBidStrategy{Type: BiddingGoalViewableCPM}}, false) {
		t.Fatal("bidding validation failed")
	}
	if !validLineItemType(LineItemDisplayDefault) || validLineItemType("LINE_ITEM_TYPE_YOUTUBE_AND_PARTNERS_ACTION") ||
		!validReadLineItemType("LINE_ITEM_TYPE_YOUTUBE_AND_PARTNERS_ACTION") ||
		!validLineItemFlight(LineItemFlight{Type: LineItemFlightInherited}) ||
		validLineItemFlight(LineItemFlight{Type: LineItemFlightCustom}) ||
		!validLineItemBudget(LineItemBudget{AllocationType: LineItemBudgetFixed, MaxAmount: "1"}) ||
		validLineItemBudget(LineItemBudget{AllocationType: LineItemBudgetUnlimited, MaxAmount: "1"}) ||
		!validPartnerRevenue(PartnerRevenueModel{MarkupType: PartnerRevenueMediaCost, MarkupAmount: "0"}) ||
		validPoliticalStatus("unknown") || !validUpdateEntityStatus(EntityStatusActive) || validUpdateEntityStatus(EntityStatusDraft) {
		t.Fatal("line item validation failed")
	}
}
