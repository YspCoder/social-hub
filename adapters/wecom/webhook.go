package wecom

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"social-hub/pkg/socialhub"
)

const maxWebhookBodyBytes = 1 << 20

type encryptedEnvelope struct {
	ToUserName string `xml:"ToUserName"`
	Encrypt    string `xml:"Encrypt"`
	AgentID    int64  `xml:"AgentID"`
}

// Verify authenticates an encrypted WeCom callback without decoding its
// application message.
func (c *Client) Verify(_ context.Context, request *http.Request, body []byte) error {
	if c.webhookToken == "" || c.aesKey == "" {
		return unsupported("webhook_verify", "webhook token and AES key are not configured")
	}
	if request == nil || request.Method != http.MethodPost || len(body) == 0 || len(body) > maxWebhookBodyBytes {
		return invalidArgument("webhook_verify", "WeCom webhook must be a bounded, non-empty POST body")
	}
	envelope, err := decodeEncryptedEnvelope(body, "webhook_verify")
	if err != nil {
		return err
	}
	query := request.URL.Query()
	if !validWebhookSignature(query.Get("msg_signature"), c.webhookToken, query.Get("timestamp"), query.Get("nonce"), envelope.Encrypt) {
		return platformError("webhook_verify", socialhub.CodePermissionDenied, socialhub.ClassPermanent, nil)
	}
	return nil
}

// Decode decrypts and normalizes one verified WeCom callback.
func (c *Client) Decode(_ context.Context, request *http.Request, body []byte) ([]socialhub.Event, error) {
	if c.webhookToken == "" || c.aesKey == "" {
		return nil, unsupported("webhook_decode", "webhook token and AES key are not configured")
	}
	if request == nil || request.Method != http.MethodPost || len(body) == 0 || len(body) > maxWebhookBodyBytes {
		return nil, invalidArgument("webhook_decode", "WeCom webhook must be a bounded, non-empty POST body")
	}
	envelope, err := decodeEncryptedEnvelope(body, "webhook_decode")
	if err != nil {
		return nil, err
	}
	plaintext, err := decryptWebhookPayload(c.aesKey, c.corpID, envelope.Encrypt)
	if err != nil {
		return nil, err
	}
	var message IncomingMessage
	if err := xml.Unmarshal(plaintext, &message); err != nil {
		return nil, platformError("webhook_decode", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	if strings.TrimSpace(message.MessageType) == "" {
		return nil, invalidArgument("webhook_decode", "decrypted callback is missing MsgType")
	}
	message.Raw = append([]byte(nil), plaintext...)
	eventType := "wecom.message." + strings.ToLower(message.MessageType)
	if strings.EqualFold(message.MessageType, "event") {
		if strings.TrimSpace(message.Event) == "" {
			return nil, invalidArgument("webhook_decode", "event callback is missing Event")
		}
		eventType = "wecom.event." + strings.ToLower(message.Event)
	}
	eventID := message.MessageID
	if eventID == "" {
		digest := sha256.Sum256(plaintext)
		eventID = hex.EncodeToString(digest[:])
	}
	return []socialhub.Event{{
		ID: eventID, Type: eventType, Platform: "wecom", AccountID: c.accountID, Payload: message,
	}}, nil
}

// HandleChallenge verifies and decrypts the echostr used when configuring a
// WeCom callback URL.
func (c *Client) HandleChallenge(_ context.Context, request *http.Request) (int, []byte, error) {
	if c.webhookToken == "" || c.aesKey == "" {
		return http.StatusBadRequest, nil, unsupported("webhook_challenge", "webhook token and AES key are not configured")
	}
	if request == nil || request.Method != http.MethodGet {
		return http.StatusBadRequest, nil, invalidArgument("webhook_challenge", "WeCom webhook challenge must use GET")
	}
	query := request.URL.Query()
	echo := query.Get("echostr")
	if echo == "" || len(echo) > maxWebhookBodyBytes ||
		!validWebhookSignature(query.Get("msg_signature"), c.webhookToken, query.Get("timestamp"), query.Get("nonce"), echo) {
		return http.StatusForbidden, nil, platformError("webhook_challenge", socialhub.CodePermissionDenied, socialhub.ClassPermanent, nil)
	}
	plaintext, err := decryptWebhookPayload(c.aesKey, c.corpID, echo)
	if err != nil {
		return http.StatusForbidden, nil, err
	}
	return http.StatusOK, plaintext, nil
}

func decodeEncryptedEnvelope(body []byte, operation string) (encryptedEnvelope, error) {
	var envelope encryptedEnvelope
	if err := xml.Unmarshal(body, &envelope); err != nil {
		return encryptedEnvelope{}, platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	if strings.TrimSpace(envelope.Encrypt) == "" || len(envelope.Encrypt) > maxWebhookBodyBytes {
		return encryptedEnvelope{}, invalidArgument(operation, "encrypted callback payload is required")
	}
	return envelope, nil
}

func validWebhookSignature(signature, token, timestamp, nonce, encrypted string) bool {
	if signature == "" || token == "" || timestamp == "" || nonce == "" || encrypted == "" {
		return false
	}
	parts := []string{token, timestamp, nonce, encrypted}
	sort.Strings(parts)
	digest := sha1.Sum([]byte(strings.Join(parts, "")))
	expected := hex.EncodeToString(digest[:])
	return hmac.Equal([]byte(signature), []byte(expected))
}

func decodeAESKey(encoded string) ([]byte, error) {
	if strings.TrimSpace(encoded) != encoded || len(encoded) != 43 {
		return nil, invalidArgument("webhook_aes_key", "EncodingAESKey must contain exactly 43 base64 characters")
	}
	key, err := base64.RawStdEncoding.DecodeString(encoded)
	if err != nil || len(key) != 32 {
		return nil, invalidArgument("webhook_aes_key", "EncodingAESKey must decode to 32 bytes")
	}
	return key, nil
}

func decryptWebhookPayload(encodedKey, corpID, encrypted string) ([]byte, error) {
	key, err := decodeAESKey(encodedKey)
	if err != nil {
		return nil, err
	}
	ciphertext, err := base64.StdEncoding.DecodeString(encrypted)
	if err != nil || len(ciphertext) == 0 || len(ciphertext)%aes.BlockSize != 0 {
		return nil, platformError("webhook_decrypt", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, fmt.Errorf("invalid encrypted payload"))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, platformError("webhook_decrypt", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	plaintext := make([]byte, len(ciphertext))
	cipher.NewCBCDecrypter(block, key[:aes.BlockSize]).CryptBlocks(plaintext, ciphertext)
	plaintext, err = unpadWebhookPKCS7(plaintext)
	if err != nil {
		return nil, platformError("webhook_decrypt", socialhub.CodePermissionDenied, socialhub.ClassPermanent, err)
	}
	if len(plaintext) < 20 {
		return nil, platformError("webhook_decrypt", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, fmt.Errorf("decrypted payload is too short"))
	}
	messageLength := int(binary.BigEndian.Uint32(plaintext[16:20]))
	if messageLength > len(plaintext)-20 {
		return nil, platformError("webhook_decrypt", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, fmt.Errorf("invalid message length"))
	}
	messageEnd := 20 + messageLength
	if !hmac.Equal(plaintext[messageEnd:], []byte(corpID)) {
		return nil, platformError("webhook_decrypt", socialhub.CodePermissionDenied, socialhub.ClassPermanent, fmt.Errorf("CorpID mismatch"))
	}
	return plaintext[20:messageEnd], nil
}

func unpadWebhookPKCS7(input []byte) ([]byte, error) {
	if len(input) == 0 {
		return nil, fmt.Errorf("empty padded payload")
	}
	padding := int(input[len(input)-1])
	if padding == 0 || padding > 32 || padding > len(input) {
		return nil, fmt.Errorf("invalid PKCS7 padding")
	}
	for _, value := range input[len(input)-padding:] {
		if int(value) != padding {
			return nil, fmt.Errorf("invalid PKCS7 padding")
		}
	}
	return input[:len(input)-padding], nil
}

var _ socialhub.WebhookHandler = (*Client)(nil)
var _ socialhub.ChallengeHandler = (*Client)(nil)
