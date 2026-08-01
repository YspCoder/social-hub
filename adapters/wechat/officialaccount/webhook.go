package officialaccount

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/sha1"
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

// IncomingMessage is the common subset of Official Account callback XML.
type IncomingMessage struct {
	ToUserName   string `xml:"ToUserName" json:"to_user_name"`
	FromUserName string `xml:"FromUserName" json:"from_user_name"`
	CreateTime   int64  `xml:"CreateTime" json:"create_time"`
	MessageType  string `xml:"MsgType" json:"message_type"`
	Content      string `xml:"Content" json:"content,omitempty"`
	MessageID    string `xml:"MsgId" json:"message_id,omitempty"`
	MediaID      string `xml:"MediaId" json:"media_id,omitempty"`
	Event        string `xml:"Event" json:"event,omitempty"`
	EventKey     string `xml:"EventKey" json:"event_key,omitempty"`
}

type encryptedEnvelope struct {
	Encrypt string `xml:"Encrypt"`
}

func (c *Client) Verify(_ context.Context, request *http.Request, body []byte) error {
	if c.webhookToken == "" {
		return &socialhub.Error{Code: socialhub.CodeUnsupported, Class: socialhub.ClassPermanent, Platform: "wechat", Product: "official-account", Op: "webhook_verify", PlatformMessage: "webhook token is not configured"}
	}
	query := request.URL.Query()
	timestamp, nonce := query.Get("timestamp"), query.Get("nonce")
	if timestamp == "" || nonce == "" {
		return wrapError("webhook_verify", socialhub.CodePermissionDenied, socialhub.ClassPermanent, nil)
	}
	if query.Get("encrypt_type") == "aes" {
		var envelope encryptedEnvelope
		if err := xml.Unmarshal(body, &envelope); err != nil || envelope.Encrypt == "" {
			return wrapError("webhook_verify", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
		if !validSignature(query.Get("msg_signature"), c.webhookToken, timestamp, nonce, envelope.Encrypt) {
			return wrapError("webhook_verify", socialhub.CodePermissionDenied, socialhub.ClassPermanent, nil)
		}
		return nil
	}
	if !validSignature(query.Get("signature"), c.webhookToken, timestamp, nonce) {
		return wrapError("webhook_verify", socialhub.CodePermissionDenied, socialhub.ClassPermanent, nil)
	}
	return nil
}

func (c *Client) Decode(_ context.Context, request *http.Request, body []byte) ([]socialhub.Event, error) {
	payload := body
	if request.URL.Query().Get("encrypt_type") == "aes" {
		var envelope encryptedEnvelope
		if err := xml.Unmarshal(body, &envelope); err != nil {
			return nil, wrapError("webhook_decode", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
		decrypted, err := decryptMessage(c.aesKey, c.appID, envelope.Encrypt)
		if err != nil {
			return nil, err
		}
		payload = decrypted
	}
	var message IncomingMessage
	if err := xml.Unmarshal(payload, &message); err != nil {
		return nil, wrapError("webhook_decode", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	eventType := "message." + message.MessageType
	if message.MessageType == "event" {
		eventType = "event." + strings.ToLower(message.Event)
	}
	eventID := message.MessageID
	if eventID == "" {
		digest := sha1.Sum(payload)
		eventID = hex.EncodeToString(digest[:])
	}
	return []socialhub.Event{{ID: eventID, Type: eventType, Platform: "wechat", AccountID: c.accountID, Payload: message}}, nil
}

// HandleChallenge validates the initial server URL challenge.
func (c *Client) HandleChallenge(_ context.Context, request *http.Request) (int, []byte, error) {
	query := request.URL.Query()
	timestamp, nonce, echo := query.Get("timestamp"), query.Get("nonce"), query.Get("echostr")
	if query.Get("encrypt_type") == "aes" {
		if !validSignature(query.Get("msg_signature"), c.webhookToken, timestamp, nonce, echo) {
			return http.StatusForbidden, nil, wrapError("webhook_challenge", socialhub.CodePermissionDenied, socialhub.ClassPermanent, nil)
		}
		decrypted, err := decryptMessage(c.aesKey, c.appID, echo)
		if err != nil {
			return http.StatusForbidden, nil, err
		}
		return http.StatusOK, decrypted, nil
	}
	if !validSignature(query.Get("signature"), c.webhookToken, timestamp, nonce) {
		return http.StatusForbidden, nil, wrapError("webhook_challenge", socialhub.CodePermissionDenied, socialhub.ClassPermanent, nil)
	}
	return http.StatusOK, []byte(echo), nil
}

func validSignature(signature, token, timestamp, nonce string, extra ...string) bool {
	if signature == "" || token == "" || timestamp == "" || nonce == "" {
		return false
	}
	parts := append([]string{token, timestamp, nonce}, extra...)
	sort.Strings(parts)
	digest := sha1.Sum([]byte(strings.Join(parts, "")))
	expected := hex.EncodeToString(digest[:])
	return hmac.Equal([]byte(signature), []byte(expected))
}

func decryptMessage(encodedKey, appID, encrypted string) ([]byte, error) {
	if encodedKey == "" {
		return nil, wrapError("webhook_decrypt", socialhub.CodeUnauthenticated, socialhub.ClassUserAction, fmt.Errorf("AES key is not configured"))
	}
	key, err := base64.StdEncoding.DecodeString(encodedKey + "=")
	if err != nil || len(key) != 32 {
		return nil, wrapError("webhook_decrypt", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, fmt.Errorf("invalid AES key"))
	}
	ciphertext, err := base64.StdEncoding.DecodeString(encrypted)
	if err != nil || len(ciphertext) == 0 || len(ciphertext)%aes.BlockSize != 0 {
		return nil, wrapError("webhook_decrypt", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, fmt.Errorf("invalid encrypted payload"))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, wrapError("webhook_decrypt", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	plaintext := make([]byte, len(ciphertext))
	mode := cipher.NewCBCDecrypter(block, key[:aes.BlockSize])
	mode.CryptBlocks(plaintext, ciphertext)
	plaintext, err = unpadPKCS7(plaintext, 32)
	if err != nil {
		return nil, wrapError("webhook_decrypt", socialhub.CodePermissionDenied, socialhub.ClassPermanent, err)
	}
	if len(plaintext) < 20 {
		return nil, wrapError("webhook_decrypt", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, fmt.Errorf("decrypted payload is too short"))
	}
	messageLength := int(binary.BigEndian.Uint32(plaintext[16:20]))
	messageEnd := 20 + messageLength
	if messageLength < 0 || messageEnd > len(plaintext) {
		return nil, wrapError("webhook_decrypt", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, fmt.Errorf("invalid message length"))
	}
	if !bytes.Equal(plaintext[messageEnd:], []byte(appID)) {
		return nil, wrapError("webhook_decrypt", socialhub.CodePermissionDenied, socialhub.ClassPermanent, fmt.Errorf("app ID mismatch"))
	}
	return plaintext[20:messageEnd], nil
}

func unpadPKCS7(input []byte, blockSize int) ([]byte, error) {
	if len(input) == 0 {
		return nil, fmt.Errorf("empty padded payload")
	}
	padding := int(input[len(input)-1])
	if padding == 0 || padding > blockSize || padding > len(input) {
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
