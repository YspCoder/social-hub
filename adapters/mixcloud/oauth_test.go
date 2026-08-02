package mixcloud

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"social-hub/pkg/socialhub"
)

func TestOAuthBrowserFlow(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet || request.URL.Path != "/oauth/access_token" || request.UserAgent() != "social-hub-tests/1.0" {
			t.Errorf("request=%s %s headers=%v", request.Method, request.URL, request.Header)
		}
		query := request.URL.Query()
		if query.Get("client_id") != "client-id" || query.Get("client_secret") != "client-secret" || query.Get("code") == "" {
			t.Errorf("query=%v", query)
		}
		if request.Header.Get("Authorization") != "" {
			t.Errorf("unexpected Authorization header")
		}
		switch query.Get("code") {
		case "code-1":
			if query.Get("redirect_uri") != "https://client.example/callback" {
				t.Errorf("redirect=%q", query.Get("redirect_uri"))
			}
			writer.Header().Set("Content-Type", "application/x-www-form-urlencoded")
			_, _ = writer.Write([]byte("access_token=token%2Bvalue"))
		case "code-json":
			writeJSON(writer, http.StatusOK, `{"access_token":"json-token"}`)
		case "code-oob":
			if _, exists := query["redirect_uri"]; !exists || query.Get("redirect_uri") != "" {
				t.Errorf("OOB redirect query=%v", query)
			}
			_, _ = writer.Write([]byte("access_token=oob-token"))
		default:
			http.Error(writer, "bad code", http.StatusBadRequest)
		}
	}))
	defer server.Close()
	client := &OAuthClient{
		ClientID: "client-id", ClientSecret: "client-secret", AuthURL: server.URL + "/oauth/authorize",
		TokenURL: server.URL + "/oauth/access_token", UserAgent: "social-hub-tests/1.0", HTTPClient: server.Client(),
	}
	authorize, err := client.AuthorizationURL("https://client.example/callback", "state-1")
	if err != nil {
		t.Fatal(err)
	}
	parsed, _ := url.Parse(authorize)
	if parsed.Query().Get("client_id") != "client-id" || parsed.Query().Get("redirect_uri") != "https://client.example/callback" || parsed.Query().Get("state") != "state-1" {
		t.Fatalf("authorization URL=%s", authorize)
	}
	token, err := client.Exchange(context.Background(), "code-1", "https://client.example/callback")
	if err != nil || token.AccessToken != "token+value" || token.TokenType != "OAuth" || !token.ExpiresAt.IsZero() || token.RefreshToken != "" {
		t.Fatalf("token=%#v err=%v", token, err)
	}
	token, err = client.Exchange(context.Background(), "code-json", "https://client.example/callback")
	if err != nil || token.AccessToken != "json-token" {
		t.Fatalf("JSON token=%#v err=%v", token, err)
	}
	authorize, err = client.AuthorizationURL("", "state-oob")
	parsed, _ = url.Parse(authorize)
	if err != nil || parsed.Query().Has("redirect_uri") {
		t.Fatalf("OOB authorization=%s err=%v", authorize, err)
	}
	token, err = client.Exchange(context.Background(), "code-oob", "")
	if err != nil || token.AccessToken != "oob-token" {
		t.Fatalf("OOB token=%#v err=%v", token, err)
	}
}

func TestOAuthValidationAndErrors(t *testing.T) {
	base := &OAuthClient{
		ClientID: "client-id", ClientSecret: "client-secret", AuthURL: "https://www.mixcloud.com/oauth/authorize",
		TokenURL: "https://www.mixcloud.com/oauth/access_token", UserAgent: "sdk/1", HTTPClient: http.DefaultClient,
	}
	if _, err := base.AuthorizationURL("bad/path", "state"); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("redirect=%v", err)
	}
	if _, err := base.AuthorizationURL("https://client.example/callback", ""); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("state=%v", err)
	}
	if _, err := base.Exchange(context.Background(), "", ""); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("code=%v", err)
	}
	incomplete := *base
	incomplete.ClientSecret = ""
	if _, err := incomplete.AuthorizationURL("", "state"); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("incomplete=%v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Retry-After", "3")
		writer.WriteHeader(http.StatusBadRequest)
		_, _ = writer.Write([]byte("error=invalid_grant&error_description=bad+code"))
	}))
	defer server.Close()
	failed := *base
	failed.AuthURL, failed.TokenURL, failed.HTTPClient = server.URL+"/authorize", server.URL, server.Client()
	_, err := failed.Exchange(context.Background(), "bad-code", "")
	var platformErr *socialhub.Error
	if !errors.As(err, &platformErr) || platformErr.PlatformCode != "invalid_grant" || platformErr.PlatformMessage != "bad code" || strings.Contains(err.Error(), "client-secret") {
		t.Fatalf("OAuth error=%#v", err)
	}

	badSuccess := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_, _ = writer.Write([]byte("not-a-token"))
	}))
	defer badSuccess.Close()
	failed.TokenURL, failed.HTTPClient = badSuccess.URL, badSuccess.Client()
	if _, err := failed.Exchange(context.Background(), "code", ""); err == nil {
		t.Fatal("missing access token succeeded")
	}
}

func TestOAuthResponseSizeLimit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_, _ = writer.Write([]byte(strings.Repeat("x", int(maxOAuthResponseBytes)+1)))
	}))
	defer server.Close()
	client := &OAuthClient{
		ClientID: "client-id", ClientSecret: "client-secret", AuthURL: server.URL + "/authorize",
		TokenURL: server.URL, UserAgent: "sdk/1", HTTPClient: server.Client(),
	}
	if _, err := client.Exchange(context.Background(), "code", ""); err == nil {
		t.Fatal("oversized token response succeeded")
	}
}
