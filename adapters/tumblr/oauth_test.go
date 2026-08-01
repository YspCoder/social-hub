package tumblr

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
		if request.Method != http.MethodPost || request.Header.Get("Content-Type") != "application/x-www-form-urlencoded" || request.ParseForm() != nil {
			http.Error(writer, "bad request", http.StatusBadRequest)
			return
		}
		if request.Form.Get("client_id") != "client-id" || request.Form.Get("client_secret") != "client-secret" {
			http.Error(writer, "bad client", http.StatusUnauthorized)
			return
		}
		switch request.Form.Get("grant_type") {
		case "authorization_code":
			if request.Form.Get("code") != "auth-code" || request.Form.Get("redirect_uri") != "https://app.test/callback" {
				http.Error(writer, "bad exchange", http.StatusBadRequest)
				return
			}
			_, _ = writer.Write([]byte(`{"access_token":"code-token","refresh_token":"refresh-1","token_type":"bearer","expires_in":3600,"scope":"basic write offline_access"}`))
		case "refresh_token":
			if request.Form.Get("refresh_token") != "refresh-1" {
				http.Error(writer, "bad refresh", http.StatusBadRequest)
				return
			}
			_, _ = writer.Write([]byte(`{"access_token":"refreshed","expires_in":60,"scope":"basic write"}`))
		case "client_credentials":
			if request.Form.Get("scope") != "basic" {
				http.Error(writer, "bad scope", http.StatusBadRequest)
				return
			}
			_, _ = writer.Write([]byte(`{"access_token":"app-token","token_type":"Custom","expires_in":0,"scope":"basic"}`))
		default:
			http.Error(writer, "bad grant", http.StatusBadRequest)
		}
	}))
	defer server.Close()
	client := &OAuthClient{
		ClientID: "client-id", ClientSecret: "client-secret", AuthURL: server.URL + "/oauth2/authorize",
		TokenURL: server.URL, HTTPClient: server.Client(), Clock: fixedClock{now: testNow},
	}
	authorizationURL, err := client.AuthorizationURL("https://app.test/callback", "state-1", []string{"basic", "write", "offline_access"})
	if err != nil {
		t.Fatal(err)
	}
	parsed, _ := url.Parse(authorizationURL)
	query := parsed.Query()
	if query.Get("client_id") != "client-id" || query.Get("response_type") != "code" || query.Get("state") != "state-1" || query.Get("scope") != "basic write offline_access" || query.Get("redirect_uri") != "https://app.test/callback" {
		t.Fatalf("authorization query=%v", query)
	}
	token, err := client.Exchange(context.Background(), "auth-code", "https://app.test/callback")
	if err != nil || token.AccessToken != "code-token" || token.RefreshToken != "refresh-1" || token.TokenType != "Bearer" || !token.ExpiresAt.Equal(testNow.Add(time.Hour)) || len(token.Scopes) != 3 {
		t.Fatalf("exchange token=%#v error=%v", token, err)
	}
	token, err = client.Refresh(context.Background(), "refresh-1")
	if err != nil || token.AccessToken != "refreshed" || token.RefreshToken != "refresh-1" || token.TokenType != "Bearer" || !token.ExpiresAt.Equal(testNow.Add(time.Minute)) {
		t.Fatalf("refresh token=%#v error=%v", token, err)
	}
	token, err = client.ClientCredentials(context.Background(), []string{"basic"})
	if err != nil || token.AccessToken != "app-token" || token.TokenType != "Custom" || !token.ExpiresAt.IsZero() {
		t.Fatalf("client token=%#v error=%v", token, err)
	}
}

func TestOAuthValidationAndFailures(t *testing.T) {
	client := &OAuthClient{ClientID: "id", ClientSecret: "secret", AuthURL: "https://www.tumblr.com/oauth2/authorize", TokenURL: "https://api.tumblr.com/v2/oauth2/token", HTTPClient: http.DefaultClient, Clock: fixedClock{now: testNow}}
	invalid := []func() error{
		func() error { _, err := client.AuthorizationURL("", "state", []string{"basic"}); return err },
		func() error {
			_, err := client.AuthorizationURL("https://user:pass@app.test/callback", "state", []string{"basic"})
			return err
		},
		func() error {
			_, err := client.AuthorizationURL("https://app.test/callback", "", []string{"basic"})
			return err
		},
		func() error {
			_, err := client.AuthorizationURL("https://app.test/callback", "state", []string{"unknown"})
			return err
		},
		func() error {
			_, err := client.Exchange(context.Background(), "", "https://app.test/callback")
			return err
		},
		func() error { _, err := client.Refresh(context.Background(), " "); return err },
		func() error { _, err := client.ClientCredentials(context.Background(), nil); return err },
	}
	for index, call := range invalid {
		if err := call(); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Fatalf("validation %d=%v", index, err)
		}
	}
	badAuth := *client
	badAuth.AuthURL = "://bad"
	if _, err := badAuth.AuthorizationURL("https://app.test/callback", "state", []string{"basic"}); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("bad authorization endpoint=%v", err)
	}
	incomplete := &OAuthClient{}
	if _, err := incomplete.Refresh(context.Background(), "refresh"); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("incomplete client=%v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/denied":
			writer.WriteHeader(http.StatusUnauthorized)
			_, _ = writer.Write([]byte(`{"error":"invalid_grant","error_description":"code expired"}`))
		case "/bad-json":
			_, _ = writer.Write([]byte(`{`))
		case "/missing":
			_, _ = writer.Write([]byte(`{"access_token":""}`))
		case "/expiry":
			_, _ = writer.Write([]byte(`{"access_token":"token","expires_in":999999999}`))
		default:
			http.NotFound(writer, request)
		}
	}))
	base := &OAuthClient{ClientID: "id", ClientSecret: "secret", HTTPClient: server.Client(), Clock: fixedClock{now: testNow}}
	for _, endpoint := range []string{"/bad-json", "/missing", "/expiry"} {
		base.TokenURL = server.URL + endpoint
		_, err := base.Refresh(context.Background(), "refresh")
		var platformErr *socialhub.Error
		if !errors.As(err, &platformErr) || platformErr.Code != socialhub.CodePlatformError {
			t.Fatalf("endpoint %s=%#v", endpoint, err)
		}
	}
	base.TokenURL = server.URL + "/denied"
	_, err := base.Refresh(context.Background(), "refresh")
	var platformErr *socialhub.Error
	if !errors.Is(err, socialhub.ErrUnauthenticated) || !errors.As(err, &platformErr) || platformErr.PlatformCode != "invalid_grant" || platformErr.PlatformMessage != "code expired" {
		t.Fatalf("denied error=%#v", err)
	}
	server.Close()
	base.TokenURL = server.URL
	if _, err := base.Refresh(context.Background(), "refresh"); !errors.Is(err, socialhub.ErrUnavailable) {
		t.Fatalf("network error=%v", err)
	}
	if validOAuthScopes(nil) || validOAuthScopes([]string{"basic", "bad"}) || !validOAuthScopes([]string{"basic"}) || !strings.Contains(boundedMessage(strings.Repeat("x", 520), 512), "x") {
		t.Fatal("OAuth helper mismatch")
	}
}
