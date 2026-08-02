package kick

import (
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"net/http"
	"strings"
	"time"

	"social-hub/pkg/socialhub"
)

const maxWebhookBodyBytes = 8 << 20

const (
	headerMessageID      = "Kick-Event-Message-Id"
	headerSubscriptionID = "Kick-Event-Subscription-Id"
	headerSignature      = "Kick-Event-Signature"
	headerTimestamp      = "Kick-Event-Message-Timestamp"
	headerEventType      = "Kick-Event-Type"
	headerEventVersion   = "Kick-Event-Version"
)

func parseWebhookPublicKey(value string) (*rsa.PublicKey, error) {
	block, remainder := pem.Decode([]byte(value))
	if block == nil || block.Type != "PUBLIC KEY" || strings.TrimSpace(string(remainder)) != "" {
		return nil, errors.New("kick: webhook public key must contain one PEM PKIX PUBLIC KEY")
	}
	parsed, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	publicKey, ok := parsed.(*rsa.PublicKey)
	if !ok || publicKey.Size() < 256 {
		return nil, errors.New("kick: webhook public key must be an RSA key of at least 2048 bits")
	}
	return publicKey, nil
}

func (client *Client) Verify(_ context.Context, request *http.Request, body []byte) error {
	if client.webhookPublicKey == nil {
		return unsupported("webhook_verify", "Kick webhook public key is not configured")
	}
	if request == nil || request.Method != http.MethodPost || len(body) == 0 || len(body) > maxWebhookBodyBytes {
		return invalidArgument("webhook_verify", "Kick webhook must be a bounded, non-empty POST body")
	}
	metadata, signatureText, err := parseWebhookHeaders(request, true)
	if err != nil {
		return err
	}
	signature, err := base64.StdEncoding.Strict().DecodeString(signatureText)
	if err != nil || len(signature) != client.webhookPublicKey.Size() {
		return platformError("webhook_verify", socialhub.CodePermissionDenied, socialhub.ClassPermanent, err)
	}
	hasher := sha256.New()
	_, _ = hasher.Write([]byte(metadata.MessageID))
	_, _ = hasher.Write([]byte("."))
	_, _ = hasher.Write([]byte(request.Header.Get(headerTimestamp)))
	_, _ = hasher.Write([]byte("."))
	_, _ = hasher.Write(body)
	if err := rsa.VerifyPKCS1v15(client.webhookPublicKey, crypto.SHA256, hasher.Sum(nil), signature); err != nil {
		return platformError("webhook_verify", socialhub.CodePermissionDenied, socialhub.ClassPermanent, err)
	}
	return nil
}

func (client *Client) Decode(_ context.Context, request *http.Request, body []byte) ([]socialhub.Event, error) {
	if request == nil || request.Method != http.MethodPost || len(body) == 0 || len(body) > maxWebhookBodyBytes {
		return nil, invalidArgument("webhook_decode", "Kick webhook must be a bounded, non-empty POST body")
	}
	metadata, _, err := parseWebhookHeaders(request, false)
	if err != nil {
		return nil, err
	}
	version, supported := supportedEventVersions[metadata.EventType]
	if !supported || metadata.Version != "1" || version != 1 {
		return nil, unsupported("webhook_decode", "unsupported Kick event type or version")
	}
	metadata.Raw = append(json.RawMessage(nil), body...)

	var payload any
	switch metadata.EventType {
	case "chat.message.sent":
		value := &ChatMessageEvent{WebhookMetadata: metadata}
		if err := decodeWebhookBody(body, value); err != nil || !validPathID(value.MessageID, 512) ||
			!validWebhookUser(value.Broadcaster, false) || !validWebhookUser(value.Sender, true) || value.CreatedAt.IsZero() {
			return nil, invalidWebhookPayload(err)
		}
		payload = value
	case "channel.followed":
		value := &ChannelFollowedEvent{WebhookMetadata: metadata}
		if err := decodeWebhookBody(body, value); err != nil || !validWebhookUser(value.Broadcaster, false) || !validWebhookUser(value.Follower, false) {
			return nil, invalidWebhookPayload(err)
		}
		payload = value
	case "channel.subscription.renewal":
		value := &ChannelSubscriptionRenewalEvent{WebhookMetadata: metadata}
		if err := decodeWebhookBody(body, value); err != nil || !validWebhookUser(value.Broadcaster, false) ||
			!validWebhookUser(value.Subscriber, false) || value.Duration <= 0 || value.CreatedAt.IsZero() || value.ExpiresAt.IsZero() {
			return nil, invalidWebhookPayload(err)
		}
		payload = value
	case "channel.subscription.gifts":
		value := &ChannelSubscriptionGiftsEvent{WebhookMetadata: metadata}
		if err := decodeWebhookBody(body, value); err != nil || !validWebhookUser(value.Broadcaster, false) ||
			!validWebhookUser(value.Gifter, true) || !validWebhookUsers(value.Giftees) || value.CreatedAt.IsZero() || value.ExpiresAt.IsZero() {
			return nil, invalidWebhookPayload(err)
		}
		payload = value
	case "channel.subscription.new":
		value := &ChannelSubscriptionNewEvent{WebhookMetadata: metadata}
		if err := decodeWebhookBody(body, value); err != nil || !validWebhookUser(value.Broadcaster, false) ||
			!validWebhookUser(value.Subscriber, false) || value.Duration <= 0 || value.CreatedAt.IsZero() || value.ExpiresAt.IsZero() {
			return nil, invalidWebhookPayload(err)
		}
		payload = value
	case "channel.reward.redemption.updated":
		value := &ChannelRewardRedemptionUpdatedEvent{WebhookMetadata: metadata}
		if err := decodeWebhookBody(body, value); err != nil || !validOpaque(value.ID, 128) || !validOpaque(value.Reward.ID, 128) ||
			!validWebhookUser(value.Broadcaster, false) || !validWebhookUser(value.Redeemer, false) || !validRedemptionStatus(value.Status) || value.RedeemedAt.IsZero() {
			return nil, invalidWebhookPayload(err)
		}
		payload = value
	case "livestream.status.updated":
		value := &LivestreamStatusUpdatedEvent{WebhookMetadata: metadata}
		if err := decodeWebhookBody(body, value); err != nil || !validWebhookUser(value.Broadcaster, false) || value.StartedAt.IsZero() ||
			(value.IsLive && value.EndedAt != nil) || (!value.IsLive && value.EndedAt == nil) {
			return nil, invalidWebhookPayload(err)
		}
		payload = value
	case "livestream.metadata.updated":
		value := &LivestreamMetadataUpdatedEvent{WebhookMetadata: metadata}
		if err := decodeWebhookBody(body, value); err != nil || !validWebhookUser(value.Broadcaster, false) || value.Metadata.Category.ID <= 0 {
			return nil, invalidWebhookPayload(err)
		}
		payload = value
	case "moderation.banned":
		value := &ModerationBannedEvent{WebhookMetadata: metadata}
		if err := decodeWebhookBody(body, value); err != nil || !validWebhookUser(value.Broadcaster, false) ||
			!validWebhookUser(value.Moderator, false) || !validWebhookUser(value.BannedUser, false) || value.Metadata.CreatedAt.IsZero() {
			return nil, invalidWebhookPayload(err)
		}
		payload = value
	case "kicks.gifted":
		value := &KicksGiftedEvent{WebhookMetadata: metadata}
		if err := decodeWebhookBody(body, value); err != nil || !validWebhookUser(value.Broadcaster, false) ||
			!validWebhookUser(value.Sender, false) || value.Gift.Amount <= 0 || value.CreatedAt.IsZero() {
			return nil, invalidWebhookPayload(err)
		}
		payload = value
	}
	return []socialhub.Event{{
		ID: metadata.MessageID, Type: metadata.EventType, Platform: "kick", AccountID: client.accountID, Payload: payload,
	}}, nil
}

func parseWebhookHeaders(request *http.Request, requireSignature bool) (WebhookMetadata, string, error) {
	messageID := strings.TrimSpace(request.Header.Get(headerMessageID))
	subscriptionID := strings.TrimSpace(request.Header.Get(headerSubscriptionID))
	eventType := strings.TrimSpace(request.Header.Get(headerEventType))
	version := strings.TrimSpace(request.Header.Get(headerEventVersion))
	timestampText := strings.TrimSpace(request.Header.Get(headerTimestamp))
	signature := strings.TrimSpace(request.Header.Get(headerSignature))
	if !validOpaque(messageID, 128) || !validOpaque(subscriptionID, 128) || !validOpaque(eventType, 128) ||
		!validOpaque(version, 16) || !validOpaque(timestampText, 64) || (requireSignature && !validOpaque(signature, 4096)) {
		return WebhookMetadata{}, "", invalidArgument("webhook_headers", "required Kick event headers are missing or invalid")
	}
	timestamp, err := time.Parse(time.RFC3339Nano, timestampText)
	if err != nil {
		return WebhookMetadata{}, "", invalidArgument("webhook_headers", "Kick event timestamp must be RFC3339")
	}
	return WebhookMetadata{
		MessageID: messageID, SubscriptionID: subscriptionID, EventType: eventType,
		Version: version, MessageTimestamp: timestamp,
	}, signature, nil
}

func decodeWebhookBody(body []byte, target any) error {
	return json.Unmarshal(body, target)
}

func invalidWebhookPayload(cause error) error {
	return platformError("webhook_decode", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, cause)
}

func validWebhookUser(user WebhookUser, anonymousAllowed bool) bool {
	if user.IsAnonymous {
		return anonymousAllowed && user.UserID == nil
	}
	return user.UserID != nil && *user.UserID > 0 && user.Username != nil && strings.TrimSpace(*user.Username) != ""
}

func validWebhookUsers(users []WebhookUser) bool {
	if len(users) == 0 {
		return false
	}
	for _, user := range users {
		if !validWebhookUser(user, false) {
			return false
		}
	}
	return true
}

func validRedemptionStatus(status string) bool {
	return status == "pending" || status == "accepted" || status == "rejected"
}
