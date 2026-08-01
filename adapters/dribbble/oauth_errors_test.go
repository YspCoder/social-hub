package dribbble

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

func TestOAuthAuthorizationAndExchange(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.URL.Path != "/oauth/token" || request.ParseForm() != nil ||
			request.Header.Get("Accept") != "application/json" || request.Header.Get("Content-Type") != "application/x-www-form-urlencoded" ||
			request.PostForm.Get("client_id") != "client-id" || request.PostForm.Get("client_secret") != "client-secret" ||
			request.PostForm.Get("code") != "auth-code" || request.PostForm.Get("redirect_uri") != "https://app.example/callback" {
			writer.WriteHeader(http.StatusBadRequest)
			return
		}
		writeJSON(writer, http.StatusOK, `{"access_token":"oauth-token","token_type":"bearer","scope":"public upload"}`)
	}))
	defer server.Close()
	adapter, _ := newTestClient(t, server, []string{"public", "upload"})
	oauth, err := adapter.OAuth(context.Background(), "designer")
	if err != nil {
		t.Fatal(err)
	}
	authorizationURL, err := oauth.AuthorizationURL("https://app.example/callback", "state-value", []string{"public", "upload"})
	if err != nil {
		t.Fatal(err)
	}
	parsed, _ := url.Parse(authorizationURL)
	if parsed.Path != "/oauth/authorize" || parsed.Query().Get("client_id") != "client-id" || parsed.Query().Get("redirect_uri") != "https://app.example/callback" || parsed.Query().Get("scope") != "public upload" || parsed.Query().Get("state") != "state-value" {
		t.Fatalf("authorization URL=%s", authorizationURL)
	}
	token, err := oauth.Exchange(context.Background(), "auth-code", "https://app.example/callback")
	if err != nil || token.AccessToken != "oauth-token" || token.TokenType != "Bearer" || token.RefreshToken != "" || !token.ExpiresAt.IsZero() || len(token.Scopes) != 2 {
		t.Fatalf("token=%#v err=%v", token, err)
	}
}

func TestOAuthValidationErrorsAndRedirectRefusal(t *testing.T) {
	client := &OAuthClient{}
	if _, err := client.AuthorizationURL("bad", "", nil); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("authorize validation=%v", err)
	}
	client.ClientID, client.AuthURL = "client-id", "https://dribbble.com/oauth/authorize"
	if _, err := client.AuthorizationURL("https://app.example/callback", "state", []string{"unknown"}); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("scope validation=%v", err)
	}
	if _, err := client.Exchange(context.Background(), "", "bad"); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("exchange validation=%v", err)
	}

	t.Run("wrapper error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
			writeJSON(writer, http.StatusOK, `{"error":"access_denied","error_description":"user denied access"}`)
		}))
		defer server.Close()
		oauth := &OAuthClient{ClientID: "id", ClientSecret: "secret", TokenURL: server.URL, HTTPClient: server.Client()}
		_, err := oauth.Exchange(context.Background(), "code", "https://app.example/callback")
		var platformErr *socialhub.Error
		if !errors.As(err, &platformErr) || platformErr.Code != socialhub.CodeUnauthenticated || platformErr.Class != socialhub.ClassUserAction || platformErr.PlatformCode != "access_denied" || platformErr.PlatformMessage != "user denied access" {
			t.Fatalf("error=%#v", err)
		}
	})

	t.Run("redirect", func(t *testing.T) {
		var followed bool
		target := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { followed = true }))
		defer target.Close()
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			http.Redirect(writer, request, target.URL, http.StatusTemporaryRedirect)
		}))
		defer server.Close()
		adapter, api := newTestClient(t, server, []string{"public"})
		if _, err := api.GetPost(context.Background(), "1"); err == nil || followed {
			t.Fatalf("API error=%v followed=%v", err, followed)
		}
		oauth, err := adapter.OAuth(context.Background(), "designer")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := oauth.Exchange(context.Background(), "code", "https://app.example/callback"); err == nil || followed {
			t.Fatalf("OAuth error=%v followed=%v", err, followed)
		}
	})
}

func TestHTTPErrorClassificationAndHelpers(t *testing.T) {
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
	header := http.Header{"Retry-After": {"2.5"}, "X-Request-Id": {"request-1"}}
	for _, test := range tests {
		err := decodeHTTPError(test.status, header, []byte(`{"message":"failure"}`))
		var platformErr *socialhub.Error
		if !errors.As(err, &platformErr) || platformErr.Code != test.code || platformErr.Class != test.class || platformErr.HTTPStatus != test.status || platformErr.RequestID != "request-1" || platformErr.RetryAfter != 2500*time.Millisecond || platformErr.PlatformMessage != "failure" {
			t.Fatalf("status=%d error=%#v", test.status, err)
		}
	}
	validationErr := decodeHTTPError(http.StatusUnprocessableEntity, nil, []byte(`{"errors":[{"attribute":"title","message":"is required"}]}`))
	var platformErr *socialhub.Error
	if !errors.As(validationErr, &platformErr) || platformErr.PlatformMessage != "title: is required" {
		t.Fatalf("validation error=%#v", validationErr)
	}
	if parseRetryAfter("bad") != 0 || parseRetryAfter("86401") != 0 || boundedMessage(strings.Repeat("界", 520), 512) != strings.Repeat("界", 512) || firstNonEmpty("", "value") != "value" {
		t.Fatal("error helper contract failed")
	}
}
