package marketing

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

func TestHTTPErrorClassificationRetryAndRedaction(t *testing.T) {
	header := http.Header{"Retry-After": {"2.5"}, "X-Request-ID": {"request-429"}}
	err := decodeHTTPError(http.StatusTooManyRequests, header, []byte(`{
		"request_id":"body-request","error_code":"RATE_LIMIT_EXCEEDED","display_message":"access_token=secret-value throttled"
	}`))
	hub := hubError(t, err)
	if !errors.Is(err, socialhub.ErrRateLimited) || !hub.Retryable() || hub.RetryAfter != 2500*time.Millisecond ||
		hub.RequestID != "body-request" || hub.PlatformCode != "RATE_LIMIT_EXCEEDED" || strings.Contains(hub.PlatformMessage, "secret-value") {
		t.Fatalf("rate error=%#v", hub)
	}

	classifications := []struct {
		status       int
		platformCode string
		message      string
		code         socialhub.ErrorCode
		class        socialhub.ErrorClass
	}{
		{http.StatusBadRequest, "", "bad", socialhub.CodeInvalidArgument, socialhub.ClassPermanent},
		{http.StatusUnauthorized, "", "bad", socialhub.CodeUnauthenticated, socialhub.ClassUserAction},
		{http.StatusForbidden, "", "bad", socialhub.CodePermissionDenied, socialhub.ClassUserAction},
		{http.StatusNotFound, "", "bad", socialhub.CodeNotFound, socialhub.ClassPermanent},
		{http.StatusConflict, "", "bad", socialhub.CodeConflict, socialhub.ClassPermanent},
		{http.StatusServiceUnavailable, "", "bad", socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable},
		{http.StatusTeapot, "AUTHORIZATION_PERMISSION_DENIED", "bad", socialhub.CodeApprovalRequired, socialhub.ClassUserAction},
		{http.StatusTeapot, "AUTHENTICATION_FAILURE", "bad", socialhub.CodeUnauthenticated, socialhub.ClassUserAction},
		{http.StatusTeapot, "", "bad", socialhub.CodePlatformError, socialhub.ClassPermanent},
	}
	for _, test := range classifications {
		code, class := classifyError(test.status, test.platformCode, test.message)
		if code != test.code || class != test.class {
			t.Errorf("status=%d platform=%s got=%s/%s", test.status, test.platformCode, code, class)
		}
	}
}

func TestResponseAndInputValidation(t *testing.T) {
	err := checkResponse("test", responseMeta{RequestStatus: "FAILED", RequestID: "request"}, nil)
	if hubError(t, err).Code != socialhub.CodePlatformError {
		t.Fatalf("response error=%v", err)
	}
	err = checkResponse("test", responseMeta{RequestStatus: "SUCCESS"}, []subRequestState{{
		Status: "FAILED", Reason: "fallback", Errors: []apiError{{ErrorCode: "INVALID_TOKEN", Message: "refresh_token=secret"}},
	}})
	if !errors.Is(err, socialhub.ErrUnauthenticated) || strings.Contains(hubError(t, err).PlatformMessage, "secret") {
		t.Fatalf("subrequest error=%v", err)
	}

	client := &Client{}
	ctx := context.Background()
	validStart := testNow.Truncate(time.Hour)
	misaligned := validStart.Add(time.Minute)
	invalidCalls := []func() error{
		func() error { _, err := client.ListCampaigns(ctx, ListRequest{Limit: 49}); return err },
		func() error { _, err := client.GetCampaign(ctx, "bad"); return err },
		func() error { _, err := client.CreateCampaign(ctx, CreateCampaignRequest{}); return err },
		func() error { _, err := client.UpdateCampaign(ctx, testCampaignID, UpdateEntityRequest{}); return err },
		func() error { _, err := client.SetCampaignStatus(ctx, testCampaignID, "DELETED"); return err },
		func() error { _, err := client.ListAdSquads(ctx, ListRequest{Limit: 1001}); return err },
		func() error { _, err := client.GetAdSquad(ctx, "bad"); return err },
		func() error { _, err := client.CreateAdSquad(ctx, CreateAdSquadRequest{}); return err },
		func() error { _, err := client.UpdateAdSquad(ctx, testAdSquadID, UpdateEntityRequest{}); return err },
		func() error { _, err := client.SetAdSquadStatus(ctx, testAdSquadID, "DELETED"); return err },
		func() error { _, err := client.ListAds(ctx, ListRequest{Cursor: " bad"}); return err },
		func() error { _, err := client.GetAd(ctx, "bad"); return err },
		func() error { _, err := client.CreateAd(ctx, CreateAdRequest{}); return err },
		func() error { _, err := client.UpdateAd(ctx, testAdID, UpdateEntityRequest{}); return err },
		func() error { _, err := client.SetAdStatus(ctx, testAdID, "DELETED"); return err },
		func() error { _, err := client.GetAccountStats(ctx, StatsRequest{}); return err },
		func() error {
			_, err := client.GetAccountStats(ctx, StatsRequest{
				Granularity: GranularityDay, StartTime: &misaligned, EndTime: &validStart, Fields: []string{"spend"},
			})
			return err
		},
		func() error {
			_, err := client.GetAccountStats(ctx, StatsRequest{Granularity: GranularityTotal, StartTime: &validStart, Fields: []string{"spend"}})
			return err
		},
	}
	for index, call := range invalidCalls {
		if err := call(); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Errorf("call %d error=%v", index, err)
		}
	}
}

func TestPrimitiveValidation(t *testing.T) {
	valid := []bool{
		validUUID(testAdAccountID), !validUUID("11111111-1111-1111-1111-11111111111z"),
		validOpaque("cursor", 10), !validOpaque(" cursor", 10),
		validText("name", 10), !validText(" name", 10),
		validUpperIdentifier("AD_ACCOUNT"), !validUpperIdentifier("ad-account"),
		validPage("", 0, 1000), !validPage("", 49, 1000),
		validCountryCodes([]string{"US", "ca"}), !validCountryCodes([]string{"US", "us"}),
		validFields([]string{"impressions", "spend"}), !validFields([]string{"spend", "spend"}),
		validGranularity(GranularityLifetime), !validGranularity("WEEK"),
	}
	for index, value := range valid {
		if !value {
			t.Errorf("validation %d failed", index)
		}
	}
	if err := checkResponse("test", responseMeta{RequestStatus: "success"}, []subRequestState{{Status: "success"}}); err != nil {
		t.Fatalf("lower-case success rejected: %v", err)
	}
}
