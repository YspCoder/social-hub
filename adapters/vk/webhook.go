package vk

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"social-hub/pkg/socialhub"
)

const maxCallbackBodyBytes = 8 << 20

func (c *Client) Verify(_ context.Context, request *http.Request, body []byte) error {
	if c.callbackSecret == "" || c.groupID == 0 {
		return unsupported("callback_verify", "Callback API community secret is not configured")
	}
	if request == nil || request.Method != http.MethodPost || len(body) == 0 || len(body) > maxCallbackBodyBytes {
		return invalidArgument("callback_verify", "VK callback must be a bounded, non-empty POST body")
	}
	var envelope callbackEnvelope
	if err := json.Unmarshal(body, &envelope); err != nil {
		return platformError("callback_verify", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	if envelope.GroupID != c.groupID {
		return platformError("callback_verify", socialhub.CodePermissionDenied, socialhub.ClassPermanent, nil)
	}
	provided, expected := []byte(envelope.Secret), []byte(c.callbackSecret)
	if len(provided) != len(expected) || subtle.ConstantTimeCompare(provided, expected) != 1 {
		return platformError("callback_verify", socialhub.CodePermissionDenied, socialhub.ClassPermanent, nil)
	}
	return nil
}

func (c *Client) Decode(_ context.Context, _ *http.Request, body []byte) ([]socialhub.Event, error) {
	if len(body) == 0 || len(body) > maxCallbackBodyBytes {
		return nil, invalidArgument("callback_decode", "VK callback body must be bounded and non-empty")
	}
	var envelope callbackEnvelope
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, platformError("callback_decode", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	if strings.TrimSpace(envelope.Type) == "" || envelope.GroupID <= 0 || (c.groupID > 0 && envelope.GroupID != c.groupID) {
		return nil, invalidArgument("callback_decode", "event type and matching positive community ID are required")
	}
	eventID := strings.TrimSpace(envelope.EventID)
	if eventID == "" {
		if envelope.Type != "confirmation" {
			return nil, invalidArgument("callback_decode", "event_id is required for non-confirmation callbacks")
		}
		eventID = "confirmation:" + strconv.FormatInt(envelope.GroupID, 10)
	}
	payload := CallbackEvent{
		ID: eventID, Type: envelope.Type, GroupID: envelope.GroupID, Version: envelope.Version,
		Object: append(json.RawMessage(nil), envelope.Object...),
	}
	switch envelope.Type {
	case "message_new":
		var object struct {
			Message wireMessage `json:"message"`
		}
		if err := json.Unmarshal(envelope.Object, &object); err != nil || object.Message.ID <= 0 {
			return nil, platformError("callback_decode", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
		payload.Message = mapMessage(c.accountID, object.Message)
	case "message_reply", "message_edit":
		var message wireMessage
		if err := json.Unmarshal(envelope.Object, &message); err != nil || message.ID <= 0 {
			return nil, platformError("callback_decode", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
		payload.Message = mapMessage(c.accountID, message)
	case "wall_post_new", "wall_repost":
		var post wirePost
		if err := json.Unmarshal(envelope.Object, &post); err != nil || post.ID <= 0 || post.OwnerID == 0 {
			return nil, platformError("callback_decode", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
		payload.Post = mapPost(c.accountID, post, c.clock.Now())
	case "wall_reply_new", "wall_reply_edit", "wall_reply_restore", "wall_reply_delete":
		var comment wireComment
		if err := json.Unmarshal(envelope.Object, &comment); err != nil || comment.ID <= 0 || comment.PostID <= 0 {
			return nil, platformError("callback_decode", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
		ownerID := comment.PostOwnerID
		if ownerID == 0 {
			ownerID = comment.OwnerID
		}
		if ownerID == 0 {
			return nil, invalidArgument("callback_decode", "wall reply owner ID is required")
		}
		mapped := mapComment(c.accountID, compositeID(ownerID, comment.PostID), ownerID, comment, c.clock.Now())
		payload.Comment = &mapped
	}
	return []socialhub.Event{{
		ID: eventID, Type: "vk." + envelope.Type, Platform: "vk", AccountID: c.accountID, Payload: payload,
	}}, nil
}

func (c *Client) GetCallbackConfirmationCode(ctx context.Context, options ...socialhub.CallOption) (string, error) {
	if c.groupID == 0 {
		return "", invalidArgument("groups.getCallbackConfirmationCode", "a community owner_id is required")
	}
	var response struct {
		Code string `json:"code"`
	}
	if err := c.method(ctx, "groups.getCallbackConfirmationCode", url.Values{
		"group_id": {strconv.FormatInt(c.groupID, 10)},
	}, &response, options...); err != nil {
		return "", err
	}
	if !validOpaque(response.Code, 256) {
		return "", platformError("groups.getCallbackConfirmationCode", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return response.Code, nil
}
