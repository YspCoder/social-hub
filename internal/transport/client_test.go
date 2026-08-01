package transport

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

func TestClientJSONAuthenticatesAndDecodes(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if got := request.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Fatalf("Authorization = %q", got)
		}
		if request.URL.Path != "/v2/resource" || request.URL.Query().Get("cursor") != "next" {
			t.Fatalf("request URL = %s", request.URL.String())
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"id":"123"}`))
	}))
	defer server.Close()

	client, err := New(server.URL+"/v2", server.Client(), socialhub.StaticTokenSource{Value: socialhub.Token{AccessToken: "test-token"}}, "test", "api", nil)
	if err != nil {
		t.Fatal(err)
	}
	var output struct {
		ID string `json:"id"`
	}
	if err := client.JSON(context.Background(), http.MethodGet, "resource", url.Values{"cursor": {"next"}}, nil, &output); err != nil {
		t.Fatal(err)
	}
	if output.ID != "123" {
		t.Fatalf("decoded ID = %q", output.ID)
	}
}

func TestClientMapsHTTPFailures(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusTooManyRequests)
		_, _ = writer.Write([]byte(`{"secret":"must not escape"}`))
	}))
	defer server.Close()
	client, err := New(server.URL, server.Client(), socialhub.StaticTokenSource{Value: socialhub.Token{AccessToken: "test-token"}}, "test", "api", nil)
	if err != nil {
		t.Fatal(err)
	}
	err = client.JSON(context.Background(), http.MethodGet, "resource", nil, nil, nil)
	if !errors.Is(err, socialhub.ErrRateLimited) {
		t.Fatalf("error = %v", err)
	}
}

func TestQueryAuthenticator(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if got := request.URL.Query().Get("access_token"); got != "test-token" {
			t.Fatalf("access_token = %q", got)
		}
		if got := request.Header.Get("Authorization"); got != "" {
			t.Fatalf("Authorization = %q", got)
		}
		_, _ = writer.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()
	client, err := NewWithAuthenticator(server.URL, server.Client(), socialhub.StaticTokenSource{Value: socialhub.Token{AccessToken: "test-token"}}, "test", "api", QueryAuthenticator("access_token"), nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := client.JSON(context.Background(), http.MethodGet, "resource", nil, nil, nil); err != nil {
		t.Fatal(err)
	}
}

type failingRoundTripper struct{}

func (failingRoundTripper) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, fmt.Errorf("dial failed")
}

func TestQueryTokenDoesNotEscapeThroughTransportError(t *testing.T) {
	t.Parallel()
	httpClient := &http.Client{Transport: failingRoundTripper{}}
	client, err := NewWithAuthenticator("https://api.example.com", httpClient, socialhub.StaticTokenSource{Value: socialhub.Token{AccessToken: "sensitive-token"}}, "test", "api", QueryAuthenticator("access_token"), nil)
	if err != nil {
		t.Fatal(err)
	}
	err = client.JSON(context.Background(), http.MethodGet, "resource", nil, nil, nil)
	for current := err; current != nil; current = errors.Unwrap(current) {
		if strings.Contains(current.Error(), "sensitive-token") {
			t.Fatalf("token escaped through error chain: %v", current)
		}
	}
}

func TestClientHonorsCallTimeout(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, request *http.Request) {
		<-request.Context().Done()
	}))
	defer server.Close()
	client, err := New(server.URL, server.Client(), socialhub.StaticTokenSource{Value: socialhub.Token{AccessToken: "test-token"}}, "test", "api", nil)
	if err != nil {
		t.Fatal(err)
	}
	err = client.JSON(context.Background(), http.MethodGet, "slow", nil, nil, nil, socialhub.WithCallTimeout(10*time.Millisecond))
	var platformError *socialhub.Error
	if !errors.As(err, &platformError) || !platformError.Retryable() {
		t.Fatalf("timeout error = %v", err)
	}
}
