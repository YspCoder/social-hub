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
		{"rate", http.StatusOK, `{"code":40100,"message":"slow","request_id":"req-1"}`, socialhub.CodeRateLimited, socialhub.ClassRetryable, "40100", 5 * time.Minute},
		{"asset retry", http.StatusOK, `{"code":40902,"message":"fetch failed","request_id":"req-1"}`, socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, "40902", 0},
		{"permission", http.StatusOK, `{"code":40118,"message":"denied","request_id":"req-1"}`, socialhub.CodePermissionDenied, socialhub.ClassUserAction, "40118", 0},
		{"auth", http.StatusOK, `{"code":40101,"message":"access_token=secret-token expired","request_id":"req-1"}`, socialhub.CodeUnauthenticated, socialhub.ClassUserAction, "40101", 0},
		{"not found", http.StatusOK, `{"code":40300,"message":"missing","request_id":"req-1"}`, socialhub.CodeNotFound, socialhub.ClassPermanent, "40300", 0},
		{"argument", http.StatusOK, `{"code":40700,"message":"bad","request_id":"req-1"}`, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, "40700", 0},
		{"unknown", http.StatusOK, `{"code":99999,"message":"unknown","request_id":"req-1"}`, socialhub.CodePlatformError, socialhub.ClassPermanent, "99999", 0},
		{"missing code", http.StatusOK, `{"data":{}}`, socialhub.CodePlatformError, socialhub.ClassPermanent, "", 0},
		{"missing data", http.StatusOK, `{"code":0}`, socialhub.CodePlatformError, socialhub.ClassPermanent, "", 0},
		{"http rate", http.StatusTooManyRequests, `{"code":429,"message":"slow","request_id":"req-http"}`, socialhub.CodeRateLimited, socialhub.ClassRetryable, "429", 1500 * time.Millisecond},
		{"http server", http.StatusBadGateway, `{}`, socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, "", 0},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
				writer.Header().Set("X-Request-ID", "header-request")
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
			if strings.Contains(hub.PlatformMessage, "secret-token") || strings.Contains(strings.ToLower(err.Error()), "secret-token") {
				t.Fatalf("secret leaked: %#v", hub)
			}
		})
	}
}

func TestValidationAndResponseContracts(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/v1.3/campaign/get/":
			writeJSON(writer, http.StatusOK, pageResponse(`[{"campaign_id":"101","advertiser_id":"999"}]`, 1, false))
		case "/v1.3/campaign/create/":
			writeJSON(writer, http.StatusOK, `{"code":0,"data":{"campaign_id":"bad"}}`)
		case "/v1.3/campaign/update/":
			writeJSON(writer, http.StatusOK, `{"code":0,"data":{"campaign_id":"999"}}`)
		case "/v1.3/adgroup/create/":
			writeJSON(writer, http.StatusOK, `{"code":0,"data":{"adgroup_id":"201","operation_status":"ENABLE"}}`)
		case "/v1.3/ad/create/":
			writeJSON(writer, http.StatusOK, `{"code":0,"data":{"ad_ids":["301"]}}`)
		case "/v1.3/adgroup/status/update/":
			writeJSON(writer, http.StatusOK, `{"code":0,"data":{"error_list":[{"adgroup_id":"201","error_message":"access_token=secret-token denied"}]}}`)
		case "/v1.3/report/integrated/get/":
			writeJSON(writer, http.StatusOK, pageResponse(`[{"dimensions":{"advertiser_id":"999"},"metrics":{"spend":"1"}}]`, 1, false))
		default:
			writeJSON(writer, http.StatusOK, `{"code":0,"data":{}}`)
		}
	}))
	defer server.Close()
	_, client := newTestAdapter(t, server)
	ctx := context.Background()
	name := "new"
	contractCalls := []func() error{
		func() error { _, err := client.ListCampaigns(ctx, ListCampaignsRequest{}); return err },
		func() error { _, err := client.CreateCampaign(ctx, validCampaignRequest()); return err },
		func() error {
			_, err := client.UpdateCampaign(ctx, "101", UpdateCampaignRequest{Name: &name})
			return err
		},
		func() error { _, err := client.CreateAdGroup(ctx, validAdGroupRequest()); return err },
		func() error { _, err := client.CreateAds(ctx, validAdsRequest()); return err },
		func() error { _, err := client.SetAdGroupStatus(ctx, "201", StatusDisable); return err },
		func() error {
			_, err := client.GetReport(ctx, validReportRequest("2026-08-01", "2026-08-01", []string{"stat_time_day"}))
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
			_, err := client.ListCampaigns(ctx, ListCampaignsRequest{IDs: []string{"1", "1"}})
			return err
		},
		func() error { _, err := client.CreateCampaign(ctx, CreateCampaignRequest{}); return err },
		func() error {
			input := validCampaignRequest()
			input.Fields["operation_status"] = "ENABLE"
			_, err := client.CreateCampaign(ctx, input)
			return err
		},
		func() error {
			input := validCampaignRequest()
			input.ObjectiveType = "RF_REACH"
			_, err := client.CreateCampaign(ctx, input)
			return err
		},
		func() error { _, err := client.UpdateCampaign(ctx, "", UpdateCampaignRequest{}); return err },
		func() error { _, err := client.SetCampaignStatus(ctx, "0", "BAD"); return err },
		func() error { _, err := client.ListAdGroups(ctx, ListAdGroupsRequest{PageSize: 1001}); return err },
		func() error { _, err := client.CreateAdGroup(ctx, CreateAdGroupRequest{}); return err },
		func() error {
			input := validAdGroupRequest()
			input.Fields["campaign_id"] = "999"
			_, err := client.CreateAdGroup(ctx, input)
			return err
		},
		func() error { _, err := client.UpdateAdGroup(ctx, "", UpdateAdGroupRequest{}); return err },
		func() error { _, err := client.ListAds(ctx, ListAdsRequest{IDs: []string{"bad"}}); return err },
		func() error { _, err := client.CreateAds(ctx, CreateAdsRequest{}); return err },
		func() error {
			input := validAdsRequest()
			input.Creatives[0].Fields["operation_status"] = "ENABLE"
			_, err := client.CreateAds(ctx, input)
			return err
		},
		func() error { _, err := client.SetAdStatus(ctx, "", StatusEnable); return err },
		func() error { _, err := client.GetReport(ctx, ReportRequest{}); return err },
		func() error {
			input := validReportRequest("2026-08-01", "2026-08-01", []string{"stat_time_day"})
			input.Filtering = []ReportFilter{{FieldName: "campaign_ids", FilterType: "IN", FilterValue: "bad"}}
			_, err := client.GetReport(ctx, input)
			return err
		},
	}
	for index, call := range invalidCalls {
		err := call()
		if err == nil {
			t.Fatalf("invalid call %d succeeded", index)
		}
		code := hubError(t, err).Code
		if index == 3 {
			if code != socialhub.CodeUnsupported {
				t.Fatalf("RF call code=%s", code)
			}
		} else if code != socialhub.CodeInvalidArgument {
			t.Fatalf("invalid call %d error=%v", index, err)
		}
	}
}

func TestReportInclusiveDateLimits(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writeJSON(writer, http.StatusOK, pageResponse(`[]`, 0, false))
	}))
	defer server.Close()
	_, client := newTestAdapter(t, server)
	tests := []struct {
		name       string
		start      string
		end        string
		dimensions []string
		valid      bool
	}{
		{"hour one", "2026-08-09", "2026-08-09", []string{"stat_time_hour"}, true},
		{"hour two", "2026-08-08", "2026-08-09", []string{"stat_time_hour"}, false},
		{"day thirty", "2026-07-11", "2026-08-09", []string{"stat_time_day"}, true},
		{"day thirty one", "2026-07-10", "2026-08-09", []string{"stat_time_day"}, false},
		{"aggregate 365", "2025-08-10", "2026-08-09", []string{"campaign_id"}, true},
		{"aggregate 366", "2025-08-09", "2026-08-09", []string{"campaign_id"}, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := client.GetReport(context.Background(), validReportRequest(test.start, test.end, test.dimensions))
			if test.valid && err != nil {
				t.Fatal(err)
			}
			if !test.valid && (err == nil || hubError(t, err).Code != socialhub.CodeInvalidArgument) {
				t.Fatalf("error=%v", err)
			}
		})
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
	if validEndpoint("https://user@example.com") || validEndpoint("https://example.com?q=1") || validEndpoint("relative") ||
		validHTTPURL("https://user@example.com") || validHTTPURL("relative") {
		t.Fatal("invalid endpoint accepted")
	}
	if parseRetryAfter("bad") != 0 || parseRetryAfter("999999") != 0 || businessRetryAfter(40100) != 5*time.Minute {
		t.Fatal("invalid retry duration")
	}
	if got := boundedMessage(strings.Repeat("ab", 20), 10); len([]rune(got)) != 10 {
		t.Fatalf("bounded message length=%d", len([]rune(got)))
	}
	message := redactSensitive("access_token=s3cr3t refresh_token:'r3fr3sh' secret=c1ient auth_code=c0de")
	for _, secret := range []string{"s3cr3t", "r3fr3sh", "c1ient", "c0de"} {
		if strings.Contains(message, secret) {
			t.Fatalf("redaction failed: %q", message)
		}
	}
}

func validReportRequest(start, end string, dimensions []string) ReportRequest {
	return ReportRequest{
		DataLevel: ReportLevelCampaign, StartDate: start, EndDate: end,
		Dimensions: dimensions, Metrics: []string{"spend"},
	}
}
