package twitch

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

func TestOAuthFlows(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/token":
			if request.Method != http.MethodPost || request.ParseForm() != nil || request.Form.Get("client_id") != "client-id" || request.Form.Get("client_secret") != "client-secret" {
				http.Error(writer, "bad token request", http.StatusBadRequest)
				return
			}
			switch request.Form.Get("grant_type") {
			case "authorization_code":
				if request.Form.Get("code") != "code-1" || request.Form.Get("redirect_uri") != "https://app.test/callback" {
					http.Error(writer, "bad exchange", http.StatusBadRequest)
					return
				}
				writeTestJSON(t, writer, map[string]any{"access_token": "user-access", "refresh_token": "refresh-1", "expires_in": 3600, "scope": []string{"user:write:chat"}, "token_type": "bearer"})
			case "refresh_token":
				if request.Form.Get("refresh_token") != "refresh-1" {
					http.Error(writer, "bad refresh", http.StatusBadRequest)
					return
				}
				writeTestJSON(t, writer, map[string]any{"access_token": "refreshed", "refresh_token": "refresh-2", "expires_in": 7200, "scope": []string{"user:write:chat"}})
			case "client_credentials":
				if request.Form.Get("scope") != "analytics:read:games" {
					http.Error(writer, "bad client scopes", http.StatusBadRequest)
					return
				}
				writeTestJSON(t, writer, map[string]any{"access_token": "app-access", "expires_in": 3600, "token_type": "bearer"})
			default:
				http.Error(writer, "unknown grant", http.StatusBadRequest)
			}
		case "/validate":
			if request.Method != http.MethodGet || request.Header.Get("Authorization") != "OAuth user-access" {
				http.Error(writer, "bad validation", http.StatusUnauthorized)
				return
			}
			writeTestJSON(t, writer, map[string]any{"client_id": "client-id", "login": "alice", "scopes": []string{"user:write:chat"}, "user_id": "user-1", "expires_in": 3500})
		case "/revoke":
			if request.Method != http.MethodPost || request.ParseForm() != nil || request.Form.Get("client_id") != "client-id" || request.Form.Get("token") != "user-access" {
				http.Error(writer, "bad revoke", http.StatusBadRequest)
				return
			}
			writer.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	client := &OAuthClient{
		ClientID: "client-id", ClientSecret: "client-secret", AuthURL: server.URL + "/authorize",
		TokenURL: server.URL + "/token", ValidateURL: server.URL + "/validate", RevokeURL: server.URL + "/revoke",
		HTTPClient: server.Client(), Clock: fixedClock{now: testNow},
	}
	authorizationURL, err := client.AuthorizationURL("https://app.test/callback", "state-1", []string{"user:write:chat", "clips:edit"}, true)
	if err != nil {
		t.Fatalf("authorization URL: %v", err)
	}
	parsed, _ := url.Parse(authorizationURL)
	query := parsed.Query()
	if query.Get("client_id") != "client-id" || query.Get("redirect_uri") != "https://app.test/callback" || query.Get("state") != "state-1" || query.Get("scope") != "user:write:chat clips:edit" || query.Get("force_verify") != "true" {
		t.Fatalf("authorization query: %v", query)
	}
	exchanged, err := client.Exchange(context.Background(), "code-1", "https://app.test/callback")
	if err != nil || exchanged.AccessToken != "user-access" || exchanged.RefreshToken != "refresh-1" || exchanged.ExpiresAt != testNow.Add(time.Hour) || len(exchanged.Scopes) != 1 {
		t.Fatalf("exchange: %#v %v", exchanged, err)
	}
	refreshed, err := client.Refresh(context.Background(), "refresh-1")
	if err != nil || refreshed.AccessToken != "refreshed" || refreshed.TokenType != "Bearer" || refreshed.ExpiresAt != testNow.Add(2*time.Hour) {
		t.Fatalf("refresh: %#v %v", refreshed, err)
	}
	app, err := client.ClientCredentials(context.Background(), []string{"analytics:read:games"})
	if err != nil || app.AccessToken != "app-access" || app.RefreshToken != "" {
		t.Fatalf("client credentials: %#v %v", app, err)
	}
	validation, err := client.Validate(context.Background(), "user-access")
	if err != nil || validation.ClientID != "client-id" || validation.UserID != "user-1" || validation.Login != "alice" {
		t.Fatalf("validate: %#v %v", validation, err)
	}
	if err := client.Revoke(context.Background(), "user-access"); err != nil {
		t.Fatalf("revoke: %v", err)
	}
}

func TestOAuthValidationErrors(t *testing.T) {
	client := &OAuthClient{}
	if _, err := client.AuthorizationURL("", "", nil, false); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("authorize invalid: %v", err)
	}
	client.ClientID, client.AuthURL = "id", "://bad"
	if _, err := client.AuthorizationURL("https://app.test", "state", nil, false); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("authorize endpoint: %v", err)
	}
	for name, call := range map[string]func() error{
		"exchange": func() error { _, err := client.Exchange(context.Background(), "", ""); return err },
		"refresh":  func() error { _, err := client.Refresh(context.Background(), " "); return err },
		"validate": func() error { _, err := client.Validate(context.Background(), ""); return err },
		"revoke":   func() error { return client.Revoke(context.Background(), "") },
	} {
		if err := call(); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Fatalf("%s: %v", name, err)
		}
	}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if strings.Contains(request.URL.Path, "bad-json") {
			_, _ = writer.Write([]byte("{"))
			return
		}
		writer.WriteHeader(http.StatusUnauthorized)
		writeTestJSON(t, writer, map[string]any{"status": 401, "error": "Unauthorized", "message": "invalid access token"})
	}))
	defer server.Close()
	bad := &OAuthClient{ClientID: "id", ClientSecret: "secret", TokenURL: server.URL + "/bad-json", HTTPClient: server.Client(), Clock: fixedClock{now: testNow}}
	if _, err := bad.ClientCredentials(context.Background(), nil); err == nil {
		t.Fatal("malformed token accepted")
	}
	bad.TokenURL = server.URL + "/error"
	if _, err := bad.ClientCredentials(context.Background(), nil); err == nil {
		t.Fatal("HTTP token error accepted")
	}
}
