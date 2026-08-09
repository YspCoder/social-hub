package amazonads

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

const (
	testProfileID   = "1234567890"
	testCampaignID  = "2001"
	testAdGroupID   = "3001"
	testProductAdID = "4001"
	testKeywordID   = "5001"
)

var testNow = time.Date(2026, time.August, 9, 12, 0, 0, 0, time.UTC)

type mapResolver map[string]string

func (resolver mapResolver) Resolve(_ context.Context, reference string) (string, error) {
	value, found := resolver[reference]
	if !found {
		return "", errors.New("missing test secret")
	}
	return value, nil
}

type fixedClock struct{ value time.Time }

func (clock fixedClock) Now() time.Time { return clock.value }

func testConfig(baseURL string) socialhub.AdapterConfig {
	return socialhub.AdapterConfig{
		Adapter: adapterName, Product: productName,
		Settings: map[string]any{"base_url": baseURL, "auth_url": baseURL + "/authorize", "token_url": baseURL + "/token"},
		Accounts: []socialhub.AccountConfig{{
			ID: "retail-us", ClientID: "amzn1.application-oa2-client.test",
			SecretRef: "test://client-secret", AccessTokenRef: "test://access-token",
			Approval: socialhub.ApprovalConfig{Scopes: []string{managementScope}},
			Settings: map[string]any{"profile_id": testProfileID, "region": "NA"},
		}},
	}
}

func newTestAdapter(t *testing.T, server *httptest.Server) (*Adapter, *Client) {
	t.Helper()
	adapter := &Adapter{}
	if err := adapter.Init(context.Background(), testConfig(server.URL),
		socialhub.WithHTTPClient(server.Client()),
		socialhub.WithSecretResolver(mapResolver{
			"test://client-secret": "client-secret", "test://access-token": "access-token",
		}),
		socialhub.WithClock(fixedClock{value: testNow}),
	); err != nil {
		t.Fatal(err)
	}
	common, err := adapter.Client(context.Background(), "retail-us")
	if err != nil {
		t.Fatal(err)
	}
	return adapter, common.(*Client)
}

func assertAPIRequest(t *testing.T, request *http.Request, mediaType string, mutation bool) {
	t.Helper()
	if request.Header.Get("Authorization") != "Bearer access-token" ||
		request.Header.Get("Amazon-Advertising-API-ClientId") != "amzn1.application-oa2-client.test" {
		t.Fatalf("authentication headers=%v", request.Header)
	}
	if request.URL.Path == "/v2/profiles" {
		if request.Header.Get("Amazon-Advertising-API-Scope") != "" {
			t.Fatalf("Profiles request leaked scope header: %v", request.Header)
		}
		return
	}
	if request.Header.Get("Amazon-Advertising-API-Scope") != testProfileID {
		t.Fatalf("scope headers=%v", request.Header)
	}
	if mediaType != "" && (request.Header.Get("Accept") != mediaType || request.Header.Get("Content-Type") != mediaType) {
		t.Fatalf("media type headers=%v", request.Header)
	}
	wantPrefer := ""
	if mutation {
		wantPrefer = "return=representation"
	}
	if request.Header.Get("Prefer") != wantPrefer {
		t.Fatalf("Prefer=%q want=%q", request.Header.Get("Prefer"), wantPrefer)
	}
}

func decodeBody(t *testing.T, request *http.Request) map[string]any {
	t.Helper()
	decoder := json.NewDecoder(request.Body)
	decoder.UseNumber()
	var body map[string]any
	if err := decoder.Decode(&body); err != nil {
		t.Fatal(err)
	}
	return body
}

func firstResource(t *testing.T, body map[string]any, key string) map[string]any {
	t.Helper()
	items, ok := body[key].([]any)
	if !ok || len(items) != 1 {
		t.Fatalf("%s=%#v", key, body[key])
	}
	resource, ok := items[0].(map[string]any)
	if !ok {
		t.Fatalf("resource=%#v", items[0])
	}
	return resource
}

func writeJSON(writer http.ResponseWriter, status int, value string) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	_, _ = writer.Write([]byte(value))
}

func hubError(t *testing.T, err error) *socialhub.Error {
	t.Helper()
	var result *socialhub.Error
	if !errors.As(err, &result) {
		t.Fatalf("expected *socialhub.Error, got %T: %v", err, err)
	}
	return result
}
