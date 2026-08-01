package slack

import (
	"context"
	"encoding/json"
	"strings"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

func (c *Client) Publish(ctx context.Context, input socialhub.CreatePostRequest, options ...socialhub.CallOption) (*socialhub.Post, error) {
	if c.defaultChannelID == "" {
		return nil, unsupported("publish", "configure default_channel_id for common Slack publishing")
	}
	if input.QuotePostID != nil {
		return nil, unsupported("publish", "Slack has thread replies but no generic quote-post primitive")
	}
	if input.Visibility != nil {
		return nil, unsupported("publish", "Slack message visibility is determined by the conversation")
	}
	if len(input.MediaIDs) > 0 {
		return nil, unsupported("publish", "Slack files are shared during FileWorkflow completion, not attached by file ID")
	}
	request := PostMessageRequest{ChannelID: c.defaultChannelID}
	if input.Text != nil {
		request.Text = *input.Text
	}
	if input.ReplyToID != nil {
		request.ThreadPostID = *input.ReplyToID
	}
	return c.PostMessage(ctx, request, options...)
}

func (c *Client) PostMessage(ctx context.Context, input PostMessageRequest, options ...socialhub.CallOption) (*socialhub.Post, error) {
	if err := c.requireScopes("chat.postMessage", "chat:write"); err != nil {
		return nil, err
	}
	if !validSlackID(input.ChannelID, "CGD") {
		return nil, invalidArgument("chat.postMessage", "channel_id must be a Slack conversation ID")
	}
	if strings.TrimSpace(input.Text) == "" || utf8.RuneCountInString(input.Text) > 40000 {
		return nil, invalidArgument("chat.postMessage", "text must contain 1 to 40000 Unicode code points")
	}
	resolved, err := socialhub.ResolveCallOptions(options...)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(resolved.IdempotencyKey) != "" {
		return nil, unsupported("chat.postMessage", "Slack Web API does not document an idempotency key for chat.postMessage")
	}
	request := struct {
		Channel  string `json:"channel"`
		Text     string `json:"text"`
		ThreadTS string `json:"thread_ts,omitempty"`
	}{Channel: input.ChannelID, Text: input.Text}
	if input.ThreadPostID != "" {
		channelID, timestamp, err := parseCompositeID(input.ThreadPostID, "chat.postMessage")
		if err != nil {
			return nil, err
		}
		if channelID != input.ChannelID {
			return nil, invalidArgument("chat.postMessage", "thread parent must belong to the target conversation")
		}
		request.ThreadTS = timestamp
	}
	var response struct {
		Channel string      `json:"channel"`
		TS      string      `json:"ts"`
		Message wireMessage `json:"message"`
	}
	if err := c.call(ctx, "chat.postMessage", request, &response, options...); err != nil {
		return nil, err
	}
	channelID := firstNonEmpty(response.Channel, input.ChannelID)
	if response.Message.TS == "" {
		response.Message.TS = response.TS
	}
	if response.Message.Text == "" {
		response.Message.Text = input.Text
	}
	if response.Message.ThreadTS == "" {
		response.Message.ThreadTS = request.ThreadTS
	}
	if !validSlackID(channelID, "CGD") || !validTimestamp(response.Message.TS) {
		return nil, platformError("chat.postMessage", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return mapPost(c.accountID, channelID, response.Message, c.clock.Now()), nil
}

func (c *Client) UpdateMessage(ctx context.Context, input UpdateMessageRequest, options ...socialhub.CallOption) (*socialhub.Post, error) {
	if err := c.requireScopes("chat.update", "chat:write"); err != nil {
		return nil, err
	}
	channelID, timestamp, err := parseCompositeID(input.PostID, "chat.update")
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(input.Text) == "" || utf8.RuneCountInString(input.Text) > 40000 {
		return nil, invalidArgument("chat.update", "text must contain 1 to 40000 Unicode code points")
	}
	var response struct {
		Channel string      `json:"channel"`
		TS      string      `json:"ts"`
		Text    string      `json:"text"`
		Message wireMessage `json:"message"`
	}
	if err := c.call(ctx, "chat.update", struct {
		Channel string `json:"channel"`
		TS      string `json:"ts"`
		Text    string `json:"text"`
	}{Channel: channelID, TS: timestamp, Text: input.Text}, &response, options...); err != nil {
		return nil, err
	}
	if response.Message.TS == "" {
		response.Message.TS = firstNonEmpty(response.TS, timestamp)
	}
	if response.Message.Text == "" {
		response.Message.Text = firstNonEmpty(response.Text, input.Text)
	}
	responseChannel := firstNonEmpty(response.Channel, channelID)
	if responseChannel != channelID || !validTimestamp(response.Message.TS) {
		return nil, platformError("chat.update", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return mapPost(c.accountID, responseChannel, response.Message, c.clock.Now()), nil
}

func (c *Client) PublishStatus(ctx context.Context, postID string, options ...socialhub.CallOption) (*socialhub.PublishStatus, error) {
	post, err := c.GetPost(ctx, postID, options...)
	if err != nil {
		return nil, err
	}
	if post.Status == nil {
		return nil, platformError("publish_status", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return post.Status, nil
}

func (c *Client) DeletePost(ctx context.Context, postID string, options ...socialhub.CallOption) error {
	if err := c.requireScopes("chat.delete", "chat:write"); err != nil {
		return err
	}
	channelID, timestamp, err := parseCompositeID(postID, "chat.delete")
	if err != nil {
		return err
	}
	var response struct {
		Channel string `json:"channel"`
		TS      string `json:"ts"`
	}
	if err := c.call(ctx, "chat.delete", struct {
		Channel string `json:"channel"`
		TS      string `json:"ts"`
	}{Channel: channelID, TS: timestamp}, &response, options...); err != nil {
		return err
	}
	if response.Channel != channelID || response.TS != timestamp {
		return platformError("chat.delete", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return nil
}

func (c *Client) SendMessage(ctx context.Context, input socialhub.SendMessageRequest, options ...socialhub.CallOption) (*socialhub.Message, error) {
	if len(input.RecipientIDs) > 0 {
		return nil, unsupported("messages.send", "Slack sends to one conversation ID per call")
	}
	if len(input.MediaIDs) > 0 {
		return nil, unsupported("messages.send", "use FileWorkflow to share Slack files")
	}
	text := ""
	if input.Text != nil {
		text = *input.Text
	}
	request := PostMessageRequest{ChannelID: input.ConversationID, Text: text}
	if input.ReplyToID != nil {
		request.ThreadPostID = *input.ReplyToID
	}
	post, err := c.PostMessage(ctx, request, options...)
	if err != nil {
		return nil, err
	}
	message := &socialhub.Message{
		Platform: "slack", AccountID: c.accountID, ID: post.ID, ConversationID: input.ConversationID,
		SenderID: post.AuthorID, RecipientIDs: []string{input.ConversationID}, Text: post.Text,
		SentAt: post.CreatedAt, Direction: socialhub.DirectionOutbound, Extensions: post.Extensions,
	}
	if input.ReplyToID != nil {
		reply := strings.TrimSpace(*input.ReplyToID)
		message.ReplyToID = &reply
	}
	return message, nil
}

func (c *Client) GetMessage(ctx context.Context, messageID string, options ...socialhub.CallOption) (*socialhub.Message, error) {
	channelID, _, err := parseCompositeID(messageID, "messages.get")
	if err != nil {
		return nil, err
	}
	post, err := c.GetPost(ctx, messageID, options...)
	if err != nil {
		return nil, err
	}
	var raw wireMessage
	if extension := post.Extensions["slack.message"]; len(extension) > 0 {
		_ = json.Unmarshal(extension, &raw)
	}
	message := mapMessage(c.accountID, c.actorID, channelID, raw)
	if message.ID == channelID+":" {
		return nil, platformError("messages.get", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return message, nil
}
