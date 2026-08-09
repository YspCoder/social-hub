package marketing

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"social-hub/pkg/socialhub"
)

func TestOAuthAuthorizationAndLongTermExchange(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/v1.3/oauth2/access_token/" || request.Method != http.MethodPost ||
			request.Header.Get("Access-Token") != "" {
			t.Fatalf("unexpected OAuth request: %s %s", request.Method, request.URL.Path)
		}
		var body map[string]any
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body["app_id"] != "987654" || body["secret"] != "app-secret" || body["auth_code"] != "auth-code" ||
			body["return_advertiser_ids"] != true {
			t.Errorf("OAuth body=%v", body)
		}
		writeJSON(writer, http.StatusOK, `{"code":0,"request_id":"req-1","data":{"access_token":"long-term-token","advertiser_ids":["123456789"],"scope":[1,7]}}`)
	}))
	defer server.Close()
	adapter, _ := newTestAdapter(t, server)
	oauth, err := adapter.OAuth(context.Background(), "ads-primary")
	if err != nil {
		t.Fatal(err)
	}
	authorizationURL, err := oauth.AuthorizationURL(AuthorizationRequest{RedirectURI: "https://app.example/callback", State: "state-1"})
	if err != nil || !strings.Contains(authorizationURL, "/marketing_api/auth?") ||
		!strings.Contains(authorizationURL, "app_id=987654") || !strings.Contains(authorizationURL, "state=state-1") {
		t.Fatalf("authorization URL=%q err=%v", authorizationURL, err)
	}
	token, err := oauth.Exchange(context.Background(), "auth-code")
	if err != nil || token.Token.AccessToken != "long-term-token" || token.Token.RefreshToken != "" ||
		!token.Token.ExpiresAt.IsZero() || len(token.AdvertiserIDs) != 1 || len(token.ScopeIDs) != 2 {
		t.Fatalf("token=%#v err=%v", token, err)
	}
	if _, found := reflect.TypeOf(oauth).MethodByName("Refresh"); found {
		t.Fatal("Marketing long-term OAuth must not expose refresh semantics")
	}
}

func TestOAuthValidationWrongFlowAndRedirectProtection(t *testing.T) {
	client := &OAuthClient{}
	if _, err := client.AuthorizationURL(AuthorizationRequest{}); hubError(t, err).Code != socialhub.CodeInvalidArgument {
		t.Fatalf("authorize error=%v", err)
	}
	if _, err := client.Exchange(context.Background(), ""); hubError(t, err).Code != socialhub.CodeInvalidArgument {
		t.Fatalf("exchange error=%v", err)
	}
	valid := &OAuthClient{AppID: "1", AuthorizationBaseURL: "https://ads.tiktok.com"}
	for _, input := range []AuthorizationRequest{{RedirectURI: "relative"}, {RedirectURI: "https://user@example.com/callback"}} {
		if _, err := valid.AuthorizationURL(input); err == nil {
			t.Fatalf("request should be invalid: %#v", input)
		}
	}

	shortFlow := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writeJSON(writer, http.StatusOK, `{"code":0,"data":{"access_token":"short","expires_in":86400,"refresh_token":"refresh"}}`)
	}))
	defer shortFlow.Close()
	wrong := &OAuthClient{AppID: "1", Secret: "secret", BaseURL: shortFlow.URL, HTTPClient: shortFlow.Client()}
	if _, err := wrong.Exchange(context.Background(), "code"); hubError(t, err).Code != socialhub.CodePlatformError {
		t.Fatalf("wrong-flow error=%v", err)
	}

	forwarded := false
	target := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, request *http.Request) {
		forwarded = true
		if request.Header.Get("Access-Token") != "" {
			t.Error("credential was forwarded")
		}
	}))
	defer target.Close()
	source := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		http.Redirect(writer, request, target.URL, http.StatusTemporaryRedirect)
	}))
	defer source.Close()
	adapter, _ := newTestAdapter(t, source)
	oauth, err := adapter.OAuth(context.Background(), "ads-primary")
	if err != nil {
		t.Fatal(err)
	}
	_, err = oauth.Exchange(context.Background(), "code")
	if err == nil || forwarded || strings.Contains(strings.ToLower(err.Error()), "app-secret") {
		t.Fatalf("err=%v forwarded=%v", err, forwarded)
	}
}
