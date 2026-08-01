package qq

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"social-hub/pkg/socialhub"
)

const (
	maxWebhookBodyBytes = 8 << 20
	webhookDispatchOP   = 0
	webhookValidationOP = 13
)

const (
	webhookSignatureHeader = "X-Signature-Ed25519"
	webhookTimestampHeader = "X-Signature-Timestamp"
)

type webhookEnvelope struct {
	ID       string          `json:"id"`
	OP       int             `json:"op"`
	Sequence *int64          `json:"s,omitempty"`
	Type     string          `json:"t"`
	Data     json.RawMessage `json:"d"`
}

func (c *Client) Verify(_ context.Context, request *http.Request, body []byte) error {
	if c.webhookSecret == "" {
		return unsupported("webhook_verify", "AppSecret is not configured for webhook verification")
	}
	if request == nil || request.Method != http.MethodPost || len(body) == 0 || len(body) > maxWebhookBodyBytes {
		return invalidArgument("webhook_verify", "QQ webhook must be a bounded, non-empty POST body")
	}
	timestamp := request.Header.Get(webhookTimestampHeader)
	if timestamp == "" || len(timestamp) > 64 {
		return invalidArgument("webhook_verify", "webhook signature timestamp is required")
	}
	signature, err := hex.DecodeString(request.Header.Get(webhookSignatureHeader))
	if err != nil || len(signature) != ed25519.SignatureSize {
		return platformError("webhook_verify", socialhub.CodePermissionDenied, socialhub.ClassPermanent, err)
	}
	publicKey, _, err := webhookKeys(c.webhookSecret)
	if err != nil {
		return platformError("webhook_verify", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	content := append([]byte(timestamp), body...)
	if !ed25519.Verify(publicKey, content, signature) {
		return platformError("webhook_verify", socialhub.CodePermissionDenied, socialhub.ClassPermanent, nil)
	}
	return nil
}

func (c *Client) Decode(_ context.Context, request *http.Request, body []byte) ([]socialhub.Event, error) {
	if request == nil || request.Method != http.MethodPost || len(body) == 0 || len(body) > maxWebhookBodyBytes {
		return nil, invalidArgument("webhook_decode", "QQ webhook must be a bounded, non-empty POST body")
	}
	var envelope webhookEnvelope
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, platformError("webhook_decode", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	if envelope.OP != webhookDispatchOP {
		return nil, unsupported("webhook_decode", "use ValidationResponse for op=13 and handle protocol control opcodes at the HTTP gateway")
	}
	if !validBoundedString(envelope.Type, 256) || len(envelope.Data) == 0 || string(envelope.Data) == "null" {
		return nil, invalidArgument("webhook_decode", "dispatch type and data are required")
	}
	eventID := envelope.ID
	if eventID == "" {
		digest := sha256.Sum256(body)
		eventID = hex.EncodeToString(digest[:])
	} else if !validOpaque(eventID, 512) {
		return nil, invalidArgument("webhook_decode", "event ID is invalid")
	}
	payload := WebhookEvent{
		ID: eventID, Type: envelope.Type, Sequence: envelope.Sequence,
		Data: append(json.RawMessage(nil), envelope.Data...), Raw: append(json.RawMessage(nil), body...),
	}
	return []socialhub.Event{{
		ID: eventID, Type: "qq." + strings.ToLower(envelope.Type), Platform: "qq", AccountID: c.accountID, Payload: payload,
	}}, nil
}

func (c *Client) ValidationResponse(body []byte) ([]byte, error) {
	if c.webhookSecret == "" {
		return nil, unsupported("webhook_validation", "AppSecret is not configured for webhook validation")
	}
	if len(body) == 0 || len(body) > maxWebhookBodyBytes {
		return nil, invalidArgument("webhook_validation", "validation body must be bounded and non-empty")
	}
	var envelope struct {
		OP   int `json:"op"`
		Data struct {
			PlainToken string `json:"plain_token"`
			EventTS    string `json:"event_ts"`
		} `json:"d"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, platformError("webhook_validation", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	if envelope.OP != webhookValidationOP || !validBoundedString(envelope.Data.PlainToken, 4096) || !validBoundedString(envelope.Data.EventTS, 64) {
		return nil, invalidArgument("webhook_validation", "op=13, plain_token, and event_ts are required")
	}
	_, privateKey, err := webhookKeys(c.webhookSecret)
	if err != nil {
		return nil, platformError("webhook_validation", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	signature := ed25519.Sign(privateKey, []byte(envelope.Data.EventTS+envelope.Data.PlainToken))
	response, err := json.Marshal(struct {
		PlainToken string `json:"plain_token"`
		Signature  string `json:"signature"`
	}{PlainToken: envelope.Data.PlainToken, Signature: hex.EncodeToString(signature)})
	if err != nil {
		return nil, platformError("webhook_validation", socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	return response, nil
}

func webhookKeys(secret string) (ed25519.PublicKey, ed25519.PrivateKey, error) {
	if secret == "" {
		return nil, nil, fmt.Errorf("AppSecret is empty")
	}
	seed := make([]byte, ed25519.SeedSize)
	secretBytes := []byte(secret)
	for index := range seed {
		seed[index] = secretBytes[index%len(secretBytes)]
	}
	privateKey := ed25519.NewKeyFromSeed(seed)
	publicKey := privateKey.Public().(ed25519.PublicKey)
	return publicKey, privateKey, nil
}

var _ socialhub.WebhookHandler = (*Client)(nil)
