package line

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

func TestTokenContracts(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.Header.Get("Content-Type") != "application/x-www-form-urlencoded" || request.Header.Get("Accept") != "application/json" {
			http.Error(writer, "bad request", http.StatusBadRequest)
			return
		}
		if err := request.ParseForm(); err != nil {
			http.Error(writer, "bad form", http.StatusBadRequest)
			return
		}
		switch request.URL.Path {
		case "/v2/oauth/accessToken":
			assertTokenCredentials(t, request.Form)
			writeTestJSON(t, writer, map[string]any{"access_token": "short-token", "token_type": "bearer", "expires_in": 3600})
		case "/oauth2/v3/token":
			assertTokenCredentials(t, request.Form)
			writeTestJSON(t, writer, map[string]any{"access_token": "stateless-token", "expires_in": 900})
		case "/v2/oauth/verify":
			if request.Form.Get("access_token") != "channel-token" {
				http.Error(writer, "bad token", http.StatusBadRequest)
				return
			}
			writeTestJSON(t, writer, map[string]any{"client_id": "1234567890", "expires_in": 600, "scope": "profile chat_message.write"})
		case "/v2/oauth/revoke":
			if request.Form.Get("access_token") != "channel-token" {
				http.Error(writer, "bad token", http.StatusBadRequest)
				return
			}
			writer.WriteHeader(http.StatusOK)
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	adapter, _ := newTestAdapter(t, server, true)
	client, err := adapter.Tokens(context.Background(), "main")
	if err != nil {
		t.Fatal(err)
	}

	short, err := client.IssueShortLived(context.Background())
	if err != nil || short.AccessToken != "short-token" || short.TokenType != "Bearer" || !short.ExpiresAt.Equal(testNow.Add(time.Hour)) {
		t.Fatalf("short token=%#v error=%v", short, err)
	}
	stateless, err := client.IssueStateless(context.Background())
	if err != nil || stateless.AccessToken != "stateless-token" || stateless.TokenType != "Bearer" || !stateless.ExpiresAt.Equal(testNow.Add(15*time.Minute)) {
		t.Fatalf("stateless token=%#v error=%v", stateless, err)
	}
	info, err := client.Verify(context.Background(), " channel-token ")
	if err != nil || info.ChannelID != "1234567890" || !info.ExpiresAt.Equal(testNow.Add(10*time.Minute)) || len(info.Scopes) != 2 || info.Scopes[1] != "chat_message.write" {
		t.Fatalf("token info=%#v error=%v", info, err)
	}
	if err := client.Revoke(context.Background(), " channel-token "); err != nil {
		t.Fatal(err)
	}
}

func assertTokenCredentials(t *testing.T, values url.Values) {
	t.Helper()
	if values.Get("grant_type") != "client_credentials" || values.Get("client_id") != "1234567890" || values.Get("client_secret") != "channel-secret" {
		t.Errorf("credentials=%v", values)
	}
}

func TestTokenValidationAndErrors(t *testing.T) {
	invalidClients := []*TokenClient{
		{},
		{ChannelID: "id", ChannelSecret: "secret", BaseURL: "bad", HTTPClient: http.DefaultClient, Clock: fixedClock{now: testNow}},
		{ChannelID: "id", ChannelSecret: "secret", BaseURL: "https://example.test", HTTPClient: nil, Clock: fixedClock{now: testNow}},
	}
	for index, client := range invalidClients {
		if _, err := client.IssueShortLived(context.Background()); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Fatalf("invalid client %d=%v", index, err)
		}
	}
	valid := &TokenClient{ChannelID: "id", ChannelSecret: "secret", BaseURL: "https://example.test", HTTPClient: http.DefaultClient, Clock: fixedClock{now: testNow}}
	if _, err := valid.Verify(context.Background(), " "); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("blank verify token=%v", err)
	}
	if err := valid.Revoke(context.Background(), " "); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("blank revoke token=%v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("X-Line-Request-Id", "token-request")
		writer.Header().Set("Retry-After", "3")
		switch request.URL.Path {
		case "/v2/oauth/accessToken":
			writeTestJSON(t, writer, map[string]any{"access_token": "", "expires_in": 0})
		case "/oauth2/v3/token":
			writer.WriteHeader(http.StatusTooManyRequests)
			_, _ = writer.Write([]byte(`{"error":"rate_limited","error_description":"slow down"}`))
		case "/v2/oauth/verify":
			writeTestJSON(t, writer, map[string]any{"client_id": "", "expires_in": -1})
		case "/v2/oauth/revoke":
			writer.WriteHeader(http.StatusUnauthorized)
			_, _ = writer.Write([]byte(`{"error":"invalid_client","error_description":"invalid credentials"}`))
		}
	}))
	defer server.Close()
	client := &TokenClient{ChannelID: "id", ChannelSecret: "secret", BaseURL: server.URL, HTTPClient: server.Client(), Clock: fixedClock{now: testNow}}
	if _, err := client.IssueShortLived(context.Background()); !hasErrorCode(err, socialhub.CodePlatformError) {
		t.Fatalf("malformed issue=%v", err)
	}
	_, err := client.IssueStateless(context.Background())
	var platformErr *socialhub.Error
	if !errors.As(err, &platformErr) || platformErr.Code != socialhub.CodeRateLimited || platformErr.RequestID != "token-request" || platformErr.RetryAfter != 3*time.Second {
		t.Fatalf("rate limit=%#v", err)
	}
	if _, err := client.Verify(context.Background(), "token"); !hasErrorCode(err, socialhub.CodePlatformError) {
		t.Fatalf("malformed verify=%v", err)
	}
	if err := client.Revoke(context.Background(), "token"); !errors.Is(err, socialhub.ErrUnauthenticated) {
		t.Fatalf("revoke auth=%v", err)
	}
}
