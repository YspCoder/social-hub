package amazonads

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

func TestPartialMutationAndHTTPErrorClassification(t *testing.T) {
	partial := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("x-amzn-RequestId", "request-207")
		writeJSON(writer, http.StatusMultiStatus, `{"campaigns":{"success":[],"error":[{"index":0,"errors":[{"errorType":"INVALID_ARGUMENT","errorValue":{"message":"access_token=secret-value"}}]}]}}`)
	}))
	defer partial.Close()
	_, client := newTestAdapter(t, partial)
	_, err := client.CreateCampaign(context.Background(), validCampaignRequest())
	hub := hubError(t, err)
	if !errors.Is(err, socialhub.ErrInvalidArgument) || hub.HTTPStatus != http.StatusMultiStatus || hub.RequestID != "request-207" || hub.PlatformCode != "INVALID_ARGUMENT" || strings.Contains(hub.PlatformMessage, "secret-value") {
		t.Fatalf("partial error=%#v", hub)
	}

	limited := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Retry-After", "2.5")
		writer.Header().Set("x-amz-request-id", "request-429")
		writeJSON(writer, http.StatusTooManyRequests, `{"code":"THROTTLED","message":"quota"}`)
	}))
	defer limited.Close()
	_, client = newTestAdapter(t, limited)
	_, err = client.ListCampaigns(context.Background(), ListCampaignsRequest{})
	hub = hubError(t, err)
	if !errors.Is(err, socialhub.ErrRateLimited) || !hub.Retryable() || hub.RetryAfter != 2500*time.Millisecond || hub.RequestID != "request-429" {
		t.Fatalf("rate error=%#v", hub)
	}
}

func TestValidationDecimalAndContracts(t *testing.T) {
	encoded, err := json.Marshal(struct {
		Amount Decimal `json:"amount"`
	}{Amount: "1234567890.123456"})
	if err != nil || string(encoded) != `{"amount":1234567890.123456}` {
		t.Fatalf("decimal=%s err=%v", encoded, err)
	}
	var decimal Decimal
	if err := json.Unmarshal([]byte(`"0.25"`), &decimal); err != nil || decimal != "0.25" {
		t.Fatalf("decimal=%q err=%v", decimal, err)
	}
	for _, value := range []Decimal{"", "-1", "1e2", ".1", "1."} {
		if _, err := json.Marshal(value); err == nil {
			t.Errorf("decimal %q unexpectedly valid", value)
		}
	}
	var profile Profile
	if err := json.Unmarshal([]byte(`{"profileId":"1234567890","dailyBudget":"5.00"}`), &profile); err != nil || profile.ID != testProfileID {
		t.Fatalf("profile=%#v err=%v", profile, err)
	}
	if json.Unmarshal([]byte(`{"profileId":"bad"}`), &profile) == nil {
		t.Fatal("invalid profile ID accepted")
	}

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/v2/profiles":
			writeJSON(writer, http.StatusOK, `[{"profileId":999}]`)
		case "/sp/campaigns/list":
			writeJSON(writer, http.StatusOK, `{"campaigns":[{"campaignId":"bad"}]}`)
		case "/sp/campaigns":
			writeJSON(writer, http.StatusOK, `{"campaigns":{"success":[{"campaignId":"999","campaign":{"campaignId":"998"}}],"error":[]}}`)
		}
	}))
	defer server.Close()
	_, client := newTestAdapter(t, server)
	if _, err := client.GetProfile(context.Background()); !errors.Is(err, socialhub.ErrNotFound) {
		t.Fatalf("profile contract=%v", err)
	}
	if _, err := client.ListCampaigns(context.Background(), ListCampaignsRequest{}); hubError(t, err).Code != socialhub.CodePlatformError {
		t.Fatalf("list contract=%v", err)
	}
	if _, err := client.CreateCampaign(context.Background(), validCampaignRequest()); hubError(t, err).Code != socialhub.CodePlatformError {
		t.Fatalf("mutation contract=%v", err)
	}
}

func TestWorkflowInputValidation(t *testing.T) {
	client := &Client{}
	ctx := context.Background()
	invalidCalls := []func() error{
		func() error { _, err := client.ListCampaigns(ctx, ListCampaignsRequest{MaxResults: 1001}); return err },
		func() error { _, err := client.CreateCampaign(ctx, CreateCampaignRequest{}); return err },
		func() error { _, err := client.UpdateCampaign(ctx, "bad", UpdateCampaignRequest{}); return err },
		func() error { _, err := client.SetCampaignState(ctx, testCampaignID, StateProposed); return err },
		func() error { return client.ArchiveCampaign(ctx, "bad") },
		func() error {
			_, err := client.ListAdGroups(ctx, ListAdGroupsRequest{IDs: []string{"bad"}})
			return err
		},
		func() error { _, err := client.CreateAdGroup(ctx, CreateAdGroupRequest{}); return err },
		func() error { _, err := client.UpdateAdGroup(ctx, testAdGroupID, UpdateAdGroupRequest{}); return err },
		func() error { _, err := client.SetAdGroupState(ctx, testAdGroupID, StateProposed); return err },
		func() error { return client.ArchiveAdGroup(ctx, "bad") },
		func() error {
			_, err := client.ListProductAds(ctx, ListProductAdsRequest{AdGroupIDs: []string{"bad"}})
			return err
		},
		func() error {
			_, err := client.CreateProductAd(ctx, CreateProductAdRequest{CampaignID: testCampaignID, AdGroupID: testAdGroupID, ASIN: "B000000001", SKU: "both"})
			return err
		},
		func() error { _, err := client.SetProductAdState(ctx, testProductAdID, StateProposed); return err },
		func() error { return client.ArchiveProductAd(ctx, "bad") },
		func() error {
			_, err := client.ListKeywords(ctx, ListKeywordsRequest{MatchTypes: []MatchType{"BAD"}})
			return err
		},
		func() error { _, err := client.CreateKeyword(ctx, CreateKeywordRequest{}); return err },
		func() error { _, err := client.UpdateKeyword(ctx, testKeywordID, UpdateKeywordRequest{}); return err },
		func() error { _, err := client.SetKeywordState(ctx, testKeywordID, StateProposed); return err },
		func() error { return client.ArchiveKeyword(ctx, "bad") },
		func() error { _, err := client.CreateReport(ctx, CreateReportRequest{}); return err },
		func() error { _, err := client.GetReport(ctx, "bad/report"); return err },
	}
	for index, call := range invalidCalls {
		if err := call(); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Errorf("call %d error=%v", index, err)
		}
	}
}

func validCampaignRequest() CreateCampaignRequest {
	return CreateCampaignRequest{Name: "Brand", TargetingType: TargetingManual, StartDate: "2026-08-10", DailyBudget: "10.00", DynamicBidding: DynamicBidding{Strategy: BiddingLegacyForSales}}
}
