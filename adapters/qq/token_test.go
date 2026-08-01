package qq

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

type mutableClock struct{ now time.Time }

func (clock *mutableClock) Now() time.Time { return clock.now }

func TestAppTokenSourceCacheStoreRefreshAndInvalidate(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.URL.Path != "/app/getAppAccessToken" || request.Header.Get("Content-Type") != "application/json" {
			http.Error(writer, "bad token request", http.StatusBadRequest)
			return
		}
		var payload map[string]string
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil || payload["appId"] != "102012345" || payload["clientSecret"] != "app-secret" {
			http.Error(writer, "bad token body", http.StatusBadRequest)
			return
		}
		call := calls.Add(1)
		writeTestJSON(t, writer, map[string]any{"access_token": "token-" + strconv.Itoa(int(call)), "expires_in": "120"})
	}))
	defer server.Close()
	clock := &mutableClock{now: testNow}
	store := socialhub.NewMemoryTokenStore()
	key := socialhub.TokenKey{Platform: "qq", Product: productName, Tenant: "102012345", Account: "main"}
	newSource := func() *appTokenSource {
		return &appTokenSource{
			tokenURL: server.URL + "/app/getAppAccessToken", appID: "102012345", secret: "app-secret",
			httpClient: server.Client(), clock: clock, store: store, key: key,
		}
	}
	source := newSource()
	first, err := source.Token(context.Background())
	if err != nil || first.AccessToken != "token-1" || first.TokenType != "QQBot" || !first.ExpiresAt.Equal(testNow.Add(2*time.Minute)) {
		t.Fatalf("first token=%#v err=%v", first, err)
	}
	second, err := source.Token(context.Background())
	if err != nil || second.AccessToken != "token-1" || calls.Load() != 1 {
		t.Fatalf("cached token=%#v calls=%d err=%v", second, calls.Load(), err)
	}
	stored, err := newSource().Token(context.Background())
	if err != nil || stored.AccessToken != "token-1" || calls.Load() != 1 {
		t.Fatalf("stored token=%#v calls=%d err=%v", stored, calls.Load(), err)
	}
	clock.now = testNow.Add(time.Minute)
	refreshed, err := source.Token(context.Background())
	if err != nil || refreshed.AccessToken != "token-2" || calls.Load() != 2 {
		t.Fatalf("refreshed token=%#v calls=%d err=%v", refreshed, calls.Load(), err)
	}
	source.Invalidate(context.Background())
	if _, err := store.Get(context.Background(), key); !errors.Is(err, socialhub.ErrNotFound) {
		t.Fatalf("store after invalidation=%v", err)
	}
	afterInvalidate, err := source.Token(context.Background())
	if err != nil || afterInvalidate.AccessToken != "token-3" || calls.Load() != 3 {
		t.Fatalf("after invalidation=%#v calls=%d err=%v", afterInvalidate, calls.Load(), err)
	}
}

func TestAppTokenSourceErrors(t *testing.T) {
	tests := []struct {
		name   string
		body   string
		status int
		want   socialhub.ErrorCode
	}{
		{name: "credential", body: `{"code":100016,"message":"invalid"}`, status: http.StatusOK, want: socialhub.CodeUnauthenticated},
		{name: "malformed", body: `{`, status: http.StatusOK, want: socialhub.CodePlatformError},
		{name: "invalid fields", body: `{"access_token":"","expires_in":0}`, status: http.StatusOK, want: socialhub.CodePlatformError},
		{name: "http rate", body: `{"err_code":100001,"message":"rate"}`, status: http.StatusTooManyRequests, want: socialhub.CodeRateLimited},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
				writer.WriteHeader(test.status)
				_, _ = writer.Write([]byte(test.body))
			}))
			defer server.Close()
			source := &appTokenSource{
				tokenURL: server.URL, appID: "app", secret: "secret", httpClient: server.Client(), clock: fixedClock{now: testNow},
			}
			if _, err := source.Token(context.Background()); errorCode(err) != test.want {
				t.Fatalf("error=%v code=%s", err, errorCode(err))
			}
		})
	}
}
