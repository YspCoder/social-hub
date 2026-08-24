package tencentads

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"social-hub/pkg/socialhub"
)

func TestOAuthAuthorizationAndTokenFlows(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/oauth/token" || request.Method != http.MethodGet {
			t.Fatalf("unexpected OAuth request: %s %s", request.Method, request.URL.Path)
		}
		query := request.URL.Query()
		if query.Get("client_id") != "789" || query.Get("client_secret") != "app-secret" || query.Get("access_token") != "" {
			t.Errorf("OAuth query=%v", query)
		}
		switch query.Get("grant_type") {
		case "authorization_code":
			if query.Get("authorization_code") != "auth-code" || query.Get("redirect_uri") != "https://app.example/callback" {
				t.Errorf("exchange query=%v", query)
			}
			writeJSON(writer, http.StatusOK, `{"code":0,"data":{"authorizer_info":{"account_id":123456,"scope_list":["ADS_MANAGEMENT"]},"access_token":"user-token","refresh_token":"refresh-1","access_token_expires_in":3600,"refresh_token_expires_in":7200}}`)
		case "refresh_token":
			if query.Get("refresh_token") != "refresh-1" {
				t.Errorf("refresh query=%v", query)
			}
			writeJSON(writer, http.StatusOK, `{"code":0,"data":{"authorizer_info":{"account_id":123456},"access_token":"user-token-2","refresh_token":"refresh-2","access_token_expires_in":3600,"refresh_token_expires_in":7200}}`)
		default:
			http.Error(writer, "bad grant", http.StatusBadRequest)
		}
	}))
	defer server.Close()
	adapter, _ := newTestAdapter(t, server)
	oauth, err := adapter.OAuth(context.Background(), "ads-primary")
	if err != nil {
		t.Fatal(err)
	}
	authorizationURL, err := oauth.AuthorizationURL(AuthorizationRequest{
		RedirectURI: "https://app.example/callback", State: "state-1", Scope: "ADS_MANAGEMENT",
		AccountType: "ACCOUNT_TYPE_ADVERTISER", AccountDisplayNumber: 123, Fields: []string{"account_id"},
	})
	if err != nil || !strings.Contains(authorizationURL, "/oauth/authorize?") || !strings.Contains(authorizationURL, "client_id=789") ||
		!strings.Contains(authorizationURL, "fields=%5B%22account_id%22%5D") {
		t.Fatalf("authorization URL=%q err=%v", authorizationURL, err)
	}
	exchanged, err := oauth.Exchange(context.Background(), "auth-code", "https://app.example/callback")
	if err != nil || exchanged.Token.AccessToken != "user-token" || exchanged.Token.RefreshToken != "refresh-1" ||
		exchanged.Authorizer.AccountID != testAdvertiserID || !exchanged.Token.ExpiresAt.Equal(testNow.Add(3600*1e9)) {
		t.Fatalf("exchange=%#v err=%v", exchanged, err)
	}
	refreshed, err := oauth.Refresh(context.Background(), "refresh-1")
	if err != nil || refreshed.Token.AccessToken != "user-token-2" || refreshed.RefreshExpiresAt.IsZero() {
		t.Fatalf("refresh=%#v err=%v", refreshed, err)
	}
}

func TestOAuthValidationAndContractErrors(t *testing.T) {
	client := &OAuthClient{}
	if _, err := client.AuthorizationURL(AuthorizationRequest{}); hubError(t, err).Code != socialhub.CodeInvalidArgument {
		t.Fatalf("authorize error=%v", err)
	}
	if _, err := client.Exchange(context.Background(), "", "bad"); hubError(t, err).Code != socialhub.CodeInvalidArgument {
		t.Fatalf("exchange error=%v", err)
	}
	if _, err := client.Refresh(context.Background(), ""); hubError(t, err).Code != socialhub.CodeInvalidArgument {
		t.Fatalf("refresh error=%v", err)
	}
	valid := &OAuthClient{ClientID: 1, AuthorizationBaseURL: "https://developers.e.qq.com"}
	invalidRequests := []AuthorizationRequest{
		{RedirectURI: "https://app.example:8443/callback"},
		{RedirectURI: "https://app.example/callback", AccountType: "bad"},
		{RedirectURI: "https://app.example/callback", Fields: []string{"Bad"}},
	}
	for _, input := range invalidRequests {
		if _, err := valid.AuthorizationURL(input); err == nil {
			t.Fatalf("request should be invalid: %#v", input)
		}
	}

	tests := []struct {
		name string
		body string
	}{
		{"business", `{"code":30102,"message":"access_token=secret expired"}`},
		{"missing code", `{"data":{}}`},
		{"missing data", `{"code":0}`},
		{"missing token", `{"code":0,"data":{"access_token":"","refresh_token":"refresh","access_token_expires_in":1,"refresh_token_expires_in":1}}`},
		{"invalid lifetime", `{"code":0,"data":{"access_token":"token","refresh_token":"refresh","access_token_expires_in":0,"refresh_token_expires_in":1}}`},
		{"invalid account", `{"code":0,"data":{"authorizer_info":{"account_id":-1},"access_token":"token","refresh_token":"refresh","access_token_expires_in":1,"refresh_token_expires_in":1}}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
				writeJSON(writer, http.StatusOK, test.body)
			}))
			defer server.Close()
			oauth := &OAuthClient{
				ClientID: 1, ClientSecret: "secret", AuthorizationBaseURL: server.URL, TokenBaseURL: server.URL,
				HTTPClient: server.Client(), Clock: fixedClock{now: testNow},
			}
			_, err := oauth.Refresh(context.Background(), "refresh")
			if err == nil {
				t.Fatal("expected OAuth error")
			}
			if strings.Contains(err.Error(), "access_token=secret") {
				t.Fatalf("secret leaked in error: %v", err)
			}
		})
	}
}

func TestOAuthBoundedResponseHTTPErrorAndRedirect(t *testing.T) {
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
			oauth := &OAuthClient{
				ClientID: 1, ClientSecret: "client-secret", TokenBaseURL: server.URL,
				HTTPClient: server.Client(), Clock: fixedClock{now: testNow},
			}
			_, err := oauth.Refresh(context.Background(), "refresh-secret")
			if err == nil || hubError(t, err).Code != test.code || strings.Contains(err.Error(), "client-secret") || strings.Contains(err.Error(), "refresh-secret") {
				t.Fatalf("error=%v", err)
			}
		})
	}

	forwarded := false
	target := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { forwarded = true }))
	defer target.Close()
	source := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		http.Redirect(writer, request, target.URL, http.StatusFound)
	}))
	defer source.Close()
	httpClient := *source.Client()
	httpClient.CheckRedirect = rejectRedirect
	oauth := &OAuthClient{
		ClientID: 1, ClientSecret: "client-secret", TokenBaseURL: source.URL,
		HTTPClient: &httpClient, Clock: fixedClock{now: testNow},
	}
	_, err := oauth.Refresh(context.Background(), "refresh-secret")
	if err == nil || forwarded || strings.Contains(err.Error(), "client-secret") || strings.Contains(err.Error(), "refresh-secret") {
		t.Fatalf("err=%v forwarded=%v", err, forwarded)
	}
}
