package unitypublisher

import (
	"context"
	"errors"
	"math"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

func TestUnityErrorClassificationRateLimitsAndRedaction(t *testing.T) {
	body := []byte(`{"title":"Too Many Requests","detail":"access_token=secret-value exhausted","code":50,"type":"https://services.docs.unity.com/docs/errors/#50","status":429,"requestId":"request-body"}`)
	header := http.Header{
		"Retry-After": {"2.5"}, "RateLimit-Policy": {"20;w=1, 8000;w=3600"},
		"RateLimit": {"limit=20, remaining=0, reset=1"}, "Unity-RateLimit": {"limit=20, remaining=0, reset=1; limit=8000, remaining=0, reset=42"},
	}
	err := decodeHTTPError(http.StatusTooManyRequests, header, body)
	var api *APIError
	if !errors.As(err, &api) || !api.Retryable() || api.Hub.Code != socialhub.CodeRateLimited || api.Hub.RetryAfter != 2500*time.Millisecond ||
		api.Hub.RequestID != "request-body" || strings.Contains(api.Hub.PlatformMessage, "secret-value") || api.Hub.PlatformCode != "50" ||
		api.RateLimitPolicy == "" || api.RateLimit == "" || api.UnityRateLimit == "" {
		t.Fatalf("error=%#v", api)
	}
	if !errors.Is(err, socialhub.ErrRateLimited) || api.Error() == "" || api.Unwrap() == nil {
		t.Fatalf("wrapped error=%v", err)
	}
	if (&APIError{}).Error() == "" || (*APIError)(nil).Error() == "" || (*APIError)(nil).Unwrap() != nil || (*APIError)(nil).Retryable() {
		t.Fatal("nil API error contract failed")
	}
}

func TestErrorClassificationAndQuotaPolicy(t *testing.T) {
	tests := []struct {
		status int
		code   string
		want   socialhub.ErrorCode
		class  socialhub.ErrorClass
	}{
		{403, "63", socialhub.CodeApprovalRequired, socialhub.ClassUserAction},
		{403, "64", socialhub.CodeApprovalRequired, socialhub.ClassUserAction},
		{403, "65", socialhub.CodeUnauthenticated, socialhub.ClassUserAction},
		{400, "", socialhub.CodeInvalidArgument, socialhub.ClassPermanent},
		{413, "", socialhub.CodeInvalidArgument, socialhub.ClassPermanent},
		{415, "", socialhub.CodeInvalidArgument, socialhub.ClassPermanent},
		{422, "", socialhub.CodeInvalidArgument, socialhub.ClassPermanent},
		{401, "", socialhub.CodeUnauthenticated, socialhub.ClassUserAction},
		{402, "", socialhub.CodeApprovalRequired, socialhub.ClassUserAction},
		{403, "", socialhub.CodePermissionDenied, socialhub.ClassUserAction},
		{404, "", socialhub.CodeNotFound, socialhub.ClassPermanent},
		{410, "", socialhub.CodeNotFound, socialhub.ClassPermanent},
		{409, "", socialhub.CodeConflict, socialhub.ClassPermanent},
		{424, "", socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable},
		{429, "", socialhub.CodeRateLimited, socialhub.ClassRetryable},
		{503, "", socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable},
		{418, "", socialhub.CodePlatformError, socialhub.ClassPermanent},
	}
	for _, test := range tests {
		code, class := classifyError(test.status, test.code)
		if code != test.want || class != test.class {
			t.Errorf("status=%d platformCode=%q got=%s/%s", test.status, test.code, code, class)
		}
	}
	policy := DefaultQuotaPolicy()
	if policy.IPRequestsPerSecond != 40 || policy.ReadRequestsPerSecond != 20 || policy.ReadRequestsPerHour != 8000 ||
		policy.CreateRequestsPerSecond != 1 || policy.CreateRequestsPerHour != 60 ||
		policy.MutationRequestsPerSecond != 1 || policy.MutationRequestsPerHour != 200 {
		t.Fatalf("quota policy=%#v", policy)
	}
	if parseRetryAfter("1.5") != 1500*time.Millisecond || parseRetryAfter("bad") != 0 || firstNonEmpty("", "value") != "value" ||
		redactSensitive("secret_key: topsecret") == "secret_key: topsecret" {
		t.Fatal("error helpers failed")
	}
	malformed := decodeHTTPError(http.StatusBadRequest, http.Header{"X-Request-Id": {"request-header"}}, []byte("bad request"))
	if api := malformed.(*APIError); api.Hub.RequestID != "request-header" || api.Hub.PlatformCode != "http_400" {
		t.Fatalf("fallback error=%#v", api)
	}
}

func TestValidationPrimitivesAndTypedConfigurations(t *testing.T) {
	if !validEndpoint("https://example.test/path") || validEndpoint("https://user:pass@example.test") || validEndpoint("mailto:test@example.test") ||
		!validOrganizationID(testOrganizationID) || validOrganizationID("01") || validOrganizationID("bad") ||
		!validPathID(testApplicationID) || validPathID("bad/id") || !validUUID(testPlacementID) || validUUID("Rewarded_Placement") ||
		!validPlatform(PlatformVisionOS) || validPlatform("other") || !validStore(StoreAPK) || validStore("other") ||
		!finiteNonNegative(0) || finiteNonNegative(math.NaN()) || finiteNonNegative(-1) {
		t.Fatal("validation primitive mismatch")
	}
	admin := &AdminConfigurations{AllowSkipInSeconds: 5, VideoPlayableSkipInSeconds: 5, CloseTimerDuration: 5, TapsToClose: 2}
	valid := []PlacementRequest{
		{Name: "Rewarded", AdFormat: AdFormatRewarded},
		{Name: "Rewarded", AdFormat: AdFormatRewarded, AdFormatConfigurations: RewardedConfigurations{Name: "coins", Value: 100, AdminSettings: admin}},
		{Name: "Interstitial", AdFormat: AdFormatInterstitial, AdFormatConfigurations: InterstitialConfigurations{AdminSettings: admin}},
		{Name: "Banner", AdFormat: AdFormatBanner, AdFormatConfigurations: BannerConfigurations{BannerRefreshRate: 30, AdminSettings: admin}},
	}
	for _, value := range valid {
		if !validPlacementRequest(value) {
			t.Errorf("valid placement rejected: %#v", value)
		}
	}
	invalid := []PlacementRequest{
		{},
		{Name: "Native", AdFormat: AdFormatNative},
		{Name: "Mismatch", AdFormat: AdFormatBanner, AdFormatConfigurations: RewardedConfigurations{Name: "coins", Value: 1}},
		{Name: "Rewarded", AdFormat: AdFormatRewarded, AdFormatConfigurations: RewardedConfigurations{}},
		{Name: "Banner", AdFormat: AdFormatBanner, AdFormatConfigurations: BannerConfigurations{BannerRefreshRate: math.NaN()}},
		{Name: "Interstitial", AdFormat: AdFormatInterstitial, AdFormatConfigurations: InterstitialConfigurations{AdminSettings: &AdminConfigurations{TapsToClose: -1}}},
	}
	for _, value := range invalid {
		if validPlacementRequest(value) {
			t.Errorf("invalid placement accepted: %#v", value)
		}
	}
	transportErr := &url.Error{Op: "Get", URL: "https://example.test?token=secret", Err: errors.New("dial failed")}
	if sanitizeCause(transportErr).Error() != "dial failed" || sanitizeCause(nil) != nil || hubError(t, platformContractError("test", "bad")).Code != socialhub.CodePlatformError {
		t.Fatal("typed error helpers failed")
	}
}

func TestInvalidCallsDoNotReachNetwork(t *testing.T) {
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { requestCount++ }))
	defer server.Close()
	_, client := newTestAdapter(t, server)
	ctx := context.Background()
	badPlacement := PlacementRequest{Name: "Bad", AdFormat: AdFormatNative}
	for _, invoke := range []func() error{
		func() error {
			_, err := client.CreateApplication(ctx, CreateApplicationRequest{}, MutationOptions{})
			return err
		},
		func() error { _, err := client.GetApplication(ctx, "bad/id"); return err },
		func() error {
			_, err := client.UpdateApplication(ctx, testApplicationID, UpdateApplicationRequest{}, MutationOptions{})
			return err
		},
		func() error { _, err := client.GetApplicationTestMode(ctx, "bad/id"); return err },
		func() error {
			_, err := client.UpdateApplicationTestMode(ctx, testApplicationID, UpdateTestModeRequest{TestMode: "bad"}, MutationOptions{})
			return err
		},
		func() error {
			_, err := client.ListApplicationPlacements(ctx, testApplicationID, ListApplicationPlacementsRequest{AdFormats: []AdFormat{AdFormatNative}})
			return err
		},
		func() error {
			_, err := client.CreatePlacement(ctx, testApplicationID, badPlacement, MutationOptions{})
			return err
		},
		func() error { _, err := client.GetPlacement(ctx, testApplicationID, "Rewarded_Placement"); return err },
		func() error {
			_, err := client.UpdatePlacement(ctx, testApplicationID, testPlacementID, badPlacement, MutationOptions{})
			return err
		},
		func() error { return client.ArchivePlacement(ctx, "bad/id", testPlacementID, MutationOptions{}) },
		func() error { return client.RestorePlacement(ctx, testApplicationID, "bad", MutationOptions{}) },
		func() error { _, err := client.ListTestDevices(ctx, "other"); return err },
		func() error {
			_, err := client.CreateTestDevice(ctx, CreateTestDeviceRequest{}, MutationOptions{})
			return err
		},
		func() error { _, err := client.GetTestDevice(ctx, "bad"); return err },
		func() error {
			_, err := client.UpdateTestDevice(ctx, testDeviceID, UpdateTestDeviceRequest{}, MutationOptions{})
			return err
		},
		func() error { return client.DeleteTestDevice(ctx, "bad", MutationOptions{}) },
	} {
		if err := invoke(); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Errorf("validation error=%v", err)
		}
	}
	if requestCount != 0 {
		t.Fatalf("invalid calls made %d requests", requestCount)
	}
}

func TestResponseOwnershipAndContractChecks(t *testing.T) {
	tests := []struct {
		name     string
		response any
		invoke   func(*Client) error
		code     socialhub.ErrorCode
	}{
		{"application list", []Application{{ID: "bad/id"}}, func(client *Client) error { _, err := client.ListApplications(context.Background()); return err }, socialhub.CodePlatformError},
		{"application get", Application{ID: "other", Name: "Other", Platform: PlatformAndroid}, func(client *Client) error {
			_, err := client.GetApplication(context.Background(), testApplicationID)
			return err
		}, socialhub.CodePermissionDenied},
		{"test mode", ApplicationTestMode{ID: "other"}, func(client *Client) error {
			_, err := client.GetApplicationTestMode(context.Background(), testApplicationID)
			return err
		}, socialhub.CodePermissionDenied},
		{"placement list", []Placement{placementWithApplication("other")}, func(client *Client) error {
			_, err := client.ListApplicationPlacements(context.Background(), testApplicationID, ListApplicationPlacementsRequest{})
			return err
		}, socialhub.CodePermissionDenied},
		{"placement get", placementWithID("123e4567-e89b-12d3-a456-426614174000"), func(client *Client) error {
			_, err := client.GetPlacement(context.Background(), testApplicationID, testPlacementID)
			return err
		}, socialhub.CodePermissionDenied},
		{"organization placement", []OrganizationPlacement{{PlacementID: "", Name: "bad"}}, func(client *Client) error {
			_, err := client.ListOrganizationPlacements(context.Background())
			return err
		}, socialhub.CodePlatformError},
		{"test device list", []TestDevice{{ID: "bad"}}, func(client *Client) error { _, err := client.ListTestDevices(context.Background(), ""); return err }, socialhub.CodePlatformError},
		{"test device get", TestDevice{ID: "123e4567-e89b-12d3-a456-426614174000", Name: "Other", AdvertisingID: "id"}, func(client *Client) error {
			_, err := client.GetTestDevice(context.Background(), testDeviceID)
			return err
		}, socialhub.CodePermissionDenied},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) { writeJSON(t, writer, http.StatusOK, test.response) }))
			defer server.Close()
			_, client := newTestAdapter(t, server)
			if err := test.invoke(client); hubError(t, err).Code != test.code {
				t.Fatalf("error=%v", err)
			}
		})
	}
}

func placementWithApplication(applicationID string) Placement {
	value := placementFixture()
	value.ApplicationID = applicationID
	return value
}

func placementWithID(id string) Placement {
	value := placementFixture()
	value.ID = id
	return value
}
