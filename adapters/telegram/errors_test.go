package telegram

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

func TestRateLimitResponseMapsRetryAfter(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte(`{"ok":false,"error_code":429,"description":"Too Many Requests","parameters":{"retry_after":5}}`))
	}))
	defer server.Close()
	_, client := newTestAdapter(t, server, "", false)
	text := "hello"
	_, err := client.SendMessage(context.Background(), socialhub.SendMessageRequest{ConversationID: "9", Text: &text})
	if !errors.Is(err, socialhub.ErrRateLimited) {
		t.Fatalf("error=%v", err)
	}
	var platformError *socialhub.Error
	if !errors.As(err, &platformError) || !platformError.Retryable() || platformError.RetryAfter != 5*time.Second || platformError.PlatformCode != "429" {
		t.Fatalf("platform error=%#v", platformError)
	}
}

func TestCanceledCallIsNotRetryable(t *testing.T) {
	err := mapError("send_message", context.Canceled)
	var platformError *socialhub.Error
	if !errors.As(err, &platformError) || platformError.Retryable() || !errors.Is(err, context.Canceled) {
		t.Fatalf("error=%#v", err)
	}
}
