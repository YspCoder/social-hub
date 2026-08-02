package deviantart

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

func TestOAuth21Flows(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("User-Agent") != "social-hub-tests/1.0" || request.Header.Get("Content-Type") != "application/x-www-form-urlencoded" {
			t.Errorf("headers=%v", request.Header)
		}
		mustForm(t, request)
		switch request.URL.Path {
		case "/oauth2/token":
			if request.Form.Get("client_id") != "12345" || request.Form.Get("client_secret") != "client-secret" {
				t.Errorf("credentials form=%v", request.Form)
			}
			switch request.Form.Get("grant_type") {
			case "authorization_code":
				if request.Form.Get("code_verifier") == "" || request.Form.Get("redirect_uri") != "https://client.example/callback" {
					t.Errorf("exchange form=%v", request.Form)
				}
				writeJSON(writer, http.StatusOK, `{"expires_in":3600,"status":"success","access_token":"user-access","refresh_token":"refresh-1","token_type":"Bearer","scope":"basic browse user"}`)
			case "refresh_token":
				if request.Form.Get("refresh_token") != "refresh-1" {
					t.Errorf("refresh form=%v", request.Form)
				}
				writeJSON(writer, http.StatusOK, `{"expires_in":3600,"status":"success","access_token":"refreshed-access","refresh_token":"refresh-2","token_type":"bearer","scope":"browse"}`)
			case "client_credentials":
				writeJSON(writer, http.StatusOK, `{"expires_in":3600,"status":"success","access_token":"app-access","token_type":"Bearer","scope":"browse"}`)
			default:
				writeJSON(writer, http.StatusBadRequest, `{"error":"unsupported_grant_type"}`)
			}
		case "/oauth2/revoke":
			if request.Form.Get("token") != "refresh-2" || request.Form.Get("revoke_refresh_only") != "true" {
				t.Errorf("revoke form=%v", request.Form)
			}
			writeJSON(writer, http.StatusOK, `{"success":true}`)
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()

	client := &OAuthClient{
		ClientID: "12345", ClientSecret: "client-secret", AuthURL: server.URL + "/oauth2/authorize",
		TokenURL: server.URL + "/oauth2/token", RevokeURL: server.URL + "/oauth2/revoke",
		UserAgent: "social-hub-tests/1.0", HTTPClient: server.Client(), Clock: fixedClock{now: testNow},
	}
	pkce, err := NewPKCE()
	if err != nil || !validPKCEValue(pkce.Verifier) || !validPKCEValue(pkce.Challenge) {
		t.Fatalf("PKCE=%#v err=%v", pkce, err)
	}
	authorize, err := client.AuthorizationURL("https://client.example/callback", "state-1", pkce, []string{"basic", "browse", "user"})
	if err != nil {
		t.Fatal(err)
	}
	parsed, _ := url.Parse(authorize)
	if parsed.Query().Get("response_type") != "code" || parsed.Query().Get("code_challenge") != pkce.Challenge ||
		parsed.Query().Get("code_challenge_method") != "S256" || parsed.Query().Get("scope") != "basic browse user" {
		t.Fatalf("authorization URL=%s", authorize)
	}
	token, err := client.Exchange(context.Background(), "code-1", "https://client.example/callback", pkce.Verifier)
	if err != nil || token.AccessToken != "user-access" || token.RefreshToken != "refresh-1" || !token.ExpiresAt.Equal(testNow.Add(time.Hour)) || len(token.Scopes) != 3 {
		t.Fatalf("exchange token=%#v err=%v", token, err)
	}
	token, err = client.Refresh(context.Background(), "refresh-1")
	if err != nil || token.AccessToken != "refreshed-access" || token.RefreshToken != "refresh-2" {
		t.Fatalf("refresh token=%#v err=%v", token, err)
	}
	token, err = client.ClientCredentials(context.Background())
	if err != nil || token.AccessToken != "app-access" || token.RefreshToken != "" {
		t.Fatalf("app token=%#v err=%v", token, err)
	}
	if err := client.Revoke(context.Background(), "refresh-2", true); err != nil {
		t.Fatal(err)
	}
}

func TestPublicOAuthClientOmitsSecret(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		mustForm(t, request)
		if request.Form.Get("client_secret") != "" || request.Form.Get("client_id") != "12345" || request.Form.Get("code_verifier") == "" {
			t.Errorf("public form=%v", request.Form)
		}
		writeJSON(writer, http.StatusOK, `{"expires_in":3600,"status":"success","access_token":"public-access","refresh_token":"public-refresh","token_type":"Bearer","scope":"basic user"}`)
	}))
	defer server.Close()
	client := &OAuthClient{
		ClientID: "12345", TokenURL: server.URL, UserAgent: "sdk/1", HTTPClient: server.Client(), Clock: fixedClock{now: testNow},
	}
	pkce, _ := NewPKCE()
	token, err := client.Exchange(context.Background(), "code", "https://client.example/callback", pkce.Verifier)
	if err != nil || token.AccessToken != "public-access" {
		t.Fatalf("token=%#v err=%v", token, err)
	}
	if _, err := client.ClientCredentials(context.Background()); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("public client credentials=%v", err)
	}
}

func TestOAuthValidationAndFailures(t *testing.T) {
	pkce, _ := NewPKCE()
	base := &OAuthClient{
		ClientID: "12345", ClientSecret: "secret", AuthURL: "https://www.deviantart.com/oauth2/authorize",
		TokenURL: "https://www.deviantart.com/oauth2/token", RevokeURL: "https://www.deviantart.com/oauth2/revoke",
		UserAgent: "sdk/1", HTTPClient: http.DefaultClient, Clock: fixedClock{now: testNow},
	}
	if _, err := base.AuthorizationURL("bad", "state", pkce, []string{"browse"}); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("callback=%v", err)
	}
	if _, err := base.AuthorizationURL("https://client.example/cb", "state", PKCE{}, []string{"browse"}); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("PKCE=%v", err)
	}
	if _, err := base.AuthorizationURL("https://client.example/cb", "state", pkce, []string{"unknown"}); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("scope=%v", err)
	}
	if _, err := base.Exchange(context.Background(), "", "https://client.example/cb", pkce.Verifier); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("exchange=%v", err)
	}
	if _, err := base.Refresh(context.Background(), ""); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("refresh=%v", err)
	}
	if err := base.Revoke(context.Background(), "", false); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("revoke=%v", err)
	}

	tests := []struct {
		name   string
		body   string
		status int
	}{
		{"OAuth error", `{"error":"invalid_grant","error_description":"bad code"}`, http.StatusBadRequest},
		{"success error", `{"error":"server_error","error_description":"retry"}`, http.StatusOK},
		{"bad JSON", `{`, http.StatusOK},
		{"missing refresh", `{"access_token":"a","expires_in":3600}`, http.StatusOK},
		{"bad token type", `{"access_token":"a","refresh_token":"r","expires_in":3600,"token_type":"MAC"}`, http.StatusOK},
		{"bad status", `{"access_token":"a","refresh_token":"r","expires_in":3600,"status":"failed"}`, http.StatusOK},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				writeJSON(writer, test.status, test.body)
			}))
			defer server.Close()
			client := *base
			client.TokenURL = server.URL
			client.HTTPClient = server.Client()
			if _, err := client.Exchange(context.Background(), "code", "https://client.example/cb", pkce.Verifier); err == nil {
				t.Fatal("expected OAuth error")
			}
		})
	}

	revokeServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writeJSON(writer, http.StatusOK, `{"success":false}`)
	}))
	defer revokeServer.Close()
	revokeClient := *base
	revokeClient.RevokeURL = revokeServer.URL
	revokeClient.HTTPClient = revokeServer.Client()
	if err := revokeClient.Revoke(context.Background(), "token", false); err == nil {
		t.Fatal("false revoke response succeeded")
	}
}

func TestOAuthResponseSizeLimit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusOK)
		_, _ = writer.Write([]byte(strings.Repeat("x", int(maxOAuthResponseBytes)+1)))
	}))
	defer server.Close()
	client := &OAuthClient{
		ClientID: "12345", ClientSecret: "secret", TokenURL: server.URL, UserAgent: "sdk/1",
		HTTPClient: server.Client(), Clock: fixedClock{now: testNow},
	}
	pkce, _ := NewPKCE()
	if _, err := client.Exchange(context.Background(), "code", "https://client.example/cb", pkce.Verifier); err == nil {
		t.Fatal("oversized OAuth response succeeded")
	}
}
