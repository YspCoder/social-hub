package discord

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

func TestRateLimitMapsFractionalRetryAfter(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusTooManyRequests)
		writeJSON(writer, `{"message":"rate limited","retry_after":0.25,"global":false}`)
	}))
	defer server.Close()
	_, client := newTestClient(t, server, "")
	_, err := client.GetUser(context.Background(), "42")
	if !errors.Is(err, socialhub.ErrRateLimited) {
		t.Fatalf("error=%v", err)
	}
	var platformError *socialhub.Error
	if !errors.As(err, &platformError) || !platformError.Retryable() || platformError.RetryAfter != 250*time.Millisecond {
		t.Fatalf("platform error=%#v", platformError)
	}
}

func TestDiscordBusinessErrorClassification(t *testing.T) {
	err := decodeHTTPError(http.StatusBadRequest, nil, []byte(`{"code":50013,"message":"Missing Permissions"}`))
	if !errors.Is(err, socialhub.ErrPermissionDenied) {
		t.Fatalf("error=%v", err)
	}
}
