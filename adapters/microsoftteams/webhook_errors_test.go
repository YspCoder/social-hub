package microsoftteams

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

func TestWebhookChallengeVerificationAndDecode(t *testing.T) {
	_, client, _ := newTestAdapter(t, http.NotFoundHandler(), CloudGlobal, TokenDelegated, true, true, allTestScopes())
	challenge := httptest.NewRequest(http.MethodPost, "/hook?validationToken=opaque%20token%2Bvalue", nil)
	token, err := client.HandleChallenge(challenge)
	if err != nil || token != "opaque token+value" {
		t.Fatalf("challenge=%q error=%v", token, err)
	}
	if err := client.Verify(context.Background(), challenge, nil); err != nil {
		t.Fatalf("verify challenge=%v", err)
	}
	events, err := client.Decode(context.Background(), challenge, nil)
	if err != nil || len(events) != 1 || events[0].Type != "microsoftteams.validation" || events[0].Payload != "opaque token+value" {
		t.Fatalf("challenge events=%#v error=%v", events, err)
	}

	body := []byte(`{"value":[{"subscriptionId":"subscription-1","subscriptionExpirationDateTime":"2026-08-01T13:00:00Z","clientState":"client-state","changeType":"created","resource":"chats/chat-1/messages/root-1","tenantId":"tenant-1","resourceData":{"@odata.type":"#Microsoft.Graph.chatMessage","@odata.id":"chats/chat-1/messages/root-1","id":"root-1"}}]}`)
	request := httptest.NewRequest(http.MethodPost, "/hook", nil)
	if err := client.Verify(context.Background(), request, body); err != nil {
		t.Fatalf("verify notification=%v", err)
	}
	events, err = client.Decode(context.Background(), request, body)
	if err != nil || len(events) != 1 || !strings.HasPrefix(events[0].ID, "graph:") || events[0].Type != "microsoftteams.created" || events[0].Platform != "microsoft-teams" || events[0].AccountID != "main" {
		t.Fatalf("events=%#v error=%v", events, err)
	}
	notification, ok := events[0].Payload.(Notification)
	if !ok || notification.ResourceData.ID != testRootID || len(notification.Raw) == 0 {
		t.Fatalf("notification=%#v", events[0].Payload)
	}
}

func TestWebhookValidationFailures(t *testing.T) {
	_, client, _ := newTestAdapter(t, http.NotFoundHandler(), CloudGlobal, TokenDelegated, true, true, allTestScopes())
	_, noSecret, _ := newTestAdapter(t, http.NotFoundHandler(), CloudGlobal, TokenDelegated, true, false, allTestScopes())
	validRequest := httptest.NewRequest(http.MethodPost, "/hook", nil)
	validBody := []byte(`{"value":[{"id":"notification-1","subscriptionId":"subscription-1","clientState":"client-state","changeType":"updated","resource":"chats/chat-1/messages/root-1","tenantId":"tenant-1"}]}`)
	if err := noSecret.Verify(context.Background(), validRequest, validBody); !errors.Is(err, socialhub.ErrUnsupported) {
		t.Fatalf("no secret=%v", err)
	}
	if _, err := noSecret.Decode(context.Background(), validRequest, validBody); !errors.Is(err, socialhub.ErrUnsupported) {
		t.Fatalf("decode without secret=%v", err)
	}
	if err := client.Verify(context.Background(), nil, validBody); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("nil request=%v", err)
	}
	if err := client.Verify(context.Background(), httptest.NewRequest(http.MethodGet, "/hook", nil), validBody); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("GET request=%v", err)
	}
	if _, err := client.HandleChallenge(httptest.NewRequest(http.MethodPost, "/hook", nil)); err == nil {
		t.Fatal("missing challenge token accepted")
	}
	if _, err := client.HandleChallenge(httptest.NewRequest(http.MethodGet, "/hook?validationToken=x", nil)); err == nil {
		t.Fatal("GET challenge accepted")
	}

	invalidBodies := [][]byte{
		nil,
		[]byte(`{"value":[]}`),
		[]byte(`{"validationTokens":["jwt"],"value":[{}]}`),
		[]byte(`{"value":[{"id":"notification-1","subscriptionId":"subscription-1","clientState":"wrong","changeType":"created","resource":"resource","tenantId":"tenant-1"}]}`),
		[]byte(`{"value":[{"id":"notification-1","subscriptionId":"subscription-1","clientState":"client-state","changeType":"created","resource":"resource","tenantId":"other"}]}`),
		[]byte(`{"value":[{"id":"notification-1","subscriptionId":"subscription-1","clientState":"client-state","changeType":"created","resource":"resource","tenantId":"tenant-1","encryptedContent":{"data":"ciphertext"}}]}`),
		[]byte(`{"value":[{"id":"notification-1","clientState":"client-state","changeType":"created","resource":"resource","tenantId":"tenant-1"}]}`),
		[]byte(`{"value":[null]}`),
	}
	for index, body := range invalidBodies {
		if err := client.Verify(context.Background(), validRequest, body); err == nil {
			t.Fatalf("invalid webhook %d accepted", index)
		}
		if _, err := client.Decode(context.Background(), validRequest, body); err == nil {
			t.Fatalf("invalid decode %d accepted", index)
		}
	}
}

func TestGraphHTTPErrorMapping(t *testing.T) {
	tests := []struct {
		status int
		code   string
		want   socialhub.ErrorCode
		class  socialhub.ErrorClass
	}{
		{http.StatusBadRequest, "BadRequest", socialhub.CodeInvalidArgument, socialhub.ClassPermanent},
		{http.StatusUnauthorized, "InvalidAuthenticationToken", socialhub.CodeUnauthenticated, socialhub.ClassUserAction},
		{http.StatusForbidden, "Authorization_RequestDenied", socialhub.CodePermissionDenied, socialhub.ClassUserAction},
		{http.StatusForbidden, "ErrorAccessDenied", socialhub.CodePermissionDenied, socialhub.ClassUserAction},
		{http.StatusNotFound, "ItemNotFound", socialhub.CodeNotFound, socialhub.ClassPermanent},
		{http.StatusConflict, "Conflict", socialhub.CodeConflict, socialhub.ClassPermanent},
		{http.StatusTooManyRequests, "TooManyRequests", socialhub.CodeRateLimited, socialhub.ClassRetryable},
		{http.StatusServiceUnavailable, "ServiceUnavailable", socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable},
		{http.StatusTeapot, "Unknown", socialhub.CodePlatformError, socialhub.ClassPermanent},
	}
	for _, test := range tests {
		header := make(http.Header)
		header.Set("Retry-After", "3")
		header.Set("request-id", "header-request")
		body := []byte(`{"error":{"code":"` + test.code + `","message":"bounded message","innerError":{"request-id":"inner-request"}}}`)
		err := decodeHTTPError(test.status, header, body)
		var platformErr *socialhub.Error
		if !errors.As(err, &platformErr) || platformErr.Code != test.want || platformErr.Class != test.class || platformErr.HTTPStatus != test.status || platformErr.RequestID != "header-request" || platformErr.RetryAfter != 3*time.Second {
			t.Fatalf("HTTP %d/%s error=%#v", test.status, test.code, platformErr)
		}
	}
	if code, class := classifyError(http.StatusOK, "activityLimitReached"); code != socialhub.CodeRateLimited || class != socialhub.ClassRetryable {
		t.Fatalf("platform throttle=%s/%s", code, class)
	}
	if retryAfter("0") != 0 || retryAfter("later") != 0 || retryAfter("2") != 2*time.Second || retryAfter("999999") != 0 {
		t.Fatal("retry-after mismatch")
	}
	long := strings.Repeat("x", 513)
	if len(boundedMessage(long, 512)) != 512 || firstNonEmpty(" ", " value ") != "value" || stringPointer(" ") != nil {
		t.Fatal("error helper mismatch")
	}
	err := operationError(decodeHTTPError(http.StatusForbidden, nil, []byte(`{"error":{"code":"ErrorAccessDenied"}}`)), "send")
	var platformErr *socialhub.Error
	if !errors.As(err, &platformErr) || platformErr.Op != "send" || platformErr.Code != socialhub.CodePermissionDenied {
		t.Fatalf("operation error=%#v", platformErr)
	}
}

func TestSubscriptionValidation(t *testing.T) {
	valid := CreateSubscriptionRequest{
		Resource: "/teams/team-1/channels/channel-1/messages", ChangeTypes: []string{"created"},
		NotificationURL: "https://hooks.example/graph", ExpirationDateTime: testNow.Add(30 * time.Minute),
	}
	changeType, err := validateSubscriptionInput("test", valid, testNow)
	if err != nil || changeType != "created" {
		t.Fatalf("valid subscription=%q error=%v", changeType, err)
	}
	tests := []CreateSubscriptionRequest{
		{Resource: "https://graph.microsoft.com/chats", ChangeTypes: []string{"created"}, NotificationURL: valid.NotificationURL, ExpirationDateTime: valid.ExpirationDateTime},
		{Resource: valid.Resource, ChangeTypes: []string{"created"}, NotificationURL: "http://hooks.example", ExpirationDateTime: valid.ExpirationDateTime},
		{Resource: valid.Resource, ChangeTypes: []string{"changed"}, NotificationURL: valid.NotificationURL, ExpirationDateTime: valid.ExpirationDateTime},
		{Resource: valid.Resource, ChangeTypes: []string{"created"}, NotificationURL: valid.NotificationURL, ExpirationDateTime: testNow},
		{Resource: valid.Resource, ChangeTypes: []string{"created"}, NotificationURL: valid.NotificationURL, ExpirationDateTime: testNow.Add(2 * time.Hour)},
		{Resource: valid.Resource, ChangeTypes: []string{"created"}, NotificationURL: valid.NotificationURL, LifecycleNotificationURL: "http://hooks.example/lifecycle", ExpirationDateTime: testNow.Add(2 * time.Hour)},
	}
	for index, input := range tests {
		if _, err := validateSubscriptionInput("test", input, testNow); err == nil {
			t.Fatalf("invalid subscription %d accepted", index)
		}
	}
	valid.ExpirationDateTime = testNow.Add(2 * time.Hour)
	valid.LifecycleNotificationURL = "https://hooks.example/lifecycle"
	if _, err := validateSubscriptionInput("test", valid, testNow); err != nil {
		t.Fatalf("lifecycle subscription=%v", err)
	}
	if validResource("resource") || validResource("/teams/team") || validResource("/bad\nresource") || !validResource("/chats/chat/messages?$search=Hello") || !validResource("/users/user/chats/getAllMessages") || validHTTPSURL("http://example.com") || !validHTTPSURL("https://example.com/hook") {
		t.Fatal("subscription helper mismatch")
	}

	_, noSecret, _ := newTestAdapter(t, http.NotFoundHandler(), CloudGlobal, TokenDelegated, true, false, allTestScopes())
	if _, err := noSecret.CreateSubscription(context.Background(), valid); !errors.Is(err, socialhub.ErrUnsupported) {
		t.Fatalf("subscription without secret=%v", err)
	}
	if _, err := noSecret.RenewSubscription(context.Background(), "subscription-1", testNow); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("past renewal=%v", err)
	}
}
