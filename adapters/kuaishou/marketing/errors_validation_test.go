package marketing

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

func TestBusinessAndHTTPErrorClassification(t *testing.T) {
	tests := []struct {
		name       string
		status     int
		body       string
		wantCode   socialhub.ErrorCode
		wantClass  socialhub.ErrorClass
		provider   string
		retryAfter time.Duration
	}{
		{"rate", http.StatusOK, `{"code":400001,"message":"slow","request_id":"req-1"}`, socialhub.CodeRateLimited, socialhub.ClassRetryable, "400001", time.Minute},
		{"temporary", http.StatusOK, `{"code":410000,"message":"busy"}`, socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, "410000", 0},
		{"permission", http.StatusOK, `{"code":400004,"message":"denied"}`, socialhub.CodePermissionDenied, socialhub.ClassUserAction, "400004", 0},
		{"argument", http.StatusOK, `{"code":401001,"message":"bad"}`, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, "401001", 0},
		{"unknown", http.StatusOK, `{"code":999999,"message":"unknown"}`, socialhub.CodePlatformError, socialhub.ClassPermanent, "999999", 0},
		{"missing code", http.StatusOK, `{"data":{}}`, socialhub.CodePlatformError, socialhub.ClassPermanent, "", 0},
		{"http rate", http.StatusTooManyRequests, `{"code":429,"message":"slow"}`, socialhub.CodeRateLimited, socialhub.ClassRetryable, "429", 1500 * time.Millisecond},
		{"http server", http.StatusBadGateway, `{}`, socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, "", 0},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
				writer.Header().Set("X-Request-ID", "header-req")
				if test.name == "http rate" {
					writer.Header().Set("Retry-After", "1.5")
				}
				writeJSON(writer, test.status, test.body)
			}))
			defer server.Close()
			_, client := newTestAdapter(t, server)
			_, err := client.ListCampaigns(context.Background(), ListCampaignsRequest{})
			hub := hubError(t, err)
			if hub.Code != test.wantCode || hub.Class != test.wantClass || hub.PlatformCode != test.provider || hub.RetryAfter != test.retryAfter {
				t.Fatalf("error=%#v", hub)
			}
			if test.provider != "" && hub.RequestID == "" {
				t.Fatal("request ID missing")
			}
		})
	}
}

func TestValidationAndResponseContracts(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/gw/dsp/campaign/list":
			writeJSON(writer, http.StatusOK, `{"code":0,"data":{"total_count":1,"details":[{"campaign_id":1,"advertiser_id":999}]}}`)
		case "/gw/dsp/campaign/create":
			writeJSON(writer, http.StatusOK, `{"code":0,"data":{"campaign_id":0}}`)
		case "/gw/dsp/campaign/update":
			writeJSON(writer, http.StatusOK, `{"code":0,"data":{"campaign_id":2}}`)
		case "/v1/ad_unit/update/status":
			writeJSON(writer, http.StatusOK, `{"code":0,"data":{"unit_ids":[999]}}`)
		case "/v1/creative/update/status":
			writeJSON(writer, http.StatusOK, `{"code":0,"data":{"errors":[{"creative_id":303,"error_code":1,"error_msg":"denied"}]}}`)
		case "/v1/report/account_report":
			writeJSON(writer, http.StatusOK, `{"code":0,"data":{"total_count":1,"details":[{"account_id":999}]}}`)
		default:
			writeJSON(writer, http.StatusOK, `{"code":0,"data":{}}`)
		}
	}))
	defer server.Close()
	_, client := newTestAdapter(t, server)
	ctx := context.Background()
	contractCalls := []func() error{
		func() error { _, err := client.ListCampaigns(ctx, ListCampaignsRequest{}); return err },
		func() error { _, err := client.CreateCampaign(ctx, validCampaignRequest()); return err },
		func() error { name := "x"; return client.UpdateCampaign(ctx, 1, UpdateCampaignRequest{Name: &name}) },
		func() error { _, err := client.SetUnitStatus(ctx, 202, PutStatusPaused); return err },
		func() error { _, err := client.SetCreativeStatus(ctx, 303, PutStatusPaused); return err },
		func() error {
			_, err := client.GetReport(ctx, ReportRequest{Level: ReportLevelAccount, StartDate: "2026-08-01", EndDate: "2026-08-02"})
			return err
		},
	}
	for index, call := range contractCalls {
		if err := call(); err == nil || hubError(t, err).Code != socialhub.CodePlatformError {
			t.Fatalf("contract call %d error=%v", index, err)
		}
	}

	invalidCalls := []func() error{
		func() error {
			_, err := client.ListCampaigns(ctx, ListCampaignsRequest{IDs: []int64{1, 1}})
			return err
		},
		func() error { _, err := client.CreateCampaign(ctx, CreateCampaignRequest{}); return err },
		func() error {
			input := validCampaignRequest()
			input.Fields["put_status"] = 1
			_, err := client.CreateCampaign(ctx, input)
			return err
		},
		func() error { return client.UpdateCampaign(ctx, 0, UpdateCampaignRequest{}) },
		func() error { _, err := client.SetCampaignStatus(ctx, 0, 0); return err },
		func() error { _, err := client.ListUnits(ctx, ListUnitsRequest{StartDate: "bad"}); return err },
		func() error {
			input := validUnitRequest()
			input.CPABid = 0
			_, err := client.CreateUnit(ctx, input)
			return err
		},
		func() error {
			input := validUnitRequest()
			input.Fields["target"] = map[string]any{}
			_, err := client.CreateUnit(ctx, input)
			return err
		},
		func() error { _, err := client.SetUnitStatus(ctx, 0, 0); return err },
		func() error { _, err := client.ListCreatives(ctx, ListCreativesRequest{PageSize: 201}); return err },
		func() error { _, err := client.CreateCreative(ctx, CreateCreativeRequest{}); return err },
		func() error {
			input := validCreativeRequest()
			input.Fields["unit_id"] = 1
			_, err := client.CreateCreative(ctx, input)
			return err
		},
		func() error { _, err := client.SetCreativeStatus(ctx, 0, 0); return err },
		func() error { _, err := client.GetReport(ctx, ReportRequest{}); return err },
		func() error {
			_, err := client.GetReport(ctx, ReportRequest{Level: ReportLevelAccount, StartDate: "2026-07-01", EndDate: "2026-08-01"})
			return err
		},
		func() error {
			_, err := client.GetReport(ctx, ReportRequest{Level: ReportLevelAccount, StartDate: "2026-08-01", EndDate: "2026-08-08"})
			return err
		},
		func() error {
			_, err := client.GetReport(ctx, ReportRequest{Level: ReportLevelAccount, StartDate: "2026-08-01", EndDate: "2026-08-02", ExtendInfo: []string{"photo"}})
			return err
		},
	}
	for index, call := range invalidCalls {
		if err := call(); err == nil || hubError(t, err).Code != socialhub.CodeInvalidArgument {
			t.Fatalf("invalid call %d error=%v", index, err)
		}
	}
}

func TestAPIRedirectRejectsTokenForwarding(t *testing.T) {
	forwarded := false
	target := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, request *http.Request) {
		forwarded = true
		if request.Header.Get("Access-Token") != "" {
			t.Error("credential was forwarded")
		}
	}))
	defer target.Close()
	source := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		http.Redirect(writer, request, target.URL, http.StatusFound)
	}))
	defer source.Close()
	_, client := newTestAdapter(t, source)
	_, err := client.ListCampaigns(context.Background(), ListCampaignsRequest{})
	if err == nil || forwarded || strings.Contains(strings.ToLower(err.Error()), "access-token") {
		t.Fatalf("err=%v forwarded=%v", err, forwarded)
	}
}

func TestHelpers(t *testing.T) {
	if validEndpoint("https://user@example.com") || validEndpoint("https://example.com?q=1") || validEndpoint("relative") {
		t.Fatal("invalid endpoint accepted")
	}
	if parseRetryAfter("bad") != 0 || parseRetryAfter("999999") != 0 || businessRetryAfter(400001) != time.Minute {
		t.Fatal("invalid retry duration")
	}
	if got := boundedMessage(strings.Repeat("界", 20), 10); len([]rune(got)) != 10 {
		t.Fatalf("bounded message length=%d", len([]rune(got)))
	}
	message := redactSensitive("access_token=s3cr3t refresh_token:'r3fr3sh' secret=c1ient auth_code=c0de")
	if strings.Contains(message, "s3cr3t") || strings.Contains(message, "r3fr3sh") || strings.Contains(message, "c1ient") || strings.Contains(message, "c0de") {
		t.Fatalf("redaction failed: %q", message)
	}
}
