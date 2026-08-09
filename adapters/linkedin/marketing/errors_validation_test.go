package marketing

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

func TestHTTPErrorClassificationRetryAndRedaction(t *testing.T) {
	header := http.Header{"Retry-After": {"2.5"}, "X-Li-Uuid": {"request-123"}}
	err := decodeHTTPError(http.StatusTooManyRequests, header, []byte(`{"serviceErrorCode":65600,"message":"quota authorization Bearer-secret access_token=token-value"}`))
	hub := hubError(t, err)
	if !errors.Is(err, socialhub.ErrRateLimited) || hub.Class != socialhub.ClassRetryable || hub.RequestID != "request-123" ||
		hub.RetryAfter != 2500*time.Millisecond || hub.PlatformCode != "65600" || strings.Contains(hub.PlatformMessage, "Bearer-secret") || strings.Contains(hub.PlatformMessage, "token-value") {
		t.Fatalf("error=%#v", hub)
	}

	cases := []struct {
		status int
		target error
	}{
		{http.StatusBadRequest, socialhub.ErrInvalidArgument},
		{http.StatusUnauthorized, socialhub.ErrUnauthenticated},
		{http.StatusForbidden, socialhub.ErrPermissionDenied},
		{http.StatusNotFound, socialhub.ErrNotFound},
		{http.StatusConflict, socialhub.ErrConflict},
		{http.StatusServiceUnavailable, socialhub.ErrUnavailable},
	}
	for _, test := range cases {
		if err := decodeHTTPError(test.status, nil, []byte(`{"code":"problem","message":"failure"}`)); !errors.Is(err, test.target) {
			t.Errorf("status %d error=%v", test.status, err)
		}
	}
}

func TestNumericIDAndValidationBoundaries(t *testing.T) {
	validJSON := []string{`1234567890123456789`, `"12345"`}
	for _, encoded := range validJSON {
		var id NumericID
		if err := json.Unmarshal([]byte(encoded), &id); err != nil || id == "" {
			t.Errorf("encoded=%s id=%q err=%v", encoded, id, err)
		}
	}
	for _, encoded := range []string{`0`, `-1`, `1.5`, `"bad"`, `null`, `{}`} {
		var id NumericID
		if err := json.Unmarshal([]byte(encoded), &id); err == nil {
			t.Errorf("encoded=%s unexpectedly valid", encoded)
		}
	}
	if validMoney(Money{Amount: "0", CurrencyCode: "USD"}) || validMoney(Money{Amount: "1e3", CurrencyCode: "USD"}) ||
		validLocale(Locale{Language: "EN", Country: "us"}) || validSchedule(RunSchedule{Start: 2, End: 1}) {
		t.Fatal("invalid value accepted")
	}
	invalidTargeting := validCampaignRequest().TargetingCriteria
	invalidTargeting.Include.And = invalidTargeting.Include.And[:1]
	if validTargeting(invalidTargeting) || validFields([]string{"clicks", "clicks"}) || validOAuthScopes([]string{readAdsScope, readAdsScope}) {
		t.Fatal("invalid targeting, fields, or scopes accepted")
	}
}

func TestWorkflowInputValidationAndBatchError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		assertAPIRequest(t, request)
		if request.Method == http.MethodPost && strings.HasSuffix(request.URL.Path, "/creatives") {
			writeValue(t, writer, http.StatusOK, map[string]any{"elements": []any{map[string]any{
				"status": 400, "error": map[string]any{"code": "INVALID_VALUE", "message": "bad creative"},
			}}})
			return
		}
		t.Fatal("unexpected request")
	}))
	defer server.Close()
	_, client := newTestAdapter(t, server)

	invalidCalls := []func() error{
		func() error { _, err := client.GetCampaignGroup(context.Background(), "bad"); return err },
		func() error {
			_, err := client.ListCampaignGroups(context.Background(), ListRequest{MaxResults: 1001})
			return err
		},
		func() error {
			_, err := client.UpdateCampaignGroup(context.Background(), testCampaignGroupID, UpdateCampaignGroupRequest{})
			return err
		},
		func() error {
			_, err := client.SetCampaignGroupStatus(context.Background(), testCampaignGroupID, StatusCompleted)
			return err
		},
		func() error { return client.ArchiveCampaignGroup(context.Background(), "bad") },
		func() error { _, err := client.GetCampaign(context.Background(), "bad"); return err },
		func() error {
			_, err := client.ListCampaigns(context.Background(), ListRequest{Statuses: []Status{"BAD"}})
			return err
		},
		func() error {
			_, err := client.CreateCampaign(context.Background(), CreateCampaignRequest{})
			return err
		},
		func() error {
			_, err := client.UpdateCampaign(context.Background(), testCampaignID, UpdateCampaignRequest{})
			return err
		},
		func() error {
			_, err := client.SetCampaignStatus(context.Background(), testCampaignID, StatusDraft)
			return err
		},
		func() error { return client.ArchiveCampaign(context.Background(), "bad") },
		func() error { _, err := client.GetCreative(context.Background(), "bad"); return err },
		func() error {
			_, err := client.ListCreatives(context.Background(), ListCreativesRequest{MaxResults: 101})
			return err
		},
		func() error {
			_, err := client.CreateCreative(context.Background(), CreateCreativeRequest{})
			return err
		},
		func() error {
			_, err := client.SetCreativeStatus(context.Background(), testCreativeID, StatusDraft)
			return err
		},
		func() error { return client.ArchiveCreative(context.Background(), "bad") },
		func() error { _, err := client.GetAdAnalytics(context.Background(), AnalyticsRequest{}); return err },
	}
	for index, call := range invalidCalls {
		if err := call(); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Errorf("call %d error=%v", index, err)
		}
	}

	_, err := client.CreateCreative(context.Background(), CreateCreativeRequest{
		CampaignID: testCampaignID, ContentURN: "urn:li:share:6778045555198214144",
	})
	if !errors.Is(err, socialhub.ErrInvalidArgument) || hubError(t, err).PlatformCode != "INVALID_VALUE" {
		t.Fatalf("batch error=%v", err)
	}
}

func TestSecretResolutionAndClientConfigurationFailures(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	defer server.Close()
	adapter := &Adapter{}
	config := testConfig(server.URL)
	if err := adapter.Init(context.Background(), config, socialhub.WithHTTPClient(server.Client()), socialhub.WithSecretResolver(mapResolver{})); err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.Client(context.Background(), "b2b-demand"); !errors.Is(err, socialhub.ErrUnauthenticated) {
		t.Fatalf("client secret resolution=%v", err)
	}
	if _, err := adapter.OAuth(context.Background(), "b2b-demand"); !errors.Is(err, socialhub.ErrUnauthenticated) {
		t.Fatalf("OAuth secret resolution=%v", err)
	}
}
