package kakao

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

func TestOAuthAuthorizationExchangeRefreshAndOptionalSecret(t *testing.T) {
	call := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.URL.Path != "/oauth/token" || request.ParseForm() != nil {
			t.Errorf("request=%s %s", request.Method, request.URL.Path)
			writer.WriteHeader(http.StatusBadRequest)
			return
		}
		call++
		switch call {
		case 1:
			if request.Form.Get("grant_type") != "authorization_code" || request.Form.Get("client_id") != "rest-api-key" ||
				request.Form.Get("client_secret") != "client-secret" || request.Form.Get("code") != "auth-code" ||
				request.Form.Get("redirect_uri") != "https://app.example.test/callback" {
				t.Errorf("exchange form=%v", request.Form)
			}
			writeTestJSON(t, writer, map[string]any{
				"token_type": "bearer", "access_token": "access-1", "expires_in": 21600,
				"refresh_token": "refresh-1", "refresh_token_expires_in": 5184000,
				"scope": "profile_nickname talk_message", "id_token": "id-token",
			})
		case 2:
			if request.Form.Get("grant_type") != "refresh_token" || request.Form.Get("refresh_token") != "refresh-1" || request.Form.Get("client_secret") != "client-secret" {
				t.Errorf("refresh form=%v", request.Form)
			}
			writeTestJSON(t, writer, map[string]any{"token_type": "bearer", "access_token": "access-2", "expires_in": 21600})
		case 3:
			if request.Form.Get("client_secret") != "" || request.Form.Get("grant_type") != "authorization_code" {
				t.Errorf("secret-disabled form=%v", request.Form)
			}
			writeTestJSON(t, writer, map[string]any{
				"token_type": "bearer", "access_token": "access-3", "expires_in": 21600,
				"refresh_token": "refresh-3", "refresh_token_expires_in": 5184000,
			})
		default:
			t.Errorf("unexpected OAuth call %d", call)
		}
	}))
	defer server.Close()
	adapter, _ := newTestClient(t, server, true, true)
	oauth, err := adapter.OAuth(context.Background(), "main")
	if err != nil {
		t.Fatal(err)
	}
	authorizationURL, err := oauth.AuthorizationURL(
		"https://app.example.test/callback", "state-1", []string{"profile_nickname", "talk_message"},
	)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := url.Parse(authorizationURL)
	if err != nil || parsed.Path != "/oauth/authorize" || parsed.Query().Get("response_type") != "code" ||
		parsed.Query().Get("client_id") != "rest-api-key" || parsed.Query().Get("state") != "state-1" ||
		parsed.Query().Get("scope") != "profile_nickname,talk_message" {
		t.Fatalf("authorization URL=%s err=%v", authorizationURL, err)
	}
	exchanged, err := oauth.Exchange(context.Background(), "auth-code", "https://app.example.test/callback")
	if err != nil || exchanged.Token.AccessToken != "access-1" || exchanged.Token.RefreshToken != "refresh-1" ||
		exchanged.Token.TokenType != "Bearer" || !exchanged.Token.ExpiresAt.Equal(testNow.Add(6*time.Hour)) ||
		exchanged.RefreshExpiresAt == nil || !exchanged.RefreshExpiresAt.Equal(testNow.Add(60*24*time.Hour)) || exchanged.IDToken != "id-token" || len(exchanged.Token.Scopes) != 2 {
		t.Fatalf("exchange=%#v err=%v", exchanged, err)
	}
	refreshed, err := oauth.Refresh(context.Background(), "refresh-1")
	if err != nil || refreshed.Token.AccessToken != "access-2" || refreshed.Token.RefreshToken != "refresh-1" || refreshed.RefreshExpiresAt != nil {
		t.Fatalf("refresh=%#v err=%v", refreshed, err)
	}

	config := testConfig(server.URL, true, true)
	config.Accounts[0].SecretRef = ""
	withoutSecret := &Adapter{}
	if err := withoutSecret.Init(context.Background(), config,
		socialhub.WithHTTPClient(server.Client()), socialhub.WithSecretResolver(testResolver{"test://access-token": "access-token"}),
		socialhub.WithClock(fixedClock{now: testNow}),
	); err != nil {
		t.Fatal(err)
	}
	secretDisabledOAuth, err := withoutSecret.OAuth(context.Background(), "main")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := secretDisabledOAuth.Exchange(context.Background(), "auth-code", "https://app.example.test/callback"); err != nil {
		t.Fatal(err)
	}
	if call != 3 {
		t.Fatalf("OAuth calls=%d", call)
	}
}

func TestOAuthValidationAndErrors(t *testing.T) {
	client := OAuthClient{ClientID: "client", ClientSecret: "secret", AuthURL: defaultAuthURL, TokenURL: defaultTokenURL, HTTPClient: http.DefaultClient, Clock: fixedClock{now: testNow}}
	if _, err := client.AuthorizationURL("", "state", nil); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("empty redirect=%v", err)
	}
	if _, err := client.AuthorizationURL("https://app.example.test/callback", "", nil); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("empty state=%v", err)
	}
	if _, err := client.AuthorizationURL("https://app.example.test/callback", "state", []string{"bad scope"}); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("bad scope=%v", err)
	}
	if _, err := client.Exchange(context.Background(), "", "https://app.example.test/callback"); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("empty code=%v", err)
	}
	if _, err := client.Refresh(context.Background(), ""); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("empty refresh=%v", err)
	}
	incomplete := OAuthClient{ClientID: "client", TokenURL: defaultTokenURL}
	if _, err := incomplete.Refresh(context.Background(), "refresh"); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("incomplete client=%v", err)
	}

	tests := []struct {
		name   string
		status int
		body   string
		want   socialhub.ErrorCode
	}{
		{name: "invalid client", status: http.StatusBadRequest, body: `{"error":"invalid_client","error_description":"bad secret"}`, want: socialhub.CodeUnauthenticated},
		{name: "rate", status: http.StatusBadRequest, body: `{"error":"invalid_request","error_code":"KOE237"}`, want: socialhub.CodeRateLimited},
		{name: "server", status: http.StatusServiceUnavailable, body: `{"error":"server_error"}`, want: socialhub.CodeTemporarilyUnavailable},
		{name: "malformed", status: http.StatusOK, body: `{`, want: socialhub.CodePlatformError},
		{name: "missing fields", status: http.StatusOK, body: `{"access_token":"","expires_in":0}`, want: socialhub.CodePlatformError},
		{name: "missing refresh", status: http.StatusOK, body: `{"access_token":"access","expires_in":100}`, want: socialhub.CodePlatformError},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
				writer.WriteHeader(test.status)
				_, _ = writer.Write([]byte(test.body))
			}))
			defer server.Close()
			oauth := OAuthClient{
				ClientID: "client", ClientSecret: "secret", TokenURL: server.URL,
				HTTPClient: server.Client(), Clock: fixedClock{now: testNow},
			}
			_, err := oauth.Exchange(context.Background(), "code", "https://app.example.test/callback")
			if errorCode(err) != test.want {
				t.Fatalf("error=%v code=%s want=%s", err, errorCode(err), test.want)
			}
		})
	}
}
