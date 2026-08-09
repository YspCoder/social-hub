package toutiao

import (
	"context"
	"errors"
	"fmt"
	"mime"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

func TestOAuthAuthorizationExchangeRefreshAndClientToken(t *testing.T) {
	t.Parallel()
	apiServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		t.Fatalf("OAuth request reached business API: %s", request.URL)
	}))
	defer apiServer.Close()
	oauthServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.Header.Get("Accept") != "application/json" {
			writer.WriteHeader(http.StatusBadRequest)
			return
		}
		mediaType, _, err := mime.ParseMediaType(request.Header.Get("Content-Type"))
		if err != nil {
			writer.WriteHeader(http.StatusBadRequest)
			return
		}
		if mediaType == "multipart/form-data" {
			_ = request.ParseMultipartForm(1 << 20)
		} else {
			_ = request.ParseForm()
		}
		if request.Form.Get("client_key") != "client-key" {
			writer.WriteHeader(http.StatusBadRequest)
			return
		}
		switch request.URL.Path {
		case "/oauth/access_token/":
			if mediaType != "application/x-www-form-urlencoded" || request.Form.Get("client_secret") != "client-secret" || request.Form.Get("code") != "code-1" || request.Form.Get("grant_type") != "authorization_code" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			_, _ = writer.Write([]byte(`{"data":{"access_token":"user-token-1","refresh_token":"refresh-1","open_id":"open-id-1","expires_in":"1296000","refresh_expires_in":"2592000","scope":"user_info,toutiao.video.data","error_code":"0"}}`))
		case "/oauth/refresh_token/":
			if mediaType != "multipart/form-data" || request.Form.Get("refresh_token") != "refresh-1" || request.Form.Get("grant_type") != "refresh_token" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			_, _ = writer.Write([]byte(`{"data":{"access_token":"user-token-2","refresh_token":"refresh-2","open_id":"open-id-1","expires_in":1296000,"refresh_expires_in":2592000,"scope":"user_info","error_code":0}}`))
		case "/oauth/client_token/":
			if mediaType != "multipart/form-data" || request.Form.Get("client_secret") != "client-secret" || request.Form.Get("grant_type") != "client_credential" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			_, _ = writer.Write([]byte(`{"data":{"access_token":"client-token-1","expires_in":"7200","error_code":"0"}}`))
		default:
			writer.WriteHeader(http.StatusNotFound)
		}
	}))
	defer oauthServer.Close()
	adapter, _ := newTestAdapter(t, apiServer, oauthServer)
	oauth, err := adapter.OAuth(context.Background(), "primary")
	if err != nil {
		t.Fatal(err)
	}
	authorizationURL, err := oauth.AuthorizationURL("https://app.example/callback?tenant=1", "state-1", []string{"user_info", "toutiao.video.data"})
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := url.Parse(authorizationURL)
	if err != nil || parsed.Host != strings.TrimPrefix(oauthServer.URL, "http://") || parsed.Path != "/oauth/authorize/" || parsed.Query().Get("client_key") != "client-key" || parsed.Query().Get("response_type") != "code" || parsed.Query().Get("scope") != "user_info,toutiao.video.data" || parsed.Query().Get("state") != "state-1" {
		t.Fatalf("authorization URL=%q err=%v", authorizationURL, err)
	}
	userToken, err := oauth.Exchange(context.Background(), "code-1")
	if err != nil || userToken.OpenID != "open-id-1" || userToken.Token.AccessToken != "user-token-1" || userToken.Token.TokenType != "ToutiaoUser" || len(userToken.Token.Scopes) != 2 {
		t.Fatalf("user token=%#v err=%v", userToken, err)
	}
	if !userToken.Token.ExpiresAt.Equal(testNow.Add(15*24*time.Hour)) || !userToken.RefreshExpiresAt.Equal(testNow.Add(30*24*time.Hour)) {
		t.Fatalf("token expiry=%s refresh expiry=%s", userToken.Token.ExpiresAt, userToken.RefreshExpiresAt)
	}
	refreshed, err := oauth.Refresh(context.Background(), "refresh-1")
	if err != nil || refreshed.Token.AccessToken != "user-token-2" || refreshed.Token.RefreshToken != "refresh-2" {
		t.Fatalf("refreshed=%#v err=%v", refreshed, err)
	}
	clientToken, err := oauth.ClientToken(context.Background())
	if err != nil || clientToken.AccessToken != "client-token-1" || clientToken.TokenType != "ToutiaoClient" || !clientToken.ExpiresAt.Equal(testNow.Add(2*time.Hour)) {
		t.Fatalf("client token=%#v err=%v", clientToken, err)
	}
}

func TestOAuthValidationAndResponseLimits(t *testing.T) {
	t.Parallel()
	clock := fixedClock{now: testNow}
	valid := &OAuthClient{ClientKey: "key", ClientSecret: "secret", AuthURL: "https://example.com/oauth/authorize/", TokenBaseURL: "https://example.com", HTTPClient: http.DefaultClient, Clock: clock}
	for _, test := range []struct {
		name string
		call func() error
	}{
		{"bad redirect", func() error {
			_, err := valid.AuthorizationURL("javascript:alert(1)", "state", []string{"user_info"})
			return err
		}},
		{"empty state", func() error {
			_, err := valid.AuthorizationURL("https://app.example/callback", "", []string{"user_info"})
			return err
		}},
		{"bad scopes", func() error {
			_, err := valid.AuthorizationURL("https://app.example/callback", "state", []string{"bad scope"})
			return err
		}},
		{"empty code", func() error { _, err := valid.Exchange(context.Background(), ""); return err }},
		{"empty refresh", func() error { _, err := valid.Refresh(context.Background(), ""); return err }},
		{"incomplete client", func() error { _, err := (&OAuthClient{}).ClientToken(context.Background()); return err }},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := test.call(); !errors.Is(err, socialhub.ErrInvalidArgument) {
				t.Fatalf("error=%v", err)
			}
		})
	}

	t.Run("oversized", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
			_, _ = writer.Write([]byte(strings.Repeat("x", int(maxOAuthResponseBytes)+1)))
		}))
		defer server.Close()
		client := &OAuthClient{ClientKey: "key", ClientSecret: "secret", TokenBaseURL: server.URL, HTTPClient: server.Client(), Clock: clock}
		_, err := client.ClientToken(context.Background())
		var platformErr *socialhub.Error
		if !errors.As(err, &platformErr) || platformErr.Code != socialhub.CodePlatformError || platformErr.PlatformMessage != "response exceeded size limit" {
			t.Fatalf("error=%#v", err)
		}
	})

	t.Run("redirect refused", func(t *testing.T) {
		target := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
			t.Fatal("redirect target must not be reached")
		}))
		defer target.Close()
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
			http.Redirect(writer, &http.Request{}, target.URL, http.StatusFound)
		}))
		defer server.Close()
		client := &OAuthClient{ClientKey: "key", ClientSecret: "secret", TokenBaseURL: server.URL, HTTPClient: &http.Client{CheckRedirect: rejectRedirect}, Clock: clock}
		_, err := client.ClientToken(context.Background())
		var platformErr *socialhub.Error
		if !errors.As(err, &platformErr) || platformErr.HTTPStatus != http.StatusFound {
			t.Fatalf("error=%#v", err)
		}
	})
}

func TestOAuthPlatformAndMalformedResponses(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		body string
		want error
	}{
		{"provider error", `{"data":{"error_code":10004,"description":"scope approval required"},"extra":{"logid":"request-1"}}`, socialhub.ErrApprovalRequired},
		{"missing token", `{"data":{"error_code":0,"expires_in":7200}}`, nil},
		{"malformed", `{`, nil},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
				_, _ = fmt.Fprint(writer, test.body)
			}))
			defer server.Close()
			client := &OAuthClient{ClientKey: "key", ClientSecret: "secret", TokenBaseURL: server.URL, HTTPClient: server.Client(), Clock: fixedClock{now: testNow}}
			_, err := client.ClientToken(context.Background())
			if test.want != nil && !errors.Is(err, test.want) {
				t.Fatalf("error=%v want=%v", err, test.want)
			}
			if test.want == nil && err == nil {
				t.Fatal("expected response validation error")
			}
		})
	}
}

func TestOAuthRejectsInvalidUserTokenFields(t *testing.T) {
	t.Parallel()
	tests := []string{
		`{"data":{"access_token":"token","refresh_token":"refresh","open_id":"open","expires_in":1,"refresh_expires_in":0,"scope":"user_info","error_code":0}}`,
		`{"data":{"access_token":"token","open_id":"open","expires_in":1,"scope":"bad scope!","error_code":0}}`,
	}
	for _, body := range tests {
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
			_, _ = writer.Write([]byte(body))
		}))
		client := &OAuthClient{ClientKey: "key", ClientSecret: "secret", TokenBaseURL: server.URL, HTTPClient: server.Client(), Clock: fixedClock{now: testNow}}
		_, err := client.Exchange(context.Background(), "code")
		server.Close()
		if !hasCode(err, socialhub.CodePlatformError) {
			t.Fatalf("body=%s error=%v", body, err)
		}
	}
}
