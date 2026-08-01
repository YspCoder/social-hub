package dailymotion

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

func TestOAuthClientCredentialsAndTokenSourceCache(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.Header.Get("Content-Type") != "application/x-www-form-urlencoded" {
			http.Error(writer, "bad request", http.StatusBadRequest)
			return
		}
		if err := request.ParseForm(); err != nil || request.Form.Get("grant_type") != "client_credentials" || request.Form.Get("client_id") != "client-id" || request.Form.Get("client_secret") != "client-secret" || request.Form.Get("scope") != "video.manage playlist.read" {
			http.Error(writer, "bad form", http.StatusBadRequest)
			return
		}
		calls.Add(1)
		writeJSON(writer, http.StatusOK, `{"access_token":"jwt-token","token_type":"Bearer","expires_in":1800,"scope":"video.manage playlist.read"}`)
	}))
	defer server.Close()
	oauth := OAuthClient{ClientID: "client-id", ClientSecret: "client-secret", TokenURL: server.URL, HTTPClient: server.Client(), Clock: fixedClock{testNow}}
	token, err := oauth.ClientCredentials(context.Background(), []string{ScopeVideoManage, ScopePlaylistRead})
	if err != nil || token.AccessToken != "jwt-token" || token.TokenType != "Bearer" || !token.ExpiresAt.Equal(testNow.Add(30*time.Minute)) || len(token.Scopes) != 2 {
		t.Fatalf("token=%#v err=%v", token, err)
	}
	store := socialhub.NewMemoryTokenStore()
	source := &clientTokenSource{oauth: oauth, scopes: []string{ScopeVideoManage, ScopePlaylistRead}, store: store, key: socialhub.TokenKey{Platform: "dailymotion"}}
	first, err := source.Token(context.Background())
	if err != nil || first.AccessToken != "jwt-token" {
		t.Fatalf("first=%#v err=%v", first, err)
	}
	second, err := source.Token(context.Background())
	if err != nil || second.AccessToken != first.AccessToken || calls.Load() != 2 {
		t.Fatalf("second=%#v calls=%d err=%v", second, calls.Load(), err)
	}
	other := &clientTokenSource{oauth: oauth, scopes: source.scopes, store: store, key: source.key}
	if _, err := other.Token(context.Background()); err != nil || calls.Load() != 2 {
		t.Fatalf("stored token calls=%d err=%v", calls.Load(), err)
	}
}

func TestOAuthAndHTTPErrorMapping(t *testing.T) {
	tests := []struct {
		name, body string
		status     int
		want       socialhub.ErrorCode
	}{
		{"oauth validation", `{"detail":[{"msg":"invalid scope","type":"value_error"}]}`, 422, socialhub.CodeInvalidArgument},
		{"oauth server", `{}`, 500, socialhub.CodeTemporarilyUnavailable},
		{"oauth malformed success", `{`, 200, socialhub.CodePlatformError},
		{"oauth bad lifetime", `{"access_token":"token","expires_in":0}`, 200, socialhub.CodePlatformError},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) { writeJSON(writer, test.status, test.body) }))
			defer server.Close()
			oauth := OAuthClient{ClientID: "id", ClientSecret: "secret", TokenURL: server.URL, HTTPClient: server.Client(), Clock: fixedClock{testNow}}
			_, err := oauth.ClientCredentials(context.Background(), []string{ScopeVideoRead})
			if errorCode(err) != test.want {
				t.Fatalf("error=%v", err)
			}
		})
	}
	if _, err := (&OAuthClient{}).ClientCredentials(context.Background(), nil); errorCode(err) != socialhub.CodeInvalidArgument {
		t.Fatalf("invalid client=%v", err)
	}

	header := http.Header{"Retry-After": {"3"}}
	err := decodeHTTPError(http.StatusForbidden, header, []byte(`{"error":{"code":"MISSING_PERMISSIONS","message":"missing","correlation_id":"request-1","details":{"missing_permissions":["video.manage"]},"documentation_url":"https://example.com"}}`))
	var platformErr *socialhub.Error
	if !errors.As(err, &platformErr) || platformErr.Code != socialhub.CodeApprovalRequired || platformErr.RetryAfter != 3*time.Second || platformErr.RequestID != "request-1" || len(platformErr.RequiredScopes) != 1 {
		t.Fatalf("error=%#v", err)
	}
	for status, want := range map[int]socialhub.ErrorCode{400: socialhub.CodeInvalidArgument, 401: socialhub.CodeUnauthenticated, 404: socialhub.CodeNotFound, 409: socialhub.CodeConflict, 429: socialhub.CodeRateLimited, 503: socialhub.CodeTemporarilyUnavailable} {
		if code := errorCode(decodeHTTPError(status, nil, nil)); code != want {
			t.Fatalf("status=%d code=%s", status, code)
		}
	}
	if parseRetryAfter("bad") != 0 || parseRetryAfter("999999") != 0 || !strings.Contains(boundedMessage(strings.Repeat("x", 20), 5), "xxxxx") {
		t.Fatal("error helpers")
	}
}
