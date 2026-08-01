package mastodon

import (
	"context"
	"net/http"
	"net/url"
	"strings"

	"social-hub/pkg/socialhub"
)

func (c *Client) Publish(ctx context.Context, input socialhub.CreatePostRequest, options ...socialhub.CallOption) (*socialhub.Post, error) {
	if err := input.Validate(); err != nil {
		return nil, platformError("publish", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	if err := c.requireScopes("publish", "write:statuses"); err != nil {
		return nil, err
	}
	values := url.Values{}
	if input.Text != nil {
		values.Set("status", *input.Text)
	}
	for _, mediaID := range input.MediaIDs {
		if strings.TrimSpace(mediaID) == "" {
			return nil, invalidArgument("publish", "media IDs must not be empty")
		}
		values.Add("media_ids[]", mediaID)
	}
	if input.ReplyToID != nil {
		values.Set("in_reply_to_id", *input.ReplyToID)
	}
	if input.QuotePostID != nil {
		values.Set("quoted_status_id", *input.QuotePostID)
	}
	if input.Visibility != nil {
		if !validVisibility(*input.Visibility) {
			return nil, invalidArgument("publish", "visibility must be public, unlisted, private, or direct")
		}
		values.Set("visibility", *input.Visibility)
	}
	var response mastodonStatus
	if _, err := c.form(ctx, http.MethodPost, "/api/v1/statuses", values, &response, options...); err != nil {
		return nil, err
	}
	if response.ID == "" {
		return nil, platformError("publish", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return mapStatus(c.accountID, response, c.clock.Now()), nil
}

func (c *Client) PublishStatus(ctx context.Context, postID string, options ...socialhub.CallOption) (*socialhub.PublishStatus, error) {
	post, err := c.GetPost(ctx, postID, options...)
	if err != nil {
		return nil, err
	}
	return &socialhub.PublishStatus{ID: post.ID, State: socialhub.PublishStatePublished, UpdatedAt: post.CreatedAt}, nil
}

func (c *Client) DeletePost(ctx context.Context, postID string, options ...socialhub.CallOption) error {
	if strings.TrimSpace(postID) == "" {
		return invalidArgument("delete_post", "status ID is required")
	}
	if err := c.requireScopes("delete_post", "write:statuses"); err != nil {
		return err
	}
	return c.transport.JSON(ctx, http.MethodDelete, "/api/v1/statuses/"+url.PathEscape(postID), nil, nil, nil, options...)
}

func (c *Client) React(ctx context.Context, input socialhub.ReactionRequest, options ...socialhub.CallOption) error {
	return c.setReaction(ctx, input, false, options...)
}

func (c *Client) RemoveReaction(ctx context.Context, input socialhub.ReactionRequest, options ...socialhub.CallOption) error {
	return c.setReaction(ctx, input, true, options...)
}

func (c *Client) setReaction(ctx context.Context, input socialhub.ReactionRequest, remove bool, options ...socialhub.CallOption) error {
	if strings.TrimSpace(input.TargetID) == "" {
		return invalidArgument("react", "target status ID is required")
	}
	if c.userID != "" && input.ActorID != "" && input.ActorID != c.userID {
		return invalidArgument("react", "actor must be the configured Mastodon account")
	}
	action, requiredScope := "", ""
	switch input.Kind {
	case socialhub.ReactionLike:
		action, requiredScope = "favourite", "write:favourites"
	case socialhub.ReactionRepost:
		action, requiredScope = "reblog", "write:statuses"
	default:
		return invalidArgument("react", "reaction must be LIKE or REPOST")
	}
	if remove {
		action = "un" + action
	}
	if err := c.requireScopes("react", requiredScope); err != nil {
		return err
	}
	var response mastodonStatus
	_, err := c.form(ctx, http.MethodPost, "/api/v1/statuses/"+url.PathEscape(input.TargetID)+"/"+action, url.Values{}, &response, options...)
	return err
}

func (c *Client) Comment(ctx context.Context, input socialhub.CreateCommentRequest, options ...socialhub.CallOption) (*socialhub.Comment, error) {
	if strings.TrimSpace(input.PostID) == "" || strings.TrimSpace(input.Text) == "" {
		return nil, invalidArgument("comment", "status ID and text are required")
	}
	target := input.PostID
	if input.ParentID != nil {
		target = *input.ParentID
	}
	text := input.Text
	post, err := c.Publish(ctx, socialhub.CreatePostRequest{Text: &text, ReplyToID: &target}, options...)
	if err != nil {
		return nil, err
	}
	comment := socialhub.Comment{
		Platform: "mastodon", AccountID: c.accountID, ID: post.ID, PostID: input.PostID,
		ParentID: stringPointer(target), Text: input.Text, CreatedAt: post.CreatedAt,
	}
	if post.AuthorID != nil {
		comment.AuthorID = post.AuthorID
	}
	return &comment, nil
}

func (c *Client) DeleteComment(ctx context.Context, commentID string, options ...socialhub.CallOption) error {
	return c.DeletePost(ctx, commentID, options...)
}

func validVisibility(value string) bool {
	switch value {
	case "public", "unlisted", "private", "direct":
		return true
	default:
		return false
	}
}
