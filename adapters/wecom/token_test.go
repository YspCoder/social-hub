package wecom

import (
	"context"
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

func (c *mutableClock) Now() time.Time { return c.now }

func TestCorpTokenSourceCacheStoreRefreshAndInvalidate(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/cgi-bin/gettoken" || request.URL.Query().Get("corpid") != "ww-corp-id" || request.URL.Query().Get("corpsecret") != "corp-secret" {
			http.Error(writer, "bad token request", http.StatusBadRequest)
			return
		}
		call := calls.Add(1)
		writeTestJSON(t, writer, map[string]any{"errcode": 0, "access_token": "token-" + strconv.Itoa(int(call)), "expires_in": 600})
	}))
	defer server.Close()
	clock := &mutableClock{now: testNow}
	store := socialhub.NewMemoryTokenStore()
	key := socialhub.TokenKey{Platform: "wecom", Product: productName, Tenant: "ww-corp-id", Account: "main", Subject: "1000002"}
	newSource := func() *corpTokenSource {
		return &corpTokenSource{
			baseURL: server.URL, corpID: "ww-corp-id", secret: "corp-secret", httpClient: server.Client(),
			clock: clock, store: store, key: key,
		}
	}
	source := newSource()
	first, err := source.Token(context.Background())
	if err != nil || first.AccessToken != "token-1" || !first.ExpiresAt.Equal(testNow.Add(10*time.Minute)) {
		t.Fatalf("first token=%#v err=%v", first, err)
	}
	second, err := source.Token(context.Background())
	if err != nil || second.AccessToken != "token-1" || calls.Load() != 1 {
		t.Fatalf("cached token=%#v calls=%d err=%v", second, calls.Load(), err)
	}
	fromStore, err := newSource().Token(context.Background())
	if err != nil || fromStore.AccessToken != "token-1" || calls.Load() != 1 {
		t.Fatalf("stored token=%#v calls=%d err=%v", fromStore, calls.Load(), err)
	}

	clock.now = testNow.Add(5 * time.Minute)
	refreshed, err := source.Token(context.Background())
	if err != nil || refreshed.AccessToken != "token-2" || calls.Load() != 2 {
		t.Fatalf("refreshed token=%#v calls=%d err=%v", refreshed, calls.Load(), err)
	}
	source.Invalidate(context.Background())
	if _, err := store.Get(context.Background(), key); !errors.Is(err, socialhub.ErrNotFound) {
		t.Fatalf("token store after invalidation=%v", err)
	}
	afterInvalidate, err := source.Token(context.Background())
	if err != nil || afterInvalidate.AccessToken != "token-3" || calls.Load() != 3 {
		t.Fatalf("token after invalidation=%#v calls=%d err=%v", afterInvalidate, calls.Load(), err)
	}
}

func TestCorpTokenSourceErrors(t *testing.T) {
	tests := []struct {
		name string
		body string
		code int
		want socialhub.ErrorCode
	}{
		{name: "business", body: `{"errcode":40013,"errmsg":"invalid corpid"}`, code: http.StatusOK, want: socialhub.CodeUnauthenticated},
		{name: "malformed", body: `{`, code: http.StatusOK, want: socialhub.CodePlatformError},
		{name: "invalid fields", body: `{"errcode":0,"access_token":"","expires_in":0}`, code: http.StatusOK, want: socialhub.CodePlatformError},
		{name: "http limit", body: `{"errcode":45009,"errmsg":"limit"}`, code: http.StatusTooManyRequests, want: socialhub.CodeRateLimited},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
				writer.WriteHeader(test.code)
				_, _ = writer.Write([]byte(test.body))
			}))
			defer server.Close()
			source := &corpTokenSource{
				baseURL: server.URL, corpID: "corp", secret: "secret", httpClient: server.Client(),
				clock: fixedClock{now: testNow},
			}
			if _, err := source.Token(context.Background()); errorCode(err) != test.want {
				t.Fatalf("error=%v code=%s", err, errorCode(err))
			}
		})
	}
}
