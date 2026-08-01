package slack

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

func signedSlackRequest(body []byte, secret string, timestamp int64) *http.Request {
	request := httptest.NewRequest(http.MethodPost, "/events/slack", nil)
	timestampText := strconv.FormatInt(timestamp, 10)
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte("v0:" + timestampText + ":"))
	_, _ = mac.Write(body)
	request.Header.Set("X-Slack-Request-Timestamp", timestampText)
	request.Header.Set("X-Slack-Signature", "v0="+hex.EncodeToString(mac.Sum(nil)))
	return request
}

func TestEventsVerificationAndTypedDecoding(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	_, client := newTestAdapter(t, server, testChannelID, true, allTestScopes())
	tests := []struct {
		body      string
		eventType string
		check     func(t *testing.T, payload EventsPayload)
	}{
		{
			body:      `{"type":"event_callback","team_id":"T123ABC","api_app_id":"A123ABC","event_id":"Ev123ABC","event_context":"EC123","event":{"type":"message","channel":"C123ABC","user":"U999ABC","text":"hello","ts":"1785571200.000001","files":[{"id":"F123ABC","mimetype":"image/png"}]}}`,
			eventType: "slack.message",
			check: func(t *testing.T, payload EventsPayload) {
				if payload.Post == nil || payload.Post.ID != testChannelID+":"+testTimestamp || payload.Message == nil || payload.Message.ID != testChannelID+":"+testTimestamp || payload.Message.Direction != socialhub.DirectionInbound || len(payload.Message.Media) != 1 {
					t.Fatalf("message payload=%#v", payload)
				}
			},
		},
		{
			body:      `{"type":"event_callback","team_id":"T123ABC","api_app_id":"A123ABC","event_id":"Ev124ABC","event":{"type":"reaction_added","user":"U999ABC","reaction":"eyes","item_user":"U123ABC","item":{"type":"message","channel":"C123ABC","ts":"1785571200.000001"}}}`,
			eventType: "slack.reaction_added",
			check: func(t *testing.T, payload EventsPayload) {
				if payload.Reaction == nil || payload.Reaction.Reaction != "eyes" || payload.Reaction.TargetID != testChannelID+":"+testTimestamp || !payload.Reaction.Added {
					t.Fatalf("reaction payload=%#v", payload)
				}
			},
		},
		{
			body:      `{"type":"event_callback","team_id":"T123ABC","api_app_id":"A123ABC","event_id":"Ev124DEF","event":{"type":"message","subtype":"message_changed","channel":"C123ABC","message":{"type":"message","user":"U999ABC","text":"edited","ts":"1785571200.000001"}}}`,
			eventType: "slack.message",
			check: func(t *testing.T, payload EventsPayload) {
				if payload.Message == nil || payload.Message.Text == nil || *payload.Message.Text != "edited" {
					t.Fatalf("changed message payload=%#v", payload)
				}
			},
		},
		{
			body:      `{"type":"event_callback","team_id":"T123ABC","api_app_id":"A123ABC","event_id":"Ev124GHI","event":{"type":"reaction_removed","user":"U999ABC","reaction":"eyes","item":{"type":"file","file":"F123ABC"}}}`,
			eventType: "slack.reaction_removed",
			check: func(t *testing.T, payload EventsPayload) {
				if payload.Reaction != nil || !strings.Contains(string(payload.Raw), `"type":"file"`) {
					t.Fatalf("file reaction payload=%#v", payload)
				}
			},
		},
		{
			body:      `{"type":"event_callback","team_id":"T123ABC","api_app_id":"A123ABC","event_id":"Ev125ABC","event":{"type":"file_shared","file_id":"F123ABC","user_id":"U999ABC","channel_id":"C123ABC"}}`,
			eventType: "slack.file_shared",
			check: func(t *testing.T, payload EventsPayload) {
				if payload.File == nil || payload.File.ID != testFileID || payload.File.State != socialhub.MediaStateReady {
					t.Fatalf("file payload=%#v", payload)
				}
			},
		},
		{
			body:      `{"type":"event_callback","team_id":"T123ABC","api_app_id":"A123ABC","event_id":"Ev126ABC","event":{"type":"channel_created","channel":{"id":"C999ABC"}}}`,
			eventType: "slack.channel_created",
			check: func(t *testing.T, payload EventsPayload) {
				if payload.Post != nil || payload.Reaction != nil || !strings.Contains(string(payload.Raw), "channel_created") {
					t.Fatalf("unknown payload=%#v", payload)
				}
			},
		},
		{
			body:      `{"type":"url_verification","challenge":"challenge-value"}`,
			eventType: "slack.url_verification",
			check: func(t *testing.T, payload EventsPayload) {
				if payload.Challenge != "challenge-value" || !strings.HasPrefix(payload.ID, "url_verification:") {
					t.Fatalf("challenge payload=%#v", payload)
				}
			},
		},
		{
			body:      `{"type":"app_rate_limited","team_id":"T123ABC","api_app_id":"A123ABC","minute_rate_limited":1785585600}`,
			eventType: "slack.app_rate_limited",
			check: func(t *testing.T, payload EventsPayload) {
				if payload.ID != "app_rate_limited:T123ABC:1785585600" {
					t.Fatalf("rate payload=%#v", payload)
				}
			},
		},
	}
	for _, test := range tests {
		body := []byte(test.body)
		request := signedSlackRequest(body, "signing-secret", testNow.Unix())
		request.Header.Set("X-Slack-Retry-Num", "2")
		request.Header.Set("X-Slack-Retry-Reason", "http_timeout")
		if err := client.Verify(context.Background(), request, body); err != nil {
			t.Fatalf("verify %s: %v", test.eventType, err)
		}
		events, err := client.Decode(context.Background(), request, body)
		if err != nil || len(events) != 1 || events[0].Type != test.eventType || events[0].Platform != "slack" || events[0].AccountID != "main" {
			t.Fatalf("events=%#v error=%v", events, err)
		}
		payload, ok := events[0].Payload.(EventsPayload)
		if !ok || payload.RetryNumber != 2 || payload.RetryReason != "http_timeout" {
			t.Fatalf("payload=%#v", events[0].Payload)
		}
		test.check(t, payload)
	}
}

func TestEventsVerificationAndDecodeValidation(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	_, client := newTestAdapter(t, server, testChannelID, true, allTestScopes())
	_, noSecret := newTestAdapter(t, server, testChannelID, false, allTestScopes())
	valid := []byte(`{"type":"event_callback","team_id":"T123ABC","event_id":"Ev123ABC","event":{"type":"channel_created"}}`)
	request := signedSlackRequest(valid, "signing-secret", testNow.Unix())
	if err := noSecret.Verify(context.Background(), request, valid); !errors.Is(err, socialhub.ErrUnsupported) {
		t.Fatalf("verify without secret=%v", err)
	}
	if err := client.Verify(context.Background(), nil, valid); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("nil request=%v", err)
	}
	if err := client.Verify(context.Background(), httptest.NewRequest(http.MethodGet, "/", nil), valid); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("GET request=%v", err)
	}
	invalidRequests := []*http.Request{
		httptest.NewRequest(http.MethodPost, "/", nil),
		signedSlackRequest(valid, "signing-secret", 0),
		signedSlackRequest(valid, "signing-secret", testNow.Add(-6*time.Minute).Unix()),
		signedSlackRequest(valid, "wrong-secret", testNow.Unix()),
	}
	badHex := signedSlackRequest(valid, "signing-secret", testNow.Unix())
	badHex.Header.Set("X-Slack-Signature", "v0=not-hex")
	invalidRequests = append(invalidRequests, badHex)
	for index, invalid := range invalidRequests {
		if err := client.Verify(context.Background(), invalid, valid); err == nil {
			t.Fatalf("verify validation %d accepted", index)
		}
	}
	wrongTeam := []byte(`{"type":"event_callback","team_id":"T999ABC","event_id":"Ev123ABC","event":{"type":"channel_created"}}`)
	if err := client.Verify(context.Background(), signedSlackRequest(wrongTeam, "signing-secret", testNow.Unix()), wrongTeam); !errors.Is(err, socialhub.ErrPermissionDenied) {
		t.Fatalf("wrong team verify=%v", err)
	}
	malformed := []byte("{")
	if err := client.Verify(context.Background(), signedSlackRequest(malformed, "signing-secret", testNow.Unix()), malformed); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("malformed verify=%v", err)
	}

	invalidBodies := [][]byte{
		nil,
		[]byte("{"),
		[]byte(`{"team_id":"T123ABC"}`),
		[]byte(`{"type":"unknown"}`),
		[]byte(`{"type":"url_verification"}`),
		[]byte(`{"type":"app_rate_limited","team_id":"T123ABC"}`),
		[]byte(`{"type":"event_callback","team_id":"T999ABC","event_id":"Ev123ABC","event":{"type":"channel_created"}}`),
		[]byte(`{"type":"event_callback","team_id":"T123ABC","event":{}}`),
		[]byte(`{"type":"event_callback","team_id":"T123ABC","event_id":"Ev123ABC","event":{}}`),
		[]byte(`{"type":"event_callback","team_id":"T123ABC","event_id":"Ev123ABC","event":{"type":"message","channel":"bad","ts":"1.0"}}`),
		[]byte(`{"type":"event_callback","team_id":"T123ABC","event_id":"Ev123ABC","event":{"type":"reaction_added","reaction":"","item":{}}}`),
		[]byte(`{"type":"event_callback","team_id":"T123ABC","event_id":"Ev123ABC","event":{"type":"file_shared","file_id":"bad"}}`),
	}
	for index, body := range invalidBodies {
		if _, err := client.Decode(context.Background(), nil, body); err == nil {
			t.Fatalf("decode validation %d accepted", index)
		}
	}
}

func TestSlackAPIErrorMapping(t *testing.T) {
	tests := []struct {
		platform string
		code     socialhub.ErrorCode
		class    socialhub.ErrorClass
	}{
		{"invalid_auth", socialhub.CodeUnauthenticated, socialhub.ClassUserAction},
		{"missing_scope", socialhub.CodeApprovalRequired, socialhub.ClassUserAction},
		{"no_permission", socialhub.CodePermissionDenied, socialhub.ClassUserAction},
		{"channel_not_found", socialhub.CodeNotFound, socialhub.ClassPermanent},
		{"already_reacted", socialhub.CodeConflict, socialhub.ClassPermanent},
		{"ratelimited", socialhub.CodeRateLimited, socialhub.ClassRetryable},
		{"internal_error", socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable},
		{"deprecated_endpoint", socialhub.CodeUnsupported, socialhub.ClassPermanent},
		{"invalid_arguments", socialhub.CodeInvalidArgument, socialhub.ClassPermanent},
		{"unknown_error", socialhub.CodePlatformError, socialhub.ClassPermanent},
	}
	for _, test := range tests {
		response := apiEnvelope{Error: test.platform, Warning: "warning", Needed: "chat:write,files:write"}
		response.Metadata.Messages = []string{"detail"}
		err := apiResponseError("method", response)
		platformErr := requireErrorCode(t, err, test.code)
		if platformErr.Class != test.class || platformErr.PlatformCode != test.platform || platformErr.PlatformMessage != "warning: detail" || platformErr.Op != "method" {
			t.Fatalf("Slack error %s=%#v", test.platform, platformErr)
		}
		if test.platform == "missing_scope" && (len(platformErr.RequiredScopes) != 2 || platformErr.ApprovalURL == "") {
			t.Fatalf("missing scope error=%#v", platformErr)
		}
	}
}

func TestSlackHTTPAndEnvelopeErrors(t *testing.T) {
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
		header.Set("X-Slack-Req-Id", "slack-request")
		header.Set("Retry-After", "3")
		err := decodeHTTPError(test.status, header, nil)
		platformErr := requireErrorCode(t, err, test.code)
		if platformErr.Class != test.class || platformErr.HTTPStatus != test.status || platformErr.RequestID != "slack-request" || platformErr.RetryAfter != 3*time.Second {
			t.Fatalf("HTTP %d error=%#v", test.status, platformErr)
		}
	}
	header := make(http.Header)
	header.Set("X-Request-ID", "request-id")
	header.Set("Retry-After", "2")
	err := decodeHTTPError(http.StatusTooManyRequests, header, []byte(`{"ok":false,"error":"ratelimited"}`))
	platformErr := requireErrorCode(t, err, socialhub.CodeRateLimited)
	if platformErr.RetryAfter != 2*time.Second || platformErr.RequestID != "request-id" {
		t.Fatalf("envelope HTTP error=%#v", platformErr)
	}

	mode := "malformed"
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch mode {
		case "malformed":
			_, _ = io.WriteString(writer, `{"ok":`)
		case "business":
			writeTestJSON(t, writer, map[string]any{"ok": false, "error": "missing_scope", "needed": "chat:write"})
		case "wrong-type":
			writeTestJSON(t, writer, map[string]any{"ok": true, "value": "text"})
		case "ok":
			writeTestJSON(t, writer, map[string]any{"ok": true})
		}
	}))
	defer server.Close()
	_, client := newTestAdapter(t, server, testChannelID, false, nil)
	var output struct {
		Value int `json:"value"`
	}
	err = client.call(context.Background(), "test", nil, &output)
	requireErrorCode(t, err, socialhub.CodePlatformError)
	mode = "business"
	err = client.call(context.Background(), "test", nil, &output)
	requireErrorCode(t, err, socialhub.CodeApprovalRequired)
	mode = "wrong-type"
	err = client.call(context.Background(), "test", nil, &output)
	requireErrorCode(t, err, socialhub.CodePlatformError)
	mode = "ok"
	if err := client.call(context.Background(), "test", nil, nil); err != nil {
		t.Fatalf("nil output=%v", err)
	}
}

func TestErrorAndMappingHelpers(t *testing.T) {
	if retryAfter("0") != 0 || retryAfter("later") != 0 || retryAfter("2") != 2*time.Second || retryAfter("999999") != 0 {
		t.Fatal("retry-after parsing mismatch")
	}
	if scopes := splitScopes("chat:write, files:write users:read"); len(scopes) != 3 {
		t.Fatalf("scopes=%v", scopes)
	}
	long := strings.Repeat("界", 513)
	if got := boundedMessage(long, 512); len([]rune(got)) != 512 || boundedMessage("short", 512) != "short" {
		t.Fatal("bounded message mismatch")
	}
	if firstNonEmpty(" ", " value ") != "value" || stringPointer(" ") != nil || intPointer(0) != nil {
		t.Fatal("mapping helper mismatch")
	}
	if media := mapFile(wireFile{ID: testFileID, Mimetype: "audio/mpeg", DurationMS: 100}); media.Type != socialhub.MediaTypeAudio || media.Duration == nil {
		t.Fatalf("audio mapping=%#v", media)
	}
}
