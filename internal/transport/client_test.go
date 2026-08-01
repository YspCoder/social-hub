package transport

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
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
