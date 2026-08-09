package youtubereporting

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

const (
	testAccountID    socialhub.AccountID = "youtube-reporting-main"
	testOwnerID                          = "owner_test_123"
	testAccessToken                      = "youtube-reporting-access-token"
	testJobID                            = "job-123"
	testReportID                         = "report-456"
	testReportTypeID                     = "channel_basic_a3"
)

var testNow = time.Date(2026, time.August, 10, 12, 0, 0, 0, time.UTC)

type mapResolver map[string]string

func (resolver mapResolver) Resolve(_ context.Context, reference string) (string, error) {
	value, found := resolver[reference]
	if !found {
		return "", errors.New("secret not found")
	}
	return value, nil
}

type mutableClock struct {
	mu    sync.RWMutex
	value time.Time
}

func (clock *mutableClock) Now() time.Time {
	clock.mu.RLock()
	defer clock.mu.RUnlock()
	return clock.value
}

func (clock *mutableClock) Set(value time.Time) {
	clock.mu.Lock()
	clock.value = value
	clock.mu.Unlock()
}

func staticConfig(baseURL string) socialhub.AdapterConfig {
	return socialhub.AdapterConfig{
		Adapter: adapterName, Product: productName,
		Settings: map[string]any{"base_url": baseURL, "auth_url": baseURL + "/authorize", "token_url": baseURL + "/token"},
		Accounts: []socialhub.AccountConfig{{
			ID: testAccountID, ClientID: "client-id", SecretRef: "test://client-secret", AccessTokenRef: "test://access-token",
			Approval: socialhub.ApprovalConfig{Scopes: append([]string(nil), supportedScopes...)},
			Settings: map[string]any{},
		}},
	}
}

func managedConfig(baseURL string) socialhub.AdapterConfig {
	config := staticConfig(baseURL)
	config.Accounts[0].AccessTokenRef = ""
	config.Accounts[0].Approval.Scopes = nil
	config.Accounts[0].Settings["refresh_token_ref"] = "test://refresh-token"
	return config
}

func ownerConfig(baseURL string) socialhub.AdapterConfig {
	config := staticConfig(baseURL)
	config.Accounts[0].Settings["content_owner_id"] = testOwnerID
	return config
}

func serviceAccountConfig(baseURL string) socialhub.AdapterConfig {
	config := ownerConfig(baseURL)
	config.Accounts[0].AccessTokenRef = ""
	config.Accounts[0].ClientID = ""
	config.Accounts[0].SecretRef = ""
	config.Accounts[0].Approval.Scopes = nil
	config.Accounts[0].Settings["service_account_email"] = "reports@test-project.iam.gserviceaccount.com"
	config.Accounts[0].Settings["private_key_ref"] = "test://private-key"
	return config
}

func newStaticClient(t *testing.T, server *httptest.Server, config socialhub.AdapterConfig) (*Adapter, *Client) {
	t.Helper()
	adapter := &Adapter{}
	if err := adapter.Init(context.Background(), config,
		socialhub.WithHTTPClient(server.Client()),
		socialhub.WithSecretResolver(mapResolver{"test://access-token": testAccessToken, "test://client-secret": "client-secret"}),
		socialhub.WithClock(&mutableClock{value: testNow}),
	); err != nil {
		t.Fatal(err)
	}
	common, err := adapter.Client(context.Background(), testAccountID)
	if err != nil {
		t.Fatal(err)
	}
	return adapter, common.(*Client)
}

func assertAPIRequest(t *testing.T, request *http.Request, method, requestPath string) {
	t.Helper()
	if request.Method != method || request.URL.Path != requestPath {
		t.Fatalf("request=%s %s", request.Method, request.URL.Path)
	}
	if request.Header.Get("Authorization") != "Bearer "+testAccessToken {
		t.Errorf("Authorization=%q", request.Header.Get("Authorization"))
	}
	if request.URL.Query().Get("access_token") != "" {
		t.Errorf("access token leaked into query")
	}
}

func writeJSON(t *testing.T, writer http.ResponseWriter, status int, value any) {
	t.Helper()
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	if err := json.NewEncoder(writer).Encode(value); err != nil {
		t.Fatal(err)
	}
}

func reportTypeFixture() map[string]any {
	return map[string]any{"id": testReportTypeID, "name": "Channel basic", "deprecateTime": "2027-01-01T00:00:00Z"}
}

func jobFixture() map[string]any {
	return map[string]any{
		"id": testJobID, "reportTypeId": testReportTypeID, "name": "Daily channel report",
		"createTime": "2026-08-10T12:00:00.123456Z",
	}
}

func reportFixture(downloadURL string) map[string]any {
	return map[string]any{
		"id": testReportID, "jobId": testJobID,
		"startTime": "2026-08-08T08:00:00Z", "endTime": "2026-08-09T08:00:00Z",
		"createTime": "2026-08-10T12:00:00.123456Z", "jobExpireTime": "2027-01-01T00:00:00Z",
		"downloadUrl": downloadURL,
	}
}

func requireHubError(t *testing.T, err error) *socialhub.Error {
	t.Helper()
	if err == nil {
		t.Fatal("expected error")
	}
	var hub *socialhub.Error
	if !errors.As(err, &hub) {
		t.Fatalf("error type=%T value=%v", err, err)
	}
	return hub
}

func cloneMap(input map[string]any) map[string]any {
	output := make(map[string]any, len(input))
	for key, value := range input {
		output[key] = value
	}
	return output
}
