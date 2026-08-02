package kick

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

func TestWebhookVerificationAndTypedDecoding(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	publicDER, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	publicPEM := string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: publicDER}))
	publicKey, err := parseWebhookPublicKey(publicPEM)
	if err != nil {
		t.Fatal(err)
	}
	client := &Client{accountID: "main", webhookPublicKey: publicKey}
	user := func(id int64, name string) map[string]any {
		return map[string]any{"is_anonymous": false, "user_id": id, "username": name, "is_verified": false, "profile_picture": "https://img.test/u", "channel_slug": name, "identity": nil}
	}
	timestamp := "2026-08-02T10:00:00Z"
	cases := []struct {
		name     string
		body     map[string]any
		expected any
	}{
		{"chat.message.sent", map[string]any{"message_id": "chat-1", "broadcaster": user(1, "host"), "sender": user(2, "sender"), "content": "hello", "emotes": []any{}, "created_at": timestamp}, &ChatMessageEvent{}},
		{"channel.followed", map[string]any{"broadcaster": user(1, "host"), "follower": user(2, "follower")}, &ChannelFollowedEvent{}},
		{"channel.subscription.renewal", map[string]any{"broadcaster": user(1, "host"), "subscriber": user(2, "subscriber"), "duration": 3, "created_at": timestamp, "expires_at": "2026-09-02T10:00:00Z"}, &ChannelSubscriptionRenewalEvent{}},
		{"channel.subscription.gifts", map[string]any{"broadcaster": user(1, "host"), "gifter": user(2, "gifter"), "giftees": []any{user(3, "giftee")}, "created_at": timestamp, "expires_at": "2026-09-02T10:00:00Z"}, &ChannelSubscriptionGiftsEvent{}},
		{"channel.subscription.new", map[string]any{"broadcaster": user(1, "host"), "subscriber": user(2, "subscriber"), "duration": 1, "created_at": timestamp, "expires_at": "2026-09-02T10:00:00Z"}, &ChannelSubscriptionNewEvent{}},
		{"channel.reward.redemption.updated", map[string]any{"id": "redemption-1", "user_input": "hello", "status": "pending", "redeemed_at": timestamp, "reward": map[string]any{"id": "reward-1", "title": "Request", "cost": 100, "description": "desc"}, "redeemer": user(2, "redeemer"), "broadcaster": user(1, "host")}, &ChannelRewardRedemptionUpdatedEvent{}},
		{"livestream.status.updated", map[string]any{"broadcaster": user(1, "host"), "is_live": true, "title": "Live", "started_at": timestamp, "ended_at": nil}, &LivestreamStatusUpdatedEvent{}},
		{"livestream.metadata.updated", map[string]any{"broadcaster": user(1, "host"), "metadata": map[string]any{"title": "Live", "language": "en", "has_mature_content": false, "category": map[string]any{"id": 9, "name": "Science", "thumbnail": "https://img.test/c"}}}, &LivestreamMetadataUpdatedEvent{}},
		{"moderation.banned", map[string]any{"broadcaster": user(1, "host"), "moderator": user(2, "mod"), "banned_user": user(3, "banned"), "metadata": map[string]any{"reason": "spam", "created_at": timestamp, "expires_at": nil}}, &ModerationBannedEvent{}},
		{"kicks.gifted", map[string]any{"broadcaster": user(1, "host"), "sender": user(2, "sender"), "gift": map[string]any{"amount": 500, "name": "Gift", "type": "LEVEL_UP", "tier": "MID", "message": "w", "pinned_time_seconds": 60}, "created_at": timestamp}, &KicksGiftedEvent{}},
	}
	for index, test := range cases {
		body, err := json.Marshal(test.body)
		if err != nil {
			t.Fatal(err)
		}
		request := signedKickRequest(t, privateKey, body, test.name, "1")
		if index == 0 {
			if err := client.Verify(context.Background(), request, body); err != nil {
				t.Fatalf("verify: %v", err)
			}
		}
		events, err := client.Decode(context.Background(), request, body)
		if err != nil || len(events) != 1 || events[0].ID != "delivery-1" || events[0].Type != test.name || events[0].Platform != "kick" || reflect.TypeOf(events[0].Payload) != reflect.TypeOf(test.expected) {
			t.Fatalf("%s: %#v %v", test.name, events, err)
		}
		if chat, ok := events[0].Payload.(*ChatMessageEvent); ok {
			if chat.MessageID != "chat-1" || chat.SubscriptionID != "subscription-1" || !chat.MessageTimestamp.Equal(testNow) || len(chat.Raw) == 0 {
				t.Fatalf("chat metadata: %#v", chat)
			}
		}
	}

	body := []byte(`{"message_id":"chat-1","broadcaster":{"user_id":1,"username":"host"},"sender":{"user_id":2,"username":"sender"},"content":"hello","created_at":"2026-08-02T10:00:00Z"}`)
	request := signedKickRequest(t, privateKey, body, "chat.message.sent", "1")
	if err := client.Verify(context.Background(), request, append(body, ' ')); !errors.Is(err, socialhub.ErrPermissionDenied) {
		t.Fatalf("tampered body: %v", err)
	}
	request.Header.Set(headerSignature, "bad")
	if err := client.Verify(context.Background(), request, body); !errors.Is(err, socialhub.ErrPermissionDenied) {
		t.Fatalf("bad signature: %v", err)
	}
}

func signedKickRequest(t *testing.T, privateKey *rsa.PrivateKey, body []byte, eventType, version string) *http.Request {
	t.Helper()
	timestamp := testNow.Format(time.RFC3339Nano)
	hasher := sha256.New()
	_, _ = hasher.Write([]byte("delivery-1." + timestamp + "."))
	_, _ = hasher.Write(body)
	signature, err := rsa.SignPKCS1v15(rand.Reader, privateKey, crypto.SHA256, hasher.Sum(nil))
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "https://app.test/kick", nil)
	request.Header.Set(headerMessageID, "delivery-1")
	request.Header.Set(headerSubscriptionID, "subscription-1")
	request.Header.Set(headerSignature, base64.StdEncoding.EncodeToString(signature))
	request.Header.Set(headerTimestamp, timestamp)
	request.Header.Set(headerEventType, eventType)
	request.Header.Set(headerEventVersion, version)
	return request
}

func TestWebhookValidationErrors(t *testing.T) {
	client := &Client{}
	body := []byte(`{}`)
	if err := client.Verify(context.Background(), nil, body); !errors.Is(err, socialhub.ErrUnsupported) {
		t.Fatalf("missing key: %v", err)
	}
	if _, err := parseWebhookPublicKey("bad"); err == nil {
		t.Fatal("invalid public key accepted")
	}
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	client.webhookPublicKey = &privateKey.PublicKey
	if err := client.Verify(context.Background(), nil, body); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("nil request: %v", err)
	}
	request := signedKickRequest(t, privateKey, body, "unknown", "1")
	if _, err := client.Decode(context.Background(), request, body); !errors.Is(err, socialhub.ErrUnsupported) {
		t.Fatalf("unknown event: %v", err)
	}
	request = signedKickRequest(t, privateKey, []byte("{"), "chat.message.sent", "1")
	if _, err := client.Decode(context.Background(), request, []byte("{")); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("malformed event: %v", err)
	}
	request = signedKickRequest(t, privateKey, body, "chat.message.sent", "2")
	if _, err := client.Decode(context.Background(), request, body); !errors.Is(err, socialhub.ErrUnsupported) {
		t.Fatalf("version: %v", err)
	}
	request = signedKickRequest(t, privateKey, body, "chat.message.sent", "1")
	request.Header.Set(headerTimestamp, "bad")
	if err := client.Verify(context.Background(), request, body); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("timestamp: %v", err)
	}
	if validWebhookUser(WebhookUser{}, false) || !validWebhookUser(WebhookUser{IsAnonymous: true}, true) || validWebhookUsers(nil) ||
		!validRedemptionStatus("accepted") || validRedemptionStatus("done") {
		t.Fatal("webhook validators mismatch")
	}
}
