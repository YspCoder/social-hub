package tencentads

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

func TestWorkflowValidation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("invalid request reached server")
	}))
	defer server.Close()
	_, client := newTestAdapter(t, server)
	ctx := context.Background()
	badName := " "
	negative := int64(-1)

	tests := []struct {
		name string
		call func() error
	}{
		{"campaign list field", func() error {
			_, err := client.ListCampaigns(ctx, ListCampaignsRequest{Fields: []string{"Bad"}})
			return err
		}},
		{"campaign list filter", func() error {
			_, err := client.ListCampaigns(ctx, ListCampaignsRequest{Filtering: []Filtering{{Field: "id", Operator: "bad", Values: []string{"1"}}}})
			return err
		}},
		{"campaign create", func() error { _, err := client.CreateCampaign(ctx, CreateCampaignRequest{}); return err }},
		{"campaign budgets", func() error { _, err := client.CreateCampaign(ctx, validCampaignRequest(1, 1)); return err }},
		{"campaign reserved", func() error {
			input := validCampaignRequest(1, 0)
			input.Fields = map[string]any{"access_token": "leak"}
			_, err := client.CreateCampaign(ctx, input)
			return err
		}},
		{"campaign update empty", func() error { return client.UpdateCampaign(ctx, 1, UpdateCampaignRequest{}) }},
		{"campaign update name", func() error { return client.UpdateCampaign(ctx, 1, UpdateCampaignRequest{Name: &badName}) }},
		{"campaign update budget", func() error { return client.UpdateCampaign(ctx, 1, UpdateCampaignRequest{DailyBudget: &negative}) }},
		{"campaign status", func() error { return client.SetCampaignStatus(ctx, 0, "AD_STATUS_DELETED") }},
		{"adgroup create", func() error { _, err := client.CreateAdGroup(ctx, CreateAdGroupRequest{}); return err }},
		{"adgroup dates", func() error {
			input := validAdGroupRequest()
			input.EndDate = "2026-01-01"
			_, err := client.CreateAdGroup(ctx, input)
			return err
		}},
		{"adgroup reserved", func() error {
			input := validAdGroupRequest()
			input.Fields = map[string]any{"configured_status": "AD_STATUS_NORMAL"}
			_, err := client.CreateAdGroup(ctx, input)
			return err
		}},
		{"adgroup update", func() error { return client.UpdateAdGroup(ctx, 1, UpdateAdGroupRequest{}) }},
		{"adgroup patch", func() error { return client.UpdateAdGroup(ctx, 1, UpdateAdGroupRequest{BidAmount: &negative}) }},
		{"adgroup status", func() error { return client.SetAdGroupStatus(ctx, 1, "BAD") }},
		{"creative create", func() error { _, err := client.CreateAdCreative(ctx, CreateAdCreativeRequest{}); return err }},
		{"creative reserved", func() error {
			input := validCreativeRequest()
			input.Fields = map[string]any{"campaign_id": 9}
			_, err := client.CreateAdCreative(ctx, input)
			return err
		}},
		{"creative update", func() error { return client.UpdateAdCreative(ctx, 1, UpdateAdCreativeRequest{}) }},
		{"creative patch", func() error { return client.UpdateAdCreative(ctx, 1, UpdateAdCreativeRequest{Name: &badName}) }},
		{"report required", func() error { _, err := client.GetReport(ctx, ReportRequest{}); return err }},
		{"report dates", func() error {
			input := validReportRequest()
			input.DateRange.EndDate = "bad"
			_, err := client.GetReport(ctx, input)
			return err
		}},
		{"report group", func() error {
			input := validReportRequest()
			input.GroupBy = []string{"Bad"}
			_, err := client.GetReport(ctx, input)
			return err
		}},
		{"report order", func() error {
			input := validReportRequest()
			input.OrderBy = []OrderBy{{SortField: "cost", SortType: "bad"}}
			_, err := client.GetReport(ctx, input)
			return err
		}},
		{"report timeline", func() error {
			input := validReportRequest()
			input.TimeLine = " x"
			_, err := client.GetReport(ctx, input)
			return err
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.call()
			if err == nil || !errors.Is(err, socialhub.ErrInvalidArgument) {
				t.Fatalf("error=%v", err)
			}
		})
	}
}

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
		{"auth", http.StatusOK, `{"code":30102,"message":"access_token=secret expired"}`, socialhub.CodeUnauthenticated, socialhub.ClassUserAction, "30102", 0},
		{"rate", http.StatusOK, `{"code":11017,"message":"minute limit"}`, socialhub.CodeRateLimited, socialhub.ClassRetryable, "11017", time.Minute},
		{"permission", http.StatusOK, `{"code":12203,"message":"denied"}`, socialhub.CodePermissionDenied, socialhub.ClassUserAction, "12203", 0},
		{"argument", http.StatusOK, `{"code":12000,"message":"missing"}`, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, "12000", 0},
		{"unknown", http.StatusOK, `{"code":99999,"message":"unknown"}`, socialhub.CodePlatformError, socialhub.ClassPermanent, "99999", 0},
		{"missing code", http.StatusOK, `{"data":{"list":[]}}`, socialhub.CodePlatformError, socialhub.ClassPermanent, "", 0},
		{"missing data", http.StatusOK, `{"code":0}`, socialhub.CodePlatformError, socialhub.ClassPermanent, "", 0},
		{"http rate", http.StatusTooManyRequests, `{"code":429,"message":"slow"}`, socialhub.CodeRateLimited, socialhub.ClassRetryable, "429", 1500 * time.Millisecond},
		{"http server", http.StatusBadGateway, `{}`, socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, "", 0},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
				writer.Header().Set("X-TSA-Trace-Id", "trace-1")
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
			if test.provider != "" && hub.RequestID != "trace-1" {
				t.Fatalf("request id=%q", hub.RequestID)
			}
			if strings.Contains(err.Error(), "access_token=secret") {
				t.Fatalf("secret leaked: %v", err)
			}
		})
	}
}

func TestResponseContractFailures(t *testing.T) {
	tests := []struct {
		name string
		path string
		body string
		call func(*Client) error
	}{
		{"cross account advertiser", "/advertiser/get", `{"code":0,"data":{"list":[{"account_id":999}],"page_info":{"page":1,"page_size":1,"total_number":1,"total_page":1}}}`, func(client *Client) error { _, err := client.GetAdvertiser(context.Background()); return err }},
		{"cross account campaign", "/campaigns/get", `{"code":0,"data":{"list":[{"campaign_id":1,"account_id":999}],"page_info":{"page":1,"page_size":20,"total_number":1,"total_page":1}}}`, func(client *Client) error {
			_, err := client.ListCampaigns(context.Background(), ListCampaignsRequest{})
			return err
		}},
		{"bad page", "/campaigns/get", `{"code":0,"data":{"list":[],"page_info":{"page":0}}}`, func(client *Client) error {
			_, err := client.ListCampaigns(context.Background(), ListCampaignsRequest{})
			return err
		}},
		{"bad campaign id", "/campaigns/add", `{"code":0,"data":{"campaign_id":0}}`, func(client *Client) error {
			_, err := client.CreateCampaign(context.Background(), validCampaignRequest(1, 0))
			return err
		}},
		{"campaign update mismatch", "/campaigns/update", `{"code":0,"data":{"campaign_id":2}}`, func(client *Client) error {
			name := "name"
			return client.UpdateCampaign(context.Background(), 1, UpdateCampaignRequest{Name: &name})
		}},
		{"campaign status failed", "/campaigns/update_configured_status", `{"code":0,"data":{"list":[],"fail_id_list":[1]}}`, func(client *Client) error {
			return client.SetCampaignStatus(context.Background(), 1, ConfiguredStatusSuspend)
		}},
		{"campaign status item", "/campaigns/update_configured_status", `{"code":0,"data":{"list":[{"code":12203,"campaign_id":1}],"fail_id_list":[]}}`, func(client *Client) error {
			return client.SetCampaignStatus(context.Background(), 1, ConfiguredStatusSuspend)
		}},
		{"adgroup status missing", "/adgroups/update_configured_status", `{"code":0,"data":{"list":[],"fail_id_list":[]}}`, func(client *Client) error {
			return client.SetAdGroupStatus(context.Background(), 1, ConfiguredStatusSuspend)
		}},
		{"creative update mismatch", "/adcreatives/update", `{"code":0,"data":{"adcreative_id":2}}`, func(client *Client) error {
			name := "name"
			return client.UpdateAdCreative(context.Background(), 1, UpdateAdCreativeRequest{Name: &name})
		}},
		{"cross account report", "/daily_reports/get", `{"code":0,"data":{"list":[{"account_id":999}],"page_info":{"page":1,"page_size":20,"total_number":1,"total_page":1}}}`, func(client *Client) error {
			_, err := client.GetReport(context.Background(), validReportRequest())
			return err
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				if request.URL.Path != test.path {
					t.Errorf("path=%s want=%s", request.URL.Path, test.path)
				}
				writeJSON(writer, http.StatusOK, test.body)
			}))
			defer server.Close()
			_, client := newTestAdapter(t, server)
			if err := test.call(client); err == nil || hubError(t, err).Code == "" {
				t.Fatalf("error=%v", err)
			}
		})
	}
}

func TestAPIRedirectRejectsTokenForwarding(t *testing.T) {
	forwarded := false
	target := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, request *http.Request) {
		forwarded = true
		if request.URL.Query().Get("access_token") != "" {
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
	if err == nil || forwarded || strings.Contains(err.Error(), "access-token") {
		t.Fatalf("err=%v forwarded=%v", err, forwarded)
	}
}

func TestHelpers(t *testing.T) {
	if validEndpoint("https://user@example.com") || validEndpoint("https://example.com?q=1") || validEndpoint("relative") {
		t.Fatal("invalid endpoint accepted")
	}
	if parseRetryAfter("bad") != 0 || parseRetryAfter("999999") != 0 || businessRetryAfter(11018) != 24*time.Hour {
		t.Fatal("invalid retry duration")
	}
	if got := boundedMessage(strings.Repeat("界", 20), 10); len([]rune(got)) != 10 {
		t.Fatalf("bounded message length=%d", len([]rune(got)))
	}
	message := redactSensitive("access_token = s3cr3t refresh_token:'r3fr3sh' client_secret=c1ient authorization_code=c0de")
	if strings.Contains(message, "s3cr3t") || strings.Contains(message, "r3fr3sh") || strings.Contains(message, "c1ient") || strings.Contains(message, "c0de") {
		t.Fatalf("redaction failed: %q", message)
	}
	if firstNonEmpty("", " value ") != " value " || validStatus("BAD") || !validStatus(ConfiguredStatusNormal) {
		t.Fatal("helper result invalid")
	}
}

func validCampaignRequest(daily, total int64) CreateCampaignRequest {
	return CreateCampaignRequest{
		Name: "Launch", CampaignType: "CAMPAIGN_TYPE_NORMAL", PromotedObjectType: "PROMOTED_OBJECT_TYPE_LINK",
		DailyBudget: daily, TotalBudget: total,
	}
}

func validAdGroupRequest() CreateAdGroupRequest {
	return CreateAdGroupRequest{
		CampaignID: 1, Name: "Group", PromotedObjectType: "PROMOTED_OBJECT_TYPE_LINK",
		BillingEvent: "BILLINGEVENT_IMPRESSION", OptimizationGoal: "OPTIMIZATIONGOAL_IMPRESSION",
		BeginDate: "2026-08-01", EndDate: "2026-08-31",
	}
}

func validCreativeRequest() CreateAdCreativeRequest {
	return CreateAdCreativeRequest{
		CampaignID: 1, Name: "Creative", PromotedObjectType: "PROMOTED_OBJECT_TYPE_LINK", TemplateID: 2,
	}
}

func validReportRequest() ReportRequest {
	return ReportRequest{
		Granularity: ReportDaily, Level: "REPORT_LEVEL_ADGROUP",
		DateRange: ReportDateRange{StartDate: "2026-08-01", EndDate: "2026-08-02"},
	}
}
