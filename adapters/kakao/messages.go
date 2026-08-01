package kakao

import (
	"context"
	"encoding/json"
	"net/url"
	"strconv"

	"social-hub/pkg/socialhub"
)

const maxTemplateBytes = 64 << 10

type textTemplatePayload struct {
	ObjectType  string   `json:"object_type"`
	Text        string   `json:"text"`
	Link        Link     `json:"link"`
	ButtonTitle string   `json:"button_title,omitempty"`
	Buttons     []Button `json:"buttons,omitempty"`
}

type messageResponse struct {
	ResultCode              *int     `json:"result_code"`
	SuccessfulReceiverUUIDs []string `json:"successful_receiver_uuids"`
	FailureInfo             []struct {
		Code          int      `json:"code"`
		Message       string   `json:"msg"`
		ReceiverUUIDs []string `json:"receiver_uuids"`
	} `json:"failure_info"`
}

func (c *Client) SendDefault(ctx context.Context, input DefaultMessageRequest, options ...socialhub.CallOption) (*MessageResult, error) {
	path, err := c.messagePath("send_default_message", input.Target, input.ReceiverUUIDs, true)
	if err != nil {
		return nil, err
	}
	if err := input.Template.validate(); err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(textTemplatePayload{
		ObjectType: "text", Text: input.Template.Text, Link: input.Template.Link,
		ButtonTitle: input.Template.ButtonTitle, Buttons: input.Template.Buttons,
	})
	if err != nil || len(encoded) > maxTemplateBytes {
		return nil, invalidArgument("send_default_message", "template exceeds the encoded size limit")
	}
	values := url.Values{"template_object": {string(encoded)}}
	if input.Target == MessageTargetFriends {
		receivers, _ := json.Marshal(input.ReceiverUUIDs)
		values.Set("receiver_uuids", string(receivers))
	}
	return c.sendTemplate(ctx, "send_default_message", path, input.Target, input.ReceiverUUIDs, values, options...)
}

func (c *Client) SendCustom(ctx context.Context, input CustomMessageRequest, options ...socialhub.CallOption) (*MessageResult, error) {
	path, err := c.messagePath("send_custom_message", input.Target, input.ReceiverUUIDs, false)
	if err != nil {
		return nil, err
	}
	if input.TemplateID <= 0 {
		return nil, invalidArgument("send_custom_message", "template ID must be positive")
	}
	for key, value := range input.Arguments {
		if !validBoundedString(key, 1024) || !validOptionalString(value, 4096) {
			return nil, invalidArgument("send_custom_message", "template arguments contain invalid keys or values")
		}
	}
	arguments, err := json.Marshal(input.Arguments)
	if err != nil || len(arguments) > maxTemplateBytes {
		return nil, invalidArgument("send_custom_message", "template arguments exceed the encoded size limit")
	}
	values := url.Values{"template_id": {strconv.FormatInt(input.TemplateID, 10)}}
	if len(input.Arguments) > 0 {
		values.Set("template_args", string(arguments))
	}
	if input.Target == MessageTargetFriends {
		receivers, _ := json.Marshal(input.ReceiverUUIDs)
		values.Set("receiver_uuids", string(receivers))
	}
	return c.sendTemplate(ctx, "send_custom_message", path, input.Target, input.ReceiverUUIDs, values, options...)
}

func (c *Client) messagePath(operation string, target MessageTarget, receivers []string, defaultTemplate bool) (string, error) {
	suffix := "/send"
	if defaultTemplate {
		suffix = "/default/send"
	}
	switch target {
	case MessageTargetSelf:
		if len(receivers) != 0 {
			return "", invalidArgument(operation, "self messages do not accept receiver UUIDs")
		}
		return "/v2/api/talk/memo" + suffix, nil
	case MessageTargetFriends:
		if err := c.requireFriendApproval(operation); err != nil {
			return "", err
		}
		if len(receivers) < 1 || len(receivers) > 5 {
			return "", invalidArgument(operation, "friend messages require one to five receiver UUIDs")
		}
		seen := make(map[string]struct{}, len(receivers))
		for _, receiver := range receivers {
			if !validBoundedString(receiver, 512) {
				return "", invalidArgument(operation, "receiver UUID is invalid")
			}
			if _, duplicate := seen[receiver]; duplicate {
				return "", invalidArgument(operation, "receiver UUIDs must be unique")
			}
			seen[receiver] = struct{}{}
		}
		return "/v1/api/talk/friends/message" + suffix, nil
	default:
		return "", invalidArgument(operation, "target must be self or friends")
	}
}

func (c *Client) sendTemplate(ctx context.Context, operation, path string, target MessageTarget, requested []string, values url.Values, options ...socialhub.CallOption) (*MessageResult, error) {
	var response messageResponse
	if err := c.form(ctx, path, values, &response, options...); err != nil {
		return nil, err
	}
	if target == MessageTargetSelf {
		if response.ResultCode == nil || *response.ResultCode != 0 {
			return nil, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
		}
		return &MessageResult{Target: target, ResultCode: *response.ResultCode}, nil
	}
	requestedSet := make(map[string]struct{}, len(requested))
	for _, receiver := range requested {
		requestedSet[receiver] = struct{}{}
	}
	seen := make(map[string]struct{}, len(requested))
	result := &MessageResult{Target: target}
	if response.ResultCode != nil {
		result.ResultCode = *response.ResultCode
		if *response.ResultCode != 0 {
			return nil, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
		}
	}
	for _, receiver := range response.SuccessfulReceiverUUIDs {
		if _, requested := requestedSet[receiver]; !requested || !validBoundedString(receiver, 512) {
			return nil, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
		}
		if _, duplicate := seen[receiver]; duplicate {
			return nil, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
		}
		seen[receiver] = struct{}{}
		result.SuccessfulReceiverUUIDs = append(result.SuccessfulReceiverUUIDs, receiver)
	}
	for _, failure := range response.FailureInfo {
		mapped := MessageFailure{Code: failure.Code, Message: boundedMessage(failure.Message, 512)}
		for _, receiver := range failure.ReceiverUUIDs {
			if _, requested := requestedSet[receiver]; !requested || !validBoundedString(receiver, 512) {
				return nil, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
			}
			if _, duplicate := seen[receiver]; duplicate {
				return nil, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
			}
			seen[receiver] = struct{}{}
			mapped.ReceiverUUIDs = append(mapped.ReceiverUUIDs, receiver)
		}
		if len(mapped.ReceiverUUIDs) == 0 {
			return nil, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
		}
		result.Failures = append(result.Failures, mapped)
	}
	return result, nil
}

func (c *Client) SendMessage(ctx context.Context, input socialhub.SendMessageRequest, options ...socialhub.CallOption) (*socialhub.Message, error) {
	if c.defaultLinkURL == "" {
		return nil, unsupported("send_message", "configure account.settings.default_link_url for common text messages")
	}
	if input.Text == nil {
		return nil, invalidArgument("send_message", "text is required")
	}
	if len(input.MediaIDs) != 0 {
		return nil, unsupported("send_message", "Kakao default text templates do not accept common media IDs")
	}
	if input.ReplyToID != nil {
		return nil, unsupported("send_message", "Kakao Talk Message API does not expose message replies")
	}
	target := MessageTarget(input.ConversationID)
	request := DefaultMessageRequest{
		Target: target, ReceiverUUIDs: append([]string(nil), input.RecipientIDs...),
		Template: TextTemplate{Text: *input.Text, Link: Link{WebURL: c.defaultLinkURL, MobileWebURL: c.defaultLinkURL}},
	}
	result, err := c.SendDefault(ctx, request, options...)
	if err != nil {
		return nil, err
	}
	extension, _ := json.Marshal(result)
	recipients := []string{c.userID}
	if target == MessageTargetFriends {
		recipients = deliveredReceivers(input.RecipientIDs, result)
	}
	return &socialhub.Message{
		Platform: "kakao", AccountID: c.accountID, ConversationID: input.ConversationID,
		RecipientIDs: recipients, Text: input.Text, Direction: socialhub.DirectionOutbound,
		Extensions: map[string]json.RawMessage{"kakao.delivery": extension},
	}, nil
}

func (c *Client) GetMessage(context.Context, string, ...socialhub.CallOption) (*socialhub.Message, error) {
	return nil, unsupported("get_message", "Kakao Talk Message API does not expose message lookup")
}

func deliveredReceivers(requested []string, result *MessageResult) []string {
	if len(result.SuccessfulReceiverUUIDs) > 0 {
		return append([]string(nil), result.SuccessfulReceiverUUIDs...)
	}
	failed := make(map[string]struct{})
	for _, failure := range result.Failures {
		for _, receiver := range failure.ReceiverUUIDs {
			failed[receiver] = struct{}{}
		}
	}
	delivered := make([]string, 0, len(requested)-len(failed))
	for _, receiver := range requested {
		if _, failed := failed[receiver]; !failed {
			delivered = append(delivered, receiver)
		}
	}
	return delivered
}
