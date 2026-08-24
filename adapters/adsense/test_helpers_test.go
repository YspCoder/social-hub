package adsense

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
	testAccountID    socialhub.AccountID = "adsense-main"
	testPublisherID                      = "pub-123456"
	testAdClientID                       = "ca-pub-123456"
	testAdUnitID                         = "unit-1"
	testChannelID                        = "channel:1"
	testURLChannelID                     = "url-channel_1"
	testSiteID                           = "example.com"
	testIssueID                          = "issue~1"
	testReportID                         = "report-1"
	testAccessToken                      = "adsense-access-token"
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
			ID: testAccountID, ClientID: "client-id", SecretRef: "test://client-secret",
			AccessTokenRef: "test://access-token", Approval: socialhub.ApprovalConfig{Scopes: []string{fullScope}},
			Settings: map[string]any{"publisher_id": testPublisherID},
		}},
	}
}

func managedConfig(baseURL string) socialhub.AdapterConfig {
	config := staticConfig(baseURL)
	config.Accounts[0].AccessTokenRef = ""
	config.Accounts[0].Settings["refresh_token_ref"] = "test://refresh-token"
	return config
}

func newStaticClient(t *testing.T, server *httptest.Server) (*Adapter, *Client) {
	t.Helper()
	adapter := &Adapter{}
	if err := adapter.Init(context.Background(), staticConfig(server.URL),
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

func assertAPIRequest(t *testing.T, request *http.Request) {
	t.Helper()
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

func accountName() string                       { return "accounts/" + testPublisherID }
func adClientName() string                      { return accountName() + "/adclients/" + testAdClientID }
func nestedName(collection, id string) string   { return adClientName() + "/" + collection + "/" + id }
func resourceName(collection, id string) string { return accountName() + "/" + collection + "/" + id }

func validDateFixture() Date { return Date{Year: 2026, Month: 8, Day: 9} }

func policyIssueResource() PolicyIssue {
	return PolicyIssue{
		Name: resourceName("policyIssues", testIssueID), EntityType: PolicyEntitySite, Site: "example.com",
		AdClients:      []string{adClientName(), "accounts/pub-999/adclients/ca-pub-999"},
		PolicyTopics:   []PolicyTopic{{Topic: "MALWARE", Type: PolicyTopicPolicy, MustFix: true}},
		AdRequestCount: "12", Action: EnforcementAdServingRestricted,
		FirstDetectedDate: validDateFixture(), LastDetectedDate: validDateFixture(),
	}
}

func reportResult(headers []map[string]any) map[string]any {
	cells := make([]map[string]string, len(headers))
	for index := range cells {
		cells[index] = map[string]string{"value": "1"}
	}
	return map[string]any{
		"headers": headers, "rows": []any{map[string]any{"cells": cells}},
		"totals": map[string]any{"cells": cells}, "averages": map[string]any{"cells": cells},
		"startDate": validDateFixture(), "endDate": validDateFixture(), "totalMatchedRows": "1",
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
