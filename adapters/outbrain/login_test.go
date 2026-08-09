package outbrain

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

func TestLoginToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet || request.URL.Path != "/login" || request.Header.Get("Accept") != "application/json" {
			t.Fatalf("request=%s %s headers=%v", request.Method, request.URL, request.Header)
		}
		username, password, ok := request.BasicAuth()
		if !ok || username != testUsername || password != testPassword {
			t.Fatalf("basic auth username=%q password=%q ok=%v", username, password, ok)
		}
		writeJSON(t, writer, http.StatusOK, map[string]string{"OB-TOKEN-V1": testAccessToken})
	}))
	defer server.Close()
	clock := &mutableClock{value: testNow}
	login := LoginClient{Username: testUsername, Password: testPassword, BaseURL: server.URL, HTTPClient: cloneHTTPClient(server.Client()), Clock: clock}
	token, err := login.Token(context.Background())
	if err != nil || token.AccessToken != testAccessToken || token.TokenType != "OB-TOKEN-V1" || !token.ExpiresAt.Equal(testNow.Add(30*24*time.Hour)) {
		t.Fatalf("token=%#v err=%v", token, err)
	}
}

func TestLoginTokenValidationErrorsAndBounds(t *testing.T) {
	clock := &mutableClock{value: testNow}
	invalid := []LoginClient{
		{},
		{Username: testUsername, Password: testPassword, BaseURL: "https://example.test/", HTTPClient: http.DefaultClient, Clock: clock},
		{Username: "bad\nuser", Password: testPassword, BaseURL: "https://example.test", HTTPClient: http.DefaultClient, Clock: clock},
	}
	for index := range invalid {
		if _, err := invalid[index].Token(context.Background()); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Fatalf("invalid %d error=%v", index, err)
		}
	}

	tests := []struct {
		name string
		body string
	}{
		{"malformed", `{"OB-TOKEN-V1":`},
		{"missing token", `{}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) { _, _ = writer.Write([]byte(test.body)) }))
			defer server.Close()
			login := LoginClient{Username: testUsername, Password: testPassword, BaseURL: server.URL, HTTPClient: cloneHTTPClient(server.Client()), Clock: clock}
			if _, err := login.Token(context.Background()); hubError(t, err).Code != socialhub.CodePlatformError {
				t.Fatalf("error=%v", err)
			}
		})
	}

	large := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte(strings.Repeat("x", int(maxLoginResponseBytes)+1)))
	}))
	defer large.Close()
	login := LoginClient{Username: testUsername, Password: testPassword, BaseURL: large.URL, HTTPClient: cloneHTTPClient(large.Client()), Clock: clock}
	if _, err := login.Token(context.Background()); hubError(t, err).Code != socialhub.CodePlatformError {
		t.Fatalf("large response error=%v", err)
	}
}

func TestLoginTokenSourceCachesAndRefreshesOnce(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		value := calls.Add(1)
		writeJSON(t, writer, http.StatusOK, map[string]string{"OB-TOKEN-V1": "token-" + string(rune('0'+value))})
	}))
	defer server.Close()
	clock := &mutableClock{value: testNow}
	store := socialhub.NewMemoryTokenStore()
	source := &loginTokenSource{
		login: LoginClient{Username: testUsername, Password: testPassword, BaseURL: server.URL, HTTPClient: cloneHTTPClient(server.Client()), Clock: clock},
		store: store, key: socialhub.TokenKey{Platform: platformName, Product: productName, Account: string(testAccountID)},
	}
	var wait sync.WaitGroup
	errorsFound := make(chan error, 20)
	for range 20 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			token, err := source.Token(context.Background())
			if err != nil || token.AccessToken != "token-1" {
				errorsFound <- errors.New("unexpected cached token")
			}
		}()
	}
	wait.Wait()
	close(errorsFound)
	for err := range errorsFound {
		t.Fatal(err)
	}
	if calls.Load() != 1 {
		t.Fatalf("login calls=%d", calls.Load())
	}
	clock.Set(testNow.Add(29 * 24 * time.Hour))
	token, err := source.Token(context.Background())
	if err != nil || token.AccessToken != "token-2" || calls.Load() != 2 {
		t.Fatalf("refreshed token=%#v calls=%d err=%v", token, calls.Load(), err)
	}
}

func TestAdapterLoginModeAuthenticatesAPIAndCachesToken(t *testing.T) {
	var loginCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/login":
			loginCalls.Add(1)
			username, password, ok := request.BasicAuth()
			if !ok || username != testUsername || password != testPassword {
				t.Fatalf("login credentials username=%q password=%q ok=%v", username, password, ok)
			}
			writeJSON(t, writer, http.StatusOK, map[string]string{"OB-TOKEN-V1": testAccessToken})
		case "/marketers/" + testMarketerID:
			if request.Header.Get("OB-TOKEN-V1") != testAccessToken || request.Header.Get("Authorization") != "" {
				t.Fatalf("API headers=%v", request.Header)
			}
			writeJSON(t, writer, http.StatusOK, marketerFixture())
		default:
			t.Fatalf("unexpected path=%s", request.URL.Path)
		}
	}))
	defer server.Close()
	clock := &mutableClock{value: testNow}
	adapter := &Adapter{}
	if err := adapter.Init(context.Background(), loginConfig(server.URL),
		socialhub.WithHTTPClient(server.Client()),
		socialhub.WithSecretResolver(mapResolver{"secret://outbrain-password": testPassword}),
		socialhub.WithClock(clock), socialhub.WithTokenStore(socialhub.NewMemoryTokenStore()),
	); err != nil {
		t.Fatal(err)
	}
	value, err := adapter.Client(context.Background(), testAccountID)
	if err != nil {
		t.Fatal(err)
	}
	client := value.(*Client)
	for range 2 {
		if _, err := client.GetMarketer(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	if loginCalls.Load() != 1 {
		t.Fatalf("login calls=%d", loginCalls.Load())
	}
}

func TestLoginAndAPIRedirectsAreRejected(t *testing.T) {
	var reached atomic.Int32
	destination := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { reached.Add(1) }))
	defer destination.Close()
	redirect := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		http.Redirect(writer, request, destination.URL, http.StatusFound)
	}))
	defer redirect.Close()
	clock := &mutableClock{value: testNow}
	login := LoginClient{Username: testUsername, Password: testPassword, BaseURL: redirect.URL, HTTPClient: cloneHTTPClient(redirect.Client()), Clock: clock}
	if _, err := login.Token(context.Background()); hubError(t, err).HTTPStatus != http.StatusFound {
		t.Fatalf("login redirect error=%v", err)
	}
	if reached.Load() != 0 {
		t.Fatalf("redirect destination reached %d times", reached.Load())
	}

	_, client := newTestAdapter(t, redirect)
	if _, err := client.ListMarketers(context.Background()); hubError(t, err).HTTPStatus != http.StatusFound {
		t.Fatalf("API redirect error=%v", err)
	}
	if reached.Load() != 0 {
		t.Fatalf("API redirect destination reached %d times", reached.Load())
	}
}
