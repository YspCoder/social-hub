package unitystatistics

import (
	"context"
	"errors"
	"net/http/httptest"
	"testing"

	"social-hub/pkg/socialhub"
)

const (
	testAccountID      socialhub.AccountID = "unity-stats-primary"
	testOrganizationID                     = "5772916123937"
	testBearerToken                        = "unity-stats-bearer"
	testKeyID                              = "7e0f1152-e0dd-4b14-8e37-04cab07efeb0"
	testSecretKey                          = "unity-stats-secret"
)

type mapResolver map[string]string

func (resolver mapResolver) Resolve(_ context.Context, reference string) (string, error) {
	value, found := resolver[reference]
	if !found {
		return "", errors.New("secret not found")
	}
	return value, nil
}

func testOptions() []socialhub.Option {
	return []socialhub.Option{socialhub.WithSecretResolver(mapResolver{
		"secret://unity-stats-bearer": testBearerToken,
		"secret://unity-stats-key":    testSecretKey,
	})}
}

func testConfig(baseURL string) socialhub.AdapterConfig {
	return socialhub.AdapterConfig{
		Adapter: adapterName, Product: productName,
		Settings: map[string]any{"base_url": baseURL},
		Accounts: []socialhub.AccountConfig{{
			ID: testAccountID, AccessTokenRef: "secret://unity-stats-bearer",
			Settings: map[string]any{"organization_id": testOrganizationID},
		}},
	}
}

func basicConfig(baseURL string) socialhub.AdapterConfig {
	config := testConfig(baseURL)
	config.Accounts[0].AccessTokenRef = ""
	config.Accounts[0].ClientID = testKeyID
	config.Accounts[0].SecretRef = "secret://unity-stats-key"
	return config
}

func newTestAdapter(t *testing.T, server *httptest.Server) (*Adapter, *Client) {
	t.Helper()
	adapter := &Adapter{}
	if err := adapter.Init(context.Background(), testConfig(server.URL), testOptions()...); err != nil {
		t.Fatal(err)
	}
	common, err := adapter.Client(context.Background(), testAccountID)
	if err != nil {
		t.Fatal(err)
	}
	return adapter, common.(*Client)
}

func requireHubError(t *testing.T, err error) *socialhub.Error {
	t.Helper()
	var hub *socialhub.Error
	if !errors.As(err, &hub) {
		t.Fatalf("error=%v is not *socialhub.Error", err)
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
