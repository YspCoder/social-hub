package patreon

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

func TestOAuthAuthorizationExchangeAndRefresh(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.URL.Path != "/api/oauth2/token" || request.ParseForm() != nil ||
			request.Header.Get("Accept") != "application/json" || request.Header.Get("Content-Type") != "application/x-www-form-urlencoded" ||
			request.PostForm.Get("client_id") != "client-id" || request.PostForm.Get("client_secret") != "client-secret" {
			writer.WriteHeader(http.StatusBadRequest)
			return
		}
		switch request.PostForm.Get("grant_type") {
		case "authorization_code":
			if request.PostForm.Get("code") != "auth-code" || request.PostForm.Get("redirect_uri") != "https://app.example/callback" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			writeJSON(writer, http.StatusOK, `{"access_token":"access-1","refresh_token":"refresh-1","expires_in":3600,"scope":"identity campaigns.posts","token_type":"bearer"}`)
		case "refresh_token":
			if request.PostForm.Get("refresh_token") != "refresh-1" || request.PostForm.Get("redirect_uri") != "" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			writeJSON(writer, http.StatusOK, `{"access_token":"access-2","refresh_token":"refresh-2","expires_in":7200,"scope":"identity campaigns.posts","token_type":"Bearer"}`)
		default:
			writer.WriteHeader(http.StatusBadRequest)
		}
	}))
	defer server.Close()
	adapter, _ := newTestClient(t, server, true, false, []string{"identity"})
	oauth, err := adapter.OAuth(context.Background(), "creator")
	if err != nil {
		t.Fatal(err)
	}
	authorizationURL, err := oauth.AuthorizationURL("https://app.example/callback", "state-value", []string{"identity", "campaigns.posts"})
	if err != nil {
		t.Fatal(err)
	}
	parsed, _ := url.Parse(authorizationURL)
	query := parsed.Query()
	if parsed.Path != "/oauth2/authorize" || query.Get("response_type") != "code" || query.Get("client_id") != "client-id" || query.Get("redirect_uri") != "https://app.example/callback" || query.Get("state") != "state-value" || query.Get("scope") != "identity campaigns.posts" {
		t.Fatalf("authorization URL=%s", authorizationURL)
	}
	token, err := oauth.Exchange(context.Background(), "auth-code", "https://app.example/callback")
	expectedExpiry := time.Date(2026, 8, 2, 3, 3, 4, 0, time.UTC)
	if err != nil || token.AccessToken != "access-1" || token.RefreshToken != "refresh-1" || token.TokenType != "Bearer" || !token.ExpiresAt.Equal(expectedExpiry) || len(token.Scopes) != 2 {
		t.Fatalf("exchange token=%#v err=%v", token, err)
	}
	refreshed, err := oauth.Refresh(context.Background(), "refresh-1")
	if err != nil || refreshed.AccessToken != "access-2" || refreshed.RefreshToken != "refresh-2" || !refreshed.ExpiresAt.Equal(expectedExpiry.Add(time.Hour)) {
		t.Fatalf("refresh token=%#v err=%v", refreshed, err)
	}
}

func TestOAuthValidationAndPayloadErrors(t *testing.T) {
	client := &OAuthClient{}
	if _, err := client.AuthorizationURL("bad", "", nil); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("authorize validation=%v", err)
	}
	client.ClientID, client.AuthURL = "id", "https://www.patreon.com/oauth2/authorize"
	if _, err := client.AuthorizationURL("https://app.example/callback", "state", []string{"unknown"}); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("scope validation=%v", err)
	}
	if _, err := client.Exchange(context.Background(), "", "bad"); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("exchange validation=%v", err)
	}
	if _, err := client.Refresh(context.Background(), ""); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("refresh validation=%v", err)
	}
	client.ClientSecret = "secret"
	if _, err := client.Refresh(context.Background(), "refresh"); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("incomplete client=%v", err)
	}

	tests := []struct {
		name  string
		body  string
		code  socialhub.ErrorCode
		class socialhub.ErrorClass
	}{
		{"invalid request", `{"error":"invalid_request","error_description":"bad request"}`, socialhub.CodeInvalidArgument, socialhub.ClassPermanent},
		{"invalid grant", `{"error":"invalid_grant","error_description":"expired"}`, socialhub.CodeUnauthenticated, socialhub.ClassUserAction},
		{"server", `{"error":"server_error","error_description":"retry"}`, socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
				writeJSON(writer, http.StatusOK, test.body)
			}))
			defer server.Close()
			oauth := testOAuthClient(server)
			_, err := oauth.Refresh(context.Background(), "refresh")
			var platformErr *socialhub.Error
			if !errors.As(err, &platformErr) || platformErr.Code != test.code || platformErr.Class != test.class {
				t.Fatalf("error=%#v", err)
			}
		})
	}

	badBodies := []string{
		`not-json`,
		`{"access_token":"access","refresh_token":"refresh","expires_in":0}`,
		`{"access_token":"access","expires_in":3600}`,
		`{"access_token":"access","refresh_token":"refresh","expires_in":3600,"token_type":"bad type"}`,
	}
	for index, body := range badBodies {
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
			writeJSON(writer, http.StatusOK, body)
		}))
		oauth := testOAuthClient(server)
		_, err := oauth.Refresh(context.Background(), "refresh")
		server.Close()
		if !isPlatformError(err) {
			t.Fatalf("bad body %d error=%v", index, err)
		}
	}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writeJSON(writer, http.StatusOK, `{"access_token":"`+strings.Repeat("x", int(maxOAuthResponseBytes))+`"}`)
	}))
	defer server.Close()
	if _, err := testOAuthClient(server).Refresh(context.Background(), "refresh"); !isPlatformError(err) {
		t.Fatalf("oversized response=%v", err)
	}
}

func TestRedirectRefusal(t *testing.T) {
	var followed bool
	target := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { followed = true }))
	defer target.Close()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		http.Redirect(writer, request, target.URL, http.StatusTemporaryRedirect)
	}))
	defer server.Close()
	adapter, client := newTestClient(t, server, true, false, []string{"identity"})
	if _, err := client.GetUser(context.Background(), "me"); err == nil || followed {
		t.Fatalf("API error=%v followed=%v", err, followed)
	}
	oauth, err := adapter.OAuth(context.Background(), "creator")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := oauth.Refresh(context.Background(), "refresh"); err == nil || followed {
		t.Fatalf("OAuth error=%v followed=%v", err, followed)
	}
}

func TestHTTPErrorClassificationAndHelpers(t *testing.T) {
	tests := []struct {
		status int
		code   socialhub.ErrorCode
		class  socialhub.ErrorClass
	}{
		{http.StatusBadRequest, socialhub.CodeInvalidArgument, socialhub.ClassPermanent},
		{http.StatusMethodNotAllowed, socialhub.CodeInvalidArgument, socialhub.ClassPermanent},
		{http.StatusNotAcceptable, socialhub.CodeInvalidArgument, socialhub.ClassPermanent},
		{http.StatusUnprocessableEntity, socialhub.CodeInvalidArgument, socialhub.ClassPermanent},
		{http.StatusRequestEntityTooLarge, socialhub.CodeInvalidArgument, socialhub.ClassPermanent},
		{http.StatusUnauthorized, socialhub.CodeUnauthenticated, socialhub.ClassUserAction},
		{http.StatusForbidden, socialhub.CodePermissionDenied, socialhub.ClassUserAction},
		{http.StatusNotFound, socialhub.CodeNotFound, socialhub.ClassPermanent},
		{http.StatusGone, socialhub.CodeNotFound, socialhub.ClassPermanent},
		{http.StatusConflict, socialhub.CodeConflict, socialhub.ClassPermanent},
		{http.StatusTooManyRequests, socialhub.CodeRateLimited, socialhub.ClassRetryable},
		{http.StatusServiceUnavailable, socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable},
		{http.StatusTeapot, socialhub.CodePlatformError, socialhub.ClassPermanent},
	}
	header := http.Header{"X-Request-Id": {"header-request"}}
	body := []byte(`{"errors":[{"id":"body-request","status":"429","code_name":"RequestThrottled","detail":"slow down","retry_after_seconds":9}]}`)
	for _, test := range tests {
		err := decodeHTTPError(test.status, header, body)
		var platformErr *socialhub.Error
		if !errors.As(err, &platformErr) || platformErr.Code != test.code || platformErr.Class != test.class || platformErr.PlatformCode != "RequestThrottled" || platformErr.PlatformMessage != "slow down" || platformErr.RequestID != "body-request" || platformErr.RetryAfter != 9*time.Second {
			t.Fatalf("status=%d error=%#v", test.status, err)
		}
	}
	header.Set("Retry-After", "2.5")
	var platformErr *socialhub.Error
	_ = errors.As(decodeHTTPError(http.StatusTooManyRequests, header, body), &platformErr)
	if platformErr.RetryAfter != 2500*time.Millisecond {
		t.Fatalf("header retry=%v", platformErr.RetryAfter)
	}
	if parseRetryAfter("bad") != 0 || parseRetryAfter("86401") != 0 || boundedMessage(strings.Repeat("界", 520), 512) != strings.Repeat("界", 512) || firstNonEmpty("", "value") != "value" {
		t.Fatal("error helper contract failed")
	}
	if !validCallbackURL("https://app.example/callback?x=1") || validCallbackURL("https://user:pass@app.example/callback") || validCallbackURL("relative") {
		t.Fatal("callback helper contract failed")
	}
	if err := oauthError("oauth", "temporarily_unavailable", "retry"); !errors.Is(err, socialhub.ErrUnavailable) {
		t.Fatalf("oauth helper=%v", err)
	}
}

func testOAuthClient(server *httptest.Server) *OAuthClient {
	return &OAuthClient{
		ClientID: "id", ClientSecret: "secret", AuthURL: server.URL + "/authorize", TokenURL: server.URL,
		HTTPClient: server.Client(), Clock: fixedClock{now: time.Date(2026, 8, 2, 0, 0, 0, 0, time.UTC)},
	}
}

func isPlatformError(err error) bool {
	var platformErr *socialhub.Error
	return errors.As(err, &platformErr) && platformErr.Code == socialhub.CodePlatformError
}
