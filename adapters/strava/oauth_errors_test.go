package strava

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
		if request.URL.Path != "/oauth/token" || request.Method != http.MethodPost || request.Header.Get("Content-Type") != "application/x-www-form-urlencoded" || request.ParseForm() != nil {
			t.Fatalf("token request=%s %s content-type=%q", request.Method, request.URL.Path, request.Header.Get("Content-Type"))
		}
		if request.Form.Get("client_id") != "12345" || request.Form.Get("client_secret") != "client-secret" {
			t.Errorf("credentials form=%v", request.Form)
		}
		switch request.Form.Get("grant_type") {
		case "authorization_code":
			if request.Form.Get("code") != "auth-code" {
				t.Errorf("exchange form=%v", request.Form)
			}
			writeJSON(writer, http.StatusOK, `{"token_type":"Bearer","expires_at":1785726123,"expires_in":21600,"refresh_token":"refresh-1","access_token":"access-1","scope":"read activity:read activity:write"}`)
		case "refresh_token":
			if request.Form.Get("refresh_token") != "refresh-1" {
				t.Errorf("refresh form=%v", request.Form)
			}
			writeJSON(writer, http.StatusOK, `{"token_type":"bearer","expires_in":21600,"refresh_token":"refresh-2","access_token":"access-2"}`)
		default:
			t.Fatalf("grant=%q", request.Form.Get("grant_type"))
		}
	}))
	defer server.Close()
	adapter, _ := newTestAdapter(t, server, false, nil)
	oauth, err := adapter.OAuth(context.Background(), "athlete")
	if err != nil {
		t.Fatal(err)
	}
	authorize, err := oauth.AuthorizationURL("https://app.example/callback", "state-value", []string{"read", "activity:read", "activity:write"})
	if err != nil {
		t.Fatal(err)
	}
	parsed, _ := url.Parse(authorize)
	if parsed.Path != "/authorize" || parsed.Query().Get("client_id") != "12345" || parsed.Query().Get("redirect_uri") != "https://app.example/callback" || parsed.Query().Get("state") != "state-value" || parsed.Query().Get("scope") != "read,activity:read,activity:write" {
		t.Fatalf("authorize URL=%s", authorize)
	}
	token, err := oauth.Exchange(context.Background(), "auth-code")
	if err != nil || token.AccessToken != "access-1" || token.RefreshToken != "refresh-1" || token.TokenType != "Bearer" || !token.ExpiresAt.Equal(time.Unix(1785726123, 0).UTC()) || len(token.Scopes) != 3 {
		t.Fatalf("exchange token=%#v err=%v", token, err)
	}
	token, err = oauth.Refresh(context.Background(), "refresh-1")
	if err != nil || token.AccessToken != "access-2" || token.RefreshToken != "refresh-2" || !token.ExpiresAt.Equal(testNow.Add(6*time.Hour)) {
		t.Fatalf("refresh token=%#v err=%v", token, err)
	}
}

func TestOAuthAndHTTPErrorValidation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/oauth/token":
			writeJSON(writer, http.StatusBadRequest, `{"message":"Bad Request","errors":[{"resource":"AuthorizationCode","field":"code","code":"invalid"}]}`)
		case "/api/activities/" + testActivityID:
			writer.Header().Set("Retry-After", "2.5")
			writer.Header().Set("X-Request-Id", "request-id")
			writeJSON(writer, http.StatusTooManyRequests, `{"message":"Rate Limit Exceeded","errors":[{"resource":"Application","field":"rate limit","code":"exceeded"}]}`)
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	adapter, client := newTestAdapter(t, server, false, nil)
	oauth, err := adapter.OAuth(context.Background(), "athlete")
	if err != nil {
		t.Fatal(err)
	}
	invalidAuthorizations := []struct {
		redirect string
		state    string
		scopes   []string
	}{
		{"ftp://app.example/callback", "state", []string{"read"}},
		{"https://app.example/callback", "", []string{"read"}},
		{"https://app.example/callback", "state", []string{"unknown"}},
		{"https://app.example/callback", "state", []string{"read", "read"}},
	}
	for index, input := range invalidAuthorizations {
		if _, err := oauth.AuthorizationURL(input.redirect, input.state, input.scopes); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Fatalf("authorization %d error=%v", index, err)
		}
	}
	if _, err := oauth.Exchange(context.Background(), ""); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("empty code error=%v", err)
	}
	if _, err := oauth.Refresh(context.Background(), ""); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("empty refresh error=%v", err)
	}
	if _, err := oauth.Exchange(context.Background(), "bad-code"); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("platform OAuth error=%v", err)
	}
	_, err = client.GetPost(context.Background(), testActivityID)
	var hubError *socialhub.Error
	if !errors.As(err, &hubError) || !errors.Is(err, socialhub.ErrRateLimited) || !hubError.Retryable() || hubError.RetryAfter != 2500*time.Millisecond || hubError.PlatformCode != "exceeded" || !strings.Contains(hubError.PlatformMessage, "Application rate limit") || hubError.RequestID != "request-id" {
		t.Fatalf("rate error=%#v raw=%v", hubError, err)
	}
	if code, class := classifyError(http.StatusServiceUnavailable); code != socialhub.CodeTemporarilyUnavailable || class != socialhub.ClassRetryable {
		t.Fatalf("server classification=%s/%s", code, class)
	}
	if parseRetryAfter("bad") != 0 || parseRetryAfter("90000") != 0 || boundedMessage(strings.Repeat("界", 300), 20) != strings.Repeat("界", 20) {
		t.Fatal("error helper contract failed")
	}
}

func TestOAuthRejectsMalformedSuccess(t *testing.T) {
	responses := []string{
		`not-json`,
		`{"access_token":"a","refresh_token":"r","expires_in":0}`,
		`{"access_token":"a","refresh_token":"r","expires_at":4102444800}`,
	}
	for index, body := range responses {
		t.Run(string(rune('a'+index)), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				writeJSON(writer, http.StatusOK, body)
			}))
			defer server.Close()
			adapter, _ := newTestAdapter(t, server, false, nil)
			oauth, err := adapter.OAuth(context.Background(), "athlete")
			if err != nil {
				t.Fatal(err)
			}
			if _, err := oauth.Exchange(context.Background(), "code"); errorCode(err) != socialhub.CodePlatformError {
				t.Fatalf("error=%v code=%s", err, errorCode(err))
			}
		})
	}
}
