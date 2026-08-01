package matrix

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"social-hub/pkg/socialhub"
)

const (
	eventDocURL     = "https://spec.matrix.org/v1.19/client-server-api/#put_matrixclientv3roomsroomidsendeventtypetxnid"
	messagesDocURL  = "https://spec.matrix.org/v1.19/client-server-api/#get_matrixclientv3roomsroomidmessages"
	relationsDocURL = "https://spec.matrix.org/v1.19/client-server-api/#get_matrixclientv1roomsroomidrelationseventid"
	mediaDocURL     = "https://spec.matrix.org/v1.19/client-server-api/#post_matrixmediav3upload"
	syncDocURL      = "https://spec.matrix.org/v1.19/client-server-api/#get_matrixclientv3sync"
)

func (client *Client) GetUser(ctx context.Context, userID string, options ...socialhub.CallOption) (*socialhub.User, error) {
	if !validUserID(userID) {
		return nil, invalidArgument("get_user", "a valid Matrix user ID is required")
	}
	var response profileResponse
	if err := client.json(ctx, http.MethodGet, matrixPath("/_matrix/client/v3/profile", userID), nil, nil, &response, options...); err != nil {
		return nil, err
	}
	return client.mapUser(userID, response), nil
}

func (client *Client) GetEvent(ctx context.Context, roomID, eventID string, options ...socialhub.CallOption) (*Event, error) {
	if !validRoomID(roomID) || !validEventID(eventID) {
		return nil, invalidArgument("get_event", "valid Matrix room and event IDs are required")
	}
	var response Event
	path := matrixPath("/_matrix/client/v3/rooms", roomID, "event", eventID)
	if err := client.json(ctx, http.MethodGet, path, nil, nil, &response, options...); err != nil {
		return nil, err
	}
	if response.EventID != eventID || response.RoomID != "" && response.RoomID != roomID {
		return nil, platformError("get_event", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	if response.RoomID == "" {
		response.RoomID = roomID
	}
	return &response, nil
}

func (client *Client) ListRoomMessages(ctx context.Context, input RoomMessagesRequest, options ...socialhub.CallOption) (socialhub.Page[Event], error) {
	if !validRoomID(input.RoomID) || input.MaxResults < 0 || input.Cursor != "" && !validOpaque(input.Cursor, maxOpaqueLength) {
		return socialhub.Page[Event]{}, invalidArgument("list_room_messages", "room ID, cursor, or max results is invalid")
	}
	direction := input.Direction
	if direction == "" {
		direction = "b"
	}
	if direction != "b" && direction != "f" {
		return socialhub.Page[Event]{}, invalidArgument("list_room_messages", "direction must be b or f")
	}
	limit := input.MaxResults
	if limit == 0 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}
	query := url.Values{"dir": {direction}, "limit": {strconv.Itoa(limit)}}
	if input.Cursor != "" {
		query.Set("from", input.Cursor)
	}
	var response roomMessagesResponse
	path := matrixPath("/_matrix/client/v3/rooms", input.RoomID, "messages")
	if err := client.json(ctx, http.MethodGet, path, query, nil, &response, options...); err != nil {
		return socialhub.Page[Event]{}, err
	}
	for index := range response.Chunk {
		if response.Chunk[index].RoomID == "" {
			response.Chunk[index].RoomID = input.RoomID
		}
	}
	var next *string
	if response.End != "" && response.End != input.Cursor {
		next = pointer(response.End)
	}
	return socialhub.Page[Event]{Items: response.Chunk, NextCursor: next, HasMore: next != nil}, nil
}

func (client *Client) SendText(ctx context.Context, input SendTextRequest, options ...socialhub.CallOption) (*EventReference, error) {
	if !validRoomID(input.RoomID) || !validText(input.Text) {
		return nil, invalidArgument("send_text", "valid room ID and non-empty bounded text are required")
	}
	if input.MessageType == "" {
		input.MessageType = MessageTypeText
	}
	if input.MessageType != MessageTypeText && input.MessageType != MessageTypeNotice && input.MessageType != MessageTypeEmote {
		return nil, invalidArgument("send_text", "message type must be m.text, m.notice, or m.emote")
	}
	if input.ReplyToID != "" && !validEventID(input.ReplyToID) || input.ThreadRootID != "" && !validEventID(input.ThreadRootID) {
		return nil, invalidArgument("send_text", "reply and thread root must be valid Matrix event IDs")
	}
	content := MessageContent{MessageType: input.MessageType, Body: input.Text}
	if input.ThreadRootID != "" {
		replyID := firstNonEmpty(input.ReplyToID, input.ThreadRootID)
		content.RelatesTo = &Relation{RelationType: RelationThread, EventID: input.ThreadRootID, InReplyTo: &InReplyTo{EventID: replyID}}
	} else if input.ReplyToID != "" {
		content.RelatesTo = &Relation{InReplyTo: &InReplyTo{EventID: input.ReplyToID}}
	}
	return client.sendEvent(ctx, input.RoomID, EventTypeMessage, content, options...)
}

func (client *Client) SendMedia(ctx context.Context, input SendMediaRequest, options ...socialhub.CallOption) (*EventReference, error) {
	if !validRoomID(input.RoomID) || !validText(input.Body) || !validMXCURI(input.MXCURI) || input.Size < 0 || input.Width < 0 || input.Height < 0 || input.Duration < 0 {
		return nil, invalidArgument("send_media", "room, body, mxc URI, size, dimensions, or duration is invalid")
	}
	if input.MessageType != MessageTypeImage && input.MessageType != MessageTypeVideo && input.MessageType != MessageTypeAudio && input.MessageType != MessageTypeFile {
		return nil, invalidArgument("send_media", "message type must be an image, video, audio, or file type")
	}
	if input.MIME != "" && !validMIME(input.MIME) {
		return nil, invalidArgument("send_media", "MIME type is invalid")
	}
	content := MessageContent{MessageType: input.MessageType, Body: input.Body, URL: input.MXCURI}
	if input.MIME != "" || input.Size > 0 || input.Width > 0 || input.Height > 0 || input.Duration > 0 {
		content.Info = &MediaInfo{MIMEType: input.MIME, Size: input.Size, Width: input.Width, Height: input.Height, Duration: input.Duration.Milliseconds()}
	}
	return client.sendEvent(ctx, input.RoomID, EventTypeMessage, content, options...)
}

func (client *Client) sendEvent(ctx context.Context, roomID, eventType string, content any, options ...socialhub.CallOption) (*EventReference, error) {
	transaction, err := transactionID(options...)
	if err != nil {
		return nil, err
	}
	var response eventIDResponse
	path := matrixPath("/_matrix/client/v3/rooms", roomID, "send", eventType, transaction)
	if err := client.json(ctx, http.MethodPut, path, nil, content, &response, options...); err != nil {
		return nil, err
	}
	if !validEventID(response.EventID) {
		return nil, platformError("send_event", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return &EventReference{RoomID: roomID, EventID: response.EventID, ID: composeID(roomID, response.EventID)}, nil
}

func (client *Client) Redact(ctx context.Context, roomID, eventID, reason string, options ...socialhub.CallOption) (*EventReference, error) {
	if !validRoomID(roomID) || !validEventID(eventID) || len(reason) > 1<<20 || strings.ContainsFunc(reason, unsafeControl) {
		return nil, invalidArgument("redact", "room ID, event ID, or reason is invalid")
	}
	transaction, err := transactionID(options...)
	if err != nil {
		return nil, err
	}
	var response eventIDResponse
	path := matrixPath("/_matrix/client/v3/rooms", roomID, "redact", eventID, transaction)
	if err := client.json(ctx, http.MethodPut, path, nil, struct {
		Reason string `json:"reason,omitempty"`
	}{Reason: reason}, &response, options...); err != nil {
		return nil, err
	}
	if !validEventID(response.EventID) {
		return nil, platformError("redact", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return &EventReference{RoomID: roomID, EventID: response.EventID, ID: composeID(roomID, response.EventID)}, nil
}

func (client *Client) GetPost(ctx context.Context, postID string, options ...socialhub.CallOption) (*socialhub.Post, error) {
	roomID, eventID, err := parseCompositeID("get_post", postID, client.defaultRoomID)
	if err != nil {
		return nil, err
	}
	event, err := client.GetEvent(ctx, roomID, eventID, options...)
	if err != nil {
		return nil, err
	}
	return client.mapPost(roomID, *event)
}

func (client *Client) ListPosts(ctx context.Context, input socialhub.ListPostsRequest, options ...socialhub.CallOption) (socialhub.Page[socialhub.Post], error) {
	if client.defaultRoomID == "" {
		return socialhub.Page[socialhub.Post]{}, invalidArgument("list_posts", "account.settings.default_room_id is required")
	}
	if input.UserID != "" {
		return socialhub.Page[socialhub.Post]{}, unsupported("list_posts", "Matrix history is room-based; user filtering is not supported by this endpoint")
	}
	if input.StartTime != nil || input.EndTime != nil {
		return socialhub.Page[socialhub.Post]{}, unsupported("list_posts", "Matrix room history does not accept time-range filters")
	}
	events, err := client.ListRoomMessages(ctx, RoomMessagesRequest{RoomID: client.defaultRoomID, Cursor: input.Cursor, MaxResults: input.MaxResults, Direction: "b"}, options...)
	if err != nil {
		return socialhub.Page[socialhub.Post]{}, err
	}
	items := make([]socialhub.Post, 0, len(events.Items))
	for _, event := range events.Items {
		if event.Type != EventTypeMessage {
			continue
		}
		post, err := client.mapPost(client.defaultRoomID, event)
		if err != nil {
			return socialhub.Page[socialhub.Post]{}, err
		}
		items = append(items, *post)
	}
	return socialhub.Page[socialhub.Post]{Items: items, NextCursor: events.NextCursor, HasMore: events.HasMore}, nil
}

func (client *Client) Publish(ctx context.Context, input socialhub.CreatePostRequest, options ...socialhub.CallOption) (*socialhub.Post, error) {
	if client.defaultRoomID == "" {
		return nil, invalidArgument("publish", "account.settings.default_room_id is required")
	}
	if input.Text == nil || !validText(*input.Text) {
		return nil, invalidArgument("publish", "non-empty bounded text is required")
	}
	if len(input.MediaIDs) > 0 || input.QuotePostID != nil || input.Visibility != nil {
		return nil, unsupported("publish", "common Matrix publishing supports text and replies; use EventWorkflow for media and relations")
	}
	replyID := ""
	if input.ReplyToID != nil {
		roomID, eventID, err := parseCompositeID("publish", *input.ReplyToID, client.defaultRoomID)
		if err != nil || roomID != client.defaultRoomID {
			return nil, invalidArgument("publish", "reply target must belong to the configured default room")
		}
		replyID = eventID
	}
	reference, err := client.SendText(ctx, SendTextRequest{RoomID: client.defaultRoomID, MessageType: MessageTypeText, Text: *input.Text, ReplyToID: replyID}, options...)
	if err != nil {
		return nil, err
	}
	now := client.clock.Now().UTC()
	event := Event{Type: EventTypeMessage, RoomID: reference.RoomID, EventID: reference.EventID, Sender: client.userID, OriginServerTS: now.UnixMilli()}
	event.Content, _ = jsonMarshal(MessageContent{MessageType: MessageTypeText, Body: *input.Text, RelatesTo: replyRelation(replyID)})
	return client.mapPost(reference.RoomID, event)
}

func (client *Client) PublishStatus(ctx context.Context, postID string, options ...socialhub.CallOption) (*socialhub.PublishStatus, error) {
	post, err := client.GetPost(ctx, postID, options...)
	if err != nil {
		return nil, err
	}
	return post.Status, nil
}

func (client *Client) DeletePost(ctx context.Context, postID string, options ...socialhub.CallOption) error {
	roomID, eventID, err := parseCompositeID("delete_post", postID, client.defaultRoomID)
	if err != nil {
		return err
	}
	_, err = client.Redact(ctx, roomID, eventID, "", options...)
	return err
}

func (client *Client) SendMessage(ctx context.Context, input socialhub.SendMessageRequest, options ...socialhub.CallOption) (*socialhub.Message, error) {
	roomID := input.ConversationID
	if roomID == "" {
		roomID = client.defaultRoomID
	}
	if !validRoomID(roomID) || input.Text == nil || !validText(*input.Text) {
		return nil, invalidArgument("send_message", "valid conversation room ID and non-empty bounded text are required")
	}
	if len(input.RecipientIDs) > 0 || len(input.MediaIDs) > 0 {
		return nil, unsupported("send_message", "Matrix room membership determines recipients; use EventWorkflow for media")
	}
	replyID := ""
	if input.ReplyToID != nil {
		replyRoom, eventID, err := parseCompositeID("send_message", *input.ReplyToID, roomID)
		if err != nil || replyRoom != roomID {
			return nil, invalidArgument("send_message", "reply target must belong to the destination room")
		}
		replyID = eventID
	}
	reference, err := client.SendText(ctx, SendTextRequest{RoomID: roomID, MessageType: MessageTypeText, Text: *input.Text, ReplyToID: replyID}, options...)
	if err != nil {
		return nil, err
	}
	now := client.clock.Now().UTC()
	event := Event{Type: EventTypeMessage, RoomID: roomID, EventID: reference.EventID, Sender: client.userID, OriginServerTS: now.UnixMilli()}
	event.Content, _ = jsonMarshal(MessageContent{MessageType: MessageTypeText, Body: *input.Text, RelatesTo: replyRelation(replyID)})
	return client.mapMessage(roomID, event, socialhub.DirectionOutbound)
}

func (client *Client) GetMessage(ctx context.Context, messageID string, options ...socialhub.CallOption) (*socialhub.Message, error) {
	roomID, eventID, err := parseCompositeID("get_message", messageID, client.defaultRoomID)
	if err != nil {
		return nil, err
	}
	event, err := client.GetEvent(ctx, roomID, eventID, options...)
	if err != nil {
		return nil, err
	}
	direction := socialhub.DirectionInbound
	if event.Sender == client.userID {
		direction = socialhub.DirectionOutbound
	}
	return client.mapMessage(roomID, *event, direction)
}

func replyRelation(eventID string) *Relation {
	if eventID == "" {
		return nil
	}
	return &Relation{InReplyTo: &InReplyTo{EventID: eventID}}
}

func jsonMarshal(value any) ([]byte, error) {
	return json.Marshal(value)
}

var _ EventWorkflow = (*Client)(nil)
