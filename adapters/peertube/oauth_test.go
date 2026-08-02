package peertube

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

const (
	testClientID     = "v1ikx5hnfop4mdpnci8nsqh93c45rldf"
	testClientSecret = "AjWiOapPltI6EnsWQwlFarRtLh4u8tDt"
)

func TestOAuthDiscoverPasswordAndRefresh(t *testing.T) {
	var passwordCalls, refreshCalls int
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/api/v1/oauth-clients/local":
			if request.Method != http.MethodGet || request.Header.Get("Accept") != "application/json" {
				t.Errorf("discover request=%s headers=%v", request.Method, request.Header)
			}
			writeJSON(writer, http.StatusOK, `{"client_id":"`+testClientID+`","client_secret":"`+testClientSecret+`"}`)
		case "/api/v1/users/token":
			if request.Method != http.MethodPost || request.Header.Get("Content-Type") != "application/x-www-form-urlencoded" {
				t.Errorf("token request=%s headers=%v", request.Method, request.Header)
			}
			if err := request.ParseForm(); err != nil {
				t.Fatal(err)
			}
			if request.Form.Get("client_id") != testClientID || request.Form.Get("client_secret") != testClientSecret {
				t.Errorf("client form=%v", request.Form)
			}
			switch request.Form.Get("grant_type") {
			case "password":
				passwordCalls++
				if request.Form.Get("username") != "alice" || request.Form.Get("password") != "correct horse" || request.Header.Get("X-PeerTube-OTP") != "123456" {
					t.Errorf("password request form=%v otp=%q", request.Form, request.Header.Get("X-PeerTube-OTP"))
				}
				writeJSON(writer, http.StatusOK, `{"token_type":"Bearer","access_token":"access-1","refresh_token":"refresh-1","expires_in":86400,"refresh_token_expires_in":1209600}`)
			case "refresh_token":
				refreshCalls++
				if request.Form.Get("refresh_token") != "refresh-1" || request.Header.Get("X-PeerTube-OTP") != "" {
					t.Errorf("refresh request form=%v", request.Form)
				}
				writeJSON(writer, http.StatusOK, `{"token_type":"bearer","access_token":"access-2","refresh_token":"refresh-2","expires_in":3600}`)
			default:
				http.Error(writer, "bad grant", http.StatusBadRequest)
			}
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	oauth := &OAuthClient{InstanceURL: server.URL, HTTPClient: server.Client(), Clock: fixedClock{testNow}}
	local, err := oauth.Discover(context.Background())
	if err != nil || local.ClientID != testClientID || local.ClientSecret != testClientSecret {
		t.Fatalf("local=%#v err=%v", local, err)
	}
	token, err := oauth.Password(context.Background(), local, "alice", "correct horse", "123456")
	if err != nil || token.AccessToken != "access-1" || token.RefreshToken != "refresh-1" || !token.ExpiresAt.Equal(testNow.Add(24*time.Hour)) {
		t.Fatalf("password token=%#v err=%v", token, err)
	}
	refreshed, err := oauth.Refresh(context.Background(), local, token.RefreshToken)
	if err != nil || refreshed.AccessToken != "access-2" || refreshed.TokenType != "Bearer" || !refreshed.ExpiresAt.Equal(testNow.Add(time.Hour)) {
		t.Fatalf("refresh token=%#v err=%v", refreshed, err)
	}
	if passwordCalls != 1 || refreshCalls != 1 {
		t.Fatalf("calls password=%d refresh=%d", passwordCalls, refreshCalls)
	}
}

func TestOAuthValidationAndErrors(t *testing.T) {
	validClient := OAuthLocalClient{ClientID: testClientID, ClientSecret: testClientSecret}
	tests := []struct {
		name string
		call func(*OAuthClient) error
	}{
		{"discover config", func(o *OAuthClient) error { _, err := o.Discover(context.Background()); return err }},
		{"password fields", func(o *OAuthClient) error {
			_, err := o.Password(context.Background(), validClient, "", "", "")
			return err
		}},
		{"password client", func(o *OAuthClient) error {
			_, err := o.Password(context.Background(), OAuthLocalClient{}, "alice", "password", "")
			return err
		}},
		{"refresh field", func(o *OAuthClient) error { _, err := o.Refresh(context.Background(), validClient, ""); return err }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			oauth := &OAuthClient{}
			if test.name != "discover config" {
				oauth = &OAuthClient{InstanceURL: "https://video.example", HTTPClient: http.DefaultClient, Clock: fixedClock{testNow}}
			}
			if code := errorCode(test.call(oauth)); code != socialhub.CodeInvalidArgument {
				t.Fatalf("code=%s", code)
			}
		})
	}

	t.Run("problem details", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
			writer.Header().Set("Retry-After", "9")
			writeJSON(writer, http.StatusUnauthorized, `{"type":"about:blank","code":"missing_two_factor","detail":"OTP required"}`)
		}))
		defer server.Close()
		oauth := &OAuthClient{InstanceURL: server.URL, HTTPClient: server.Client(), Clock: fixedClock{testNow}}
		_, err := oauth.Password(context.Background(), validClient, "alice", "password", "")
		if errorCode(err) != socialhub.CodeUnauthenticated {
			t.Fatalf("error=%v", err)
		}
	})

	t.Run("invalid discovery", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
			writeJSON(writer, http.StatusOK, `{"client_id":"","client_secret":"bad"}`)
		}))
		defer server.Close()
		_, err := (&OAuthClient{InstanceURL: server.URL, HTTPClient: server.Client(), Clock: fixedClock{testNow}}).Discover(context.Background())
		if errorCode(err) != socialhub.CodePlatformError {
			t.Fatalf("error=%v", err)
		}
	})

	t.Run("invalid token", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
			writeJSON(writer, http.StatusOK, `{"access_token":"","expires_in":0}`)
		}))
		defer server.Close()
		_, err := (&OAuthClient{InstanceURL: server.URL, HTTPClient: server.Client(), Clock: fixedClock{testNow}}).Refresh(context.Background(), validClient, "refresh")
		if errorCode(err) != socialhub.CodePlatformError {
			t.Fatalf("error=%v", err)
		}
	})

	if validOAuthCredential(" bad") || validOAuthCredential("") || validOAuthCredential(strings.Repeat("x", 129)) || !validOAuthCredential("good") {
		t.Fatal("OAuth credential validation is wrong")
	}
}

func writeJSON(writer http.ResponseWriter, status int, value string) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	_, _ = writer.Write([]byte(value))
}
