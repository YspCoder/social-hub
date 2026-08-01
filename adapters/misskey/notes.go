package misskey

import (
	"context"
	"strings"

	"social-hub/pkg/socialhub"
)

type createNotePoll struct {
	Choices      []string `json:"choices"`
	Multiple     bool     `json:"multiple,omitempty"`
	ExpiresAt    *int64   `json:"expiresAt,omitempty"`
	ExpiredAfter *int64   `json:"expiredAfter,omitempty"`
}

type createNotePayload struct {
	Text               *string             `json:"text,omitempty"`
	FileIDs            []string            `json:"fileIds,omitempty"`
	ReplyID            *string             `json:"replyId,omitempty"`
	RenoteID           *string             `json:"renoteId,omitempty"`
	Visibility         NoteVisibility      `json:"visibility"`
	VisibleUserIDs     []string            `json:"visibleUserIds,omitempty"`
	ContentWarning     *string             `json:"cw,omitempty"`
	LocalOnly          bool                `json:"localOnly,omitempty"`
	ReactionAcceptance *ReactionAcceptance `json:"reactionAcceptance,omitempty"`
	ChannelID          *string             `json:"channelId,omitempty"`
	Poll               *createNotePoll     `json:"poll,omitempty"`
}

func (c *Client) Publish(ctx context.Context, input socialhub.CreatePostRequest, options ...socialhub.CallOption) (*socialhub.Post, error) {
	if err := input.Validate(); err != nil {
		return nil, platformError("publish", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	visibility := VisibilityPublic
	if input.Visibility != nil {
		visibility = NoteVisibility(*input.Visibility)
	}
	return c.CreateNote(ctx, CreateNoteRequest{
		Text: input.Text, FileIDs: input.MediaIDs, ReplyID: input.ReplyToID,
		RenoteID: input.QuotePostID, Visibility: visibility,
	}, options...)
}

func (c *Client) CreateNote(ctx context.Context, input CreateNoteRequest, options ...socialhub.CallOption) (*socialhub.Post, error) {
	if err := c.requirePermissions("create_note", "write:notes"); err != nil {
		return nil, err
	}
	payload, err := c.validateCreateNote(input)
	if err != nil {
		return nil, err
	}
	var response struct {
		CreatedNote misskeyNote `json:"createdNote"`
	}
	if err := c.post(ctx, "notes/create", payload, &response, options...); err != nil {
		return nil, err
	}
	if response.CreatedNote.ID == "" {
		return nil, platformError("create_note", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return c.mapNote(response.CreatedNote)
}

func (c *Client) validateCreateNote(input CreateNoteRequest) (createNotePayload, error) {
	if input.Visibility == "" {
		input.Visibility = VisibilityPublic
	}
	if !validVisibility(input.Visibility) {
		return createNotePayload{}, invalidArgument("create_note", "visibility is invalid")
	}
	if input.Text != nil && (!validContentString(*input.Text, 1<<20) || strings.TrimSpace(*input.Text) == "") {
		return createNotePayload{}, invalidArgument("create_note", "text is invalid")
	}
	if err := validateIDs("create_note", input.FileIDs, 16); err != nil {
		return createNotePayload{}, err
	}
	if err := validateIDs("create_note", input.VisibleUserIDs, 0); err != nil {
		return createNotePayload{}, err
	}
	for _, id := range []*string{input.ReplyID, input.RenoteID, input.ChannelID} {
		if id != nil && !validID(*id) {
			return createNotePayload{}, invalidArgument("create_note", "note or channel ID is invalid")
		}
	}
	if input.ContentWarning != nil && (!validContentString(*input.ContentWarning, 100) || strings.TrimSpace(*input.ContentWarning) == "") {
		return createNotePayload{}, invalidArgument("create_note", "content warning must contain 1 to 100 valid characters")
	}
	if input.ReactionAcceptance != nil && !validReactionAcceptance(*input.ReactionAcceptance) {
		return createNotePayload{}, invalidArgument("create_note", "reaction acceptance is invalid")
	}
	if input.Visibility == VisibilitySpecified && len(input.VisibleUserIDs) == 0 {
		return createNotePayload{}, invalidArgument("create_note", "specified visibility requires visible user IDs")
	}
	if input.Visibility != VisibilitySpecified && len(input.VisibleUserIDs) > 0 {
		return createNotePayload{}, invalidArgument("create_note", "visible user IDs require specified visibility")
	}
	payload := createNotePayload{
		Text: input.Text, FileIDs: append([]string(nil), input.FileIDs...), ReplyID: input.ReplyID,
		RenoteID: input.RenoteID, Visibility: input.Visibility,
		VisibleUserIDs: append([]string(nil), input.VisibleUserIDs...), ContentWarning: input.ContentWarning,
		LocalOnly: input.LocalOnly, ReactionAcceptance: input.ReactionAcceptance, ChannelID: input.ChannelID,
	}
	if input.Poll != nil {
		poll, err := c.validatePoll(*input.Poll)
		if err != nil {
			return createNotePayload{}, err
		}
		payload.Poll = poll
	}
	if payload.Text == nil && len(payload.FileIDs) == 0 && payload.Poll == nil && payload.RenoteID == nil {
		return createNotePayload{}, invalidArgument("create_note", "a note requires text, files, a poll, or a Renote target")
	}
	return payload, nil
}

func (c *Client) validatePoll(input Poll) (*createNotePoll, error) {
	if len(input.Choices) < 2 || len(input.Choices) > 10 {
		return nil, invalidArgument("create_note", "poll requires 2 to 10 choices")
	}
	seen := make(map[string]struct{}, len(input.Choices))
	for _, choice := range input.Choices {
		if !validBoundedString(choice, 50) || strings.TrimSpace(choice) == "" {
			return nil, invalidArgument("create_note", "poll choices must contain 1 to 50 valid characters")
		}
		if _, exists := seen[choice]; exists {
			return nil, invalidArgument("create_note", "poll choices must be unique")
		}
		seen[choice] = struct{}{}
	}
	if input.ExpiresAt != nil && input.ExpireAfter != 0 {
		return nil, invalidArgument("create_note", "poll expiry time and duration cannot be combined")
	}
	poll := &createNotePoll{Choices: append([]string(nil), input.Choices...), Multiple: input.Multiple}
	if input.ExpiresAt != nil {
		if !input.ExpiresAt.After(c.clock.Now()) {
			return nil, invalidArgument("create_note", "poll expiry must be in the future")
		}
		value := input.ExpiresAt.UnixMilli()
		poll.ExpiresAt = &value
	}
	if input.ExpireAfter != 0 {
		if input.ExpireAfter < 0 {
			return nil, invalidArgument("create_note", "poll expiry duration must be positive")
		}
		value := input.ExpireAfter.Milliseconds()
		if value < 1 {
			return nil, invalidArgument("create_note", "poll expiry duration must be at least one millisecond")
		}
		poll.ExpiredAfter = &value
	}
	return poll, nil
}

func (c *Client) PublishStatus(ctx context.Context, postID string, options ...socialhub.CallOption) (*socialhub.PublishStatus, error) {
	post, err := c.GetPost(ctx, postID, options...)
	if err != nil {
		return nil, err
	}
	return post.Status, nil
}

func (c *Client) DeletePost(ctx context.Context, postID string, options ...socialhub.CallOption) error {
	if !validID(postID) {
		return invalidArgument("delete_post", "note ID is invalid")
	}
	if err := c.requirePermissions("delete_post", "write:notes"); err != nil {
		return err
	}
	return c.post(ctx, "notes/delete", struct {
		NoteID string `json:"noteId"`
	}{NoteID: postID}, nil, options...)
}

func (c *Client) React(ctx context.Context, input socialhub.ReactionRequest, options ...socialhub.CallOption) error {
	if input.Kind == socialhub.ReactionRepost {
		return unsupported("react", "common repost cannot preserve the created Renote ID; use CreateNote with RenoteID")
	}
	if input.Kind != socialhub.ReactionLike {
		return invalidArgument("react", "reaction must be LIKE")
	}
	if err := c.validateActor(input.ActorID); err != nil {
		return err
	}
	return c.ReactWithEmoji(ctx, input.TargetID, c.defaultReaction, options...)
}

func (c *Client) ReactWithEmoji(ctx context.Context, noteID, reaction string, options ...socialhub.CallOption) error {
	if !validID(noteID) || !validBoundedString(reaction, 128) {
		return invalidArgument("react", "note ID and reaction are required")
	}
	if err := c.requirePermissions("react", "write:reactions"); err != nil {
		return err
	}
	return c.post(ctx, "notes/reactions/create", struct {
		NoteID   string `json:"noteId"`
		Reaction string `json:"reaction"`
	}{NoteID: noteID, Reaction: reaction}, nil, options...)
}

func (c *Client) RemoveReaction(ctx context.Context, input socialhub.ReactionRequest, options ...socialhub.CallOption) error {
	if input.Kind == socialhub.ReactionRepost {
		return unsupported("remove_reaction", "common repost removal cannot identify the created Renote")
	}
	if input.Kind != socialhub.ReactionLike || !validID(input.TargetID) {
		return invalidArgument("remove_reaction", "a note ID and LIKE reaction are required")
	}
	if err := c.validateActor(input.ActorID); err != nil {
		return err
	}
	if err := c.requirePermissions("remove_reaction", "write:reactions"); err != nil {
		return err
	}
	return c.post(ctx, "notes/reactions/delete", struct {
		NoteID string `json:"noteId"`
	}{NoteID: input.TargetID}, nil, options...)
}

func (c *Client) validateActor(actorID string) error {
	if actorID != "" && c.userID != "" && actorID != c.userID {
		return invalidArgument("react", "actor must be the configured Misskey account")
	}
	return nil
}

func (c *Client) Comment(ctx context.Context, input socialhub.CreateCommentRequest, options ...socialhub.CallOption) (*socialhub.Comment, error) {
	if !validID(input.PostID) || strings.TrimSpace(input.Text) == "" {
		return nil, invalidArgument("comment", "note ID and text are required")
	}
	target := input.PostID
	if input.ParentID != nil {
		if !validID(*input.ParentID) {
			return nil, invalidArgument("comment", "parent note ID is invalid")
		}
		target = *input.ParentID
	}
	text := input.Text
	post, err := c.CreateNote(ctx, CreateNoteRequest{Text: &text, ReplyID: &target, Visibility: VisibilityPublic}, options...)
	if err != nil {
		return nil, err
	}
	return &socialhub.Comment{
		Platform: "misskey", AccountID: c.accountID, ID: post.ID, PostID: input.PostID,
		AuthorID: post.AuthorID, ParentID: &target, Text: input.Text, CreatedAt: post.CreatedAt,
		Metrics: post.Metrics, Extensions: post.Extensions,
	}, nil
}

func (c *Client) DeleteComment(ctx context.Context, commentID string, options ...socialhub.CallOption) error {
	return c.DeletePost(ctx, commentID, options...)
}
