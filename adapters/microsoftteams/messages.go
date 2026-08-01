package microsoftteams

import (
	"context"
	"encoding/json"
	"strings"

	"social-hub/pkg/socialhub"
)

type messageInput struct {
	Body           MessageBody     `json:"body"`
	Subject        string          `json:"subject,omitempty"`
	Importance     string          `json:"importance,omitempty"`
	Attachments    []Attachment    `json:"attachments,omitempty"`
	HostedContents []HostedContent `json:"hostedContents,omitempty"`
}

func (c *Client) Send(ctx context.Context, input SendRequest, options ...socialhub.CallOption) (*ChatMessage, error) {
	const operation = "send_message"
	if err := input.Target.validate(operation); err != nil {
		return nil, err
	}
	if err := validateMessageInput(operation, input.Body, input.Importance, input.HostedContents); err != nil {
		return nil, err
	}
	if err := c.requireSend(operation, input.Target); err != nil {
		return nil, err
	}
	request := messageInput{
		Body: input.Body, Subject: input.Subject, Importance: input.Importance,
		Attachments: append([]Attachment(nil), input.Attachments...), HostedContents: append([]HostedContent(nil), input.HostedContents...),
	}
	var raw json.RawMessage
	if err := c.post(ctx, operation, targetCollectionPath(input.Target), request, &raw, options...); err != nil {
		return nil, err
	}
	return decodeMessage(raw)
}

func (c *Client) Reply(ctx context.Context, input ReplyRequest, options ...socialhub.CallOption) (*ChatMessage, error) {
	const operation = "reply_message"
	if err := input.Parent.validate(operation, false); err != nil {
		return nil, err
	}
	if err := validateMessageInput(operation, input.Body, "", input.HostedContents); err != nil {
		return nil, err
	}
	if err := c.requireSend(operation, input.Parent.Target); err != nil {
		return nil, err
	}
	request := messageInput{
		Body: input.Body, Attachments: append([]Attachment(nil), input.Attachments...),
		HostedContents: append([]HostedContent(nil), input.HostedContents...),
	}
	var raw json.RawMessage
	if err := c.post(ctx, operation, repliesPath(input.Parent), request, &raw, options...); err != nil {
		return nil, err
	}
	return decodeMessage(raw)
}

func (c *Client) Get(ctx context.Context, ref MessageRef, options ...socialhub.CallOption) (*ChatMessage, error) {
	const operation = "get_message"
	if err := ref.validate(operation, true); err != nil {
		return nil, err
	}
	if err := c.requireRead(operation, ref.Target); err != nil {
		return nil, err
	}
	var raw json.RawMessage
	if err := c.get(ctx, operation, messagePath(ref), nil, &raw, options...); err != nil {
		return nil, err
	}
	return decodeMessage(raw)
}

func (c *Client) Update(ctx context.Context, input UpdateRequest, options ...socialhub.CallOption) (*ChatMessage, error) {
	const operation = "update_message"
	if err := input.Message.validate(operation, true); err != nil {
		return nil, err
	}
	if c.cloud != CloudGlobal {
		return nil, unsupported(operation, "ordinary message editing is documented only for the global Microsoft Graph deployment")
	}
	if err := validateMessageInput(operation, input.Body, "", nil); err != nil {
		return nil, err
	}
	if err := c.requireEdit(operation, input.Message.Target); err != nil {
		return nil, err
	}
	var raw json.RawMessage
	if err := c.patch(ctx, operation, messagePath(input.Message), messageInput{Body: input.Body}, &raw, options...); err != nil {
		return nil, err
	}
	return decodeMessage(raw)
}

func (c *Client) SoftDelete(ctx context.Context, ref MessageRef, options ...socialhub.CallOption) error {
	const operation = "soft_delete_message"
	if err := ref.validate(operation, true); err != nil {
		return err
	}
	if err := c.requireEdit(operation, ref.Target); err != nil {
		return err
	}
	return c.post(ctx, operation, messagePath(ref)+"/softDelete", nil, nil, options...)
}

func validateMessageInput(operation string, body MessageBody, importance string, hosted []HostedContent) error {
	if body.ContentType != "text" && body.ContentType != "html" {
		return invalidArgument(operation, "body.content_type must be text or html")
	}
	if strings.TrimSpace(body.Content) == "" {
		return invalidArgument(operation, "body.content must not be empty")
	}
	if importance != "" && importance != "normal" && importance != "high" && importance != "urgent" {
		return invalidArgument(operation, "importance must be normal, high, or urgent")
	}
	total := 0
	seen := make(map[string]struct{}, len(hosted))
	for _, content := range hosted {
		if !validOpaqueID(content.TemporaryID, 128) || strings.TrimSpace(content.ContentType) == "" || len(content.ContentBytes) == 0 {
			return invalidArgument(operation, "hosted content requires a unique temporary ID, content type, and bytes")
		}
		if _, exists := seen[content.TemporaryID]; exists {
			return invalidArgument(operation, "hosted content temporary IDs must be unique")
		}
		seen[content.TemporaryID] = struct{}{}
		total += len(content.ContentBytes)
		if total > 4<<20 {
			return invalidArgument(operation, "hosted content must not exceed 4 MB per message")
		}
	}
	return nil
}

func (c *Client) SendMessage(ctx context.Context, input socialhub.SendMessageRequest, options ...socialhub.CallOption) (*socialhub.Message, error) {
	if len(input.RecipientIDs) > 0 {
		return nil, unsupported("send_message", "Teams addresses messages through a chat or channel target, not a separate recipient list")
	}
	if len(input.MediaIDs) > 0 {
		return nil, unsupported("send_message", "use MessageWorkflow with hosted content or Graph-backed attachments")
	}
	if input.Text == nil || strings.TrimSpace(*input.Text) == "" {
		return nil, invalidArgument("send_message", "non-empty text is required")
	}
	target, err := ParseConversationRef(input.ConversationID)
	if err != nil {
		return nil, err
	}
	if input.ReplyToID != nil {
		parent, err := ParseMessageRef(*input.ReplyToID)
		if err != nil || parent.ReplyID != "" || parent.Target != target {
			return nil, invalidArgument("send_message", "reply_to_id must identify a root message in the same target")
		}
		message, err := c.Reply(ctx, ReplyRequest{Parent: parent, Body: MessageBody{ContentType: "text", Content: *input.Text}}, options...)
		if err != nil {
			return nil, err
		}
		ref := MessageRef{Target: target, RootID: parent.RootID, ReplyID: message.ID}
		mapped := mapMessage(c.accountID, ref, *message, socialhub.DirectionOutbound)
		return &mapped, nil
	}
	message, err := c.Send(ctx, SendRequest{Target: target, Body: MessageBody{ContentType: "text", Content: *input.Text}}, options...)
	if err != nil {
		return nil, err
	}
	ref := MessageRef{Target: target, RootID: message.ID}
	mapped := mapMessage(c.accountID, ref, *message, socialhub.DirectionOutbound)
	return &mapped, nil
}

func (c *Client) GetMessage(ctx context.Context, id string, options ...socialhub.CallOption) (*socialhub.Message, error) {
	ref, err := ParseMessageRef(id)
	if err != nil {
		return nil, err
	}
	message, err := c.Get(ctx, ref, options...)
	if err != nil {
		return nil, err
	}
	mapped := mapMessage(c.accountID, ref, *message, c.direction(*message))
	return &mapped, nil
}

func (c *Client) Publish(ctx context.Context, input socialhub.CreatePostRequest, options ...socialhub.CallOption) (*socialhub.Post, error) {
	if c.defaultTarget == nil {
		return nil, invalidArgument("publish", "a default chat or channel target is required")
	}
	if input.Text == nil || strings.TrimSpace(*input.Text) == "" {
		return nil, invalidArgument("publish", "non-empty text is required")
	}
	if len(input.MediaIDs) > 0 || input.QuotePostID != nil || input.Visibility != nil {
		return nil, unsupported("publish", "common publish supports plain text and replies; use MessageWorkflow for Graph-specific content")
	}
	if input.ReplyToID != nil {
		parent, err := ParseMessageRef(*input.ReplyToID)
		if err != nil || parent.ReplyID != "" || parent.Target != *c.defaultTarget {
			return nil, invalidArgument("publish", "reply_to_id must identify a root message in the configured default target")
		}
		message, err := c.Reply(ctx, ReplyRequest{Parent: parent, Body: MessageBody{ContentType: "text", Content: *input.Text}}, options...)
		if err != nil {
			return nil, err
		}
		mapped := mapPost(c.accountID, MessageRef{Target: *c.defaultTarget, RootID: parent.RootID, ReplyID: message.ID}, *message)
		return &mapped, nil
	}
	message, err := c.Send(ctx, SendRequest{Target: *c.defaultTarget, Body: MessageBody{ContentType: "text", Content: *input.Text}}, options...)
	if err != nil {
		return nil, err
	}
	mapped := mapPost(c.accountID, MessageRef{Target: *c.defaultTarget, RootID: message.ID}, *message)
	return &mapped, nil
}

func (c *Client) PublishStatus(ctx context.Context, id string, options ...socialhub.CallOption) (*socialhub.PublishStatus, error) {
	post, err := c.GetPost(ctx, id, options...)
	if err != nil {
		return nil, err
	}
	return post.Status, nil
}

func (c *Client) DeletePost(ctx context.Context, id string, options ...socialhub.CallOption) error {
	ref, err := ParseMessageRef(id)
	if err != nil {
		return err
	}
	return c.SoftDelete(ctx, ref, options...)
}
