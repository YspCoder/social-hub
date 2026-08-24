package criteo

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

func TestCampaignPatchValidationBranches(t *testing.T) {
	positive, negative := 10.0, -1.0
	if validCampaignPatch(UpdateCampaignRequest{}) ||
		validCampaignPatch(UpdateCampaignRequest{SpendLimit: &PatchCampaignSpendLimit{}}) ||
		validCampaignPatch(UpdateCampaignRequest{SpendLimit: &PatchCampaignSpendLimit{Type: "bad"}}) ||
		validCampaignPatch(UpdateCampaignRequest{SpendLimit: &PatchCampaignSpendLimit{Amount: &NullableFloat{Value: &negative}}}) ||
		validCampaignPatch(UpdateCampaignRequest{SpendLimit: &PatchCampaignSpendLimit{Renewal: "bad"}}) {
		t.Fatal("invalid Campaign patch accepted")
	}
	if !validCampaignPatch(UpdateCampaignRequest{SpendLimit: &PatchCampaignSpendLimit{Amount: &NullableFloat{Value: &positive}}}) {
		t.Fatal("valid spend patch rejected")
	}
	if validCampaignPatch(UpdateCampaignRequest{BudgetAutomation: &PatchBudgetAutomation{}}) ||
		validCampaignPatch(UpdateCampaignRequest{BudgetAutomation: &PatchBudgetAutomation{Objective: "bad"}}) ||
		!validCampaignPatch(UpdateCampaignRequest{BudgetAutomation: &PatchBudgetAutomation{Enabled: boolPointer(true)}}) {
		t.Fatal("budget automation validation failed")
	}
	validScheduled := &PatchScheduledSpendLimits{
		Creations: []ScheduledSpendLimitCreation{{
			Type: SpendLimitCapped, Amount: &NullableFloat{Value: &positive}, Renewal: RenewalDaily, StartDate: "2026-08-10",
		}},
		Updates: []ScheduledSpendLimitUpdate{{
			ID: "12", Type: SpendLimitUncapped, Amount: &NullableFloat{}, Renewal: RenewalUndefined, StartDate: "2026-08-11",
		}},
		Deletions: []string{"13"},
	}
	if !validCampaignPatch(UpdateCampaignRequest{ScheduledSpendLimit: validScheduled}) {
		t.Fatal("valid scheduled spend patch rejected")
	}
	if validCampaignPatch(UpdateCampaignRequest{ScheduledSpendLimit: &PatchScheduledSpendLimits{}}) {
		t.Fatal("empty scheduled spend patch accepted")
	}
	invalidScheduled := *validScheduled
	invalidScheduled.Updates = []ScheduledSpendLimitUpdate{{ID: "bad", Type: SpendLimitCapped, Amount: &NullableFloat{Value: &positive}, Renewal: RenewalDaily, StartDate: "2026-08-10"}}
	if validCampaignPatch(UpdateCampaignRequest{ScheduledSpendLimit: &invalidScheduled}) {
		t.Fatal("invalid scheduled spend patch accepted")
	}
	if validScheduledLimit(SpendLimitCapped, nil, RenewalDaily, "2026-08-10") ||
		validScheduledLimit(SpendLimitCapped, &NullableFloat{Value: &positive}, RenewalDaily, "bad") ||
		!validScheduledLimit(SpendLimitUncapped, nil, "", "2026-08-10") {
		t.Fatal("scheduled spend validation failed")
	}
}

func TestAdSetPatchAndTargetingValidationBranches(t *testing.T) {
	positive, negative := 1.0, -1.0
	name := "Updated"
	if !validAdSetPatch(UpdateAdSetRequest{Name: &name}) || validAdSetPatch(UpdateAdSetRequest{Name: stringPointer(" bad ")}) {
		t.Fatal("name patch validation failed")
	}
	if !validAttribution(nil) || !validAttribution(&AttributionConfiguration{Method: AttributionCriteo, LookbackWindow: Lookback30D}) ||
		validAttribution(&AttributionConfiguration{Method: "bad"}) || validAttribution(&AttributionConfiguration{Method: AttributionCriteo, LookbackWindow: "bad"}) {
		t.Fatal("attribution validation failed")
	}
	if validAdSetPatch(UpdateAdSetRequest{Bidding: &PatchAdSetBidding{}}) ||
		validAdSetPatch(UpdateAdSetRequest{Bidding: &PatchAdSetBidding{BidAmount: &NullableFloat{Value: &negative}}}) ||
		!validAdSetPatch(UpdateAdSetRequest{Bidding: &PatchAdSetBidding{BidAmount: &NullableFloat{Value: &positive}}}) ||
		!validAdSetPatch(UpdateAdSetRequest{Bidding: &PatchAdSetBidding{BidAmount: &NullableFloat{}}}) {
		t.Fatal("bidding patch validation failed")
	}
	if validPatchBudget(PatchAdSetBudget{}) || validPatchBudget(PatchAdSetBudget{Amount: &NullableFloat{Value: &negative}}) ||
		!validPatchBudget(PatchAdSetBudget{Amount: &NullableFloat{Value: &positive}}) {
		t.Fatal("budget amount patch validation failed")
	}
	badStrategy := BudgetStrategy("bad")
	badRenewal := BudgetRenewal("bad")
	badSmoothing := DeliverySmoothing("bad")
	badWeek := DeliveryWeek("bad")
	if validPatchBudget(PatchAdSetBudget{Strategy: &badStrategy}) || validPatchBudget(PatchAdSetBudget{Renewal: &badRenewal}) ||
		validPatchBudget(PatchAdSetBudget{DeliverySmoothing: &badSmoothing}) || validPatchBudget(PatchAdSetBudget{DeliveryWeek: &badWeek}) {
		t.Fatal("invalid budget enum accepted")
	}
	strategy, renewal, smoothing, week := BudgetCapped, BudgetWeekly, DeliveryStandard, WeekMondayToSunday
	if !validPatchBudget(PatchAdSetBudget{Strategy: &strategy, Renewal: &renewal, DeliverySmoothing: &smoothing, DeliveryWeek: &week}) {
		t.Fatal("valid budget enums rejected")
	}
	if validAdSetPatch(UpdateAdSetRequest{Schedule: &PatchAdSetSchedule{}}) ||
		validAdSetPatch(UpdateAdSetRequest{Schedule: &PatchAdSetSchedule{StartDate: &NullableTime{Value: stringPointer("bad")}}}) ||
		!validAdSetPatch(UpdateAdSetRequest{Schedule: &PatchAdSetSchedule{EndDate: &NullableTime{}}}) {
		t.Fatal("schedule patch validation failed")
	}
	validRule := &TargetingRule{Operand: OperandIn, Values: []string{"US"}}
	if !validTargetingRule(nil) || !validTargetingRule(validRule) ||
		validTargetingRule(&TargetingRule{Operand: "bad", Values: []string{"US"}}) ||
		validTargetingRule(&TargetingRule{Operand: OperandIn}) {
		t.Fatal("targeting rule validation failed")
	}
	if !validDeliveryWeek(WeekSundayToSaturday) || validDeliveryWeek("bad") ||
		validDeliveryLimitations(DeliveryLimitations{Devices: []Device{"bad"}}) ||
		validDeliveryLimitations(DeliveryLimitations{Environments: []Environment{"bad"}}) ||
		validDeliveryLimitations(DeliveryLimitations{OperatingSystems: []OperatingSystem{"bad"}}) {
		t.Fatal("delivery validation failed")
	}
	weekly := CreateAdSetBudget{Strategy: BudgetCapped, Amount: &positive, Renewal: BudgetWeekly, DeliveryWeek: WeekMondayToSunday}
	if !validBudget(weekly) {
		t.Fatal("weekly budget rejected")
	}
	weekly.DeliveryWeek = ""
	if validBudget(weekly) {
		t.Fatal("weekly budget without delivery week accepted")
	}
	input := validCreateAdSetRequest("Timezone")
	input.Schedule.StartDate = "2026-08-10T10:00:00+08:00"
	input.Schedule.EndDate = stringPointer("2026-08-10T03:00:00Z")
	if !validCreateAdSet(input) {
		t.Fatal("timezone-equivalent valid window rejected")
	}
}

func TestErrorAndOperationHelpers(t *testing.T) {
	var nilAPI *APIError
	if nilAPI.Error() != "socialhub: criteo: platform_error" || nilAPI.Unwrap() != nil || nilAPI.Retryable() {
		t.Fatal("nil APIError behavior is invalid")
	}
	retryable := &APIError{Hub: &socialhub.Error{Class: socialhub.ClassRetryable}}
	if !retryable.Retryable() || retryable.Unwrap() != retryable.Hub {
		t.Fatal("retryable APIError behavior is invalid")
	}
	if (&Adapter{}).Name() != adapterName {
		t.Fatal("adapter name mismatch")
	}
	if withOperation(nil, "op") != nil {
		t.Fatal("nil operation error changed")
	}
	err := &socialhub.Error{Code: socialhub.CodePlatformError}
	if withOperation(err, "renamed") != err || err.Op != "renamed" {
		t.Fatal("operation was not attached")
	}
	when := time.Now().Add(2 * time.Minute).UTC().Format(http.TimeFormat)
	if delay := retryDelay(http.Header{"Retry-After": {when}}); delay <= 0 || delay > 3*time.Minute {
		t.Fatalf("HTTP-date delay=%s", delay)
	}
	reset := time.Now().Add(time.Minute).Unix()
	if delay := retryDelay(http.Header{"X-Ratelimit-Reset": {formatUnix(reset)}}); delay <= 0 || delay > 2*time.Minute {
		t.Fatalf("reset delay=%s", delay)
	}
	if boundedDelay(-time.Second) != 0 || boundedDelay(25*time.Hour) != 0 || boundedDelay(time.Second) != time.Second {
		t.Fatal("delay bounding failed")
	}
	if code, _ := classifyProblem("deprecation"); code != socialhub.CodePlatformError {
		t.Fatal("unknown problem classification failed")
	}
}

type failingTokenStore struct {
	getErr error
	putErr error
}

func (store failingTokenStore) Get(context.Context, socialhub.TokenKey) (socialhub.Token, error) {
	return socialhub.Token{}, store.getErr
}

func (store failingTokenStore) Put(context.Context, socialhub.TokenKey, socialhub.Token) error {
	return store.putErr
}

func (failingTokenStore) Delete(context.Context, socialhub.TokenKey) error { return nil }

func TestTokenStoreBranches(t *testing.T) {
	clock := &mutableClock{value: testNow}
	source := &clientTokenSource{
		oauth: OAuthClient{Clock: clock}, store: failingTokenStore{getErr: errors.New("cache unavailable")},
	}
	if _, err := source.Token(context.Background()); !errors.Is(err, socialhub.ErrUnavailable) {
		t.Fatalf("cache get error=%v", err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writeJSON(t, writer, http.StatusOK, map[string]any{"access_token": "token", "token_type": "Bearer", "expires_in": 900})
	}))
	defer server.Close()
	source = &clientTokenSource{
		oauth: OAuthClient{ClientID: "id", ClientSecret: "secret", TokenURL: server.URL, HTTPClient: server.Client(), Clock: clock},
		store: failingTokenStore{getErr: socialhub.ErrNotFound, putErr: errors.New("cache unavailable")},
	}
	if _, err := source.Token(context.Background()); !errors.Is(err, socialhub.ErrUnavailable) {
		t.Fatalf("cache put error=%v", err)
	}
	memory := socialhub.NewMemoryTokenStore()
	key := socialhub.TokenKey{Platform: platformName, Account: "cached"}
	stored := socialhub.Token{AccessToken: "stored", ExpiresAt: testNow.Add(time.Hour)}
	if err := memory.Put(context.Background(), key, stored); err != nil {
		t.Fatal(err)
	}
	source = &clientTokenSource{oauth: OAuthClient{Clock: clock}, store: memory, key: key}
	got, err := source.Token(context.Background())
	if err != nil || got.AccessToken != "stored" {
		t.Fatalf("stored token=%#v err=%v", got, err)
	}
}

func formatUnix(value int64) string {
	return strconv.FormatInt(value, 10)
}
