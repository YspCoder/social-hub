package simkl

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

func TestOAuthConfidentialAndPKCE(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/oauth/token" || request.Method != http.MethodPost || request.Header.Get("User-Agent") != "social-hub-simkl-tests/1.0" {
			http.NotFound(writer, request)
			return
		}
		if err := request.ParseForm(); err != nil {
			t.Fatal(err)
		}
		switch calls.Add(1) {
		case 1:
			if request.Form.Get("client_secret") != testClientSecret || request.Form.Get("redirect_uri") != "https://app.example/callback" ||
				request.Form.Get("code_verifier") != "" {
				t.Errorf("unexpected confidential form: %v", request.Form)
			}
		case 2:
			if request.Form.Get("client_secret") != "" || request.Form.Get("code_verifier") == "" || request.Form.Get("redirect_uri") != "" {
				t.Errorf("unexpected PKCE form: %v", request.Form)
			}
		}
		_, _ = writer.Write([]byte(`{"access_token":"new-access","token_type":"bearer","scope":"public","expires_in":157680000}`))
	}))
	defer server.Close()
	_, client := newTestClient(t, server, true, false)
	pkce, err := NewPKCE()
	if err != nil || len(pkce.Verifier) < 43 || len(pkce.Challenge) != 43 {
		t.Fatalf("unexpected PKCE: %#v, %v", pkce, err)
	}
	confidentialURL, err := client.AuthorizationURL(AuthorizationRequest{RedirectURI: "https://app.example/callback", State: "state-1"})
	if err != nil {
		t.Fatal(err)
	}
	parsed, _ := url.Parse(confidentialURL)
	if parsed.Query().Get("client_id") != "client-id" || parsed.Query().Get("state") != "state-1" || parsed.Query().Get("code_challenge") != "" {
		t.Fatalf("unexpected confidential URL: %s", confidentialURL)
	}
	pkceURL, err := client.AuthorizationURLPKCE(PKCEAuthorizationRequest{State: "state-2", PKCE: pkce})
	if err != nil {
		t.Fatal(err)
	}
	parsed, _ = url.Parse(pkceURL)
	if parsed.Query().Get("code_challenge") != pkce.Challenge || parsed.Query().Get("code_challenge_method") != "S256" || parsed.Query().Get("redirect_uri") != "" {
		t.Fatalf("unexpected PKCE URL: %s", pkceURL)
	}
	if _, err := client.AuthorizationURLPKCE(PKCEAuthorizationRequest{RedirectURI: "socialhub:/oauth/callback", State: "state-3", PKCE: pkce}); err != nil {
		t.Fatalf("custom-scheme PKCE redirect was rejected: %v", err)
	}
	token, err := client.Exchange(context.Background(), "code-1", "https://app.example/callback")
	if err != nil || token.AccessToken != "new-access" || token.RefreshToken != "" || token.TokenType != "Bearer" ||
		!token.ExpiresAt.Equal(testNow.Add(simklTokenLifetime)) {
		t.Fatalf("unexpected confidential token: %#v, %v", token, err)
	}
	token, err = client.ExchangePKCE(context.Background(), "code-2", pkce.Verifier, "")
	if err != nil || token.AccessToken != "new-access" || len(token.Scopes) != 1 || token.Scopes[0] != "public" {
		t.Fatalf("unexpected PKCE token: %#v, %v", token, err)
	}
}

func TestPINRequestPendingCompleteAndGone(t *testing.T) {
	var mode atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "" || request.URL.Query().Get("client_id") != "client-id" {
			t.Errorf("PIN request leaked bearer or missed client ID")
		}
		switch request.URL.Path {
		case "/api/oauth/pin":
			_, _ = writer.Write([]byte(`{"result":"OK","device_code":"DEVICE_CODE","user_code":"5G6JAH","verification_uri":"https://simkl.com/pin","expires_in":900,"interval":5}`))
		case "/api/oauth/pin/5G6JAH":
			switch mode.Load() {
			case 0:
				_, _ = writer.Write([]byte(`{"result":"KO","message":"Authorization pending"}`))
			case 1:
				_, _ = writer.Write([]byte(`{"result":"OK","access_token":"pin-access"}`))
			default:
				_, _ = writer.Write([]byte(`{"result":"OK","device_code":"NEW_DEVICE","user_code":"ABCDE","verification_uri":"https://simkl.com/pin","expires_in":900,"interval":5}`))
			}
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	_, client := newTestClient(t, server, false, false)
	authorization, err := client.RequestPIN(context.Background())
	if err != nil || authorization.UserCode != "5G6JAH" || authorization.Interval != 5*time.Second ||
		!authorization.ExpiresAt.Equal(testNow.Add(15*time.Minute)) {
		t.Fatalf("unexpected PIN authorization: %#v, %v", authorization, err)
	}
	_, err = client.PollPIN(context.Background(), *authorization)
	var platformErr *socialhub.Error
	if !errors.As(err, &platformErr) || platformErr.PlatformCode != "authorization_pending" ||
		!platformErr.Retryable() || platformErr.RetryAfter != 5*time.Second {
		t.Fatalf("unexpected pending error: %#v", platformErr)
	}
	mode.Store(1)
	token, err := client.PollPIN(context.Background(), *authorization)
	if err != nil || token.AccessToken != "pin-access" || !token.ExpiresAt.Equal(testNow.Add(simklTokenLifetime)) {
		t.Fatalf("unexpected PIN token: %#v, %v", token, err)
	}
	mode.Store(2)
	_, err = client.PollPIN(context.Background(), *authorization)
	if !errors.As(err, &platformErr) || platformErr.PlatformCode != "pin_code_gone" || platformErr.Retryable() {
		t.Fatalf("unexpected gone error: %#v", platformErr)
	}
	expired := *authorization
	expired.ExpiresAt = testNow
	if _, err := client.PollPIN(context.Background(), expired); !errors.As(err, &platformErr) || platformErr.PlatformCode != "expired_token" {
		t.Fatalf("unexpected expired error: %v", err)
	}
}

func TestOAuthAndPINValidation(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	_, client := newTestClient(t, server, true, false)
	pkce, _ := NewPKCE()
	pkce.Challenge = "wrong"
	tests := []func() error{
		func() error { _, err := client.AuthorizationURL(AuthorizationRequest{}); return err },
		func() error {
			_, err := client.AuthorizationURLPKCE(PKCEAuthorizationRequest{State: "state", PKCE: pkce})
			return err
		},
		func() error {
			_, err := client.Exchange(context.Background(), " bad ", "https://app.example/callback")
			return err
		},
		func() error { _, err := client.ExchangePKCE(context.Background(), "code", "short", ""); return err },
		func() error { _, err := client.PollPIN(context.Background(), PINAuthorization{}); return err },
	}
	for index, test := range tests {
		if err := test(); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Fatalf("case %d: expected invalid argument, got %v", index, err)
		}
	}
	_, noSecret := newTestClient(t, server, false, false)
	if _, err := noSecret.AuthorizationURL(AuthorizationRequest{RedirectURI: "https://app.example/callback", State: "state"}); !errors.Is(err, socialhub.ErrApprovalRequired) {
		t.Fatalf("expected secret approval error, got %v", err)
	}
}

func TestHTTPErrorMappingAndSanitizing(t *testing.T) {
	header := make(http.Header)
	header.Set("Retry-After", "7")
	header.Set("CF-Ray", "ray-1")
	err := decodeHTTPError(http.StatusTooManyRequests, header, []byte(`{"error":"rate_limit","code":429,"message":"slow down"}`))
	var platformErr *socialhub.Error
	if !errors.As(err, &platformErr) || platformErr.Code != socialhub.CodeRateLimited || !platformErr.Retryable() ||
		platformErr.RetryAfter != 7*time.Second || platformErr.RequestID != "ray-1" || platformErr.PlatformMessage != "slow down" {
		t.Fatalf("unexpected rate error: %#v", platformErr)
	}
	for _, test := range []struct {
		status      int
		body        string
		code        socialhub.ErrorCode
		retryable   bool
		approvalURL string
	}{
		{http.StatusUnauthorized, `{"error":"user_token_failed"}`, socialhub.CodeUnauthenticated, false, ""},
		{http.StatusPreconditionFailed, `{"error":"client_id_failed"}`, socialhub.CodeApprovalRequired, false, developerPortalURL},
		{http.StatusForbidden, `{"error":"forbidden"}`, socialhub.CodePermissionDenied, false, ""},
		{http.StatusNotFound, `{}`, socialhub.CodeNotFound, false, ""},
		{http.StatusInternalServerError, `{}`, socialhub.CodeTemporarilyUnavailable, true, ""},
	} {
		err := decodeHTTPError(test.status, nil, []byte(test.body))
		if !errors.As(err, &platformErr) || platformErr.Code != test.code || platformErr.Retryable() != test.retryable || platformErr.ApprovalURL != test.approvalURL {
			t.Fatalf("status %d: %#v", test.status, platformErr)
		}
	}
	inner := errors.New("dial failed")
	wrapped := &url.Error{Op: "Get", URL: "https://secret.invalid/?token=secret", Err: inner}
	if sanitized := sanitizeTransportError(wrapped); sanitized != inner {
		t.Fatalf("URL error was not sanitized: %v", sanitized)
	}
	if parseRetryAfter("invalid") != 0 || len([]rune(bounded(strings.Repeat("x", 10), 3))) != 3 {
		t.Fatal("metadata helpers failed")
	}
}
