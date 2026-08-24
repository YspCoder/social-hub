package oceanengine

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"social-hub/pkg/socialhub"
)

func TestWorkflowValidation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("invalid request reached server")
	}))
	defer server.Close()
	_, client := newTestAdapter(t, server)
	ctx := context.Background()
	negative := -1.0

	tests := []struct {
		name string
		call func() error
	}{
		{"project required", func() error { _, err := client.CreateProject(ctx, CreateProjectRequest{}); return err }},
		{"project delivery", func() error {
			input := validProjectRequest()
			input.DeliverySetting.Budget = &negative
			_, err := client.CreateProject(ctx, input)
			return err
		}},
		{"project reserved", func() error {
			input := validProjectRequest()
			input.Fields["advertiser_id"] = int64(999)
			_, err := client.CreateProject(ctx, input)
			return err
		}},
		{"project sensitive", func() error {
			input := validProjectRequest()
			input.Fields["access_token"] = "leak"
			_, err := client.CreateProject(ctx, input)
			return err
		}},
		{"project field name", func() error {
			input := validProjectRequest()
			input.Fields["Bad-Key"] = true
			_, err := client.CreateProject(ctx, input)
			return err
		}},
		{"project list", func() error {
			_, err := client.ListProjects(ctx, ListProjectsRequest{Fields: []string{"Bad"}})
			return err
		}},
		{"project update empty", func() error { return client.UpdateProject(ctx, 1, UpdateProjectRequest{}) }},
		{"project update name", func() error { name := " "; return client.UpdateProject(ctx, 1, UpdateProjectRequest{Name: &name}) }},
		{"project status", func() error { return client.SetProjectStatus(ctx, 0, "DELETE") }},
		{"promotion create", func() error { _, err := client.CreatePromotion(ctx, CreatePromotionRequest{}); return err }},
		{"promotion reserved", func() error {
			_, err := client.CreatePromotion(ctx, CreatePromotionRequest{ProjectID: 1, Name: "x", Fields: map[string]any{"operation": "ENABLE"}})
			return err
		}},
		{"promotion list", func() error { _, err := client.ListPromotions(ctx, ListPromotionsRequest{PageSize: 101}); return err }},
		{"promotion update", func() error { return client.UpdatePromotion(ctx, 1, UpdatePromotionRequest{}) }},
		{"promotion status", func() error { return client.SetPromotionStatus(ctx, 1, "PAUSE") }},
		{"report required", func() error { _, err := client.GetCustomReport(ctx, CustomReportRequest{}); return err }},
		{"report ordering", func() error {
			_, err := client.GetCustomReport(ctx, validReportRequest(ReportOrder{Field: "cost", Type: "SIDEWAYS"}))
			return err
		}},
		{"report filter", func() error {
			input := validReportRequest()
			input.Filters = []ReportFilter{{Field: "Bad", Values: []string{"x"}}}
			_, err := client.GetCustomReport(ctx, input)
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

func TestEnvelopeAndHTTPFailures(t *testing.T) {
	tests := []struct {
		name      string
		status    int
		body      string
		wantCode  socialhub.ErrorCode
		platform  string
		requestID string
		retryable bool
	}{
		{"business", http.StatusOK, `{"code":40100,"message":"permission failed","request_id":"biz-1","data":null}`, socialhub.CodePlatformError, "40100", "biz-1", false},
		{"missing code", http.StatusOK, `{"message":"bad contract","data":{"list":[]}}`, socialhub.CodePlatformError, "", "", false},
		{"missing data", http.StatusOK, `{"code":0,"message":"OK"}`, socialhub.CodePlatformError, "", "", false},
		{"unauthorized", http.StatusUnauthorized, `{"code":401,"message":"expired","request_id":"http-1"}`, socialhub.CodeUnauthenticated, "401", "http-1", false},
		{"rate limited", http.StatusTooManyRequests, `{"code":429,"message":"slow"}`, socialhub.CodeRateLimited, "429", "", true},
		{"server", http.StatusBadGateway, `{}`, socialhub.CodeTemporarilyUnavailable, "", "", true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
				if test.status == http.StatusTooManyRequests {
					writer.Header().Set("Retry-After", "1.5")
				}
				writeJSON(writer, test.status, test.body)
			}))
			defer server.Close()
			_, client := newTestAdapter(t, server)
			_, err := client.ListProjects(context.Background(), ListProjectsRequest{})
			hub := hubError(t, err)
			if hub.Code != test.wantCode || hub.PlatformCode != test.platform || hub.RequestID != test.requestID || hub.Retryable() != test.retryable {
				t.Fatalf("error=%#v", hub)
			}
			if test.status == http.StatusTooManyRequests && hub.RetryAfter.String() != "1.5s" {
				t.Fatalf("retry_after=%v", hub.RetryAfter)
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
		{"cross account project", "/open_api/v3.0/project/list/", `{"code":0,"data":{"list":[{"project_id":1,"advertiser_id":999}],"page_info":{}}}`, func(client *Client) error {
			_, err := client.ListProjects(context.Background(), ListProjectsRequest{})
			return err
		}},
		{"invalid project id", "/open_api/v3.0/project/create/", `{"code":0,"data":{"project_id":0}}`, func(client *Client) error {
			_, err := client.CreateProject(context.Background(), validProjectRequest())
			return err
		}},
		{"project partial update", "/open_api/v3.0/project/update/", `{"code":0,"request_id":"partial","data":{"project_id":1,"error_list":[{"error_code":7,"error_message":"rejected"}]}}`, func(client *Client) error {
			name := "name"
			return client.UpdateProject(context.Background(), 1, UpdateProjectRequest{Name: &name})
		}},
		{"project status error", "/open_api/v3.0/project/status/update/", `{"code":0,"data":{"errors":[{"project_id":1,"error_message":"rejected"}]}}`, func(client *Client) error { return client.SetProjectStatus(context.Background(), 1, OperationDisable) }},
		{"cross account promotion", "/open_api/v3.0/promotion/list/", `{"code":0,"data":{"list":[{"promotion_id":2,"advertiser_id":999,"project_id":1}],"page_info":{}}}`, func(client *Client) error {
			_, err := client.ListPromotions(context.Background(), ListPromotionsRequest{})
			return err
		}},
		{"invalid promotion id", "/open_api/v3.0/promotion/create/", `{"code":0,"data":{"promotion_id":0}}`, func(client *Client) error {
			_, err := client.CreatePromotion(context.Background(), CreatePromotionRequest{ProjectID: 1, Name: "name"})
			return err
		}},
		{"promotion update mismatch", "/open_api/v3.0/promotion/update/", `{"code":0,"data":{"promotion_id":999,"error_list":[]}}`, func(client *Client) error {
			return client.UpdatePromotion(context.Background(), 2, UpdatePromotionRequest{Name: "name"})
		}},
		{"promotion status error", "/open_api/v3.0/promotion/status/update/", `{"code":0,"data":{"promotion_ids":[],"errors":[{"promotion_id":2,"error_message":"rejected"}]}}`, func(client *Client) error {
			return client.SetPromotionStatus(context.Background(), 2, OperationDisable)
		}},
		{"promotion status unconfirmed", "/open_api/v3.0/promotion/status/update/", `{"code":0,"data":{"promotion_ids":[],"errors":[]}}`, func(client *Client) error {
			return client.SetPromotionStatus(context.Background(), 2, OperationDisable)
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
			if err := test.call(client); err == nil || hubError(t, err).Code != socialhub.CodePlatformError {
				t.Fatalf("error=%v", err)
			}
		})
	}
}

func TestRedirectIsRejectedWithoutTokenForwarding(t *testing.T) {
	forwarded := false
	target := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, request *http.Request) {
		forwarded = true
		if request.Header.Get("Access-Token") != "" {
			t.Error("credential was forwarded")
		}
	}))
	defer target.Close()
	source := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		http.Redirect(writer, &http.Request{}, target.URL, http.StatusFound)
	}))
	defer source.Close()
	_, client := newTestAdapter(t, source)
	_, err := client.ListProjects(context.Background(), ListProjectsRequest{})
	if err == nil || forwarded {
		t.Fatalf("err=%v forwarded=%v", err, forwarded)
	}
}

func TestHelpers(t *testing.T) {
	if validEndpoint("https://user@example.com") || validEndpoint("https://example.com?q=1") || validEndpoint("relative") {
		t.Fatal("invalid endpoint accepted")
	}
	if parseRetryAfter("bad") != 0 || parseRetryAfter("999999") != 0 {
		t.Fatal("invalid retry-after accepted")
	}
	if got := boundedMessage(strings.Repeat("界", 600), 10); len([]rune(got)) != 10 {
		t.Fatalf("bounded message length=%d", len([]rune(got)))
	}
	if firstNonEmpty("", " value ") != " value " || containsID([]int64{1, 2}, 3) {
		t.Fatal("helper result invalid")
	}
}

func validReportRequest(order ...ReportOrder) CustomReportRequest {
	return CustomReportRequest{
		Dimensions: []string{"stat_time_day"}, Metrics: []string{"cost"},
		Filters: []ReportFilter{}, OrderBy: order,
		StartTime: "2026-08-01", EndTime: "2026-08-02",
	}
}
