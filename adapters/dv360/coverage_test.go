package dv360

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

func TestCompletePatchValidationAndErrorHelpers(t *testing.T) {
	campaignCreate := validCreateCampaignRequest("Campaign")
	end := Date{Year: 2026, Month: 12, Day: 31}
	budgets := []CampaignBudget{{
		DisplayName: "Budget", BudgetAmountMicros: "1000000",
		DateRange:            DateRange{StartDate: Date{Year: 2026, Month: 8, Day: 10}, EndDate: &end},
		ExternalBudgetSource: "EXTERNAL_BUDGET_SOURCE_NONE", BudgetUnit: BudgetUnitCurrency,
	}}
	status := EntityStatusPaused
	campaignPatch := UpdateCampaignRequest{
		DisplayName: &campaignCreate.DisplayName, EntityStatus: &status,
		CampaignGoal: &campaignCreate.CampaignGoal, CampaignFlight: &campaignCreate.CampaignFlight,
		FrequencyCap: &campaignCreate.FrequencyCap, CampaignBudgets: &budgets,
	}
	mask, err := validateCampaignPatch(testCampaignID, campaignPatch)
	if err != nil || mask != "campaignBudgets,campaignFlight,campaignGoal,displayName,entityStatus,frequencyCap" {
		t.Fatalf("campaign mask=%q err=%v", mask, err)
	}

	orderCreate := validCreateInsertionOrderRequest("Order")
	orderPatch := UpdateInsertionOrderRequest{
		DisplayName: &orderCreate.DisplayName, EntityStatus: &status, Pacing: &orderCreate.Pacing,
		FrequencyCap: &orderCreate.FrequencyCap, Budget: &orderCreate.Budget, KPI: &orderCreate.KPI,
		OptimizationObjective: &orderCreate.OptimizationObjective, BidStrategy: orderCreate.BidStrategy,
	}
	mask, err = validateInsertionOrderPatch(testInsertionOrderID, orderPatch)
	if err != nil || strings.Count(mask, ",") != 7 {
		t.Fatalf("order mask=%q err=%v", mask, err)
	}

	lineCreate := validCreateLineItemRequest("Line")
	linePatch := UpdateLineItemRequest{
		DisplayName: &lineCreate.DisplayName, EntityStatus: &status, Flight: &lineCreate.Flight,
		Budget: &lineCreate.Budget, Pacing: &lineCreate.Pacing,
		PartnerRevenueModel: &lineCreate.PartnerRevenueModel, BidStrategy: &lineCreate.BidStrategy,
		FrequencyCap: &lineCreate.FrequencyCap, ContainsEUPoliticalAds: &lineCreate.ContainsEUPoliticalAds,
	}
	mask, err = validateLineItemPatch(testLineItemID, linePatch)
	if err != nil || strings.Count(mask, ",") != 8 {
		t.Fatalf("line mask=%q err=%v", mask, err)
	}

	for _, test := range []struct {
		err  error
		want error
	}{
		{ownershipError("test", "campaign"), socialhub.ErrPermissionDenied},
		{conflictError("test", "conflict"), socialhub.ErrConflict},
		{unsupportedError("test", "unsupported"), socialhub.ErrUnsupported},
	} {
		if !errors.Is(test.err, test.want) {
			t.Errorf("error=%v want=%v", test.err, test.want)
		}
	}
	if requireHubError(t, platformContractError("test", "bad response")).Code != socialhub.CodePlatformError {
		t.Fatal("platform contract code failed")
	}
	api := &APIError{Hub: &socialhub.Error{Code: socialhub.CodePlatformError}}
	if api.Error() == "" || api.Unwrap() == nil {
		t.Fatal("API error contract failed")
	}
}

func TestAdditionalValidationBranches(t *testing.T) {
	if !validDateRange(DateRange{StartDate: Date{Year: 2026, Month: 8, Day: 10}}) ||
		validDateRange(DateRange{StartDate: Date{Year: 2026, Month: 2, Day: 30}}) ||
		validFilter("line\nbreak") || validFilter(string(rune(0x7f))) ||
		!validReadEntityStatus(EntityStatusScheduledForDeletion) {
		t.Fatal("additional primitive validation failed")
	}
	tooManyBudgets := make([]CampaignBudget, 1001)
	if validCampaignBudgets(tooManyBudgets) || validCampaignBudget(CampaignBudget{
		DisplayName: "Budget", BudgetAmountMicros: "1",
		DateRange:            DateRange{StartDate: Date{Year: 2026, Month: 8, Day: 10}},
		ExternalBudgetSource: "EXTERNAL_BUDGET_SOURCE_NONE", BudgetUnit: BudgetUnitCurrency,
	}) {
		t.Fatal("campaign budget edge validation failed")
	}
	if !validPacingForBudget(Pacing{
		Type: PacingEven, Period: PacingPeriodDaily, DailyMaxImpressions: "10",
	}, BudgetUnitImpressions) || validPacingForBudget(Pacing{
		Type: PacingEven, Period: PacingPeriodDaily, DailyMaxImpressions: "10",
	}, "bad") {
		t.Fatal("impression pacing validation failed")
	}
	if !validLineItemBudget(LineItemBudget{AllocationType: LineItemBudgetAutomatic}) ||
		!validLineItemBudget(LineItemBudget{AllocationType: LineItemBudgetUnlimited}) ||
		validLineItemBudget(LineItemBudget{AllocationType: "bad"}) {
		t.Fatal("line item budget branches failed")
	}
	customEnd := Date{Year: 2026, Month: 12, Day: 31}
	if !validLineItemFlight(LineItemFlight{
		Type:      LineItemFlightCustom,
		DateRange: &DateRange{StartDate: Date{Year: 2026, Month: 8, Day: 10}, EndDate: &customEnd},
	}) || validLineItemFlight(LineItemFlight{Type: "bad"}) {
		t.Fatal("line item flight branches failed")
	}
	if validBiddingGoal("bad") || validBiddingStrategy(BiddingStrategy{
		FixedBid:          &FixedBidStrategy{BidAmountMicros: "1"},
		MaximizeSpendAuto: &MaximizeSpendBidStrategy{Type: BiddingGoalCPC},
	}, false) {
		t.Fatal("bidding edge validation failed")
	}
}

type putFailingTokenStore struct{}

func (putFailingTokenStore) Get(context.Context, socialhub.TokenKey) (socialhub.Token, error) {
	return socialhub.Token{}, socialhub.ErrNotFound
}
func (putFailingTokenStore) Put(context.Context, socialhub.TokenKey, socialhub.Token) error {
	return errors.New("write failed")
}
func (putFailingTokenStore) Delete(context.Context, socialhub.TokenKey) error { return nil }

func TestRefreshTokenSourcePutFailureAndOAuthMappings(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.ParseForm() != nil || request.Form.Get("refresh_token") != "refresh" {
			t.Fatalf("form=%v", request.Form)
		}
		writeJSON(t, writer, http.StatusOK, map[string]any{
			"access_token": "access", "expires_in": 3600, "scope": displayVideoScope,
		})
	}))
	defer server.Close()
	source := &refreshTokenSource{
		oauth: OAuthClient{
			ClientID: "client", ClientSecret: "secret", TokenURL: server.URL,
			HTTPClient: server.Client(), Clock: &mutableClock{value: testNow},
		},
		refreshToken: "refresh", store: putFailingTokenStore{},
	}
	if _, err := source.Token(context.Background()); !errors.Is(err, socialhub.ErrUnavailable) {
		t.Fatalf("cache put error=%v", err)
	}
	for code, want := range map[string]error{
		"access_denied": socialhub.ErrPermissionDenied, "temporarily_unavailable": socialhub.ErrUnavailable,
	} {
		if err := oauthError("oauth", http.StatusBadRequest, nil, code, "message"); !errors.Is(err, want) {
			t.Errorf("OAuth code=%s err=%v", code, err)
		}
	}
	future := time.Now().Add(2 * time.Second).UTC().Format(http.TimeFormat)
	delay := parseRetryAfter(future)
	if delay <= 0 || delay > 3*time.Second || parseRetryAfter("999999") != 0 {
		t.Fatalf("HTTP-date delay=%v", delay)
	}
}
