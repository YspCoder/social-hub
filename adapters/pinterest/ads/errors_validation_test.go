package ads

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

func TestHTTPAndBatchErrorClassification(t *testing.T) {
	header := http.Header{
		"Retry-After":     {"2.5"},
		"X-Pinterest-Rid": {"request-429"},
	}
	err := decodeHTTPError(http.StatusTooManyRequests, header, []byte(`{"code":8,"message":"rate limit access_token=secret-value"}`))
	hub := hubError(t, err)
	if !errors.Is(err, socialhub.ErrRateLimited) || !hub.Retryable() || hub.RetryAfter != 2500*time.Millisecond ||
		hub.RequestID != "request-429" || hub.PlatformCode != "8" || strings.Contains(hub.PlatformMessage, "secret-value") {
		t.Fatalf("rate error=%#v", hub)
	}

	resetHeader := http.Header{"X-Ratelimit-Reset": {"4"}, "X-Request-Id": {"fallback-request"}}
	hub = hubError(t, decodeHTTPError(http.StatusTooManyRequests, resetHeader, []byte(`{"message":"quota"}`)))
	if hub.RetryAfter != 4*time.Second || hub.RequestID != "fallback-request" {
		t.Fatalf("reset error=%#v", hub)
	}

	classifications := []struct {
		status       int
		platformCode int
		message      string
		code         socialhub.ErrorCode
		class        socialhub.ErrorClass
	}{
		{http.StatusBadRequest, 0, "bad", socialhub.CodeInvalidArgument, socialhub.ClassPermanent},
		{http.StatusUnauthorized, 0, "bad", socialhub.CodeUnauthenticated, socialhub.ClassUserAction},
		{http.StatusForbidden, 0, "bad", socialhub.CodePermissionDenied, socialhub.ClassUserAction},
		{http.StatusNotFound, 0, "bad", socialhub.CodeNotFound, socialhub.ClassPermanent},
		{http.StatusConflict, 0, "bad", socialhub.CodeConflict, socialhub.ClassPermanent},
		{http.StatusServiceUnavailable, 0, "bad", socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable},
		{http.StatusTeapot, 2, "bad", socialhub.CodeNotFound, socialhub.ClassPermanent},
		{http.StatusTeapot, 0, "throttled", socialhub.CodeRateLimited, socialhub.ClassRetryable},
		{http.StatusTeapot, 0, "bad", socialhub.CodePlatformError, socialhub.ClassPermanent},
	}
	for _, test := range classifications {
		code, class := classifyError(test.status, test.platformCode, test.message)
		if code != test.code || class != test.class {
			t.Errorf("status=%d platform=%d got=%s/%s", test.status, test.platformCode, code, class)
		}
	}

	batch := batchItemError("campaign_create", http.StatusOK, http.Header{"X-Pinterest-Rid": {"batch-request"}}, batchException{Code: 17, Message: "client_secret=secret"})
	hub = hubError(t, batch)
	if !errors.Is(batch, socialhub.ErrInvalidArgument) || hub.Op != "campaign_create" || hub.RequestID != "batch-request" || strings.Contains(hub.PlatformMessage, "client_secret=secret") {
		t.Fatalf("batch error=%#v", hub)
	}
}

func TestBatchContractsAndAnalyticsDecoding(t *testing.T) {
	var array batchExceptions
	if err := json.Unmarshal([]byte(`[{"code":1,"message":"first"}]`), &array); err != nil || len(array) != 1 {
		t.Fatalf("array=%#v err=%v", array, err)
	}
	var object batchExceptions
	if err := json.Unmarshal([]byte(`{"code":2,"message":"single"}`), &object); err != nil || len(object) != 1 || object[0].Code != 2 {
		t.Fatalf("object=%#v err=%v", object, err)
	}
	if err := json.Unmarshal([]byte(`null`), &object); err != nil || object != nil {
		t.Fatalf("null=%#v err=%v", object, err)
	}
	if err := json.Unmarshal([]byte(`x`), &object); err == nil {
		t.Fatal("invalid batch exceptions accepted")
	}

	metadata := transport.ResponseMetadata{StatusCode: http.StatusOK, Header: http.Header{}}
	validate := func(campaign *Campaign) error {
		if campaign.ID != testCampaignID {
			return platformContractError("test", "wrong Campaign")
		}
		return nil
	}
	valid := Campaign{ID: testCampaignID}
	result, err := requireBatchResult("test", batchResponse[Campaign]{Items: []batchItem[Campaign]{{Data: &valid}}}, metadata, validate)
	if err != nil || result.ID != testCampaignID {
		t.Fatalf("result=%#v err=%v", result, err)
	}
	contractCases := []batchResponse[Campaign]{
		{},
		{Items: []batchItem[Campaign]{{}}},
		{Items: []batchItem[Campaign]{{Data: &Campaign{ID: "wrong"}}}},
	}
	for index, response := range contractCases {
		if _, err := requireBatchResult("test", response, metadata, validate); hubError(t, err).Code != socialhub.CodePlatformError {
			t.Errorf("contract %d error=%v", index, err)
		}
	}
	if _, err := requireBatchResult("test", batchResponse[Campaign]{Items: []batchItem[Campaign]{{Exceptions: batchExceptions{{Code: 17, Message: "bad"}}}}}, metadata, validate); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("exception error=%v", err)
	}

	var row AnalyticsRow
	if err := json.Unmarshal([]byte(`{"AD_ACCOUNT_ID":"111111111111","DATE":"2026-08-01","CLICKTHROUGH_1":12}`), &row); err != nil ||
		row.AdAccountID != testAdAccountID || row.Date != "2026-08-01" || string(row.Metrics["CLICKTHROUGH_1"]) != "12" {
		t.Fatalf("analytics row=%#v err=%v", row, err)
	}
	for _, payload := range []string{`x`, `{"AD_ACCOUNT_ID":1}`, `{"DATE":1}`} {
		if err := json.Unmarshal([]byte(payload), &row); err == nil {
			t.Errorf("invalid Analytics payload accepted: %s", payload)
		}
	}
}

func TestWorkflowInputValidation(t *testing.T) {
	client := &Client{}
	ctx := context.Background()
	invalidCalls := []func() error{
		func() error { _, err := client.ListAdAccounts(ctx, ListAdAccountsRequest{MaxResults: 251}); return err },
		func() error {
			_, err := client.ListCampaigns(ctx, ListCampaignsRequest{IDs: []string{"bad"}})
			return err
		},
		func() error { _, err := client.GetCampaign(ctx, "bad"); return err },
		func() error { _, err := client.CreateCampaign(ctx, CreateCampaignRequest{}); return err },
		func() error {
			_, err := client.UpdateCampaign(ctx, testCampaignID, UpdateCampaignRequest{})
			return err
		},
		func() error { _, err := client.SetCampaignStatus(ctx, testCampaignID, StatusArchived); return err },
		func() error { return client.ArchiveCampaign(ctx, "bad") },
		func() error {
			_, err := client.ListAdGroups(ctx, ListAdGroupsRequest{CampaignIDs: []string{"bad"}})
			return err
		},
		func() error { _, err := client.GetAdGroup(ctx, "bad"); return err },
		func() error { _, err := client.CreateAdGroup(ctx, CreateAdGroupRequest{}); return err },
		func() error { _, err := client.UpdateAdGroup(ctx, testAdGroupID, UpdateAdGroupRequest{}); return err },
		func() error { _, err := client.SetAdGroupStatus(ctx, testAdGroupID, StatusArchived); return err },
		func() error { return client.ArchiveAdGroup(ctx, "bad") },
		func() error { _, err := client.ListAds(ctx, ListAdsRequest{IDs: []string{"bad"}}); return err },
		func() error { _, err := client.GetAd(ctx, "bad"); return err },
		func() error { _, err := client.CreateAd(ctx, CreateAdRequest{}); return err },
		func() error { _, err := client.UpdateAd(ctx, testAdID, UpdateAdRequest{}); return err },
		func() error { _, err := client.SetAdStatus(ctx, testAdID, StatusArchived); return err },
		func() error { return client.ArchiveAd(ctx, "bad") },
		func() error { _, err := client.GetAccountAnalytics(ctx, AnalyticsRequest{}); return err },
	}
	for index, call := range invalidCalls {
		if err := call(); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Errorf("call %d error=%v", index, err)
		}
	}
}

func TestPlatformResponseOwnershipContracts(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch {
		case request.URL.Path == "/ad_accounts":
			writeJSON(writer, http.StatusOK, `{"items":[{"id":"0"}]}`)
		case strings.HasSuffix(request.URL.Path, "/campaigns"):
			writeJSON(writer, http.StatusOK, `{"items":[{"id":"222222222222","ad_account_id":"999999999999"}]}`)
		case strings.HasSuffix(request.URL.Path, "/ad_groups"):
			writeJSON(writer, http.StatusOK, `{"items":[{"id":"333333333333","campaign_id":"bad"}]}`)
		case strings.HasSuffix(request.URL.Path, "/ads"):
			writeJSON(writer, http.StatusOK, `{"items":[{"id":"444444444444","campaign_id":"222222222222","ad_group_id":"333333333333","pin_id":"bad"}]}`)
		case strings.HasSuffix(request.URL.Path, "/analytics"):
			writeJSON(writer, http.StatusOK, `[{"AD_ACCOUNT_ID":"999999999999","DATE":"bad"}]`)
		default:
			t.Fatalf("unexpected request %s", request.URL)
		}
	}))
	defer server.Close()
	_, client := newTestAdapter(t, server)
	calls := []func() error{
		func() error {
			_, err := client.ListAdAccounts(context.Background(), ListAdAccountsRequest{})
			return err
		},
		func() error { _, err := client.ListCampaigns(context.Background(), ListCampaignsRequest{}); return err },
		func() error { _, err := client.ListAdGroups(context.Background(), ListAdGroupsRequest{}); return err },
		func() error { _, err := client.ListAds(context.Background(), ListAdsRequest{}); return err },
		func() error {
			_, err := client.GetAccountAnalytics(context.Background(), AnalyticsRequest{
				StartDate: "2026-08-01", EndDate: "2026-08-01", Columns: []string{"IMPRESSION_1"}, Granularity: GranularityDay,
			})
			return err
		},
	}
	for index, call := range calls {
		if err := call(); hubError(t, err).Code != socialhub.CodePlatformError {
			t.Errorf("call %d error=%v", index, err)
		}
	}
}
