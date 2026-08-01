package wecom

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sort"
	"strings"
	"testing"

	"social-hub/pkg/socialhub"
)

const testMessageXML = `<xml><ToUserName><![CDATA[ww-corp-id]]></ToUserName><FromUserName><![CDATA[alice]]></FromUserName><CreateTime>1785542400</CreateTime><MsgType><![CDATA[text]]></MsgType><Content><![CDATA[hello]]></Content><MsgId>123456789</MsgId><AgentID>1000002</AgentID></xml>`

const testEventXML = `<xml><ToUserName><![CDATA[ww-corp-id]]></ToUserName><FromUserName><![CDATA[alice]]></FromUserName><CreateTime>1785542400</CreateTime><MsgType><![CDATA[event]]></MsgType><Event><![CDATA[change_contact]]></Event><ChangeType><![CDATA[update_user]]></ChangeType><AgentID>1000002</AgentID></xml>`

func TestWebhookChallengeVerifyDecodeAndStableEventID(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	_, client := newTestClient(t, server, true)
	timestamp, nonce := "1785542400", "nonce-1"

	echo := encryptTestWebhookPayload(t, testAESKey, "ww-corp-id", []byte("challenge-ok"))
	challengeQuery := url.Values{
		"msg_signature": {webhookTestSignature("webhook-token", timestamp, nonce, echo)},
		"timestamp":     {timestamp}, "nonce": {nonce}, "echostr": {echo},
	}
	challenge := httptest.NewRequest(http.MethodGet, "/callback?"+challengeQuery.Encode(), nil)
	status, response, err := client.HandleChallenge(context.Background(), challenge)
	if err != nil || status != http.StatusOK || string(response) != "challenge-ok" {
		t.Fatalf("challenge status=%d response=%q err=%v", status, response, err)
	}

	encrypted := encryptTestWebhookPayload(t, testAESKey, "ww-corp-id", []byte(testMessageXML))
	envelope, err := xml.Marshal(encryptedEnvelope{ToUserName: "ww-corp-id", Encrypt: encrypted, AgentID: 1000002})
	if err != nil {
		t.Fatal(err)
	}
	callbackQuery := url.Values{
		"msg_signature": {webhookTestSignature("webhook-token", timestamp, nonce, encrypted)},
		"timestamp":     {timestamp}, "nonce": {nonce},
	}
	request := httptest.NewRequest(http.MethodPost, "/callback?"+callbackQuery.Encode(), bytes.NewReader(envelope))
	if err := client.Verify(context.Background(), request, envelope); err != nil {
		t.Fatal(err)
	}
	events, err := client.Decode(context.Background(), request, envelope)
	if err != nil || len(events) != 1 || events[0].ID != "123456789" || events[0].Type != "wecom.message.text" || events[0].Platform != "wecom" || events[0].AccountID != "main" {
		t.Fatalf("events=%#v err=%v", events, err)
	}
	payload, ok := events[0].Payload.(IncomingMessage)
	if !ok || payload.Content != "hello" || payload.AgentID != 1000002 || !bytes.Equal(payload.Raw, []byte(testMessageXML)) {
		t.Fatalf("payload=%#v", events[0].Payload)
	}

	eventEncrypted := encryptTestWebhookPayload(t, testAESKey, "ww-corp-id", []byte(testEventXML))
	eventEnvelope, _ := xml.Marshal(encryptedEnvelope{Encrypt: eventEncrypted})
	eventRequest := httptest.NewRequest(http.MethodPost, "/callback", bytes.NewReader(eventEnvelope))
	first, err := client.Decode(context.Background(), eventRequest, eventEnvelope)
	if err != nil {
		t.Fatal(err)
	}
	second, err := client.Decode(context.Background(), eventRequest, eventEnvelope)
	if err != nil || len(first) != 1 || len(second) != 1 || first[0].ID != second[0].ID || len(first[0].ID) != 64 || first[0].Type != "wecom.event.change_contact" {
		t.Fatalf("stable events first=%#v second=%#v err=%v", first, second, err)
	}
}

func TestWebhookRejectsBadSignatureCorpIDAndPadding(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	_, client := newTestClient(t, server, true)
	timestamp, nonce := "1785542400", "nonce-2"
	encrypted := encryptTestWebhookPayload(t, testAESKey, "ww-corp-id", []byte(testMessageXML))
	envelope, _ := xml.Marshal(encryptedEnvelope{Encrypt: encrypted})
	request := httptest.NewRequest(http.MethodPost, "/callback?timestamp="+timestamp+"&nonce="+nonce+"&msg_signature=bad", bytes.NewReader(envelope))
	if err := client.Verify(context.Background(), request, envelope); !errors.Is(err, socialhub.ErrPermissionDenied) {
		t.Fatalf("bad signature=%v", err)
	}

	wrongCorp := encryptTestWebhookPayload(t, testAESKey, "other-corp", []byte(testMessageXML))
	wrongEnvelope, _ := xml.Marshal(encryptedEnvelope{Encrypt: wrongCorp})
	if _, err := client.Decode(context.Background(), request, wrongEnvelope); !errors.Is(err, socialhub.ErrPermissionDenied) {
		t.Fatalf("wrong CorpID=%v", err)
	}

	ciphertext, err := base64.StdEncoding.DecodeString(encrypted)
	if err != nil || len(ciphertext) < 2*aes.BlockSize {
		t.Fatal("test ciphertext is too short")
	}
	ciphertext[len(ciphertext)-aes.BlockSize-1] ^= 1
	badPadding, _ := xml.Marshal(encryptedEnvelope{Encrypt: base64.StdEncoding.EncodeToString(ciphertext)})
	if _, err := client.Decode(context.Background(), request, badPadding); !errors.Is(err, socialhub.ErrPermissionDenied) {
		t.Fatalf("bad padding=%v", err)
	}
}

func TestWebhookValidationErrors(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	_, client := newTestClient(t, server, true)
	_, noWebhook := newTestClient(t, server, false)
	post := httptest.NewRequest(http.MethodPost, "/callback", nil)
	get := httptest.NewRequest(http.MethodGet, "/callback", nil)
	if err := noWebhook.Verify(context.Background(), post, []byte("x")); !errors.Is(err, socialhub.ErrUnsupported) {
		t.Fatalf("unconfigured verify=%v", err)
	}
	if _, err := noWebhook.Decode(context.Background(), post, []byte("x")); !errors.Is(err, socialhub.ErrUnsupported) {
		t.Fatalf("unconfigured decode=%v", err)
	}
	if status, _, err := noWebhook.HandleChallenge(context.Background(), get); status != http.StatusBadRequest || !errors.Is(err, socialhub.ErrUnsupported) {
		t.Fatalf("unconfigured challenge status=%d err=%v", status, err)
	}
	if err := client.Verify(context.Background(), nil, nil); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("nil verify=%v", err)
	}
	if _, err := client.Decode(context.Background(), get, []byte("x")); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("GET decode=%v", err)
	}
	if err := client.Verify(context.Background(), post, []byte(`<xml/>`)); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("empty envelope=%v", err)
	}
	if err := client.Verify(context.Background(), post, []byte(`<xml>`)); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("malformed envelope=%v", err)
	}
	if status, _, err := client.HandleChallenge(context.Background(), post); status != http.StatusBadRequest || !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("POST challenge status=%d err=%v", status, err)
	}
	if status, _, err := client.HandleChallenge(context.Background(), get); status != http.StatusForbidden || !errors.Is(err, socialhub.ErrPermissionDenied) {
		t.Fatalf("unsigned challenge status=%d err=%v", status, err)
	}
	if _, err := decodeAESKey("bad"); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("bad AES key=%v", err)
	}
	if validWebhookSignature("", "token", "time", "nonce", "encrypted") {
		t.Fatal("empty signature accepted")
	}
	if _, err := decryptWebhookPayload(testAESKey, "ww-corp-id", "not-base64"); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("invalid ciphertext=%v", err)
	}
	missingType := encryptTestWebhookPayload(t, testAESKey, "ww-corp-id", []byte(`<xml/>`))
	missingEnvelope, _ := xml.Marshal(encryptedEnvelope{Encrypt: missingType})
	if _, err := client.Decode(context.Background(), post, missingEnvelope); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("missing message type=%v", err)
	}
	missingEvent := encryptTestWebhookPayload(t, testAESKey, "ww-corp-id", []byte(`<xml><MsgType>event</MsgType></xml>`))
	missingEventEnvelope, _ := xml.Marshal(encryptedEnvelope{Encrypt: missingEvent})
	if _, err := client.Decode(context.Background(), post, missingEventEnvelope); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("missing event=%v", err)
	}
	oversized := []byte(strings.Repeat("x", maxWebhookBodyBytes+1))
	if err := client.Verify(context.Background(), post, oversized); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("oversized callback=%v", err)
	}
}

func webhookTestSignature(token, timestamp, nonce, encrypted string) string {
	parts := []string{token, timestamp, nonce, encrypted}
	sort.Strings(parts)
	digest := sha1.Sum([]byte(strings.Join(parts, "")))
	return hex.EncodeToString(digest[:])
}

func encryptTestWebhookPayload(t *testing.T, encodedKey, corpID string, message []byte) string {
	t.Helper()
	key, err := base64.RawStdEncoding.DecodeString(encodedKey)
	if err != nil {
		t.Fatal(err)
	}
	plaintext := make([]byte, 20+len(message)+len(corpID))
	copy(plaintext[:16], []byte("0123456789abcdef"))
	binary.BigEndian.PutUint32(plaintext[16:20], uint32(len(message)))
	copy(plaintext[20:], message)
	copy(plaintext[20+len(message):], corpID)
	padding := 32 - len(plaintext)%32
	plaintext = append(plaintext, bytes.Repeat([]byte{byte(padding)}, padding)...)
	block, err := aes.NewCipher(key)
	if err != nil {
		t.Fatal(err)
	}
	ciphertext := make([]byte, len(plaintext))
	cipher.NewCBCEncrypter(block, key[:aes.BlockSize]).CryptBlocks(ciphertext, plaintext)
	return base64.StdEncoding.EncodeToString(ciphertext)
}
