package marketing

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"social-hub/pkg/socialhub"
)

func TestOAuthAuthorizationExchangeAndRefresh(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.Header.Get("Access-Token") != "" {
			t.Fatalf("unexpected OAuth request: %s %s", request.Method, request.URL.Path)
		}
		var body map[string]any
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body["app_id"] != float64(789) || body["secret"] != "app-secret" {
			t.Errorf("OAuth body=%v", body)
		}
		switch request.URL.Path {
		case "/oauth2/authorize/access_token":
			if body["auth_code"] != "auth-code" {
				t.Errorf("exchange body=%v", body)
			}
			writeJSON(writer, http.StatusOK, `{"code":0,"data":{"access_token":"user-token","refresh_token":"refresh-1","access_token_expires_in":86400,"refresh_token_expires_in":2592000}}`)
		case "/oauth2/authorize/refresh_token":
			if body["refresh_token"] != "refresh-1" {
				t.Errorf("refresh body=%v", body)
			}
			writeJSON(writer, http.StatusOK, `{"code":0,"data":{"access_token":"user-token-2","refresh_token":"refresh-2","access_token_expires_in":86400,"refresh_token_expires_in":2592000}}`)
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	adapter, _ := newTestAdapter(t, server)
	oauth, err := adapter.OAuth(context.Background(), "ads-primary")
	if err != nil {
		t.Fatal(err)
	}
	authorizationURL, err := oauth.AuthorizationURL(AuthorizationRequest{
		RedirectURI: "https://app.example/callback", State: "state-1", Scopes: []string{"ad_manage", "report_service"}, OAuthType: "advertiser",
	})
	if err != nil || !strings.Contains(authorizationURL, "/tools/authorize?") || !strings.Contains(authorizationURL, "app_id=789") ||
		!strings.Contains(authorizationURL, "scope=%5B%22ad_manage%22%2C%22report_service%22%5D") {
		t.Fatalf("authorization URL=%q err=%v", authorizationURL, err)
	}
	exchanged, err := oauth.Exchange(context.Background(), "auth-code")
	if err != nil || exchanged.Token.AccessToken != "user-token" || exchanged.Token.RefreshToken != "refresh-1" ||
		!exchanged.Token.ExpiresAt.Equal(testNow.Add(24*60*60*1e9)) {
		t.Fatalf("exchange=%#v err=%v", exchanged, err)
	}
	refreshed, err := oauth.Refresh(context.Background(), "refresh-1")
	if err != nil || refreshed.Token.AccessToken != "user-token-2" || refreshed.RefreshExpiresAt.IsZero() {
		t.Fatalf("refresh=%#v err=%v", refreshed, err)
	}
}

func TestOAuthValidationBoundedResponseAndSecrets(t *testing.T) {
	client := &OAuthClient{}
	if _, err := client.AuthorizationURL(AuthorizationRequest{}); hubError(t, err).Code != socialhub.CodeInvalidArgument {
		t.Fatalf("authorize error=%v", err)
	}
	if _, err := client.Exchange(context.Background(), ""); hubError(t, err).Code != socialhub.CodeInvalidArgument {
		t.Fatalf("exchange error=%v", err)
	}
	if _, err := client.Refresh(context.Background(), ""); hubError(t, err).Code != socialhub.CodeInvalidArgument {
		t.Fatalf("refresh error=%v", err)
	}
	valid := &OAuthClient{AppID: 1, AuthorizationBaseURL: "https://developers.e.kuaishou.com"}
	for _, input := range []AuthorizationRequest{
		{RedirectURI: "relative"}, {RedirectURI: "https://app.example/callback", OAuthType: "bad"},
		{RedirectURI: "https://app.example/callback", Scopes: []string{"Bad"}},
	} {
		if _, err := valid.AuthorizationURL(input); err == nil {
			t.Fatalf("request should be invalid: %#v", input)
		}
	}

	tests := []struct {
		name   string
		status int
		body   string
		code   socialhub.ErrorCode
	}{
		{"business", http.StatusOK, `{"code":402000,"message":"access_token=secret-token expired"}`, socialhub.CodeUnauthenticated},
		{"missing code", http.StatusOK, `{"data":{}}`, socialhub.CodePlatformError},
		{"missing data", http.StatusOK, `{"code":0}`, socialhub.CodePlatformError},
		{"missing token", http.StatusOK, `{"code":0,"data":{"access_token":"","refresh_token":"refresh","access_token_expires_in":1,"refresh_token_expires_in":1}}`, socialhub.CodePlatformError},
		{"http rate", http.StatusTooManyRequests, `{"code":400001,"message":"slow"}`, socialhub.CodeRateLimited},
		{"json", http.StatusOK, `{`, socialhub.CodePlatformError},
		{"oversized", http.StatusOK, strings.Repeat("x", int(maxOAuthResponseBytes)+1), socialhub.CodePlatformError},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
				writeJSON(writer, test.status, test.body)
			}))
			defer server.Close()
			oauth := &OAuthClient{
				AppID: 1, Secret: "app-secret", BaseURL: server.URL, HTTPClient: server.Client(), Clock: fixedClock{now: testNow},
			}
			_, err := oauth.Refresh(context.Background(), "refresh-secret")
			if err == nil || hubError(t, err).Code != test.code || strings.Contains(err.Error(), "secret-token") ||
				strings.Contains(err.Error(), "app-secret") || strings.Contains(err.Error(), "refresh-secret") {
				t.Fatalf("error=%v", err)
			}
		})
	}
}
