package baiduads

import (
	"context"
	"encoding/json"
	"errors"
	"io"
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
		if request.Method != http.MethodPost || request.Header.Get("Content-Type") != "application/json;charset=UTF-8" {
			t.Fatalf("method=%s content-type=%q", request.Method, request.Header.Get("Content-Type"))
		}
		var body map[string]any
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body["appId"] != "baidu-app-id" || body["secretKey"] != testSecretKey || body["userId"] != float64(9001) {
			t.Fatalf("OAuth body=%v", body)
		}
		writer.Header().Set("Content-Type", "application/json")
		switch request.URL.Path {
		case "/oauth/accessToken":
			if body["grantType"] != "auth_code" || body["authCode"] != "authorization-code" {
				t.Fatalf("exchange body=%v", body)
			}
			writeOAuth(t, writer, map[string]any{
				"accessToken": "access-1", "refreshToken": "refresh-1", "expiresIn": 86400,
				"refreshExpiresIn": 2592000, "expiresTime": "20260810", "refreshExpiresTime": "20260908",
				"openId": "open-1", "userId": 9001, "scope": map[string]string{"1_2_1_1": "account", "1_0_1": "finance"},
			})
		case "/oauth/refreshToken":
			if body["refreshToken"] != "refresh-1" {
				t.Fatalf("refresh body=%v", body)
			}
			writeOAuth(t, writer, map[string]any{
				"accessToken": "access-2", "expiresIn": 7200, "refreshExpiresIn": 2000000,
				"expiresTime": "20260809", "refreshExpiresTime": "20260901", "openId": "open-1",
			})
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	adapter, _ := newTestClient(t, server)
	defer adapter.Close()
	oauth, err := adapter.OAuth(context.Background(), testAccountID)
	if err != nil {
		t.Fatal(err)
	}
	authorize, err := oauth.AuthorizationURL(AuthorizationRequest{
		Callback: "https://app.example/callback", Scope: "1_2_1_1,1_0_1", State: "csrf-state",
	})
	if err != nil {
		t.Fatal(err)
	}
	parsed, _ := url.Parse(authorize)
	if parsed.Path != "/oauth/page/index" || parsed.Query().Get("platformId") != platformID ||
		parsed.Query().Get("appId") != "baidu-app-id" || parsed.Query().Get("callback") != "https://app.example/callback" ||
		parsed.Query().Get("scope") != "1_2_1_1,1_0_1" || parsed.Query().Get("state") != "csrf-state" {
		t.Fatalf("authorization URL=%s", authorize)
	}
	token, err := oauth.Exchange(context.Background(), 9001, "authorization-code")
	if err != nil {
		t.Fatal(err)
	}
	if token.Token.AccessToken != "access-1" || token.Token.RefreshToken != "refresh-1" || token.Token.TokenType != "BaiduAds" ||
		!token.Token.ExpiresAt.Equal(testNow.Add(24*time.Hour)) || !token.RefreshExpiresAt.Equal(testNow.Add(30*24*time.Hour)) ||
		token.UserID != 9001 || token.OpenID != "open-1" || strings.Join(token.Token.Scopes, ",") != "1_0_1,1_2_1_1" {
		t.Fatalf("exchange token=%+v", token)
	}
	refreshed, err := oauth.Refresh(context.Background(), 9001, token.Token.RefreshToken)
	if err != nil {
		t.Fatal(err)
	}
	if refreshed.Token.AccessToken != "access-2" || refreshed.Token.RefreshToken != "refresh-1" || refreshed.UserID != 9001 ||
		!refreshed.Token.ExpiresAt.Equal(testNow.Add(2*time.Hour)) {
		t.Fatalf("refreshed token=%+v", refreshed)
	}
}

func TestOAuthValidationAndErrors(t *testing.T) {
	valid := &OAuthClient{
		AppID: "app", SecretKey: "secret", BaseURL: "https://u.baidu.com",
		HTTPClient: http.DefaultClient, Clock: fixedClock{value: testNow},
	}
	invalidURLs := []AuthorizationRequest{
		{},
		{Callback: "bad", Scope: "scope", State: "state"},
		{Callback: "https://user@example.com/callback", Scope: "scope", State: "state"},
		{Callback: "https://example.com/callback?query=1", Scope: "scope", State: "state"},
		{Callback: "https://example.com/callback", Scope: "", State: "state"},
		{Callback: "https://example.com/callback", Scope: "scope", State: ""},
	}
	for _, input := range invalidURLs {
		if _, err := valid.AuthorizationURL(input); requireHubError(t, err).Code != socialhub.CodeInvalidArgument {
			t.Fatalf("authorization input=%+v err=%v", input, err)
		}
	}
	if _, err := (&OAuthClient{}).AuthorizationURL(AuthorizationRequest{Callback: "https://example.com", Scope: "scope", State: "state"}); requireHubError(t, err).Code != socialhub.CodeInvalidArgument {
		t.Fatalf("incomplete authorize err=%v", err)
	}
	if _, err := valid.Exchange(context.Background(), 0, ""); requireHubError(t, err).Code != socialhub.CodeInvalidArgument {
		t.Fatalf("exchange validation err=%v", err)
	}
	if _, err := valid.Refresh(context.Background(), 0, ""); requireHubError(t, err).Code != socialhub.CodeInvalidArgument {
		t.Fatalf("refresh validation err=%v", err)
	}
	if _, err := (&OAuthClient{}).Exchange(context.Background(), 1, "code"); requireHubError(t, err).Code != socialhub.CodeInvalidArgument {
		t.Fatalf("incomplete exchange err=%v", err)
	}

	tests := []struct {
		name   string
		status int
		body   string
		want   socialhub.ErrorCode
	}{
		{"business", http.StatusOK, `{"code":600011,"message":"bad secret authorization-code"}`, socialhub.CodeInvalidArgument},
		{"rate", http.StatusTooManyRequests, `{"code":8501,"message":"slow"}`, socialhub.CodeRateLimited},
		{"malformed success", http.StatusOK, `{`, socialhub.CodePlatformError},
		{"missing code", http.StatusOK, `{"message":"missing"}`, socialhub.CodePlatformError},
		{"invalid token", http.StatusOK, `{"code":0,"message":"success","data":{"accessToken":"","refreshToken":"r","expiresIn":1,"refreshExpiresIn":1,"userId":1}}`, socialhub.CodePlatformError},
		{"mismatched user", http.StatusOK, `{"code":0,"message":"success","data":{"accessToken":"a","refreshToken":"r","expiresIn":1,"refreshExpiresIn":1,"userId":2}}`, socialhub.CodePlatformError},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				writer.Header().Set("X-B3-Traceid", "oauth-trace")
				writer.Header().Set("Retry-After", "2")
				writer.WriteHeader(test.status)
				_, _ = io.WriteString(writer, test.body)
			}))
			defer server.Close()
			client := *valid
			client.BaseURL, client.HTTPClient = server.URL, server.Client()
			_, err := client.Exchange(context.Background(), 1, "authorization-code")
			hub := requireHubError(t, err)
			if hub.Code != test.want || hub.Op != "oauth_exchange" {
				t.Fatalf("err=%+v", hub)
			}
			if test.name == "business" && strings.Contains(hub.PlatformMessage, "secret") || strings.Contains(hub.PlatformMessage, "authorization-code") {
				t.Fatalf("OAuth error leaked credential: %+v", hub)
			}
			if test.name == "rate" && (!errors.Is(err, socialhub.ErrRateLimited) || hub.RetryAfter != 2*time.Second || hub.RequestID != "oauth-trace") {
				t.Fatalf("rate err=%+v", hub)
			}
		})
	}
}

func TestOAuthResponseBoundsAndTransportFailure(t *testing.T) {
	oversized := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_, _ = io.WriteString(writer, strings.Repeat("x", int(maxOAuthResponseBytes)+1))
	}))
	client := &OAuthClient{
		AppID: "app", SecretKey: "secret", BaseURL: oversized.URL,
		HTTPClient: oversized.Client(), Clock: fixedClock{value: testNow},
	}
	if _, err := client.Exchange(context.Background(), 1, "code"); requireHubError(t, err).Code != socialhub.CodePlatformError {
		t.Fatalf("oversized err=%v", err)
	}
	oversized.Close()
	if _, err := client.Exchange(context.Background(), 1, "code"); requireHubError(t, err).Code != socialhub.CodeTemporarilyUnavailable {
		t.Fatalf("transport err=%v", err)
	}
}

func writeOAuth(t *testing.T, writer http.ResponseWriter, data any) {
	t.Helper()
	if err := json.NewEncoder(writer).Encode(map[string]any{"code": 0, "message": "success", "data": data}); err != nil {
		t.Fatal(err)
	}
}
