package strava

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"social-hub/pkg/socialhub"
)

func TestWebhookChallengeVerifyAndDecode(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	_, client := newTestAdapter(t, server, true, []string{"read", "activity:read"})
	challenge := httptest.NewRequest(http.MethodGet, "/webhook?hub.mode=subscribe&hub.verify_token=verify-token&hub.challenge=challenge-value", nil)
	status, response, err := client.HandleChallenge(context.Background(), challenge)
	if err != nil || status != http.StatusOK || string(response) != `{"hub.challenge":"challenge-value"}` {
		t.Fatalf("challenge status=%d body=%s err=%v", status, response, err)
	}
	body := []byte(`{"aspect_type":"update","event_time":1785720000,"object_id":` + testActivityID + `,"object_type":"activity","owner_id":` + testAthleteID + `,"subscription_id":2468,"updates":{"title":"Morning Ride","private":"false"}}`)
	request := httptest.NewRequest(http.MethodPost, "/webhook", nil)
	if err := client.Verify(context.Background(), request, body); err != nil {
		t.Fatal(err)
	}
	events, err := client.Decode(context.Background(), request, body)
	if err != nil || len(events) != 1 || events[0].ID == "" || events[0].Type != "strava.activity.update" || events[0].Platform != "strava" || events[0].AccountID != "athlete" {
		t.Fatalf("events=%#v err=%v", events, err)
	}
	payload, ok := events[0].Payload.(WebhookEvent)
	if !ok || payload.OwnerID != 123456789012345 || payload.SubscriptionID != 2468 || string(payload.Updates["title"]) != `"Morning Ride"` || string(payload.Raw) != string(body) {
		t.Fatalf("payload=%#v", events[0].Payload)
	}
	again, err := client.Decode(context.Background(), request, body)
	if err != nil || again[0].ID != events[0].ID {
		t.Fatalf("deterministic event=%#v err=%v", again, err)
	}
}

func TestWebhookValidationFailures(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	_, client := newTestAdapter(t, server, true, nil)
	validBody := []byte(`{"aspect_type":"create","event_time":1,"object_id":2,"object_type":"activity","owner_id":` + testAthleteID + `,"subscription_id":2468}`)
	post := httptest.NewRequest(http.MethodPost, "/webhook", nil)
	for index, test := range []struct {
		request *http.Request
		body    []byte
	}{
		{nil, validBody},
		{httptest.NewRequest(http.MethodGet, "/webhook", nil), validBody},
		{post, nil},
		{post, make([]byte, maxWebhookBodyBytes+1)},
	} {
		if err := client.Verify(context.Background(), test.request, test.body); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Fatalf("verification %d error=%v", index, err)
		}
	}
	invalidBodies := [][]byte{
		[]byte(`{`),
		[]byte(`{"aspect_type":"toggle","event_time":1,"object_id":2,"object_type":"activity","owner_id":` + testAthleteID + `,"subscription_id":2468}`),
		[]byte(`{"aspect_type":"create","event_time":1,"object_id":2,"object_type":"club","owner_id":` + testAthleteID + `,"subscription_id":2468}`),
		[]byte(`{"aspect_type":"create","event_time":0,"object_id":2,"object_type":"activity","owner_id":` + testAthleteID + `,"subscription_id":2468}`),
	}
	for index, body := range invalidBodies {
		if err := client.Verify(context.Background(), post, body); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Fatalf("body %d error=%v", index, err)
		}
	}
	wrongOwner := []byte(`{"aspect_type":"create","event_time":1,"object_id":2,"object_type":"activity","owner_id":9,"subscription_id":2468}`)
	wrongSubscription := []byte(`{"aspect_type":"create","event_time":1,"object_id":2,"object_type":"activity","owner_id":` + testAthleteID + `,"subscription_id":9}`)
	for _, body := range [][]byte{wrongOwner, wrongSubscription} {
		if err := client.Verify(context.Background(), post, body); !errors.Is(err, socialhub.ErrPermissionDenied) {
			t.Fatalf("ownership error=%v", err)
		}
	}

	badMethod := httptest.NewRequest(http.MethodPost, "/webhook", nil)
	if status, _, err := client.HandleChallenge(context.Background(), badMethod); status != http.StatusBadRequest || !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("method status=%d err=%v", status, err)
	}
	badToken := httptest.NewRequest(http.MethodGet, "/webhook?hub.mode=subscribe&hub.verify_token=wrong&hub.challenge=x", nil)
	if status, _, err := client.HandleChallenge(context.Background(), badToken); status != http.StatusForbidden || !errors.Is(err, socialhub.ErrPermissionDenied) {
		t.Fatalf("token status=%d err=%v", status, err)
	}
	empty := httptest.NewRequest(http.MethodGet, "/webhook?hub.mode=subscribe&hub.verify_token=verify-token", nil)
	if status, _, err := client.HandleChallenge(context.Background(), empty); status != http.StatusBadRequest || !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("empty status=%d err=%v", status, err)
	}
	large := httptest.NewRequest(http.MethodGet, "/webhook?hub.mode=subscribe&hub.verify_token=verify-token&hub.challenge="+strings.Repeat("x", maxChallengeBytes+1), nil)
	if status, _, err := client.HandleChallenge(context.Background(), large); status != http.StatusBadRequest || !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("large status=%d err=%v", status, err)
	}
	_, unconfigured := newTestAdapter(t, server, false, nil)
	if err := unconfigured.Verify(context.Background(), post, validBody); !errors.Is(err, socialhub.ErrUnsupported) {
		t.Fatalf("unconfigured verify error=%v", err)
	}
	if status, _, err := unconfigured.HandleChallenge(context.Background(), empty); status != http.StatusBadRequest || !errors.Is(err, socialhub.ErrUnsupported) {
		t.Fatalf("unconfigured challenge status=%d err=%v", status, err)
	}
}

func TestWireIDDecoding(t *testing.T) {
	for _, encoded := range []string{`123`, `"123"`, `12345678901234567890123456789012`} {
		var id wireID
		if err := id.UnmarshalJSON([]byte(encoded)); err != nil || id == "" {
			t.Fatalf("encoded=%s id=%q error=%v", encoded, id, err)
		}
	}
	for _, encoded := range []string{`0`, `-1`, `1.5`, `"abc"`, `123456789012345678901234567890123`} {
		var id wireID
		if err := id.UnmarshalJSON([]byte(encoded)); err == nil {
			t.Fatalf("encoded=%s must fail", encoded)
		}
	}
}
