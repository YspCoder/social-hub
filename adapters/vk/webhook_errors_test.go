package vk

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"social-hub/pkg/socialhub"
)

func TestCallbackVerificationAndEventMapping(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	_, client := newTestAdapter(t, server, TokenCommunity, -456, true)
	request := httptest.NewRequest(http.MethodPost, "/callbacks/vk", nil)
	bodies := []struct {
		body      string
		eventType string
		check     func(t *testing.T, payload CallbackEvent)
	}{
		{
			body:      `{"type":"message_new","object":{"message":{"id":10,"date":1785571200,"from_id":123,"peer_id":456,"text":"hello"}},"group_id":456,"event_id":"event-1","v":"5.199","secret":"callback-secret"}`,
			eventType: "vk.message_new",
			check: func(t *testing.T, payload CallbackEvent) {
				if payload.Message == nil || payload.Message.ID != "10" || payload.Message.Text == nil || *payload.Message.Text != "hello" {
					t.Fatalf("message payload=%#v", payload)
				}
			},
		},
		{
			body:      `{"type":"message_reply","object":{"id":11,"date":1785571200,"from_id":-456,"peer_id":123,"text":"reply","out":1},"group_id":456,"event_id":"event-2","v":"5.199","secret":"callback-secret"}`,
			eventType: "vk.message_reply",
			check: func(t *testing.T, payload CallbackEvent) {
				if payload.Message == nil || payload.Message.Direction != socialhub.DirectionOutbound {
					t.Fatalf("reply payload=%#v", payload)
				}
			},
		},
		{
			body:      `{"type":"wall_post_new","object":{"id":7,"owner_id":-456,"from_id":-456,"date":1785571200,"text":"post"},"group_id":456,"event_id":"event-3","v":"5.199","secret":"callback-secret"}`,
			eventType: "vk.wall_post_new",
			check: func(t *testing.T, payload CallbackEvent) {
				if payload.Post == nil || payload.Post.ID != "-456_7" {
					t.Fatalf("post payload=%#v", payload)
				}
			},
		},
		{
			body:      `{"type":"wall_reply_new","object":{"id":12,"post_id":7,"post_owner_id":-456,"from_id":123,"date":1785571200,"text":"comment"},"group_id":456,"event_id":"event-4","v":"5.199","secret":"callback-secret"}`,
			eventType: "vk.wall_reply_new",
			check: func(t *testing.T, payload CallbackEvent) {
				if payload.Comment == nil || payload.Comment.ID != "-456_12" || payload.Comment.PostID != "-456_7" {
					t.Fatalf("comment payload=%#v", payload)
				}
			},
		},
		{
			body:      `{"type":"wall_reply_delete","object":{"id":13,"post_id":7,"owner_id":-456,"deleter_id":123},"group_id":456,"event_id":"event-4b","v":"5.199","secret":"callback-secret"}`,
			eventType: "vk.wall_reply_delete",
			check: func(t *testing.T, payload CallbackEvent) {
				if payload.Comment == nil || payload.Comment.ID != "-456_13" || payload.Comment.PostID != "-456_7" {
					t.Fatalf("deleted comment payload=%#v", payload)
				}
			},
		},
		{
			body:      `{"type":"custom_event","object":{"custom":true},"group_id":456,"event_id":"event-5","v":"5.199","secret":"callback-secret"}`,
			eventType: "vk.custom_event",
			check: func(t *testing.T, payload CallbackEvent) {
				if payload.Post != nil || payload.Message != nil || !strings.Contains(string(payload.Object), "custom") {
					t.Fatalf("unknown payload=%#v", payload)
				}
			},
		},
		{
			body:      `{"type":"confirmation","object":{},"group_id":456,"v":"5.199","secret":"callback-secret"}`,
			eventType: "vk.confirmation",
			check: func(t *testing.T, payload CallbackEvent) {
				if payload.ID != "confirmation:456" {
					t.Fatalf("confirmation payload=%#v", payload)
				}
			},
		},
	}
	for _, test := range bodies {
		body := []byte(test.body)
		if err := client.Verify(context.Background(), request, body); err != nil {
			t.Fatalf("verify %s: %v", test.eventType, err)
		}
		events, err := client.Decode(context.Background(), request, body)
		if err != nil || len(events) != 1 || events[0].Type != test.eventType || events[0].Platform != "vk" || events[0].AccountID != "main" {
			t.Fatalf("events=%#v error=%v", events, err)
		}
		payload, ok := events[0].Payload.(CallbackEvent)
		if !ok || payload.GroupID != 456 || payload.Version != apiVersion || len(payload.Object) == 0 {
			t.Fatalf("payload=%#v", events[0].Payload)
		}
		test.check(t, payload)
	}
}

func TestCallbackValidation(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	_, client := newTestAdapter(t, server, TokenCommunity, -456, true)
	_, noSecret := newTestAdapter(t, server, TokenCommunity, -456, false)
	request := httptest.NewRequest(http.MethodPost, "/callbacks/vk", nil)
	valid := []byte(`{"type":"custom","object":{},"group_id":456,"event_id":"event","secret":"callback-secret"}`)
	if err := noSecret.Verify(context.Background(), request, valid); !errors.Is(err, socialhub.ErrUnsupported) {
		t.Fatalf("verify without secret=%v", err)
	}
	if err := client.Verify(context.Background(), nil, valid); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("nil request=%v", err)
	}
	if err := client.Verify(context.Background(), httptest.NewRequest(http.MethodGet, "/", nil), valid); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("GET request=%v", err)
	}
	for index, body := range [][]byte{
		nil,
		[]byte("{"),
		[]byte(`{"type":"custom","object":{},"group_id":999,"event_id":"event","secret":"callback-secret"}`),
		[]byte(`{"type":"custom","object":{},"group_id":456,"event_id":"event","secret":"wrong"}`),
		[]byte(`{"type":"custom","object":{},"group_id":456,"event_id":"event","secret":"callback-secreu"}`),
	} {
		if err := client.Verify(context.Background(), request, body); err == nil {
			t.Fatalf("verify validation %d accepted", index)
		}
	}
	invalidDecode := [][]byte{
		nil,
		[]byte("{"),
		[]byte(`{"object":{},"group_id":456,"event_id":"event"}`),
		[]byte(`{"type":"custom","object":{},"group_id":999,"event_id":"event"}`),
		[]byte(`{"type":"custom","object":{},"group_id":456}`),
		[]byte(`{"type":"message_new","object":{},"group_id":456,"event_id":"event"}`),
		[]byte(`{"type":"message_reply","object":{},"group_id":456,"event_id":"event"}`),
		[]byte(`{"type":"wall_post_new","object":{},"group_id":456,"event_id":"event"}`),
		[]byte(`{"type":"wall_reply_new","object":{},"group_id":456,"event_id":"event"}`),
	}
	for index, body := range invalidDecode {
		if _, err := client.Decode(context.Background(), request, body); err == nil {
			t.Fatalf("decode validation %d accepted", index)
		}
	}
}

func TestCallbackConfirmationCode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		form, ok := requireVKRequest(t, writer, request, "groups.getCallbackConfirmationCode")
		if !ok {
			return
		}
		if form.Get("group_id") != "456" {
			t.Errorf("confirmation form=%v", form)
		}
		writeTestJSON(t, writer, map[string]any{"response": map[string]any{"code": "confirm-code"}})
	}))
	defer server.Close()
	_, community := newTestAdapter(t, server, TokenCommunity, -456, false)
	code, err := community.GetCallbackConfirmationCode(context.Background())
	if err != nil || code != "confirm-code" {
		t.Fatalf("code=%q error=%v", code, err)
	}
	_, user := newTestAdapter(t, server, TokenUser, 123, false)
	if _, err := user.GetCallbackConfirmationCode(context.Background()); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("user confirmation=%v", err)
	}
}

func TestAPIErrorMapping(t *testing.T) {
	tests := []struct {
		code      int
		wantCode  socialhub.ErrorCode
		wantClass socialhub.ErrorClass
	}{
		{5, socialhub.CodeUnauthenticated, socialhub.ClassUserAction},
		{6, socialhub.CodeRateLimited, socialhub.ClassRetryable},
		{7, socialhub.CodePermissionDenied, socialhub.ClassUserAction},
		{10, socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable},
		{14, socialhub.CodeApprovalRequired, socialhub.ClassUserAction},
		{17, socialhub.CodeApprovalRequired, socialhub.ClassUserAction},
		{24, socialhub.CodeApprovalRequired, socialhub.ClassUserAction},
		{27, socialhub.CodeUnauthenticated, socialhub.ClassUserAction},
		{29, socialhub.CodeRateLimited, socialhub.ClassRetryable},
		{100, socialhub.CodeInvalidArgument, socialhub.ClassPermanent},
		{104, socialhub.CodeNotFound, socialhub.ClassPermanent},
		{999, socialhub.CodePlatformError, socialhub.ClassPermanent},
	}
	for _, test := range tests {
		err := (&apiError{Code: test.code, Subcode: 2, Message: " message ", Text: "details", CaptchaImage: "https://captcha.test", RedirectURI: "https://redirect.test"}).err("method")
		platformErr := requireErrorCode(t, err, test.wantCode)
		if platformErr.Class != test.wantClass || platformErr.PlatformCode != strings.TrimSpace(strings.Join([]string{strconv.Itoa(test.code), "2"}, ".")) || platformErr.PlatformMessage != "message: details" || platformErr.Op != "method" {
			t.Fatalf("VK code %d error=%#v", test.code, platformErr)
		}
		if test.wantCode == socialhub.CodeApprovalRequired && platformErr.ApprovalURL != "https://redirect.test" {
			t.Fatalf("approval URL=%q", platformErr.ApprovalURL)
		}
	}
	if err := (*apiError)(nil).err("method"); err != nil {
		t.Fatalf("nil API error=%v", err)
	}
}

func TestHTTPAndEnvelopeErrors(t *testing.T) {
	tests := []struct {
		status int
		code   socialhub.ErrorCode
		class  socialhub.ErrorClass
	}{
		{http.StatusBadRequest, socialhub.CodeInvalidArgument, socialhub.ClassPermanent},
		{http.StatusUnauthorized, socialhub.CodeUnauthenticated, socialhub.ClassUserAction},
		{http.StatusForbidden, socialhub.CodePermissionDenied, socialhub.ClassUserAction},
		{http.StatusNotFound, socialhub.CodeNotFound, socialhub.ClassPermanent},
		{http.StatusConflict, socialhub.CodeConflict, socialhub.ClassPermanent},
		{http.StatusTooManyRequests, socialhub.CodeRateLimited, socialhub.ClassRetryable},
		{http.StatusBadGateway, socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable},
		{http.StatusTeapot, socialhub.CodePlatformError, socialhub.ClassPermanent},
	}
	for _, test := range tests {
		header := make(http.Header)
		header.Set("X-Request-ID", "request-id")
		err := decodeHTTPError(test.status, header, nil)
		platformErr := requireErrorCode(t, err, test.code)
		if platformErr.Class != test.class || platformErr.HTTPStatus != test.status || platformErr.RequestID != "request-id" {
			t.Fatalf("HTTP %d error=%#v", test.status, platformErr)
		}
	}
	header := make(http.Header)
	header.Set("X-Correlation-ID", "correlation-id")
	err := decodeHTTPError(http.StatusBadRequest, header, []byte(`{"error":{"error_code":6,"error_msg":"slow down"}}`))
	platformErr := requireErrorCode(t, err, socialhub.CodeRateLimited)
	if platformErr.HTTPStatus != http.StatusBadRequest || platformErr.RequestID != "correlation-id" || platformErr.PlatformMessage != "slow down" {
		t.Fatalf("envelope HTTP error=%#v", platformErr)
	}

	mode := "null"
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch mode {
		case "null":
			writeTestJSON(t, writer, map[string]any{"response": nil})
		case "malformed":
			_, _ = io.WriteString(writer, `{"response":`)
		case "wrong-type":
			writeTestJSON(t, writer, map[string]any{"response": "text"})
		case "business":
			writeTestJSON(t, writer, map[string]any{"error": map[string]any{"error_code": 29, "error_msg": "quota"}})
		case "ok":
			writeTestJSON(t, writer, map[string]any{"response": 1})
		}
	}))
	defer server.Close()
	_, client := newTestAdapter(t, server, TokenUser, 123, false)
	var output int
	err = client.method(context.Background(), "test", nil, &output)
	requireErrorCode(t, err, socialhub.CodePlatformError)
	mode = "malformed"
	err = client.method(context.Background(), "test", nil, &output)
	requireErrorCode(t, err, socialhub.CodePlatformError)
	mode = "wrong-type"
	err = client.method(context.Background(), "test", nil, &output)
	requireErrorCode(t, err, socialhub.CodePlatformError)
	mode = "business"
	err = client.method(context.Background(), "test", nil, &output)
	requireErrorCode(t, err, socialhub.CodeRateLimited)
	mode = "ok"
	if err := client.method(context.Background(), "test", nil, nil); err != nil {
		t.Fatalf("nil output=%v", err)
	}
}

func TestErrorAndPaginationHelpers(t *testing.T) {
	long := strings.Repeat("界", 513)
	if got := boundedMessage(long, 512); len([]rune(got)) != 512 {
		t.Fatalf("bounded runes=%d", len([]rune(got)))
	}
	if boundedMessage("short", 512) != "short" || firstNonEmpty(" ", " value ") != "value" || len(nonEmpty(" one ", "", " two ")) != 2 {
		t.Fatal("string helpers mismatch")
	}
	items := []int{1}
	page := paged(items, 0, 20, 1)
	if page.HasMore || page.NextCursor != nil || page.PrevCursor != nil {
		t.Fatalf("single page=%#v", page)
	}
	if offset, count, err := pageParameters("", 0, 20, 100); err != nil || offset != 0 || count != 20 {
		t.Fatalf("default page=%d/%d error=%v", offset, count, err)
	}
	if offset, count, err := pageParameters("1", 1000, 20, 100); err != nil || offset != 1 || count != 100 {
		t.Fatalf("capped page=%d/%d error=%v", offset, count, err)
	}
}
