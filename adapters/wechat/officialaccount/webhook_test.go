package officialaccount

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
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
)

const testMessageXML = `<xml><ToUserName><![CDATA[official]]></ToUserName><FromUserName><![CDATA[openid-1]]></FromUserName><CreateTime>1785542400</CreateTime><MsgType><![CDATA[text]]></MsgType><Content><![CDATA[hello]]></Content><MsgId>123456789</MsgId></xml>`

func TestPlaintextWebhookChallengeVerifyAndDecode(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	_, client := newTestClient(t, server)

	timestamp, nonce := "1785542400", "nonce-1"
	signature := webhookSignature("webhook-token", timestamp, nonce)
	challenge := httptest.NewRequest(http.MethodGet, "/callback?timestamp="+timestamp+"&nonce="+nonce+"&echostr=hello&signature="+signature, nil)
	status, body, err := client.HandleChallenge(context.Background(), challenge)
	if err != nil || status != http.StatusOK || string(body) != "hello" {
		t.Fatalf("challenge status=%d body=%q err=%v", status, body, err)
	}

	request := httptest.NewRequest(http.MethodPost, "/callback?timestamp="+timestamp+"&nonce="+nonce+"&signature="+signature, bytes.NewBufferString(testMessageXML))
	if err := client.Verify(context.Background(), request, []byte(testMessageXML)); err != nil {
		t.Fatal(err)
	}
	events, err := client.Decode(context.Background(), request, []byte(testMessageXML))
	if err != nil || len(events) != 1 || events[0].ID != "123456789" || events[0].Type != "message.text" {
		t.Fatalf("events=%#v err=%v", events, err)
	}
}

func TestAESWebhookVerifyDecodeAndAppIDValidation(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	_, client := newTestClient(t, server)

	encrypted := encryptTestMessage(t, testAESKey, "wx-app-id", []byte(testMessageXML))
	envelope, err := xml.Marshal(encryptedEnvelope{Encrypt: encrypted})
	if err != nil {
		t.Fatal(err)
	}
	timestamp, nonce := "1785542400", "nonce-2"
	signature := webhookSignature("webhook-token", timestamp, nonce, encrypted)
	request := httptest.NewRequest(http.MethodPost, "/callback?encrypt_type=aes&timestamp="+timestamp+"&nonce="+nonce+"&msg_signature="+signature, bytes.NewReader(envelope))
	if err := client.Verify(context.Background(), request, envelope); err != nil {
		t.Fatal(err)
	}
	events, err := client.Decode(context.Background(), request, envelope)
	if err != nil || len(events) != 1 || events[0].ID != "123456789" {
		t.Fatalf("events=%#v err=%v", events, err)
	}

	wrongAppPayload := encryptTestMessage(t, testAESKey, "other-app", []byte(testMessageXML))
	wrongEnvelope, err := xml.Marshal(encryptedEnvelope{Encrypt: wrongAppPayload})
	if err != nil {
		t.Fatal(err)
	}
	wrongRequest := httptest.NewRequest(http.MethodPost, "/callback?encrypt_type=aes", bytes.NewReader(wrongEnvelope))
	if _, err := client.Decode(context.Background(), wrongRequest, wrongEnvelope); err == nil {
		t.Fatal("expected app ID mismatch")
	}
}

func webhookSignature(parts ...string) string {
	values := append([]string(nil), parts...)
	sort.Strings(values)
	digest := sha1.Sum([]byte(strings.Join(values, "")))
	return hex.EncodeToString(digest[:])
}

func encryptTestMessage(t *testing.T, encodedKey, appID string, message []byte) string {
	t.Helper()
	key, err := base64.StdEncoding.DecodeString(encodedKey + "=")
	if err != nil {
		t.Fatal(err)
	}
	plain := make([]byte, 20+len(message)+len(appID))
	copy(plain[:16], []byte("0123456789abcdef"))
	binary.BigEndian.PutUint32(plain[16:20], uint32(len(message)))
	copy(plain[20:], message)
	copy(plain[20+len(message):], appID)
	padding := 32 - len(plain)%32
	plain = append(plain, bytes.Repeat([]byte{byte(padding)}, padding)...)
	block, err := aes.NewCipher(key)
	if err != nil {
		t.Fatal(err)
	}
	ciphertext := make([]byte, len(plain))
	cipher.NewCBCEncrypter(block, key[:aes.BlockSize]).CryptBlocks(ciphertext, plain)
	return base64.StdEncoding.EncodeToString(ciphertext)
}
