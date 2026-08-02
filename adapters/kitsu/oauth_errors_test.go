package kitsu

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

func TestRefreshToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/oauth/token" || request.Method != http.MethodPost {
			http.NotFound(writer, request)
			return
		}
		if err := request.ParseForm(); err != nil {
			t.Fatal(err)
		}
		if request.Form.Get("grant_type") != "refresh_token" || request.Form.Get("refresh_token") != "old-refresh" ||
			request.Form.Get("client_id") != "client-id" || request.Form.Get("client_secret") != testClientSecret {
			t.Errorf("unexpected refresh form: %v", request.Form)
		}
		_, _ = writer.Write([]byte(`{"access_token":"new-access","refresh_token":"new-refresh","token_type":"bearer","expires_in":2592000,"created_at":1785657600}`))
	}))
	defer server.Close()
	_, client := newTestClient(t, server, true, true)
	token, err := client.Refresh(context.Background(), "old-refresh")
	if err != nil || token.AccessToken != "new-access" || token.RefreshToken != "new-refresh" ||
		token.ExpiresAt != time.Unix(1785657600, 0).Add(30*24*time.Hour) {
		t.Fatalf("unexpected token: %#v, %v", token, err)
	}
}

func TestHTTPErrorMappingAndSanitizing(t *testing.T) {
	header := make(http.Header)
	header.Set("Retry-After", "7")
	header.Set("X-Request-ID", "request-1")
	err := decodeHTTPError(http.StatusTooManyRequests, header, []byte(`{"errors":[{"status":"429","code":"throttled","detail":"slow down"}]}`))
	var platformErr *socialhub.Error
	if !errors.As(err, &platformErr) || platformErr.Code != socialhub.CodeRateLimited || !platformErr.Retryable() ||
		platformErr.RetryAfter != 7*time.Second || platformErr.RequestID != "request-1" || platformErr.PlatformMessage != "slow down" {
		t.Fatalf("unexpected error: %#v", platformErr)
	}
	for _, test := range []struct {
		status int
		body   string
		code   socialhub.ErrorCode
	}{
		{http.StatusUnauthorized, `{"error":"invalid_grant"}`, socialhub.CodeUnauthenticated},
		{http.StatusForbidden, `{"errors":[{"code":"forbidden"}]}`, socialhub.CodePermissionDenied},
		{http.StatusNotFound, `{}`, socialhub.CodeNotFound},
		{http.StatusInternalServerError, `{}`, socialhub.CodeTemporarilyUnavailable},
	} {
		err := decodeHTTPError(test.status, nil, []byte(test.body))
		if !errors.As(err, &platformErr) || platformErr.Code != test.code {
			t.Fatalf("status %d: %#v", test.status, platformErr)
		}
	}
	inner := errors.New("dial failed")
	wrapped := &url.Error{Op: "Get", URL: "https://secret.invalid/?token=secret", Err: inner}
	if sanitized := sanitizeTransportError(wrapped); sanitized != inner {
		t.Fatalf("URL error was not sanitized: %v", sanitized)
	}
	if parseRetryAfter("invalid") != 0 || len(bounded("abcdef", 3)) != 3 {
		t.Fatal("bounded metadata helpers failed")
	}
}

func TestRefreshValidationAndOAuthError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Retry-After", "3")
		writer.WriteHeader(http.StatusTooManyRequests)
		_, _ = writer.Write([]byte(`{"error":"temporarily_unavailable","error_description":"busy"}`))
	}))
	defer server.Close()
	_, client := newTestClient(t, server, true, true)
	if _, err := client.Refresh(context.Background(), " bad "); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("expected invalid token error, got %v", err)
	}
	_, err := client.Refresh(context.Background(), "refresh")
	var platformErr *socialhub.Error
	if !errors.As(err, &platformErr) || platformErr.Code != socialhub.CodeRateLimited || platformErr.Op != "oauth_refresh" {
		t.Fatalf("unexpected OAuth error: %#v", platformErr)
	}
}
