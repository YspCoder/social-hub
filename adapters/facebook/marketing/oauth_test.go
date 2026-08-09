package marketing

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

func TestOAuthAuthorizationAndTokenExchange(t *testing.T) {
	t.Parallel()
	var grants []string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_ = request.ParseForm()
		if request.PostForm.Get("client_secret") != "app-secret" {
			writeJSON(writer, http.StatusUnauthorized, `{"error":{"code":190,"message":"bad secret"}}`)
			return
		}
		grants = append(grants, request.PostForm.Get("grant_type"))
		writeJSON(writer, http.StatusOK, `{"access_token":"long-token","token_type":"bearer","expires_in":3600}`)
	}))
	defer server.Close()
	client := OAuthClient{
		ClientID: "app-id", ClientSecret: "app-secret", AuthURL: server.URL + "/dialog/oauth",
		TokenURL: server.URL, HTTPClient: server.Client(), Clock: fixedClock{now: testNow},
	}
	authorizationURL, err := client.AuthorizationURL("https://app.example/callback", "state", []string{managementScope, "business_management"})
	if err != nil {
		t.Fatal(err)
	}
	parsed, _ := url.Parse(authorizationURL)
	if parsed.Query().Get("state") != "state" || parsed.Query().Get("scope") != "ads_management,business_management" {
		t.Fatalf("authorization query=%v", parsed.Query())
	}
	token, err := client.Exchange(context.Background(), "code", "https://app.example/callback")
	if err != nil || token.AccessToken != "long-token" || token.ExpiresAt != testNow.Add(time.Hour) {
		t.Fatalf("token=%#v err=%v", token, err)
	}
	token, err = client.ExchangeLongLived(context.Background(), "short-token")
	if err != nil || token.AccessToken != "long-token" || len(grants) != 2 || grants[1] != "fb_exchange_token" {
		t.Fatalf("long token=%#v grants=%v err=%v", token, grants, err)
	}
}

func TestOAuthValidationErrorsAndBounds(t *testing.T) {
	t.Parallel()
	client := OAuthClient{}
	if _, err := client.AuthorizationURL("bad", "", nil); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("authorization error=%v", err)
	}
	if _, err := client.Exchange(context.Background(), "", "bad"); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("exchange error=%v", err)
	}
	if _, err := client.ExchangeLongLived(context.Background(), ""); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("long-lived error=%v", err)
	}

	errorServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writeJSON(writer, http.StatusBadRequest, `{"error":{"code":190,"message":"expired"}}`)
	}))
	defer errorServer.Close()
	client = OAuthClient{ClientID: "app", ClientSecret: "secret", TokenURL: errorServer.URL, HTTPClient: errorServer.Client(), Clock: fixedClock{now: testNow}}
	if _, err := client.ExchangeLongLived(context.Background(), "short"); !errors.Is(err, socialhub.ErrUnauthenticated) {
		t.Fatalf("platform OAuth error=%v", err)
	}

	largeServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusOK)
		_, _ = writer.Write([]byte(strings.Repeat("x", int(maxOAuthResponseBytes)+1)))
	}))
	defer largeServer.Close()
	client.TokenURL, client.HTTPClient = largeServer.URL, largeServer.Client()
	if _, err := client.ExchangeLongLived(context.Background(), "short"); hubErrorCode(err) != socialhub.CodePlatformError {
		t.Fatalf("oversized error=%v", err)
	}
}

func TestOAuthTokenWithoutExpiry(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writeJSON(writer, http.StatusOK, `{"access_token":"externally-managed"}`)
	}))
	defer server.Close()
	client := OAuthClient{
		ClientID: "app", ClientSecret: "secret", TokenURL: server.URL,
		HTTPClient: server.Client(), Clock: fixedClock{now: testNow},
	}
	token, err := client.ExchangeLongLived(context.Background(), "short")
	if err != nil || token.TokenType != "Bearer" || !token.ExpiresAt.IsZero() {
		t.Fatalf("token=%#v err=%v", token, err)
	}
}
