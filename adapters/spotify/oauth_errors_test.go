package spotify

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

func TestOAuthPKCERefreshAndClientCredentials(t *testing.T) {
	pkce, err := NewPKCE()
	if err != nil || !validPKCEValue(pkce.Verifier) || !validPKCEValue(pkce.Challenge) || pkce.Verifier == pkce.Challenge {
		t.Fatalf("PKCE=%#v err=%v", pkce, err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.ParseForm() != nil {
			writer.WriteHeader(http.StatusBadRequest)
			return
		}
		switch request.Form.Get("grant_type") {
		case "authorization_code":
			_, _, basic := request.BasicAuth()
			if basic || request.Form.Get("client_id") != "client-id" || request.Form.Get("code") != "code" || request.Form.Get("code_verifier") != pkce.Verifier {
				t.Errorf("exchange form=%v basic=%v", request.Form, basic)
			}
			writeJSON(writer, `{"access_token":"user-access","refresh_token":"refresh-1","token_type":"Bearer","expires_in":3600,"scope":"user-read-private user-library-read"}`)
		case "refresh_token":
			_, _, basic := request.BasicAuth()
			if basic || request.Form.Get("client_id") != "client-id" || request.Form.Get("refresh_token") != "refresh-1" {
				t.Errorf("refresh form=%v basic=%v", request.Form, basic)
			}
			writeJSON(writer, `{"access_token":"user-renewed","token_type":"Bearer","expires_in":3600,"scope":"user-read-private"}`)
		case "client_credentials":
			clientID, secret, basic := request.BasicAuth()
			if !basic || clientID != "client-id" || secret != "client-secret" || request.Form.Get("client_id") != "" {
				t.Errorf("client credentials form=%v basic=%v", request.Form, basic)
			}
			writeJSON(writer, `{"access_token":"app-access","token_type":"Bearer","expires_in":3600}`)
		default:
			writer.WriteHeader(http.StatusBadRequest)
		}
	}))
	defer server.Close()
	adapter, _ := newTestAdapter(t, server, "premium", allTestScopes)
	oauth, err := adapter.OAuth(context.Background(), "listener")
	if err != nil {
		t.Fatal(err)
	}
	authorizationURL, err := oauth.AuthorizationURL("https://app.example/callback", "state-value", []string{ScopeUserReadPrivate, ScopeUserLibraryRead}, pkce)
	if err != nil {
		t.Fatal(err)
	}
	parsed, _ := url.Parse(authorizationURL)
	query := parsed.Query()
	if query.Get("response_type") != "code" || query.Get("client_id") != "client-id" || query.Get("state") != "state-value" ||
		query.Get("code_challenge") != pkce.Challenge || query.Get("code_challenge_method") != "S256" || query.Get("scope") != "user-read-private user-library-read" {
		t.Fatalf("authorization URL=%s", authorizationURL)
	}
	token, err := oauth.Exchange(context.Background(), "code", "https://app.example/callback", pkce.Verifier)
	if err != nil || token.AccessToken != "user-access" || token.RefreshToken != "refresh-1" || !token.ExpiresAt.Equal(testNow.Add(time.Hour)) || len(token.Scopes) != 2 {
		t.Fatalf("token=%#v err=%v", token, err)
	}
	renewed, err := oauth.Refresh(context.Background(), token.RefreshToken)
	if err != nil || renewed.AccessToken != "user-renewed" || renewed.RefreshToken != "refresh-1" {
		t.Fatalf("renewed=%#v err=%v", renewed, err)
	}
	app, err := oauth.ClientCredentials(context.Background())
	if err != nil || app.AccessToken != "app-access" || app.RefreshToken != "" {
		t.Fatalf("app token=%#v err=%v", app, err)
	}
}

func TestOAuthValidationAndResponseFailures(t *testing.T) {
	pkce, _ := NewPKCE()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_ = request.ParseForm()
		switch request.Form.Get("code") {
		case "denied":
			writer.Header().Set("Retry-After", "7")
			writer.WriteHeader(http.StatusBadRequest)
			writeJSON(writer, `{"error":"invalid_grant","error_description":"expired authorization code"}`)
		case "bad-json":
			writeJSON(writer, `{`)
		case "bad-expiry":
			writeJSON(writer, `{"access_token":"token","refresh_token":"refresh","expires_in":0}`)
		case "large":
			_, _ = writer.Write(make([]byte, maxOAuthResponseSize+1))
		default:
			writeJSON(writer, `{"access_token":"token","expires_in":3600}`)
		}
	}))
	defer server.Close()
	oauth := &OAuthClient{ClientID: "client-id", ClientSecret: "client-secret", AuthURL: server.URL + "/authorize", TokenURL: server.URL, HTTPClient: server.Client(), Clock: fixedClock{now: testNow}}

	invalidCalls := []func() error{
		func() error {
			_, err := oauth.AuthorizationURL("http://localhost/callback", "", nil, PKCE{})
			return err
		},
		func() error {
			_, err := oauth.AuthorizationURL("https://app.example/callback", "state", []string{"bad scope"}, pkce)
			return err
		},
		func() error {
			_, err := oauth.Exchange(context.Background(), "", "https://app.example/callback", "short")
			return err
		},
		func() error { _, err := oauth.Refresh(context.Background(), ""); return err },
	}
	for index, call := range invalidCalls {
		if err := call(); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Fatalf("invalid OAuth call %d error=%v", index, err)
		}
	}
	if !validSpotifyRedirectURI("http://127.0.0.1:8080/callback") || validSpotifyRedirectURI("http://app.example/callback") || validSpotifyRedirectURI("https://localhost/callback") {
		t.Fatal("Spotify redirect URI validation mismatch")
	}
	if _, err := oauth.Exchange(context.Background(), "denied", "https://app.example/callback", pkce.Verifier); !errors.Is(err, socialhub.ErrUnauthenticated) {
		t.Fatalf("invalid grant error=%v", err)
	}
	for _, code := range []string{"bad-json", "bad-expiry", "large"} {
		if _, err := oauth.Exchange(context.Background(), code, "https://app.example/callback", pkce.Verifier); errorCode(err) != socialhub.CodePlatformError {
			t.Fatalf("code=%s error=%v", code, err)
		}
	}
	missingSecret := *oauth
	missingSecret.ClientSecret = ""
	if _, err := missingSecret.ClientCredentials(context.Background()); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("client credentials without secret error=%v", err)
	}
	missingClient := *oauth
	missingClient.ClientID = ""
	if _, err := missingClient.Refresh(context.Background(), "refresh"); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("refresh without client ID error=%v", err)
	}
}

func TestHTTPErrorClassificationAndQuotaReason(t *testing.T) {
	tests := []struct {
		status int
		code   socialhub.ErrorCode
		class  socialhub.ErrorClass
	}{
		{http.StatusBadRequest, socialhub.CodeInvalidArgument, socialhub.ClassPermanent},
		{http.StatusUnauthorized, socialhub.CodeUnauthenticated, socialhub.ClassUserAction},
		{http.StatusForbidden, socialhub.CodePermissionDenied, socialhub.ClassUserAction},
		{http.StatusNotFound, socialhub.CodeNotFound, socialhub.ClassPermanent},
		{http.StatusConflict, socialhub.CodeConflict, socialhub.ClassPermanent},
		{http.StatusTooManyRequests, socialhub.CodeRateLimited, socialhub.ClassRetryable},
		{http.StatusServiceUnavailable, socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable},
		{http.StatusTeapot, socialhub.CodePlatformError, socialhub.ClassPermanent},
	}
	for _, test := range tests {
		err := decodeHTTPError(test.status, http.Header{}, []byte(`{"error":{"status":400,"message":"failure"}}`))
		var hubError *socialhub.Error
		if !errors.As(err, &hubError) || hubError.Code != test.code || hubError.Class != test.class || hubError.PlatformMessage != "failure" {
			t.Fatalf("status=%d error=%#v", test.status, err)
		}
	}
	header := http.Header{"Retry-After": {"7"}, "Spotify-Request-Id": {"request-1"}}
	err := decodeHTTPError(http.StatusTooManyRequests, header, []byte(`{"error":{"status":429,"message":"quota reached","reason":"QUOTA_EXCEEDED"}}`))
	var hubError *socialhub.Error
	if !errors.As(err, &hubError) || hubError.PlatformCode != "QUOTA_EXCEEDED" || hubError.RetryAfter != 7*time.Second || hubError.RequestID != "request-1" || !hubError.Retryable() {
		t.Fatalf("quota error=%#v", err)
	}
	if parseRetryAfter("-1") != 0 || parseRetryAfter("invalid") != 0 || !strings.Contains(platformError("op", "message").Error(), "spotify") {
		t.Fatal("error helper mismatch")
	}
}
