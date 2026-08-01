package lark

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

func larkEventBody(t *testing.T, eventID, eventType string, event any) []byte {
	t.Helper()
	body, err := json.Marshal(map[string]any{
		"schema": "2.0",
		"header": map[string]string{
			"event_id": eventID, "event_type": eventType, "create_time": "1785571200000",
			"token": "verification-token", "app_id": testAppID, "tenant_key": testTenantKey,
		},
		"event": event,
	})
	if err != nil {
		t.Fatal(err)
	}
	return body
}

func encryptLarkEvent(t *testing.T, plain []byte, secret string) string {
	t.Helper()
	key := sha256.Sum256([]byte(secret))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		t.Fatal(err)
	}
	padding := aes.BlockSize - len(plain)%aes.BlockSize
	padded := append(append([]byte(nil), plain...), bytes.Repeat([]byte{byte(padding)}, padding)...)
	iv := []byte("0123456789abcdef")
	ciphertext := make([]byte, len(padded))
	cipher.NewCBCEncrypter(block, iv).CryptBlocks(ciphertext, padded)
	return base64.StdEncoding.EncodeToString(append(append([]byte(nil), iv...), ciphertext...))
}

func signedLarkRequest(body []byte, secret string) *http.Request {
	request := httptest.NewRequest(http.MethodPost, "/events/lark", nil)
	timestamp, nonce := "1785571200", "nonce-contract"
	hash := sha256.Sum256([]byte(timestamp + nonce + secret + string(body)))
	request.Header.Set("X-Lark-Request-Timestamp", timestamp)
	request.Header.Set("X-Lark-Request-Nonce", nonce)
	request.Header.Set("X-Lark-Signature", hex.EncodeToString(hash[:]))
	return request
}

func TestWebhookPlaintextAndEncryptedEvents(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	_, plainClient := newTestClient(t, server, TokenTenant, testChatID, testActorID, false)
	_, encryptedClient := newTestClient(t, server, TokenTenant, testChatID, testActorID, true)
	ctx := context.Background()

	messageBody := larkEventBody(t, "evt_message", "im.message.receive_v1", map[string]any{
		"sender": map[string]any{
			"sender_id": map[string]string{"open_id": "ou_sender"}, "sender_type": "user", "tenant_key": testTenantKey,
		},
		"message": map[string]any{
			"message_id": testMessageID, "chat_id": testChatID, "chat_type": "group",
			"message_type": "text", "content": `{"text":"hello"}`, "create_time": "1785571200000", "update_time": "1785571201000",
		},
	})
	plainRequest := httptest.NewRequest(http.MethodPost, "/events/lark", nil)
	if err := plainClient.Verify(ctx, plainRequest, messageBody); err != nil {
		t.Fatalf("plain verify=%v", err)
	}
	events, err := plainClient.Decode(ctx, plainRequest, messageBody)
	if err != nil || len(events) != 1 || events[0].Type != "lark.im.message.receive_v1" || events[0].Platform != "lark" || events[0].AccountID != "main" {
		t.Fatalf("message events=%#v err=%v", events, err)
	}
	payload, ok := events[0].Payload.(EventPayload)
	if !ok || payload.Message == nil || payload.Message.ID != testMessageID || payload.Message.Text == nil || *payload.Message.Text != "hello" || payload.Message.Direction != socialhub.DirectionInbound {
		t.Fatalf("message payload=%#v", events[0].Payload)
	}

	reactionBody := larkEventBody(t, "evt_reaction", "im.message.reaction.created_v1", map[string]any{
		"message_id": testMessageID, "user_id": map[string]string{"open_id": "ou_sender"},
		"operator_type": "user", "action_time": "1785571200000", "reaction_type": map[string]string{"emoji_type": "EYES"},
	})
	events, err = plainClient.Decode(ctx, plainRequest, reactionBody)
	if err != nil || len(events) != 1 {
		t.Fatalf("reaction events=%#v err=%v", events, err)
	}
	payload = events[0].Payload.(EventPayload)
	if payload.Reaction == nil || payload.Reaction.MessageID != testMessageID || payload.Reaction.ActorID != "ou_sender" || payload.Reaction.EmojiType != "EYES" || !payload.Reaction.Added {
		t.Fatalf("reaction payload=%#v", payload)
	}

	unknownBody := larkEventBody(t, "evt_unknown", "contact.user.updated_v3", map[string]any{"user_id": testUserID})
	events, err = plainClient.Decode(ctx, plainRequest, unknownBody)
	if err != nil || len(events) != 1 {
		t.Fatalf("unknown events=%#v err=%v", events, err)
	}
	payload = events[0].Payload.(EventPayload)
	if payload.Message != nil || payload.Reaction != nil || !strings.Contains(string(payload.Raw), testUserID) {
		t.Fatalf("unknown payload=%#v", payload)
	}

	encryptedValue := encryptLarkEvent(t, messageBody, "encrypt-key-for-contract-tests")
	encryptedBody, _ := json.Marshal(map[string]string{"encrypt": encryptedValue})
	encryptedRequest := signedLarkRequest(encryptedBody, "encrypt-key-for-contract-tests")
	if err := encryptedClient.Verify(ctx, encryptedRequest, encryptedBody); err != nil {
		t.Fatalf("encrypted verify=%v", err)
	}
	events, err = encryptedClient.Decode(ctx, encryptedRequest, encryptedBody)
	if err != nil || len(events) != 1 || events[0].ID != "evt_message" {
		t.Fatalf("encrypted events=%#v err=%v", events, err)
	}
}

func TestWebhookChallenge(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	_, client := newTestClient(t, server, TokenTenant, testChatID, testActorID, false)
	body := []byte(`{"type":"url_verification","token":"verification-token","challenge":"challenge-value"}`)
	request := httptest.NewRequest(http.MethodPost, "/events/lark", bytes.NewReader(body))
	status, response, err := client.HandleChallenge(context.Background(), request)
	if err != nil || status != http.StatusOK || string(response) != `{"challenge":"challenge-value"}` {
		t.Fatalf("challenge status=%d response=%s err=%v", status, response, err)
	}
	content, _ := ioReadAll(request)
	if !bytes.Equal(content, body) {
		t.Fatalf("request body was not restored: %q", content)
	}

	if status, _, err := client.HandleChallenge(context.Background(), nil); status != http.StatusBadRequest || !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("nil challenge status=%d err=%v", status, err)
	}
	nonChallenge := larkEventBody(t, "evt_unknown", "contact.user.updated_v3", map[string]any{"user_id": testUserID})
	request = httptest.NewRequest(http.MethodPost, "/events/lark", bytes.NewReader(nonChallenge))
	if status, _, err := client.HandleChallenge(context.Background(), request); status != http.StatusBadRequest || !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("non-challenge status=%d err=%v", status, err)
	}
}

func ioReadAll(request *http.Request) ([]byte, error) {
	if request == nil || request.Body == nil {
		return nil, nil
	}
	var buffer bytes.Buffer
	_, err := buffer.ReadFrom(request.Body)
	return buffer.Bytes(), err
}

func TestWebhookValidation(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	_, plainClient := newTestClient(t, server, TokenTenant, testChatID, testActorID, false)
	_, encryptedClient := newTestClient(t, server, TokenTenant, testChatID, testActorID, true)
	noToken := &Client{}
	ctx := context.Background()
	valid := larkEventBody(t, "evt_unknown", "contact.user.updated_v3", map[string]any{"user_id": testUserID})

	if err := noToken.Verify(ctx, httptest.NewRequest(http.MethodPost, "/", nil), valid); !errors.Is(err, socialhub.ErrUnsupported) {
		t.Fatalf("verify without token=%v", err)
	}
	if _, err := noToken.Decode(ctx, nil, valid); !errors.Is(err, socialhub.ErrUnsupported) {
		t.Fatalf("decode without token=%v", err)
	}
	if err := plainClient.Verify(ctx, nil, valid); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("nil verify=%v", err)
	}
	if err := plainClient.Verify(ctx, httptest.NewRequest(http.MethodGet, "/", nil), valid); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("GET verify=%v", err)
	}
	if err := encryptedClient.Verify(ctx, httptest.NewRequest(http.MethodPost, "/", nil), valid); !errors.Is(err, socialhub.ErrPermissionDenied) {
		t.Fatalf("missing signature=%v", err)
	}
	badSignature := signedLarkRequest(valid, "wrong-key")
	if err := encryptedClient.Verify(ctx, badSignature, valid); !errors.Is(err, socialhub.ErrPermissionDenied) {
		t.Fatalf("bad signature=%v", err)
	}

	wrongToken := []byte(`{"type":"url_verification","token":"wrong","challenge":"x"}`)
	if err := plainClient.Verify(ctx, httptest.NewRequest(http.MethodPost, "/", nil), wrongToken); !errors.Is(err, socialhub.ErrPermissionDenied) {
		t.Fatalf("wrong token verify=%v", err)
	}
	invalidBodies := [][]byte{
		nil,
		[]byte("{"),
		[]byte(`{"schema":"2.0","header":{"event_id":"evt","event_type":"x","token":"wrong"},"event":{}}`),
		[]byte(`{"schema":"2.0","header":{"event_id":"","event_type":"x","token":"verification-token"},"event":{}}`),
		larkEventBody(t, "evt_bad_message", "im.message.receive_v1", map[string]any{"message": map[string]string{"message_id": "bad/id", "chat_id": "bad"}}),
		larkEventBody(t, "evt_bad_reaction", "im.message.reaction.created_v1", map[string]any{"message_id": testMessageID, "reaction_type": map[string]string{}}),
	}
	for index, body := range invalidBodies {
		if _, err := plainClient.Decode(ctx, nil, body); err == nil {
			t.Fatalf("invalid decode %d accepted", index)
		}
	}
	wrongApp := larkEventBody(t, "evt_wrong_app", "contact.user.updated_v3", map[string]any{"user_id": testUserID})
	wrongApp = bytes.Replace(wrongApp, []byte(testAppID), []byte("cli_wrongapp"), 1)
	if _, err := plainClient.Decode(ctx, nil, wrongApp); !errors.Is(err, socialhub.ErrPermissionDenied) {
		t.Fatalf("wrong app decode=%v", err)
	}

	encryptedValue := encryptLarkEvent(t, valid, "encrypt-key-for-contract-tests")
	encryptedBody, _ := json.Marshal(map[string]string{"encrypt": encryptedValue})
	if _, err := plainClient.Decode(ctx, nil, encryptedBody); !errors.Is(err, socialhub.ErrUnauthenticated) {
		t.Fatalf("encrypted without key=%v", err)
	}
	if _, err := encryptedClient.Decode(ctx, nil, valid); !errors.Is(err, socialhub.ErrPermissionDenied) {
		t.Fatalf("plaintext with key=%v", err)
	}
	if _, err := encryptedClient.Decode(ctx, nil, []byte(`{"encrypt":"not-base64"}`)); !errors.Is(err, socialhub.ErrPermissionDenied) {
		t.Fatalf("malformed encrypted body=%v", err)
	}
	if _, err := decryptEvent(base64.StdEncoding.EncodeToString(make([]byte, 32)), "secret"); err == nil {
		t.Fatal("invalid encrypted padding accepted")
	}
}

func TestLarkAPIErrorMapping(t *testing.T) {
	tests := []struct {
		platformCode int
		status       int
		code         socialhub.ErrorCode
		class        socialhub.ErrorClass
	}{
		{99991400, http.StatusOK, socialhub.CodeRateLimited, socialhub.ClassRetryable},
		{99991663, http.StatusOK, socialhub.CodeUnauthenticated, socialhub.ClassUserAction},
		{99991672, http.StatusOK, socialhub.CodeApprovalRequired, socialhub.ClassUserAction},
		{230002, http.StatusOK, socialhub.CodePermissionDenied, socialhub.ClassUserAction},
		{41050, http.StatusBadRequest, socialhub.CodePermissionDenied, socialhub.ClassUserAction},
		{230011, http.StatusOK, socialhub.CodeNotFound, socialhub.ClassPermanent},
		{230001, http.StatusOK, socialhub.CodeInvalidArgument, socialhub.ClassPermanent},
		{234096, http.StatusOK, socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable},
		{0, http.StatusConflict, socialhub.CodeConflict, socialhub.ClassPermanent},
		{0, http.StatusTeapot, socialhub.CodePlatformError, socialhub.ClassPermanent},
	}
	for _, test := range tests {
		gotCode, gotClass := classifyError(test.status, test.platformCode)
		if gotCode != test.code || gotClass != test.class {
			t.Fatalf("classify status=%d platform=%d got=%s/%s", test.status, test.platformCode, gotCode, gotClass)
		}
	}

	code := 99991672
	header := make(http.Header)
	header.Set("X-Tt-Logid", "log-id")
	header.Set("Retry-After", "3")
	err := apiResponseError("operation", http.StatusForbidden, header, apiEnvelope{Code: &code, Msg: strings.Repeat("界", 513)})
	platformErr := requireErrorCode(t, err, socialhub.CodeApprovalRequired)
	if platformErr.Op != "operation" || platformErr.PlatformCode != "99991672" || platformErr.RequestID != "log-id" || platformErr.RetryAfter != 3*time.Second || len([]rune(platformErr.PlatformMessage)) != 512 || platformErr.ApprovalURL == "" {
		t.Fatalf("business error=%#v", platformErr)
	}

	header = make(http.Header)
	header.Set("X-Request-ID", "request-id")
	header.Set("Retry-After", "2")
	err = decodeHTTPError(http.StatusTooManyRequests, header, []byte(`{"code":99991400,"msg":"too fast"}`))
	platformErr = requireErrorCode(t, err, socialhub.CodeRateLimited)
	if platformErr.RequestID != "request-id" || platformErr.RetryAfter != 2*time.Second || !platformErr.Retryable() {
		t.Fatalf("HTTP envelope error=%#v", platformErr)
	}
	err = decodeHTTPError(http.StatusBadGateway, nil, []byte("not-json"))
	if platformErr = requireErrorCode(t, err, socialhub.CodeTemporarilyUnavailable); platformErr.Class != socialhub.ClassRetryable {
		t.Fatalf("HTTP fallback error=%#v", platformErr)
	}
	if retryAfter("later") != 0 || retryAfter("0") != 0 || retryAfter("86401") != 0 || retryAfter("4") != 4*time.Second || larkRequestID(http.Header{"X-Tt-Trace-Id": {"trace-id"}}) != "trace-id" {
		t.Fatal("error helper mismatch")
	}
}

func TestLarkCallEnvelopeErrors(t *testing.T) {
	mode := "business"
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch mode {
		case "business":
			writeTestJSON(t, writer, map[string]any{"code": 99991400, "msg": "rate limited"})
		case "malformed":
			_, _ = writer.Write([]byte(`{"code":"bad"}`))
		case "wrong-output":
			writeTestJSON(t, writer, map[string]any{"code": 0, "data": "text"})
		case "ok":
			writeTestJSON(t, writer, map[string]any{"code": 0})
		}
	}))
	defer server.Close()
	_, client := newTestClient(t, server, TokenTenant, testChatID, testActorID, false)
	var output struct {
		Data struct {
			Value int `json:"value"`
		} `json:"data"`
	}
	err := client.get(context.Background(), "test.operation", "/test", nil, &output)
	platformErr := requireErrorCode(t, err, socialhub.CodeRateLimited)
	if platformErr.Op != "test.operation" {
		t.Fatalf("operation=%q", platformErr.Op)
	}
	mode = "malformed"
	requireErrorCode(t, client.get(context.Background(), "test.operation", "/test", nil, &output), socialhub.CodePlatformError)
	mode = "wrong-output"
	requireErrorCode(t, client.get(context.Background(), "test.operation", "/test", nil, &output), socialhub.CodePlatformError)
	mode = "ok"
	if err := client.call(context.Background(), "test.operation", http.MethodDelete, "/test", nil, nil, nil, false); err != nil {
		t.Fatalf("nil output=%v", err)
	}
}
