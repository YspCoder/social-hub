package lark

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"social-hub/pkg/socialhub"
)

const maxWebhookBodyBytes = 8 << 20

func (c *Client) Verify(_ context.Context, request *http.Request, body []byte) error {
	if c.verificationToken == "" {
		return unsupported("webhook_verify", "webhook.token_ref is not configured")
	}
	if request == nil || request.Method != http.MethodPost || len(body) == 0 || len(body) > maxWebhookBodyBytes {
		return invalidArgument("webhook_verify", "Open Platform webhook must be a bounded, non-empty POST body")
	}
	if c.encryptKey != "" {
		timestamp := request.Header.Get("X-Lark-Request-Timestamp")
		nonce := request.Header.Get("X-Lark-Request-Nonce")
		providedText := strings.TrimSpace(request.Header.Get("X-Lark-Signature"))
		provided, err := hex.DecodeString(providedText)
		if !validHeaderValue(timestamp) || !validHeaderValue(nonce) || err != nil || len(provided) != sha256.Size {
			return platformError("webhook_verify", socialhub.CodePermissionDenied, socialhub.ClassPermanent, err)
		}
		hash := sha256.Sum256([]byte(timestamp + nonce + c.encryptKey + string(body)))
		if !hmac.Equal(hash[:], provided) {
			return platformError("webhook_verify", socialhub.CodePermissionDenied, socialhub.ClassPermanent, nil)
		}
	}
	_, envelope, err := c.decodeEventBody(body)
	if err != nil {
		return err
	}
	if !secureEqual(eventToken(envelope), c.verificationToken) {
		return platformError("webhook_verify", socialhub.CodePermissionDenied, socialhub.ClassPermanent, nil)
	}
	return c.validateEventAccount(envelope, "webhook_verify")
}

func (c *Client) Decode(_ context.Context, _ *http.Request, body []byte) ([]socialhub.Event, error) {
	if c.verificationToken == "" {
		return nil, unsupported("webhook_decode", "webhook.token_ref is not configured")
	}
	if len(body) == 0 || len(body) > maxWebhookBodyBytes {
		return nil, invalidArgument("webhook_decode", "Open Platform webhook body must be bounded and non-empty")
	}
	plain, envelope, err := c.decodeEventBody(body)
	if err != nil {
		return nil, err
	}
	if !secureEqual(eventToken(envelope), c.verificationToken) {
		return nil, platformError("webhook_decode", socialhub.CodePermissionDenied, socialhub.ClassPermanent, nil)
	}
	if err := c.validateEventAccount(envelope, "webhook_decode"); err != nil {
		return nil, err
	}
	payload := EventPayload{Schema: envelope.Schema, Challenge: envelope.Challenge}
	if envelope.Header != nil {
		payload.ID, payload.Type = envelope.Header.EventID, envelope.Header.EventType
		payload.AppID, payload.TenantKey, payload.CreateTime = envelope.Header.AppID, envelope.Header.TenantKey, envelope.Header.CreateTime
	}
	if payload.ID == "" {
		payload.ID = envelope.UUID
	}
	eventType := payload.Type
	if envelope.Type == "url_verification" {
		if !validText(envelope.Challenge, 2048) {
			return nil, invalidArgument("webhook_decode", "url_verification challenge is required")
		}
		hash := sha256.Sum256([]byte(envelope.Challenge))
		payload.ID = "url_verification:" + hex.EncodeToString(hash[:8])
		payload.Type, eventType = "url_verification", "url_verification"
		payload.Raw = append(json.RawMessage(nil), plain...)
	} else {
		if eventType == "" && len(envelope.Event) > 0 {
			var header struct {
				Type string `json:"type"`
			}
			_ = json.Unmarshal(envelope.Event, &header)
			eventType, payload.Type = header.Type, header.Type
		}
		if !validOpaqueID(payload.ID, 512) || strings.TrimSpace(eventType) == "" || len(envelope.Event) == 0 {
			return nil, invalidArgument("webhook_decode", "event callback requires event ID, event type, and event body")
		}
		payload.Raw = append(json.RawMessage(nil), envelope.Event...)
		switch eventType {
		case "im.message.receive_v1":
			var event messageEvent
			if err := json.Unmarshal(envelope.Event, &event); err != nil || !validMessageID(event.Message.MessageID) || !validChatID(event.Message.ChatID) {
				return nil, platformError("webhook_decode", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
			}
			senderID := firstUserID(event.Sender.SenderID)
			wire := wireMessage{
				MessageID: event.Message.MessageID, RootID: event.Message.RootID, ParentID: event.Message.ParentID,
				ThreadID: event.Message.ThreadID, MessageType: event.Message.MessageType,
				CreateTime: event.Message.CreateTime, UpdateTime: event.Message.UpdateTime, ChatID: event.Message.ChatID,
				Sender: wireSender{ID: senderID, IDType: string(c.userIDType), SenderType: event.Sender.SenderType, TenantKey: event.Sender.TenantKey},
				Body:   wireMessageBody{Content: event.Message.Content},
			}
			payload.Message = mapMessage(c.accountID, c.actorID, wire)
		case "im.message.reaction.created_v1", "im.message.reaction.deleted_v1":
			var event reactionEvent
			if err := json.Unmarshal(envelope.Event, &event); err != nil || !validMessageID(event.MessageID) || !validText(event.ReactionType.EmojiType, 64) {
				return nil, platformError("webhook_decode", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
			}
			payload.Reaction = &ReactionEvent{
				MessageID: event.MessageID, EmojiType: event.ReactionType.EmojiType,
				ActorID: firstUserID(event.UserID), ActionTime: event.ActionTime,
				Added: eventType == "im.message.reaction.created_v1",
			}
		}
	}
	return []socialhub.Event{{
		ID: payload.ID, Type: "lark." + eventType, Platform: "lark", AccountID: c.accountID, Payload: payload,
	}}, nil
}

// HandleChallenge verifies a URL-verification callback and returns the JSON
// challenge response expected by Feishu and Lark.
func (c *Client) HandleChallenge(ctx context.Context, request *http.Request) (int, []byte, error) {
	if request == nil || request.Body == nil {
		return http.StatusBadRequest, nil, invalidArgument("webhook_challenge", "request body is required")
	}
	body, err := io.ReadAll(io.LimitReader(request.Body, maxWebhookBodyBytes+1))
	if err != nil || len(body) == 0 || len(body) > maxWebhookBodyBytes {
		request.Body = io.NopCloser(bytes.NewReader(body))
		return http.StatusBadRequest, nil, platformError("webhook_challenge", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	request.Body = io.NopCloser(bytes.NewReader(body))
	if err := c.Verify(ctx, request, body); err != nil {
		return http.StatusForbidden, nil, err
	}
	events, err := c.Decode(ctx, request, body)
	request.Body = io.NopCloser(bytes.NewReader(body))
	if err != nil || len(events) != 1 || events[0].Type != "lark.url_verification" {
		return http.StatusBadRequest, nil, firstError(err, invalidArgument("webhook_challenge", "callback is not URL verification"))
	}
	payload, ok := events[0].Payload.(EventPayload)
	if !ok || payload.Challenge == "" {
		return http.StatusBadRequest, nil, invalidArgument("webhook_challenge", "challenge payload is invalid")
	}
	response, err := json.Marshal(map[string]string{"challenge": payload.Challenge})
	if err != nil {
		return http.StatusInternalServerError, nil, platformError("webhook_challenge", socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	return http.StatusOK, response, nil
}

func (c *Client) decodeEventBody(body []byte) ([]byte, eventEnvelope, error) {
	var outer eventEnvelope
	if err := json.Unmarshal(body, &outer); err != nil {
		return nil, eventEnvelope{}, platformError("webhook_decode", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	plain := body
	if outer.Encrypt != "" {
		if c.encryptKey == "" {
			return nil, eventEnvelope{}, platformError("webhook_decrypt", socialhub.CodeUnauthenticated, socialhub.ClassUserAction, nil)
		}
		decrypted, err := decryptEvent(outer.Encrypt, c.encryptKey)
		if err != nil {
			return nil, eventEnvelope{}, platformError("webhook_decrypt", socialhub.CodePermissionDenied, socialhub.ClassPermanent, err)
		}
		plain = decrypted
		if err := json.Unmarshal(plain, &outer); err != nil {
			return nil, eventEnvelope{}, platformError("webhook_decrypt", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
	} else if c.encryptKey != "" {
		return nil, eventEnvelope{}, platformError("webhook_decrypt", socialhub.CodePermissionDenied, socialhub.ClassPermanent, nil)
	}
	return append([]byte(nil), plain...), outer, nil
}

func decryptEvent(value, secret string) ([]byte, error) {
	ciphertext, err := base64.StdEncoding.DecodeString(value)
	if err != nil || len(ciphertext) < 2*aes.BlockSize || (len(ciphertext)-aes.BlockSize)%aes.BlockSize != 0 {
		return nil, invalidArgument("webhook_decrypt", "encrypted event payload is malformed")
	}
	key := sha256.Sum256([]byte(secret))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, err
	}
	iv := ciphertext[:aes.BlockSize]
	plain := append([]byte(nil), ciphertext[aes.BlockSize:]...)
	cipher.NewCBCDecrypter(block, iv).CryptBlocks(plain, plain)
	padding := int(plain[len(plain)-1])
	if padding <= 0 || padding > aes.BlockSize || padding > len(plain) {
		return nil, invalidArgument("webhook_decrypt", "encrypted event padding is invalid")
	}
	for _, value := range plain[len(plain)-padding:] {
		if int(value) != padding {
			return nil, invalidArgument("webhook_decrypt", "encrypted event padding is invalid")
		}
	}
	plain = plain[:len(plain)-padding]
	if !json.Valid(plain) {
		return nil, invalidArgument("webhook_decrypt", "decrypted event is not JSON")
	}
	return plain, nil
}

func eventToken(envelope eventEnvelope) string {
	if envelope.Header != nil && envelope.Header.Token != "" {
		return envelope.Header.Token
	}
	return envelope.Token
}

func (c *Client) validateEventAccount(envelope eventEnvelope, operation string) error {
	if envelope.Header == nil {
		return nil
	}
	if c.appID != "" && envelope.Header.AppID != c.appID {
		return platformError(operation, socialhub.CodePermissionDenied, socialhub.ClassPermanent, nil)
	}
	if c.tenantKey != "" && envelope.Header.TenantKey != c.tenantKey {
		return platformError(operation, socialhub.CodePermissionDenied, socialhub.ClassPermanent, nil)
	}
	return nil
}

func validHeaderValue(value string) bool {
	return value != "" && len(value) <= 256 && !strings.ContainsAny(value, "\r\n")
}

func secureEqual(left, right string) bool {
	return len(left) == len(right) && hmac.Equal([]byte(left), []byte(right))
}

func firstError(primary, fallback error) error {
	if primary != nil {
		return primary
	}
	return fallback
}

var _ socialhub.ChallengeHandler = (*Client)(nil)
