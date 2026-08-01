package twitch

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

func TestEventSubWorkflow(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer app-token" || request.Header.Get("Client-Id") != "twitch-client" {
			http.Error(writer, "bad app auth", http.StatusUnauthorized)
			return
		}
		if request.URL.Path != "/eventsub/subscriptions" {
			http.NotFound(writer, request)
			return
		}
		switch request.Method {
		case http.MethodPost:
			var body struct {
				Type      string            `json:"type"`
				Version   string            `json:"version"`
				Condition map[string]string `json:"condition"`
				Transport struct {
					Method, Callback, Secret string
				} `json:"transport"`
			}
			if err := json.NewDecoder(request.Body).Decode(&body); err != nil || body.Type != "stream.online" || body.Version != "1" || body.Condition["broadcaster_user_id"] != "user-1" || body.Transport.Method != "webhook" || body.Transport.Callback != "https://app.test/eventsub" || body.Transport.Secret != "eventsub-secret-123" {
				http.Error(writer, "bad subscription", http.StatusBadRequest)
				return
			}
			writer.WriteHeader(http.StatusAccepted)
			writeEventSubPage(t, writer, "subscription-1", "pending-next")
		case http.MethodGet:
			if request.URL.Query().Get("status") != "enabled" || request.URL.Query().Get("after") != "cursor-1" || request.URL.Query().Get("first") != "100" {
				http.Error(writer, "bad list", http.StatusBadRequest)
				return
			}
			writeEventSubPage(t, writer, "subscription-1", "list-next")
		case http.MethodDelete:
			if request.URL.Query().Get("id") != "subscription-1" {
				http.Error(writer, "bad delete", http.StatusBadRequest)
				return
			}
			writer.WriteHeader(http.StatusNoContent)
		default:
			http.Error(writer, "method", http.StatusMethodNotAllowed)
		}
	}))
	defer server.Close()
	client := newTestClient(t, server, nil)
	created, err := client.CreateWebhookSubscription(context.Background(), "stream.online", "1", map[string]string{"broadcaster_user_id": "user-1"}, "https://app.test/eventsub")
	if err != nil || len(created.Items) != 1 || created.Items[0].ID != "subscription-1" || created.TotalCost != 1 || created.MaxTotalCost != 10000 || created.NextCursor == nil {
		t.Fatalf("create subscription: %#v %v", created, err)
	}
	listed, err := client.ListSubscriptions(context.Background(), EventSubListRequest{Status: "enabled", Cursor: "cursor-1", MaxResults: 200})
	if err != nil || len(listed.Items) != 1 || listed.NextCursor == nil || *listed.NextCursor != "list-next" {
		t.Fatalf("list subscriptions: %#v %v", listed, err)
	}
	if err := client.DeleteSubscription(context.Background(), "subscription-1"); err != nil {
		t.Fatalf("delete subscription: %v", err)
	}
}

func writeEventSubPage(t *testing.T, writer http.ResponseWriter, id, cursor string) {
	t.Helper()
	writeTestJSON(t, writer, map[string]any{
		"data": []map[string]any{{
			"id": id, "status": "enabled", "type": "stream.online", "version": "1", "cost": 1,
			"condition":  map[string]string{"broadcaster_user_id": "user-1"},
			"transport":  map[string]string{"method": "webhook", "callback": "https://app.test/eventsub"},
			"created_at": "2026-08-01T10:00:00Z",
		}}, "total": 1, "total_cost": 1, "max_total_cost": 10000, "pagination": map[string]string{"cursor": cursor},
	})
}

func TestEventSubWorkflowValidation(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	client := newTestClient(t, server, nil)
	createCases := []struct {
		typeName, version, callback string
		condition                   map[string]string
	}{
		{"", "1", "https://app.test/eventsub", map[string]string{"user_id": "1"}},
		{"stream online", "1", "https://app.test/eventsub", map[string]string{"user_id": "1"}},
		{"stream.online", "", "https://app.test/eventsub", map[string]string{"user_id": "1"}},
		{"stream.online", "1", "http://app.test/eventsub", map[string]string{"user_id": "1"}},
		{"stream.online", "1", "https://app.test:8443/eventsub", map[string]string{"user_id": "1"}},
		{"stream.online", "1", "https://app.test/eventsub", nil},
		{"stream.online", "1", "https://app.test/eventsub", map[string]string{"bad key": "1"}},
	}
	for index, input := range createCases {
		if _, err := client.CreateWebhookSubscription(context.Background(), input.typeName, input.version, input.condition, input.callback); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Fatalf("create case %d: %v", index, err)
		}
	}
	client.webhookSecret = ""
	if _, err := client.CreateWebhookSubscription(context.Background(), "stream.online", "1", map[string]string{"user_id": "1"}, "https://app.test/eventsub"); !errors.Is(err, socialhub.ErrUnsupported) {
		t.Fatalf("missing secret: %v", err)
	}
	if _, err := client.ListSubscriptions(context.Background(), EventSubListRequest{Type: "stream.online", Status: "enabled"}); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("exclusive filters: %v", err)
	}
	if _, err := client.ListSubscriptions(context.Background(), EventSubListRequest{Type: "bad type"}); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("bad type: %v", err)
	}
	if err := client.DeleteSubscription(context.Background(), " "); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("empty delete: %v", err)
	}
	if !validEventSubSecret("0123456789") || validEventSubSecret("short") || validEventSubSecret("012345678\n") || !validWebhookCallback("https://app.test:443/callback") || validEventToken("bad token") {
		t.Fatal("EventSub validators mismatch")
	}
}

func TestEventSubWebhookNotificationAndChallenge(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	client := newTestClient(t, server, nil)
	notification := []byte(`{"subscription":{"id":"subscription-1","status":"enabled","type":"stream.online","version":"1","cost":0,"condition":{"broadcaster_user_id":"user-1"},"transport":{"method":"webhook","callback":"https://app.test/eventsub"},"created_at":"2026-08-01T10:00:00Z"},"event":{"id":"stream-1","broadcaster_user_id":"user-1"}}`)
	request := signedEventSubRequest(notification, "message-1", "notification", testNow, "eventsub-secret-123")
	if err := client.Verify(context.Background(), request, notification); err != nil {
		t.Fatalf("verify notification: %v", err)
	}
	events, err := client.Decode(context.Background(), request, notification)
	if err != nil || len(events) != 1 || events[0].ID != "message-1" || events[0].Type != "stream.online" || events[0].Platform != "twitch" {
		t.Fatalf("decode notification: %#v %v", events, err)
	}
	payload, ok := events[0].Payload.(EventSubPayload)
	if !ok || payload.Subscription.ID != "subscription-1" || len(payload.Event) == 0 || !bytes.Equal(payload.Raw, notification) {
		t.Fatalf("notification payload: %#v", events[0].Payload)
	}
	challenge := []byte(`{"challenge":"challenge-text","subscription":{"id":"subscription-2","status":"webhook_callback_verification_pending","type":"stream.online","version":"1","cost":0,"condition":{"broadcaster_user_id":"user-1"},"transport":{"method":"webhook","callback":"https://app.test/eventsub"},"created_at":"2026-08-01T10:00:00Z"}}`)
	challengeRequest := signedEventSubRequest(challenge, "message-2", "webhook_callback_verification", testNow, "eventsub-secret-123")
	challengeRequest.Body = io.NopCloser(bytes.NewReader(challenge))
	status, body, err := client.HandleChallenge(context.Background(), challengeRequest)
	if err != nil || status != http.StatusOK || string(body) != "challenge-text" {
		t.Fatalf("challenge: status=%d body=%q err=%v", status, body, err)
	}
	restored, _ := io.ReadAll(challengeRequest.Body)
	if !bytes.Equal(restored, challenge) {
		t.Fatal("challenge request body was not restored")
	}
	revocation := []byte(`{"subscription":{"id":"subscription-3","status":"authorization_revoked","type":"channel.follow","version":"2","cost":1,"condition":{"broadcaster_user_id":"user-1"},"transport":{"method":"webhook","callback":"https://app.test/eventsub"},"created_at":"2026-08-01T10:00:00Z"}}`)
	revocationRequest := signedEventSubRequest(revocation, "message-3", "revocation", testNow, "eventsub-secret-123")
	revocationRequest.Header.Set(headerSubType, "channel.follow")
	revocationRequest.Header.Set(headerSubVersion, "2")
	revoked, err := client.Decode(context.Background(), revocationRequest, revocation)
	if err != nil || len(revoked) != 1 || revoked[0].Type != "eventsub.revocation.channel.follow" {
		t.Fatalf("revocation: %#v %v", revoked, err)
	}
}

func signedEventSubRequest(body []byte, messageID, messageType string, timestamp time.Time, secret string) *http.Request {
	request := httptest.NewRequest(http.MethodPost, "https://app.test/eventsub", nil)
	timestampValue := timestamp.Format(time.RFC3339Nano)
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(messageID))
	_, _ = mac.Write([]byte(timestampValue))
	_, _ = mac.Write(body)
	request.Header.Set(headerMessageID, messageID)
	request.Header.Set(headerMessageType, messageType)
	request.Header.Set(headerTimestamp, timestampValue)
	request.Header.Set(headerSignature, "sha256="+hex.EncodeToString(mac.Sum(nil)))
	request.Header.Set(headerSubType, "stream.online")
	request.Header.Set(headerSubVersion, "1")
	return request
}

func TestEventSubWebhookValidationErrors(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	client := newTestClient(t, server, nil)
	body := []byte(`{"subscription":{"id":"sub","type":"stream.online","version":"1"},"event":{}}`)
	cases := []*http.Request{
		nil,
		httptest.NewRequest(http.MethodGet, "https://app.test/eventsub", nil),
		signedEventSubRequest(body, "message", "notification", testNow.Add(-11*time.Minute), "eventsub-secret-123"),
		signedEventSubRequest(body, "message", "notification", testNow.Add(11*time.Minute), "eventsub-secret-123"),
		signedEventSubRequest(body, "message", "notification", testNow, "wrong-secret"),
	}
	for index, request := range cases {
		if err := client.Verify(context.Background(), request, body); err == nil {
			t.Fatalf("verify case %d accepted", index)
		}
	}
	badTimestamp := signedEventSubRequest(body, "message", "notification", testNow, "eventsub-secret-123")
	badTimestamp.Header.Set(headerTimestamp, "bad")
	if err := client.Verify(context.Background(), badTimestamp, body); err == nil {
		t.Fatal("bad timestamp accepted")
	}
	badSignature := signedEventSubRequest(body, "message", "notification", testNow, "eventsub-secret-123")
	badSignature.Header.Set(headerSignature, "sha256=bad")
	if err := client.Verify(context.Background(), badSignature, body); err == nil {
		t.Fatal("bad signature accepted")
	}
	decodeCases := []struct {
		request *http.Request
		body    []byte
	}{
		{nil, body},
		{httptest.NewRequest(http.MethodPost, "https://app.test", nil), body},
		{signedEventSubRequest([]byte("{"), "m", "notification", testNow, "eventsub-secret-123"), []byte("{")},
		{signedEventSubRequest([]byte(`{"subscription":{}}`), "m", "notification", testNow, "eventsub-secret-123"), []byte(`{"subscription":{}}`)},
		{signedEventSubRequest(body, "m", "unknown", testNow, "eventsub-secret-123"), body},
	}
	for index, test := range decodeCases {
		if _, err := client.Decode(context.Background(), test.request, test.body); err == nil {
			t.Fatalf("decode case %d accepted", index)
		}
	}
	request := signedEventSubRequest(body, "m", "notification", testNow, "eventsub-secret-123")
	request.Header.Set(headerSubType, "channel.follow")
	if _, err := client.Decode(context.Background(), request, body); err == nil {
		t.Fatal("mismatched event type accepted")
	}
	request = signedEventSubRequest(body, "m", "notification", testNow, "eventsub-secret-123")
	request.Body = io.NopCloser(bytes.NewReader(body))
	if status, _, err := client.HandleChallenge(context.Background(), request); err == nil || status != http.StatusBadRequest {
		t.Fatalf("notification challenge accepted: %d %v", status, err)
	}
}
