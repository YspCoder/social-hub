package taboola

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

func TestHTTPErrorMappingRetryAfterAndRedaction(t *testing.T) {
	header := http.Header{"Retry-After": {"17.5"}, "X-Request-Id": {"request-123"}}
	body := []byte(`{
		"http_status":429,
		"message":"authorization: Bearer super-secret access_token=other-secret",
		"offending_field":"thumbnail_url",
		"message_code":"campaign.inventory.error.server.thumbnailInvalidResolution",
		"message_code_english_template":"client_secret=template-secret",
		"template_parameters":["bearer param-secret"]
	}`)
	err := decodeHTTPError(http.StatusTooManyRequests, header, body)
	var api *APIError
	if !errors.As(err, &api) {
		t.Fatalf("error type=%T", err)
	}
	hub := hubError(t, err)
	if hub.Code != socialhub.CodeRateLimited || !hub.Retryable() || hub.RetryAfter != 17500*time.Millisecond || hub.RequestID != "request-123" || api.OffendingField != "thumbnail_url" || api.MessageCode == "" {
		t.Fatalf("api=%#v hub=%#v", api, hub)
	}
	combined := strings.ToLower(hub.PlatformMessage + api.MessageCodeEnglishTemplate + strings.Join(api.TemplateParameters, " "))
	for _, secret := range []string{"super-secret", "other-secret", "template-secret", "param-secret"} {
		if strings.Contains(combined, secret) {
			t.Fatalf("secret %q leaked in %q", secret, combined)
		}
	}
}

func TestHTTPErrorPreservesWorkflowOperation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writeJSON(t, writer, http.StatusTooManyRequests, map[string]any{"message": "slow down"})
	}))
	defer server.Close()
	_, client := newTestAdapter(t, server)
	_, err := client.CurrentAccount(t.Context())
	hub := hubError(t, err)
	if hub.Op != "account_current" || hub.Code != socialhub.CodeRateLimited {
		t.Fatalf("hub=%#v", hub)
	}
}

func TestErrorClassificationAndHelpers(t *testing.T) {
	tests := map[int]struct {
		code  socialhub.ErrorCode
		class socialhub.ErrorClass
	}{
		http.StatusBadRequest:          {socialhub.CodeInvalidArgument, socialhub.ClassPermanent},
		http.StatusUnprocessableEntity: {socialhub.CodeInvalidArgument, socialhub.ClassPermanent},
		http.StatusUnauthorized:        {socialhub.CodeUnauthenticated, socialhub.ClassUserAction},
		http.StatusForbidden:           {socialhub.CodePermissionDenied, socialhub.ClassUserAction},
		http.StatusNotFound:            {socialhub.CodeNotFound, socialhub.ClassPermanent},
		http.StatusGone:                {socialhub.CodeNotFound, socialhub.ClassPermanent},
		http.StatusConflict:            {socialhub.CodeConflict, socialhub.ClassPermanent},
		http.StatusTooManyRequests:     {socialhub.CodeRateLimited, socialhub.ClassRetryable},
		http.StatusBadGateway:          {socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable},
		http.StatusTeapot:              {socialhub.CodePlatformError, socialhub.ClassPermanent},
	}
	for status, expected := range tests {
		code, class := classifyError(status)
		if code != expected.code || class != expected.class {
			t.Errorf("status %d = %s/%s", status, code, class)
		}
	}
	if parseRetryAfter("3") != 3*time.Second || parseRetryAfter("-1") != 0 || parseRetryAfter("invalid") != 0 {
		t.Fatal("Retry-After parsing mismatch")
	}
	if boundedMessage(strings.Repeat("界", 10), 3) != "界界界" || firstNonEmpty("", " value ") != " value " {
		t.Fatal("message helpers mismatch")
	}
	if !strings.Contains(redactSensitive("client_secret=one access_token=two authorization: Bearer three"), "[REDACTED]") {
		t.Fatal("redaction did not run")
	}
	if (&APIError{}).Error() == "" || (&APIError{}).Unwrap() != nil || (&APIError{}).Retryable() {
		t.Fatal("nil APIError behavior mismatch")
	}
}

func TestValidationHelpersAndCampaignRules(t *testing.T) {
	if !validOpaque("abc", 3) || validOpaque(" abc", 10) || validOpaque("a\n", 10) ||
		!validPathID("demo-advertiser", false) || !validPathID("123", true) || validPathID("abc", true) ||
		!validText("Text", 4) || validText(" Text", 10) || !validDestinationURL("https://example.test/path") || validDestinationURL("file:///tmp/x") ||
		!validDimension("by_campaign") || validDimension("By-Campaign") {
		t.Fatal("basic validation mismatch")
	}
	if !validReportWindow("2026-08-01", "2026-08-09", false) || validReportWindow("2026-08-09", "2026-08-01", false) ||
		!validReportWindow("2026-08-09T00:00:00", "2026-08-09T23:59:59", true) || validReportWindow("bad", "bad", true) {
		t.Fatal("report window validation mismatch")
	}

	valid := validCreateCampaignRequest()
	if err := validateCreateCampaign(valid); err != nil {
		t.Fatal(err)
	}
	cases := []CreateCampaignRequest{
		func() CreateCampaignRequest { value := valid; value.BidStrategy = "OTHER"; return value }(),
		func() CreateCampaignRequest { value := valid; value.CPC = nil; return value }(),
		func() CreateCampaignRequest { value := valid; value.SpendingLimit = nil; return value }(),
		func() CreateCampaignRequest { value := valid; value.SpendingLimitModel = SpendingNone; return value }(),
		func() CreateCampaignRequest {
			value := valid
			value.StartDate, value.EndDate = "2026-08-10", "2026-08-09"
			return value
		}(),
	}
	for index, value := range cases {
		if err := validateCreateCampaign(value); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Errorf("case %d error=%v", index, err)
		}
	}
	daily := valid
	daily.SpendingLimitModel, daily.SpendingLimit, daily.DailyCap = SpendingNone, nil, floatPointer(10)
	if err := validateCreateCampaign(daily); err != nil {
		t.Fatalf("daily Campaign error=%v", err)
	}
	maxConversions := valid
	maxConversions.BidStrategy, maxConversions.CPC = BidStrategyMaxConversions, nil
	if err := validateCreateCampaign(maxConversions); err != nil {
		t.Fatalf("max conversions error=%v", err)
	}
	model := SpendingEntire
	if !validUpdateCampaign(UpdateCampaignRequest{SpendingLimitModel: &model}) || validUpdateCampaign(UpdateCampaignRequest{}) {
		t.Fatal("update Campaign validation mismatch")
	}
	if !validUpdateItem(UpdateItemRequest{Title: stringPointer("new")}) || validUpdateItem(UpdateItemRequest{URL: stringPointer("file:///tmp/x")}) {
		t.Fatal("update Item validation mismatch")
	}
}

func TestAccountAndAPIResponseFailures(t *testing.T) {
	badAccounts := []any{
		map[string]any{"results": []map[string]any{{"account_id": "bad/id"}}},
		map[string]any{"results": []Account{{AccountID: testAdvertiserID, PartnerTypes: []string{"PUBLISHER"}, CampaignTypes: []string{"PAID"}}}},
		map[string]any{"results": []Account{{AccountID: "another", PartnerTypes: []string{"ADVERTISER"}, CampaignTypes: []string{"PAID"}}}},
	}
	for index, response := range badAccounts {
		server := httptestServerJSON(t, response)
		_, client := newTestAdapter(t, server)
		var err error
		if index == 0 {
			_, err = client.AllowedAccounts(t.Context())
		} else {
			_, err = client.ValidateConfiguredAccount(t.Context())
		}
		if err == nil {
			t.Errorf("case %d unexpectedly succeeded", index)
		}
		server.Close()
	}
}

func httptestServerJSON(t *testing.T, response any) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writeJSON(t, writer, http.StatusOK, response)
	}))
}
