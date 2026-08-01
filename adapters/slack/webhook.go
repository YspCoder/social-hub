package slack

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

const (
	maxEventsBodyBytes = 8 << 20
	maxSignatureSkew   = 5 * time.Minute
)

func (c *Client) Verify(_ context.Context, request *http.Request, body []byte) error {
	if c.signingSecret == "" {
		return unsupported("events_verify", "Slack signing secret is not configured")
	}
	if request == nil || request.Method != http.MethodPost || len(body) == 0 || len(body) > maxEventsBodyBytes {
		return invalidArgument("events_verify", "Slack event must be a bounded, non-empty POST body")
	}
	timestampText := strings.TrimSpace(request.Header.Get("X-Slack-Request-Timestamp"))
	timestamp, err := strconv.ParseInt(timestampText, 10, 64)
	if err != nil || timestamp <= 0 {
		return invalidArgument("events_verify", "X-Slack-Request-Timestamp must be a positive Unix timestamp")
	}
	now := c.clock.Now()
	requestTime := time.Unix(timestamp, 0)
	if requestTime.Before(now.Add(-maxSignatureSkew)) || requestTime.After(now.Add(maxSignatureSkew)) {
		return platformError("events_verify", socialhub.CodePermissionDenied, socialhub.ClassPermanent, nil)
	}
	signature := strings.TrimSpace(request.Header.Get("X-Slack-Signature"))
	if !strings.HasPrefix(signature, "v0=") {
		return platformError("events_verify", socialhub.CodePermissionDenied, socialhub.ClassPermanent, nil)
	}
	provided, err := hex.DecodeString(strings.TrimPrefix(signature, "v0="))
	if err != nil || len(provided) != sha256.Size {
		return platformError("events_verify", socialhub.CodePermissionDenied, socialhub.ClassPermanent, err)
	}
	mac := hmac.New(sha256.New, []byte(c.signingSecret))
	_, _ = mac.Write([]byte("v0:" + timestampText + ":"))
	_, _ = mac.Write(body)
	if !hmac.Equal(mac.Sum(nil), provided) {
		return platformError("events_verify", socialhub.CodePermissionDenied, socialhub.ClassPermanent, nil)
	}
	var envelope eventsEnvelope
	if err := json.Unmarshal(body, &envelope); err != nil {
		return platformError("events_verify", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	if envelope.TeamID != "" && envelope.TeamID != c.workspaceID {
		return platformError("events_verify", socialhub.CodePermissionDenied, socialhub.ClassPermanent, nil)
	}
	return nil
}

func (c *Client) Decode(_ context.Context, request *http.Request, body []byte) ([]socialhub.Event, error) {
	if len(body) == 0 || len(body) > maxEventsBodyBytes {
		return nil, invalidArgument("events_decode", "Slack event body must be bounded and non-empty")
	}
	var envelope eventsEnvelope
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, platformError("events_decode", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	if strings.TrimSpace(envelope.Type) == "" {
		return nil, invalidArgument("events_decode", "Slack event type is required")
	}
	if envelope.TeamID != "" && envelope.TeamID != c.workspaceID {
		return nil, platformError("events_decode", socialhub.CodePermissionDenied, socialhub.ClassPermanent, nil)
	}
	retryNumber, retryReason := 0, ""
	if request != nil {
		if value, err := strconv.Atoi(strings.TrimSpace(request.Header.Get("X-Slack-Retry-Num"))); err == nil && value >= 0 {
			retryNumber = value
		}
		retryReason = strings.TrimSpace(request.Header.Get("X-Slack-Retry-Reason"))
	}
	payload := EventsPayload{
		ID: envelope.EventID, Type: envelope.Type, TeamID: envelope.TeamID, APIAppID: envelope.APIAppID,
		EventContext: envelope.EventContext, RetryNumber: retryNumber, RetryReason: retryReason,
	}
	eventType := envelope.Type
	switch envelope.Type {
	case "url_verification":
		if !validOpaque(envelope.Challenge, 2048) {
			return nil, invalidArgument("events_decode", "url_verification challenge is required")
		}
		hash := sha256.Sum256([]byte(envelope.Challenge))
		payload.ID = "url_verification:" + hex.EncodeToString(hash[:8])
		payload.Challenge = envelope.Challenge
		payload.Raw = append(json.RawMessage(nil), body...)
	case "app_rate_limited":
		if envelope.TeamID == "" || envelope.MinuteRateLimited <= 0 {
			return nil, invalidArgument("events_decode", "app_rate_limited requires team_id and minute_rate_limited")
		}
		payload.ID = "app_rate_limited:" + envelope.TeamID + ":" + strconv.FormatInt(envelope.MinuteRateLimited, 10)
		payload.Raw = append(json.RawMessage(nil), body...)
	case "event_callback":
		if envelope.TeamID == "" || !validOpaque(envelope.EventID, 255) || len(envelope.Event) == 0 {
			return nil, invalidArgument("events_decode", "event_callback requires team_id, event_id, and event")
		}
		var eventHeader struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal(envelope.Event, &eventHeader); err != nil || strings.TrimSpace(eventHeader.Type) == "" {
			return nil, platformError("events_decode", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
		eventType = eventHeader.Type
		payload.Type = eventType
		payload.Raw = append(json.RawMessage(nil), envelope.Event...)
		switch eventType {
		case "message", "app_mention":
			var message struct {
				wireMessage
				Channel        string       `json:"channel"`
				EventTS        string       `json:"event_ts"`
				ChangedMessage *wireMessage `json:"message"`
			}
			if err := json.Unmarshal(envelope.Event, &message); err != nil || !validSlackID(message.Channel, "CGD") {
				return nil, platformError("events_decode", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
			}
			mappedMessage := message.wireMessage
			if !validTimestamp(messageTimestamp(mappedMessage)) && message.ChangedMessage != nil {
				mappedMessage = *message.ChangedMessage
			}
			if !validTimestamp(messageTimestamp(mappedMessage)) {
				return nil, invalidArgument("events_decode", "message event timestamp is required")
			}
			payload.Post = mapPost(c.accountID, message.Channel, mappedMessage, c.clock.Now())
			payload.Message = mapMessage(c.accountID, c.actorID, message.Channel, mappedMessage)
		case "reaction_added", "reaction_removed":
			var reaction wireReactionEvent
			if err := json.Unmarshal(envelope.Event, &reaction); err != nil || reaction.Reaction == "" {
				return nil, platformError("events_decode", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
			}
			if reaction.Item.Type == "message" {
				if !validSlackID(reaction.Item.Channel, "CGD") || !validTimestamp(reaction.Item.TS) {
					return nil, invalidArgument("events_decode", "message reaction target is invalid")
				}
				payload.Reaction = &ReactionEvent{
					Reaction: reaction.Reaction, UserID: reaction.User, ItemUserID: reaction.ItemUser,
					TargetID: compositeID(reaction.Item.Channel, reaction.Item.TS), Added: eventType == "reaction_added",
				}
			}
		case "file_shared":
			var file wireFileEvent
			if err := json.Unmarshal(envelope.Event, &file); err != nil || !validSlackID(file.FileID, "F") {
				return nil, platformError("events_decode", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
			}
			media := mapFile(wireFile{ID: file.FileID})
			payload.File = &media
		}
	default:
		return nil, unsupported("events_decode", "unsupported Slack callback envelope type")
	}
	return []socialhub.Event{{
		ID: payload.ID, Type: "slack." + eventType, Platform: "slack", AccountID: c.accountID, Payload: payload,
	}}, nil
}
