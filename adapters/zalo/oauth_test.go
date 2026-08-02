package zalo

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

func TestOAuthExchangeAndRotatingRefresh(t *testing.T) {
	call := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.URL.Path != "/v4/oa/access_token" || request.Header.Get("secret_key") != "app-secret" || request.ParseForm() != nil {
			t.Errorf("request=%s %s headers=%v", request.Method, request.URL.Path, request.Header)
			writer.WriteHeader(http.StatusBadRequest)
			return
		}
		call++
		if request.Form.Get("app_id") != "360846524940903967" {
			t.Errorf("app_id=%q", request.Form.Get("app_id"))
		}
		switch call {
		case 1:
			if request.Form.Get("grant_type") != "authorization_code" || request.Form.Get("code") != "authorization-code" ||
				request.Form.Get("code_verifier") != "Abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG" {
				t.Errorf("exchange form=%v", request.Form)
			}
			writeTestJSON(t, writer, map[string]any{"access_token": "access-1", "refresh_token": "refresh-1", "expires_in": "90000"})
		case 2:
			if request.Form.Get("grant_type") != "refresh_token" || request.Form.Get("refresh_token") != "refresh-1" {
				t.Errorf("refresh form=%v", request.Form)
			}
			writeTestJSON(t, writer, map[string]any{"access_token": "access-2", "refresh_token": "refresh-2", "expires_in": 90000})
		default:
			t.Errorf("unexpected OAuth call %d", call)
		}
	}))
	defer server.Close()
	adapter, _ := newTestClient(t, server, true, false)
	oauth, err := adapter.OAuth(context.Background(), "main")
	if err != nil {
		t.Fatal(err)
	}
	exchanged, err := oauth.Exchange(context.Background(), "authorization-code", "Abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG")
	if err != nil || exchanged.AccessToken != "access-1" || exchanged.RefreshToken != "refresh-1" ||
		!exchanged.ExpiresAt.Equal(testNow.Add(25*time.Hour)) {
		t.Fatalf("exchange=%#v err=%v", exchanged, err)
	}
	refreshed, err := oauth.Refresh(context.Background(), "refresh-1")
	if err != nil || refreshed.AccessToken != "access-2" || refreshed.RefreshToken != "refresh-2" ||
		!refreshed.ExpiresAt.Equal(testNow.Add(25*time.Hour)) {
		t.Fatalf("refresh=%#v err=%v", refreshed, err)
	}
	if call != 2 {
		t.Fatalf("calls=%d", call)
	}
}

func TestOAuthValidationAndErrors(t *testing.T) {
	client := OAuthClient{
		AppID: "360846524940903967", AppSecret: "secret", BaseURL: defaultOAuthBaseURL,
		HTTPClient: http.DefaultClient, Clock: fixedClock{now: testNow},
	}
	if _, err := client.Exchange(context.Background(), "", "Abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG"); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("missing code=%v", err)
	}
	if _, err := client.Exchange(context.Background(), "code", "short"); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("bad verifier=%v", err)
	}
	if _, err := client.Refresh(context.Background(), ""); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("missing refresh=%v", err)
	}
	incomplete := OAuthClient{AppID: "1"}
	if _, err := incomplete.Refresh(context.Background(), "refresh"); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("incomplete=%v", err)
	}

	tests := []struct {
		name   string
		status int
		body   string
		want   socialhub.ErrorCode
	}{
		{name: "unauthorized", status: http.StatusUnauthorized, body: `{"error":"invalid_client","error_description":"bad secret"}`, want: socialhub.CodeUnauthenticated},
		{name: "rate", status: http.StatusTooManyRequests, body: `{"error":"rate_limited"}`, want: socialhub.CodeRateLimited},
		{name: "server", status: http.StatusServiceUnavailable, body: `{}`, want: socialhub.CodeTemporarilyUnavailable},
		{name: "api envelope", status: http.StatusOK, body: `{"error":-216,"message":"invalid token"}`, want: socialhub.CodeUnauthenticated},
		{name: "malformed", status: http.StatusOK, body: `{`, want: socialhub.CodePlatformError},
		{name: "missing refresh", status: http.StatusOK, body: `{"access_token":"access","expires_in":"90000"}`, want: socialhub.CodePlatformError},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
				writer.WriteHeader(test.status)
				_, _ = writer.Write([]byte(test.body))
			}))
			defer server.Close()
			oauth := OAuthClient{
				AppID: "360846524940903967", AppSecret: "secret", BaseURL: server.URL,
				HTTPClient: server.Client(), Clock: fixedClock{now: testNow},
			}
			_, err := oauth.Refresh(context.Background(), "refresh")
			if errorCode(err) != test.want {
				t.Fatalf("error=%v code=%s want=%s", err, errorCode(err), test.want)
			}
		})
	}
}
