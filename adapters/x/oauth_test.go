package x

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestOAuthPKCEAuthorizationAndExchange(t *testing.T) {
	t.Parallel()
	pkce, err := NewPKCE()
	if err != nil {
		t.Fatal(err)
	}
	if pkce.Verifier == "" || pkce.Challenge == "" || strings.Contains(pkce.Verifier, "=") {
		t.Fatalf("PKCE = %#v", pkce)
	}

	var gotForm url.Values
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if err := request.ParseForm(); err != nil {
			writer.WriteHeader(http.StatusBadRequest)
			return
		}
		gotForm = request.Form
		username, password, ok := request.BasicAuth()
		if !ok || username != "client-id" || password != "client-secret" {
			writer.WriteHeader(http.StatusUnauthorized)
			return
		}
		_, _ = writer.Write([]byte(`{"access_token":"access","refresh_token":"refresh","token_type":"bearer","expires_in":7200,"scope":"tweet.read tweet.write"}`))
	}))
	defer server.Close()

	client := OAuthClient{ClientID: "client-id", ClientSecret: "client-secret", AuthURL: server.URL + "/authorize", TokenURL: server.URL + "/token", HTTPClient: server.Client()}
	authorizationURL, err := client.AuthorizationURL("https://app.example/callback", "state-value", []string{"tweet.read", "tweet.write"}, pkce)
	if err != nil {
		t.Fatal(err)
	}
	parsed, _ := url.Parse(authorizationURL)
	if parsed.Query().Get("state") != "state-value" || parsed.Query().Get("code_challenge") != pkce.Challenge || parsed.Query().Get("code_challenge_method") != "S256" {
		t.Fatalf("authorization query = %v", parsed.Query())
	}
	token, err := client.Exchange(context.Background(), "auth-code", "https://app.example/callback", pkce.Verifier)
	if err != nil {
		t.Fatal(err)
	}
	if token.AccessToken != "access" || token.RefreshToken != "refresh" || len(token.Scopes) != 2 {
		t.Fatalf("token = %#v", token)
	}
	if gotForm.Get("code_verifier") != pkce.Verifier || gotForm.Get("grant_type") != "authorization_code" {
		t.Fatalf("token form = %v", gotForm)
	}
}

func TestOAuthFailureIsSanitized(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusBadRequest)
		_, _ = writer.Write([]byte(`{"error":"invalid_grant","error_description":"sensitive provider detail"}`))
	}))
	defer server.Close()
	client := OAuthClient{ClientID: "client-id", TokenURL: server.URL, HTTPClient: server.Client()}
	_, err := client.Refresh(context.Background(), "refresh-token")
	if err == nil || strings.Contains(err.Error(), "sensitive provider detail") {
		t.Fatalf("error = %v", err)
	}
}
