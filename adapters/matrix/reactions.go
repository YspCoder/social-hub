package matrix

import (
	"context"
	"net/http"
	"net/url"
	"strconv"

	"social-hub/pkg/socialhub"
)

func (client *Client) SendReaction(ctx context.Context, input ReactionEventRequest, options ...socialhub.CallOption) (*EventReference, error) {
	if !validRoomID(input.RoomID) || !validEventID(input.TargetEventID) || !validText(input.Key) || len(input.Key) > 1024 {
		return nil, invalidArgument("send_reaction", "valid room, target event, and bounded reaction key are required")
	}
	content := struct {
		RelatesTo Relation `json:"m.relates_to"`
	}{RelatesTo: Relation{RelationType: RelationAnnotation, EventID: input.TargetEventID, Key: input.Key}}
	return client.sendEvent(ctx, input.RoomID, EventTypeReaction, content, options...)
}

func (client *Client) React(ctx context.Context, input socialhub.ReactionRequest, options ...socialhub.CallOption) error {
	if input.Kind != socialhub.ReactionLike || input.ActorID != "" && input.ActorID != client.userID {
		return invalidArgument("react", "Matrix common reactions support thumbs-up by the configured user")
	}
	roomID, eventID, err := parseCompositeID("react", input.TargetID, client.defaultRoomID)
	if err != nil {
		return err
	}
	_, err = client.SendReaction(ctx, ReactionEventRequest{RoomID: roomID, TargetEventID: eventID, Key: "👍"}, options...)
	return err
}

func (client *Client) RemoveReaction(context.Context, socialhub.ReactionRequest, ...socialhub.CallOption) error {
	return unsupported("remove_reaction", "Matrix removes the reaction event by redaction; use EventWorkflow.Redact with its event ID")
}

func (client *Client) ListComments(ctx context.Context, input socialhub.ListCommentsRequest, options ...socialhub.CallOption) (socialhub.Page[socialhub.Comment], error) {
	roomID, rootEventID, err := parseCompositeID("list_comments", input.PostID, client.defaultRoomID)
	if err != nil {
		return socialhub.Page[socialhub.Comment]{}, err
	}
	if input.MaxResults < 0 || input.Cursor != "" && !validOpaque(input.Cursor, maxOpaqueLength) {
		return socialhub.Page[socialhub.Comment]{}, invalidArgument("list_comments", "cursor or max results is invalid")
	}
	limit := input.MaxResults
	if limit == 0 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}
	query := url.Values{"dir": {"b"}, "limit": {strconv.Itoa(limit)}}
	if input.Cursor != "" {
		query.Set("from", input.Cursor)
	}
	path := matrixPath("/_matrix/client/v1/rooms", roomID, "relations", rootEventID, RelationThread, EventTypeMessage)
	var response relationsResponse
	if err := client.json(ctx, http.MethodGet, path, query, nil, &response, options...); err != nil {
		return socialhub.Page[socialhub.Comment]{}, err
	}
	items := make([]socialhub.Comment, 0, len(response.Chunk))
	for _, event := range response.Chunk {
		comment, err := client.mapComment(roomID, rootEventID, event)
		if err != nil {
			return socialhub.Page[socialhub.Comment]{}, err
		}
		items = append(items, *comment)
	}
	var next *string
	if response.NextBatch != "" && response.NextBatch != input.Cursor {
		next = pointer(response.NextBatch)
	}
	return socialhub.Page[socialhub.Comment]{Items: items, NextCursor: next, HasMore: next != nil}, nil
}

func (client *Client) Comment(ctx context.Context, input socialhub.CreateCommentRequest, options ...socialhub.CallOption) (*socialhub.Comment, error) {
	roomID, rootEventID, err := parseCompositeID("comment", input.PostID, client.defaultRoomID)
	if err != nil || !validText(input.Text) {
		return nil, invalidArgument("comment", "valid Matrix post ID and non-empty bounded text are required")
	}
	replyID := rootEventID
	if input.ParentID != nil {
		parentRoomID, parentEventID, parseErr := parseCompositeID("comment", *input.ParentID, roomID)
		if parseErr != nil || parentRoomID != roomID {
			return nil, invalidArgument("comment", "parent comment must belong to the same Matrix room")
		}
		replyID = parentEventID
	}
	reference, err := client.SendText(ctx, SendTextRequest{
		RoomID: roomID, MessageType: MessageTypeText, Text: input.Text,
		ReplyToID: replyID, ThreadRootID: rootEventID,
	}, options...)
	if err != nil {
		return nil, err
	}
	now := client.clock.Now().UTC()
	event := Event{Type: EventTypeMessage, RoomID: roomID, EventID: reference.EventID, Sender: client.userID, OriginServerTS: now.UnixMilli()}
	event.Content, _ = jsonMarshal(MessageContent{
		MessageType: MessageTypeText, Body: input.Text,
		RelatesTo: &Relation{RelationType: RelationThread, EventID: rootEventID, InReplyTo: &InReplyTo{EventID: replyID}},
	})
	return client.mapComment(roomID, rootEventID, event)
}

func (client *Client) DeleteComment(ctx context.Context, commentID string, options ...socialhub.CallOption) error {
	roomID, eventID, err := parseCompositeID("delete_comment", commentID, client.defaultRoomID)
	if err != nil {
		return err
	}
	_, err = client.Redact(ctx, roomID, eventID, "", options...)
	return err
}
