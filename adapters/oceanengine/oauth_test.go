package oceanengine

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"social-hub/pkg/socialhub"
)

func TestOAuthTokenFlows(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Access-Token") != "" || request.Header.Get("Authorization") != "" {
			t.Error("OAuth request must not carry API auth headers")
		}
		var body map[string]any
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body["app_id"] != float64(789) || body["secret"] != "app-secret" {
			t.Errorf("OAuth body=%v", body)
		}
		switch request.URL.Path {
		case "/open_api/oauth2/access_token/":
			if body["auth_code"] != "auth-code" {
				t.Error("auth code missing")
			}
			writeJSON(writer, http.StatusOK, `{"code":0,"data":{"access_token":"user-token","refresh_token":"refresh-1","advertiser_ids":[123456],"expires_in":3600,"refresh_token_expires_in":7200}}`)
		case "/open_api/oauth2/refresh_token/":
			writeJSON(writer, http.StatusOK, `{"code":0,"data":{"access_token":"user-token-2","refresh_token":"refresh-2","expires_in":3600,"refresh_token_expires_in":7200}}`)
		case "/open_api/oauth2/renew_token/":
			writeJSON(writer, http.StatusOK, `{"code":0,"data":{"access_token":"user-token-3","refresh_token":"refresh-3","expires_in":3600,"refresh_token_expires_in":7200}}`)
		case "/open_api/oauth2/app_access_token/":
			writeJSON(writer, http.StatusOK, `{"code":0,"data":{"access_token":"app-token","expires_in":1800}}`)
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
	exchanged, err := oauth.Exchange(context.Background(), "auth-code")
	if err != nil || exchanged.Token.AccessToken != "user-token" || exchanged.Token.RefreshToken != "refresh-1" || len(exchanged.AdvertiserIDs) != 1 || !exchanged.Token.ExpiresAt.Equal(testNow.Add(3600*1e9)) {
		t.Fatalf("exchange=%#v err=%v", exchanged, err)
	}
	refreshed, err := oauth.Refresh(context.Background(), "refresh-1")
	if err != nil || refreshed.Token.AccessToken != "user-token-2" {
		t.Fatalf("refresh=%#v err=%v", refreshed, err)
	}
	renewed, err := oauth.Renew(context.Background(), "refresh-2")
	if err != nil || renewed.Token.RefreshToken != "refresh-3" {
		t.Fatalf("renew=%#v err=%v", renewed, err)
	}
	appToken, err := oauth.AppToken(context.Background())
	if err != nil || appToken.AccessToken != "app-token" || appToken.TokenType != "OceanEngineApp" {
		t.Fatalf("app token=%#v err=%v", appToken, err)
	}
}

func TestOAuthValidationAndErrors(t *testing.T) {
	client := &OAuthClient{}
	if _, err := client.Exchange(context.Background(), ""); hubError(t, err).Code != socialhub.CodeInvalidArgument {
		t.Fatalf("exchange error=%v", err)
	}
	if _, err := client.Refresh(context.Background(), ""); hubError(t, err).Code != socialhub.CodeInvalidArgument {
		t.Fatalf("refresh error=%v", err)
	}
	if _, err := client.Renew(context.Background(), ""); hubError(t, err).Code != socialhub.CodeInvalidArgument {
		t.Fatalf("renew error=%v", err)
	}
	if _, err := client.AppToken(context.Background()); hubError(t, err).Code != socialhub.CodeInvalidArgument {
		t.Fatalf("app error=%v", err)
	}

	tests := []struct {
		name string
		body string
	}{
		{"business", `{"code":50001,"message":"denied","request_id":"oauth-1","data":null}`},
		{"missing token", `{"code":0,"data":{"access_token":"","refresh_token":"refresh","expires_in":1,"refresh_token_expires_in":1}}`},
		{"invalid lifetime", `{"code":0,"data":{"access_token":"token","refresh_token":"refresh","expires_in":0,"refresh_token_expires_in":1}}`},
		{"invalid advertiser", `{"code":0,"data":{"access_token":"token","refresh_token":"refresh","advertiser_ids":[0],"expires_in":1,"refresh_token_expires_in":1}}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
				writeJSON(writer, http.StatusOK, test.body)
			}))
			defer server.Close()
			oauth := &OAuthClient{AppID: 1, Secret: "secret", BaseURL: server.URL, HTTPClient: server.Client(), Clock: fixedClock{now: testNow}}
			if _, err := oauth.Exchange(context.Background(), "code"); err == nil || hubError(t, err).Code != socialhub.CodePlatformError {
				t.Fatalf("error=%v", err)
			}
		})
	}
}

func TestOAuthBoundedResponseAndHTTPError(t *testing.T) {
	tests := []struct {
		name   string
		status int
		body   string
		code   socialhub.ErrorCode
	}{
		{"oversized", http.StatusOK, strings.Repeat("x", int(maxOAuthResponseBytes)+1), socialhub.CodePlatformError},
		{"http", http.StatusTooManyRequests, `{"code":429,"message":"slow"}`, socialhub.CodeRateLimited},
		{"json", http.StatusOK, `{`, socialhub.CodePlatformError},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
				writeJSON(writer, test.status, test.body)
			}))
			defer server.Close()
			oauth := &OAuthClient{AppID: 1, Secret: "secret", BaseURL: server.URL, HTTPClient: server.Client(), Clock: fixedClock{now: testNow}}
			_, err := oauth.AppToken(context.Background())
			if err == nil || hubError(t, err).Code != test.code {
				t.Fatalf("error=%v", err)
			}
		})
	}
}
