package imgur

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

func TestOAuthAuthorizationAndRefresh(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/oauth2/token" || request.Method != http.MethodPost || request.Header.Get("Content-Type") != "application/x-www-form-urlencoded" || request.ParseForm() != nil {
			writeJSON(writer, http.StatusBadRequest, `{"error":"invalid_request"}`)
			return
		}
		if request.Form.Get("grant_type") != "refresh_token" || request.Form.Get("refresh_token") != "refresh-token" || request.Form.Get("client_id") != "client-id" || request.Form.Get("client_secret") != "client-secret" {
			writeJSON(writer, http.StatusBadRequest, `{"error":"invalid_request"}`)
			return
		}
		writeJSON(writer, http.StatusOK, `{"access_token":"renewed","refresh_token":"rotated","token_type":"bearer","expires_in":3600}`)
	}))
	defer server.Close()
	adapter, _ := newTestClient(t, server, true)
	oauth, err := adapter.OAuth(context.Background(), "main")
	if err != nil {
		t.Fatal(err)
	}
	authorizationURL, err := oauth.AuthorizationURL("state-value")
	if err != nil {
		t.Fatal(err)
	}
	parsed, _ := url.Parse(authorizationURL)
	if parsed.Path != "/oauth2/authorize" || parsed.Query().Get("client_id") != "client-id" || parsed.Query().Get("response_type") != "token" || parsed.Query().Get("state") != "state-value" {
		t.Fatalf("authorization URL=%s", authorizationURL)
	}
	token, err := oauth.Refresh(context.Background(), "refresh-token")
	if err != nil || token.AccessToken != "renewed" || token.RefreshToken != "rotated" || token.TokenType != "Bearer" || !token.ExpiresAt.Equal(testNow.Add(time.Hour)) {
		t.Fatalf("token=%#v err=%v", token, err)
	}
}

func TestOAuthErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.Header.Get("X-Test") {
		default:
			writeJSON(writer, http.StatusUnauthorized, `{"error":"invalid_grant","error_description":"expired"}`)
		}
	}))
	defer server.Close()
	client := &OAuthClient{ClientID: "client-id", ClientSecret: "secret", AuthURL: server.URL + "/authorize", TokenURL: server.URL, HTTPClient: server.Client(), Clock: fixedClock{testNow}}
	if _, err := client.AuthorizationURL(""); errorCode(err) != socialhub.CodeInvalidArgument {
		t.Fatalf("empty state=%v", err)
	}
	if _, err := client.Refresh(context.Background(), ""); errorCode(err) != socialhub.CodeInvalidArgument {
		t.Fatalf("empty refresh token=%v", err)
	}
	if _, err := client.Refresh(context.Background(), "refresh"); errorCode(err) != socialhub.CodeUnauthenticated {
		t.Fatalf("invalid grant=%v", err)
	}

	badRequest := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writeJSON(writer, http.StatusBadRequest, `{"error":"invalid_request","error_description":"missing"}`)
	}))
	defer badRequest.Close()
	client.TokenURL, client.HTTPClient = badRequest.URL, badRequest.Client()
	if _, err := client.Refresh(context.Background(), "refresh"); errorCode(err) != socialhub.CodeInvalidArgument {
		t.Fatalf("invalid request=%v", err)
	}

	invalidJSON := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writeJSON(writer, http.StatusOK, `{`)
	}))
	defer invalidJSON.Close()
	client.TokenURL, client.HTTPClient = invalidJSON.URL, invalidJSON.Client()
	if _, err := client.Refresh(context.Background(), "refresh"); errorCode(err) != socialhub.CodePlatformError {
		t.Fatalf("invalid JSON=%v", err)
	}

	client.HTTPClient = &http.Client{Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) {
		return nil, &url.Error{Op: "Post", URL: "https://api.imgur.com", Err: errors.New("network down")}
	})}
	client.TokenURL = "https://api.imgur.com/oauth2/token"
	if _, err := client.Refresh(context.Background(), "refresh"); errorCode(err) != socialhub.CodeTemporarilyUnavailable || strings.Contains(err.Error(), "access_token") {
		t.Fatalf("transport error=%v", err)
	}

	incomplete := &OAuthClient{}
	if _, err := incomplete.Refresh(context.Background(), "refresh"); errorCode(err) != socialhub.CodeInvalidArgument {
		t.Fatalf("incomplete OAuth=%v", err)
	}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (function roundTripperFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}
