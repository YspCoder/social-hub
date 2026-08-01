package soundcloud

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

func TestOAuth21Flows(t *testing.T) {
	pkce, err := NewPKCE()
	if err != nil || !validPKCEValue(pkce.Verifier) || !validPKCEValue(pkce.Challenge) || pkce.Verifier == pkce.Challenge {
		t.Fatalf("PKCE=%#v err=%v", pkce, err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.ParseForm() != nil {
			writer.WriteHeader(http.StatusBadRequest)
			return
		}
		grant := request.Form.Get("grant_type")
		switch grant {
		case "authorization_code":
			if request.Form.Get("client_id") != "client-id" || request.Form.Get("client_secret") != "client-secret" || request.Form.Get("code") != "code" || request.Form.Get("code_verifier") != pkce.Verifier {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			writeJSON(writer, `{"access_token":"user-access","refresh_token":"refresh-1","token_type":"bearer","expires_in":3600,"scope":""}`)
		case "client_credentials":
			clientID, secret, ok := request.BasicAuth()
			if !ok || clientID != "client-id" || secret != "client-secret" || request.Form.Get("client_id") != "" || request.Form.Get("client_secret") != "" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			writeJSON(writer, `{"access_token":"app-access","refresh_token":"app-refresh","expires_in":3599}`)
		case "refresh_token":
			if request.Form.Get("refresh_token") != "refresh-1" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			writeJSON(writer, `{"access_token":"user-renewed","refresh_token":"refresh-2","expires_in":3600}`)
		default:
			writer.WriteHeader(http.StatusBadRequest)
		}
	}))
	defer server.Close()
	adapter, _ := newTestAdapter(t, server)
	oauth, err := adapter.OAuth(context.Background(), "artist")
	if err != nil {
		t.Fatal(err)
	}
	authorizationURL, err := oauth.AuthorizationURL("my-app://soundcloud/callback", "state-value", pkce)
	if err != nil {
		t.Fatal(err)
	}
	parsed, _ := url.Parse(authorizationURL)
	query := parsed.Query()
	if query.Get("response_type") != "code" || query.Get("client_id") != "client-id" || query.Get("state") != "state-value" || query.Get("code_challenge") != pkce.Challenge || query.Get("code_challenge_method") != "S256" {
		t.Fatalf("authorization URL=%s", authorizationURL)
	}
	token, err := oauth.Exchange(context.Background(), "code", "my-app://soundcloud/callback", pkce.Verifier)
	if err != nil || token.AccessToken != "user-access" || token.RefreshToken != "refresh-1" || token.TokenType != "OAuth" || !token.ExpiresAt.Equal(testNow.Add(time.Hour)) {
		t.Fatalf("token=%#v err=%v", token, err)
	}
	appToken, err := oauth.ClientCredentials(context.Background())
	if err != nil || appToken.AccessToken != "app-access" || appToken.RefreshToken != "app-refresh" || !appToken.ExpiresAt.Equal(testNow.Add(3599*time.Second)) {
		t.Fatalf("app token=%#v err=%v", appToken, err)
	}
	renewed, err := oauth.Refresh(context.Background(), token.RefreshToken)
	if err != nil || renewed.AccessToken != "user-renewed" || renewed.RefreshToken != "refresh-2" {
		t.Fatalf("renewed=%#v err=%v", renewed, err)
	}
}

func TestOAuthValidationErrorsAndRefreshRotation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Query().Get("large") == "true" {
			writer.WriteHeader(http.StatusOK)
			_, _ = writer.Write(make([]byte, maxOAuthResponseSize+1))
			return
		}
		if request.ParseForm() == nil && request.Form.Get("grant_type") == "refresh_token" {
			writeJSON(writer, `{"access_token":"renewed","expires_in":3600}`)
			return
		}
		writer.WriteHeader(http.StatusBadRequest)
		writeJSON(writer, `{"error":"invalid_grant","message":"denied"}`)
	}))
	defer server.Close()
	adapter, _ := newTestAdapter(t, server)
	oauth, _ := adapter.OAuth(context.Background(), "artist")
	pkce, _ := NewPKCE()

	invalidCalls := []func() error{
		func() error { _, err := oauth.AuthorizationURL("bad", "", PKCE{}); return err },
		func() error { _, err := oauth.Exchange(context.Background(), "", "bad", "short"); return err },
		func() error { _, err := oauth.Refresh(context.Background(), ""); return err },
	}
	for index, call := range invalidCalls {
		if err := call(); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Fatalf("invalid OAuth call %d error=%v", index, err)
		}
	}
	if _, err := oauth.Exchange(context.Background(), "code", "https://app.example/callback", pkce.Verifier); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("OAuth response error=%v", err)
	}
	if _, err := oauth.Refresh(context.Background(), "refresh-1"); errorCode(err) != socialhub.CodePlatformError {
		t.Fatalf("missing rotated refresh token error=%v", err)
	}
	bad := *oauth
	bad.ClientID = ""
	if _, err := bad.ClientCredentials(context.Background()); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("bad OAuth client error=%v", err)
	}
}

func TestHTTPErrorClassification(t *testing.T) {
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
		err := decodeHTTPError(test.status, nil, []byte(`{"errors":[{"error_message":"failure"}]}`))
		var hubError *socialhub.Error
		if !errors.As(err, &hubError) || hubError.Code != test.code || hubError.Class != test.class || hubError.PlatformMessage != "failure" {
			t.Fatalf("status=%d error=%#v", test.status, err)
		}
	}
}
