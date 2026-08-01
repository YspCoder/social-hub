package mastodon

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

func TestMastodonOAuthAndApplicationRegistration(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch {
		case request.Method == http.MethodPost && request.URL.Path == "/api/v1/apps":
			if request.Header.Get("Content-Type") != "application/json" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			var input struct {
				ClientName   string   `json:"client_name"`
				RedirectURIs []string `json:"redirect_uris"`
				Scopes       string   `json:"scopes"`
				Website      string   `json:"website"`
			}
			if json.NewDecoder(request.Body).Decode(&input) != nil || input.ClientName != "social-hub test" || len(input.RedirectURIs) != 2 || input.Scopes != "read write" || input.Website != "https://app.example" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			writeJSON(writer, `{"id":"app-1","name":"social-hub test","website":"https://app.example","client_id":"registered-id","client_secret":"registered-secret","client_secret_expires_at":1785542400,"redirect_uris":["https://app.example/callback","urn:ietf:wg:oauth:2.0:oob"],"scopes":["read","write"]}`)
		case request.Method == http.MethodPost && request.URL.Path == "/oauth/token":
			if request.ParseForm() != nil || request.Form.Get("client_id") != "client-id" || request.Form.Get("client_secret") != "client-secret" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			switch request.Form.Get("grant_type") {
			case "authorization_code":
				if request.Form.Get("code") != "authorization-code" || request.Form.Get("redirect_uri") != "https://app.example/callback" || request.Form.Get("code_verifier") != "verifier" {
					writer.WriteHeader(http.StatusBadRequest)
					return
				}
				writeJSON(writer, `{"access_token":"user-token","token_type":"bearer","scope":"read:accounts read:statuses write:statuses"}`)
			case "client_credentials":
				if request.Form.Get("redirect_uri") != "urn:ietf:wg:oauth:2.0:oob" || request.Form.Get("scope") != "read" {
					writer.WriteHeader(http.StatusBadRequest)
					return
				}
				writeJSON(writer, `{"access_token":"app-token","token_type":"Bearer","scope":"read"}`)
			default:
				writer.WriteHeader(http.StatusBadRequest)
			}
		default:
			writer.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	adapter, _ := newTestAdapter(t, server, allTestScopes())

	registered, err := adapter.RegisterApplication(context.Background(), "fediverse-main", AppRegistrationRequest{
		ClientName: "social-hub test", RedirectURIs: []string{"https://app.example/callback", "urn:ietf:wg:oauth:2.0:oob"},
		Scopes: []string{"read", "write"}, Website: "https://app.example",
	})
	if err != nil || registered.ID != "app-1" || registered.ClientID != "registered-id" || registered.ClientSecret != "registered-secret" || registered.ClientSecretExpiresAt == nil || !registered.ClientSecretExpiresAt.Equal(time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("registered=%#v error=%v", registered, err)
	}

	oauth, err := adapter.OAuth(context.Background(), "fediverse-main")
	if err != nil {
		t.Fatal(err)
	}
	pair, err := NewPKCE()
	if err != nil || pair.Verifier == "" || pair.Challenge == "" || pair.Verifier == pair.Challenge {
		t.Fatalf("PKCE=%#v error=%v", pair, err)
	}
	authorizationURL, err := oauth.AuthorizationURLPKCE("https://app.example/callback", "state-value", []string{"read:accounts", "write:statuses"}, pair.Challenge)
	if err != nil {
		t.Fatal(err)
	}
	parsed, _ := url.Parse(authorizationURL)
	query := parsed.Query()
	if parsed.Path != "/oauth/authorize" || query.Get("response_type") != "code" || query.Get("client_id") != "client-id" || query.Get("state") != "state-value" || query.Get("scope") != "read:accounts write:statuses" || query.Get("code_challenge") != pair.Challenge || query.Get("code_challenge_method") != "S256" {
		t.Fatalf("authorization URL=%s", authorizationURL)
	}
	token, err := oauth.ExchangeWithVerifier(context.Background(), "authorization-code", "https://app.example/callback", "verifier")
	if err != nil || token.AccessToken != "user-token" || token.TokenType != "Bearer" || len(token.Scopes) != 3 || !token.ExpiresAt.IsZero() {
		t.Fatalf("token=%#v error=%v", token, err)
	}
	appToken, err := oauth.ClientCredentials(context.Background(), "urn:ietf:wg:oauth:2.0:oob", []string{"read"})
	if err != nil || appToken.AccessToken != "app-token" || len(appToken.Scopes) != 1 {
		t.Fatalf("app token=%#v error=%v", appToken, err)
	}
}

func TestMastodonOAuthValidation(t *testing.T) {
	client := OAuthClient{InstanceURL: "https://social.example", ClientID: "client-id", ClientSecret: "secret", HTTPClient: http.DefaultClient}
	if _, err := client.AuthorizationURL("", "state", []string{"read"}); err == nil {
		t.Fatal("empty redirect URI should fail")
	}
	if _, err := client.AuthorizationURLPKCE("https://app.example/callback", "state", []string{"read"}, ""); err == nil {
		t.Fatal("empty PKCE challenge should fail")
	}
	if _, err := client.Exchange(context.Background(), "", "https://app.example/callback"); err == nil {
		t.Fatal("empty authorization code should fail")
	}
	if _, err := client.ClientCredentials(context.Background(), "", []string{"read"}); err == nil {
		t.Fatal("empty client credentials redirect URI should fail")
	}
	invalid := []AppRegistrationRequest{
		{},
		{ClientName: "app", RedirectURIs: []string{"file:///callback"}, Scopes: []string{"read"}},
		{ClientName: "app", RedirectURIs: []string{"https://app.example/callback"}, Scopes: []string{"read write"}},
	}
	for _, input := range invalid {
		if err := validateRegistration(input); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Fatalf("input=%#v error=%v", input, err)
		}
	}
}
