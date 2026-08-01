package vimeo

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

func TestVimeoOAuthFlows(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		clientID, secret, ok := request.BasicAuth()
		if !ok || clientID != "client-id" || secret != "client-secret" || request.Method != http.MethodPost || request.ParseForm() != nil {
			writer.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch request.URL.Path {
		case "/oauth/access_token":
			if request.Form.Get("grant_type") != "authorization_code" || request.Form.Get("code") != "code" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			writeJSON(writer, `{"access_token":"user-token","token_type":"bearer","scope":"public upload","expires_in":3600}`)
		case "/oauth/authorize/client":
			if request.Form.Get("grant_type") != "client_credentials" || request.Form.Get("scope") != "public" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			writeJSON(writer, `{"access_token":"client-token","token_type":"Bearer","scope":"public"}`)
		default:
			writer.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	adapter, _ := newTestAdapter(t, server, nil)
	oauth, err := adapter.OAuth(context.Background(), "account")
	if err != nil {
		t.Fatal(err)
	}
	authorizationURL, err := oauth.AuthorizationURL("https://app.example/callback", "state", []string{"public", "upload"})
	if err != nil {
		t.Fatal(err)
	}
	parsed, _ := url.Parse(authorizationURL)
	if parsed.Query().Get("response_type") != "code" || parsed.Query().Get("scope") != "public upload" || parsed.Query().Get("state") != "state" {
		t.Fatalf("authorization URL=%s", authorizationURL)
	}
	token, err := oauth.Exchange(context.Background(), "code", "https://app.example/callback")
	if err != nil || token.AccessToken != "user-token" || token.TokenType != "Bearer" || !token.ExpiresAt.Equal(testNow.Add(time.Hour)) || len(token.Scopes) != 2 {
		t.Fatalf("token=%#v err=%v", token, err)
	}
	clientToken, err := oauth.ClientCredentials(context.Background(), []string{"public"})
	if err != nil || clientToken.AccessToken != "client-token" || !clientToken.ExpiresAt.IsZero() {
		t.Fatalf("client token=%#v err=%v", clientToken, err)
	}
}

func TestOAuthValidationAndErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusBadRequest)
		writeJSON(writer, `{"error":"invalid_scope","error_description":"scope denied"}`)
	}))
	defer server.Close()
	adapter, _ := newTestAdapter(t, server, nil)
	oauth, err := adapter.OAuth(context.Background(), "account")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := oauth.AuthorizationURL("bad", "", []string{"unknown"}); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("authorize validation=%v", err)
	}
	if _, err := oauth.Exchange(context.Background(), "", "bad"); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("exchange validation=%v", err)
	}
	if _, err := oauth.ClientCredentials(context.Background(), []string{"public", "public"}); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("scope validation=%v", err)
	}
	if _, err := oauth.ClientCredentials(context.Background(), []string{"public"}); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("OAuth platform error=%v", err)
	}
	if _, err := adapter.OAuth(context.Background(), "missing"); !errors.Is(err, socialhub.ErrNotFound) {
		t.Fatalf("missing account=%v", err)
	}
	adapter.config.Accounts[0].ClientID = ""
	if _, err := adapter.OAuth(context.Background(), "account"); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("missing client config=%v", err)
	}
}

func TestVimeoHTTPErrorMapping(t *testing.T) {
	tests := []struct {
		name       string
		status     int
		body       string
		code       socialhub.ErrorCode
		class      socialhub.ErrorClass
		platformID string
	}{
		{"upgrade", http.StatusForbidden, `{"error":"upgrade","error_code":4101}`, socialhub.CodeApprovalRequired, socialhub.ClassUserAction, "4101"},
		{"weekly quota", http.StatusTooManyRequests, `{"error":"quota","error_code":"4102"}`, socialhub.CodeRateLimited, socialhub.ClassRetryable, "4102"},
		{"daily quota", http.StatusTooManyRequests, `{"error_code":4104}`, socialhub.CodeRateLimited, socialhub.ClassRetryable, "4104"},
		{"bad token", http.StatusUnauthorized, `{"error_code":8002}`, socialhub.CodeUnauthenticated, socialhub.ClassUserAction, "8002"},
		{"bad content", http.StatusBadRequest, `{"error_code":2205}`, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, "2205"},
		{"not found", http.StatusNotFound, `{}`, socialhub.CodeNotFound, socialhub.ClassPermanent, ""},
		{"server", http.StatusServiceUnavailable, `{}`, socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			header := http.Header{"Retry-After": {"12"}, "X-Request-Id": {"request-1"}}
			err := decodeHTTPError(test.status, header, []byte(test.body))
			var hubError *socialhub.Error
			if !errors.As(err, &hubError) || hubError.Code != test.code || hubError.Class != test.class || hubError.PlatformCode != test.platformID || hubError.RequestID != "request-1" || hubError.RetryAfter != 12*time.Second {
				t.Fatalf("error=%#v", err)
			}
		})
	}
}
