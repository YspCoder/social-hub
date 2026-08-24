package ads

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dghubble/oauth1"

	"social-hub/pkg/socialhub"
)

type fixedNoncer string

func (nonce fixedNoncer) Nonce() string { return string(nonce) }

func TestOAuthThreeLeggedFlow(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		authorization := request.Header.Get("Authorization")
		if request.Method != http.MethodPost || !strings.HasPrefix(authorization, "OAuth ") || strings.Contains(authorization, "consumer-secret") {
			t.Fatalf("request=%s %s Authorization=%q", request.Method, request.URL, authorization)
		}
		switch request.URL.Path {
		case "/oauth/request_token":
			if !strings.Contains(authorization, "oauth_callback=") {
				t.Fatalf("request-token Authorization=%q", authorization)
			}
			writer.Header().Set("Content-Type", "application/x-www-form-urlencoded")
			_, _ = writer.Write([]byte("oauth_token=request-token&oauth_token_secret=request-secret&oauth_callback_confirmed=true"))
		case "/oauth/access_token":
			if !strings.Contains(authorization, `oauth_token="request-token"`) || !strings.Contains(authorization, `oauth_verifier="verifier"`) {
				t.Fatalf("access-token Authorization=%q", authorization)
			}
			writer.Header().Set("Content-Type", "application/x-www-form-urlencoded")
			_, _ = writer.Write([]byte("oauth_token=access-token&oauth_token_secret=access-secret&user_id=2417045708&screen_name=brand"))
		default:
			t.Fatalf("unexpected OAuth request: %s", request.URL)
		}
	}))
	defer server.Close()
	adapter, _ := newTestAdapter(t, server)
	client, err := adapter.OAuth(context.Background(), "paid-social")
	if err != nil {
		t.Fatal(err)
	}
	client.noncer = fixedNoncer("nonce")
	temporary, err := client.BeginAuthorization(context.Background(), "https://app.example.test/callback")
	if err != nil || temporary.Token != "request-token" || temporary.Secret != "request-secret" ||
		!strings.Contains(temporary.AuthorizationURL, "oauth_token=request-token") {
		t.Fatalf("temporary=%#v err=%v", temporary, err)
	}
	access, err := client.Exchange(context.Background(), *temporary, "verifier")
	if err != nil || access.Token != "access-token" || access.Secret != "access-secret" || access.UserID != "2417045708" || access.ScreenName != "brand" {
		t.Fatalf("access=%#v err=%v", access, err)
	}
}

func TestOAuthValidationAndErrors(t *testing.T) {
	client := &OAuthClient{}
	if _, err := client.BeginAuthorization(context.Background(), "bad"); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("begin error=%v", err)
	}
	if _, err := client.Exchange(context.Background(), OAuthRequestToken{}, ""); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("exchange error=%v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/x-www-form-urlencoded")
		writer.Header().Set("x-transaction-id", "oauth-request-id")
		writer.WriteHeader(http.StatusUnauthorized)
		_, _ = writer.Write([]byte("oauth_problem=token_rejected&oauth_problem_advice=authorize%20again"))
	}))
	defer server.Close()
	client = &OAuthClient{
		ConsumerKey: "consumer-key", ConsumerSecret: "consumer-secret", RequestTokenURL: server.URL,
		AuthorizeURL: server.URL, AccessTokenURL: server.URL, HTTPClient: server.Client(), noncer: oauth1.Base64Noncer{},
	}
	_, err := client.BeginAuthorization(context.Background(), "https://app.example.test/callback")
	hub := hubError(t, err)
	if !errors.Is(err, socialhub.ErrUnauthenticated) || hub.PlatformCode != "token_rejected" || hub.RequestID != "oauth-request-id" {
		t.Fatalf("OAuth error=%#v", hub)
	}
}
