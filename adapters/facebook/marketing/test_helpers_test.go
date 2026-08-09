package marketing

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

const (
	testAdAccountID = "123"
	testCampaignID  = "111"
	testAdSetID     = "222"
	testCreativeID  = "333"
	testAdID        = "444"
)

var testNow = time.Date(2026, time.August, 9, 1, 2, 3, 0, time.UTC)

type mapResolver map[string]string

func (resolver mapResolver) Resolve(_ context.Context, reference string) (string, error) {
	value, found := resolver[reference]
	if !found {
		return "", errors.New("missing test secret")
	}
	return value, nil
}

type fixedClock struct{ now time.Time }

func (clock fixedClock) Now() time.Time { return clock.now }

func newTestAdapter(t *testing.T, server *httptest.Server, scopes []string) (*Adapter, *Client) {
	t.Helper()
	config := socialhub.AdapterConfig{
		Adapter: adapterName, Product: productName,
		Settings: map[string]any{
			"base_url": server.URL + "/v25.0", "auth_url": server.URL + "/v25.0/dialog/oauth",
			"token_url": server.URL + "/v25.0/oauth/access_token",
		},
		Accounts: []socialhub.AccountConfig{{
			ID: "ads-primary", ClientID: "app-id", SecretRef: "test://app-secret",
			AccessTokenRef: "test://access-token", Approval: socialhub.ApprovalConfig{Scopes: scopes},
			Settings: map[string]any{"ad_account_id": testAdAccountID},
		}},
	}
	adapter := &Adapter{}
	if err := adapter.Init(context.Background(), config,
		socialhub.WithHTTPClient(server.Client()),
		socialhub.WithSecretResolver(mapResolver{
			"test://app-secret": "app-secret", "test://access-token": "access-token",
		}),
		socialhub.WithClock(fixedClock{now: testNow}),
	); err != nil {
		t.Fatal(err)
	}
	common, err := adapter.Client(context.Background(), "ads-primary")
	if err != nil {
		t.Fatal(err)
	}
	return adapter, common.(*Client)
}

func writeJSON(writer http.ResponseWriter, status int, value string) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	_, _ = writer.Write([]byte(value))
}

func hubErrorCode(err error) socialhub.ErrorCode {
	var hubError *socialhub.Error
	if errors.As(err, &hubError) {
		return hubError.Code
	}
	return ""
}
