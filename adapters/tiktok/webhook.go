package tiktok

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"social-hub/pkg/socialhub"
)

const webhookTolerance = 5 * time.Minute

func (c *Client) Verify(_ context.Context, request *http.Request, body []byte) error {
	if c.webhookSecret == "" {
		return unsupported("webhook_verify", "client secret is not configured")
	}
	if request == nil || request.Method != http.MethodPost {
		return invalidArgument("webhook_verify", "TikTok webhook deliveries must use POST")
	}
	timestamp, signature, err := parseSignature(request.Header.Get("TikTok-Signature"))
	if err != nil {
		return platformError("webhook_verify", socialhub.CodePermissionDenied, socialhub.ClassPermanent, err)
	}
	delta := c.clock.Now().Unix() - timestamp
	if delta < 0 {
		delta = -delta
	}
	if time.Duration(delta)*time.Second > webhookTolerance {
		return platformError("webhook_verify", socialhub.CodePermissionDenied, socialhub.ClassPermanent, nil)
	}
	mac := hmac.New(sha256.New, []byte(c.webhookSecret))
	_, _ = mac.Write([]byte(strconv.FormatInt(timestamp, 10)))
	_, _ = mac.Write([]byte("."))
	_, _ = mac.Write(body)
	if !hmac.Equal(signature, mac.Sum(nil)) {
		return platformError("webhook_verify", socialhub.CodePermissionDenied, socialhub.ClassPermanent, nil)
	}
	return nil
}

func parseSignature(header string) (int64, []byte, error) {
	var timestampValue, signatureValue string
	for _, part := range strings.Split(header, ",") {
		key, value, found := strings.Cut(strings.TrimSpace(part), "=")
		if !found {
			continue
		}
		switch key {
		case "t":
			timestampValue = value
		case "s":
			signatureValue = value
		}
	}
	timestamp, err := strconv.ParseInt(timestampValue, 10, 64)
	if err != nil || timestamp <= 0 {
		return 0, nil, invalidArgument("webhook_verify", "invalid TikTok signature timestamp")
	}
	signature, err := hex.DecodeString(signatureValue)
	if err != nil || len(signature) != sha256.Size {
		return 0, nil, invalidArgument("webhook_verify", "invalid TikTok signature digest")
	}
	return timestamp, signature, nil
}

func (c *Client) Decode(_ context.Context, _ *http.Request, body []byte) ([]socialhub.Event, error) {
	var envelope struct {
		ClientKey  string `json:"client_key"`
		Event      string `json:"event"`
		CreateTime int64  `json:"create_time"`
		UserOpenID string `json:"user_openid"`
		Content    string `json:"content"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, platformError("webhook_decode", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	if envelope.Event == "" || (c.clientKey != "" && envelope.ClientKey != c.clientKey) || (envelope.UserOpenID != "" && envelope.UserOpenID != c.openID) {
		return nil, invalidArgument("webhook_decode", "webhook event does not belong to the configured TikTok account")
	}
	var payload json.RawMessage
	if json.Valid([]byte(envelope.Content)) {
		payload = append(json.RawMessage(nil), envelope.Content...)
	} else {
		payload, _ = json.Marshal(envelope.Content)
	}
	digest := sha256.Sum256(body)
	return []socialhub.Event{{
		ID: hex.EncodeToString(digest[:]), Type: "tiktok." + envelope.Event,
		Platform: "tiktok", AccountID: c.accountID, Payload: payload,
	}}, nil
}

var _ socialhub.WebhookHandler = (*Client)(nil)
