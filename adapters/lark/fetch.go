package lark

import (
	"context"
	"net/url"
	"strconv"
	"strings"

	"social-hub/pkg/socialhub"
)

type listMessagesResponse struct {
	Data struct {
		HasMore   bool          `json:"has_more"`
		PageToken string        `json:"page_token"`
		Items     []wireMessage `json:"items"`
	} `json:"data"`
}

func (c *Client) GetUser(ctx context.Context, userID string, options ...socialhub.CallOption) (*socialhub.User, error) {
	if err := c.requireAnyScope("contact.user.get", "contact:contact.base:readonly", "contact:contact:readonly", "contact:contact:readonly_as_app"); err != nil {
		return nil, err
	}
	userID = strings.TrimSpace(userID)
	if !validOpaqueID(userID, 512) {
		return nil, invalidArgument("contact.user.get", "user_id must be a bounded opaque ID")
	}
	var response struct {
		Data struct {
			User wireUser `json:"user"`
		} `json:"data"`
	}
	path := "/open-apis/contact/v3/users/" + url.PathEscape(userID)
	if err := c.get(ctx, "contact.user.get", path, url.Values{
		"user_id_type": {string(c.userIDType)}, "department_id_type": {"open_department_id"},
	}, &response, options...); err != nil {
		return nil, err
	}
	if selectedUserID(response.Data.User, c.userIDType) != userID {
		return nil, platformError("contact.user.get", socialhub.CodeNotFound, socialhub.ClassPermanent, nil)
	}
	return mapUser(c.accountID, userID, response.Data.User), nil
}

func selectedUserID(user wireUser, kind UserIDType) string {
	switch kind {
	case UserIDUnionID:
		return user.UnionID
	case UserIDUserID:
		return user.UserID
	default:
		return user.OpenID
	}
}

func (c *Client) GetMessage(ctx context.Context, messageID string, options ...socialhub.CallOption) (*socialhub.Message, error) {
	if err := c.requireMessageRead("im.message.get"); err != nil {
		return nil, err
	}
	wire, err := c.getWireMessage(ctx, messageID, options...)
	if err != nil {
		return nil, err
	}
	return mapMessage(c.accountID, c.actorID, wire), nil
}

func (c *Client) GetPost(ctx context.Context, postID string, options ...socialhub.CallOption) (*socialhub.Post, error) {
	if err := c.requireMessageRead("im.message.get"); err != nil {
		return nil, err
	}
	wire, err := c.getWireMessage(ctx, postID, options...)
	if err != nil {
		return nil, err
	}
	return mapPost(c.accountID, wire, c.clock.Now()), nil
}

func (c *Client) getWireMessage(ctx context.Context, messageID string, options ...socialhub.CallOption) (wireMessage, error) {
	messageID = strings.TrimSpace(messageID)
	if !validMessageID(messageID) {
		return wireMessage{}, invalidArgument("im.message.get", "message_id must be a Lark message ID")
	}
	var response struct {
		Data struct {
			Items []wireMessage `json:"items"`
		} `json:"data"`
	}
	if err := c.get(ctx, "im.message.get", "/open-apis/im/v1/messages/"+url.PathEscape(messageID), nil, &response, options...); err != nil {
		return wireMessage{}, err
	}
	if len(response.Data.Items) != 1 || response.Data.Items[0].MessageID != messageID || !validChatID(response.Data.Items[0].ChatID) {
		return wireMessage{}, platformError("im.message.get", socialhub.CodeNotFound, socialhub.ClassPermanent, nil)
	}
	return response.Data.Items[0], nil
}

func (c *Client) ListPosts(ctx context.Context, input socialhub.ListPostsRequest, options ...socialhub.CallOption) (socialhub.Page[socialhub.Post], error) {
	if err := c.requireMessageRead("im.message.list"); err != nil {
		return socialhub.Page[socialhub.Post]{}, err
	}
	chatID := strings.TrimSpace(input.UserID)
	if chatID == "" {
		chatID = c.defaultChatID
	}
	if !validChatID(chatID) {
		return socialhub.Page[socialhub.Post]{}, invalidArgument("im.message.list", "user_id or default_chat_id must identify a Lark chat")
	}
	query, err := listQuery("chat", chatID, input.Cursor, input.MaxResults)
	if err != nil {
		return socialhub.Page[socialhub.Post]{}, err
	}
	query.Set("sort_type", "ByCreateTimeDesc")
	query.Set("only_thread_root_messages", "true")
	query.Set("with_sender_name", "true")
	if input.StartTime != nil {
		query.Set("start_time", strconv.FormatInt(input.StartTime.UTC().Unix(), 10))
	}
	if input.EndTime != nil {
		if input.StartTime != nil && input.EndTime.Before(*input.StartTime) {
			return socialhub.Page[socialhub.Post]{}, invalidArgument("im.message.list", "end_time must not precede start_time")
		}
		query.Set("end_time", strconv.FormatInt(input.EndTime.UTC().Unix(), 10))
	}
	var response listMessagesResponse
	if err := c.get(ctx, "im.message.list", "/open-apis/im/v1/messages", query, &response, options...); err != nil {
		return socialhub.Page[socialhub.Post]{}, err
	}
	items := make([]socialhub.Post, 0, len(response.Data.Items))
	observedAt := c.clock.Now()
	for _, message := range response.Data.Items {
		if validMessageID(message.MessageID) && (message.RootID == "" || message.RootID == message.MessageID) {
			items = append(items, *mapPost(c.accountID, message, observedAt))
		}
	}
	return larkPage(items, response.Data.PageToken, response.Data.HasMore), nil
}

func (c *Client) ListComments(ctx context.Context, input socialhub.ListCommentsRequest, options ...socialhub.CallOption) (socialhub.Page[socialhub.Comment], error) {
	if err := c.requireMessageRead("im.message.list_comments"); err != nil {
		return socialhub.Page[socialhub.Comment]{}, err
	}
	root, err := c.getWireMessage(ctx, input.PostID, options...)
	if err != nil {
		return socialhub.Page[socialhub.Comment]{}, err
	}
	containerType, containerID := "chat", root.ChatID
	if strings.HasPrefix(root.ThreadID, "omt_") && validOpaqueID(root.ThreadID, 512) {
		containerType, containerID = "thread", root.ThreadID
	}
	query, err := listQuery(containerType, containerID, input.Cursor, input.MaxResults)
	if err != nil {
		return socialhub.Page[socialhub.Comment]{}, err
	}
	query.Set("sort_type", "ByCreateTimeAsc")
	query.Set("with_sender_name", "true")
	if containerType == "chat" {
		if created, ok := larkTime(root.CreateTime); ok {
			query.Set("start_time", strconv.FormatInt(created.Unix(), 10))
		}
	}
	var response listMessagesResponse
	if err := c.get(ctx, "im.message.list_comments", "/open-apis/im/v1/messages", query, &response, options...); err != nil {
		return socialhub.Page[socialhub.Comment]{}, err
	}
	items := make([]socialhub.Comment, 0, len(response.Data.Items))
	observedAt := c.clock.Now()
	for _, message := range response.Data.Items {
		if message.MessageID == input.PostID || !validMessageID(message.MessageID) {
			continue
		}
		if message.RootID == input.PostID || (message.RootID == "" && message.ParentID == input.PostID) {
			items = append(items, mapComment(c.accountID, input.PostID, message, observedAt))
		}
	}
	return larkPage(items, response.Data.PageToken, response.Data.HasMore), nil
}

func listQuery(containerType, containerID, cursor string, maximum int) (url.Values, error) {
	if (containerType != "chat" && containerType != "thread") || !validOpaqueID(containerID, 512) {
		return nil, invalidArgument("pagination", "valid chat or thread container is required")
	}
	if !validCursor(cursor) {
		return nil, invalidArgument("pagination", "cursor must be a bounded opaque page token")
	}
	if maximum < 0 {
		return nil, invalidArgument("pagination", "max_results must not be negative")
	}
	if maximum == 0 {
		maximum = 20
	}
	if maximum > 50 {
		maximum = 50
	}
	query := url.Values{
		"container_id_type": {containerType}, "container_id": {containerID},
		"page_size": {strconv.Itoa(maximum)},
	}
	if cursor != "" {
		query.Set("page_token", cursor)
	}
	return query, nil
}

func validCursor(value string) bool {
	if len(value) > 2048 {
		return false
	}
	for _, character := range value {
		if character < 0x20 || character == 0x7f {
			return false
		}
	}
	return true
}

func larkPage[T any](items []T, cursor string, hasMore bool) socialhub.Page[T] {
	var next *string
	if strings.TrimSpace(cursor) != "" {
		next = stringPointer(cursor)
	}
	return socialhub.Page[T]{Items: items, NextCursor: next, HasMore: hasMore || next != nil}
}
