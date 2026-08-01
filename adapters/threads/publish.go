package threads

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"

	"social-hub/pkg/socialhub"
)

func (c *Client) Publish(ctx context.Context, input socialhub.CreatePostRequest, options ...socialhub.CallOption) (*socialhub.Post, error) {
	if err := input.Validate(); err != nil {
		return nil, platformError("publish", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	if len(input.MediaIDs) > 0 {
		return nil, unsupported("publish", "Threads media uses public HTTPS URLs through ContainerWorkflow")
	}
	if input.Text == nil || strings.TrimSpace(*input.Text) == "" {
		return nil, invalidArgument("publish", "common Threads publishing requires text")
	}
	if input.Visibility != nil && *input.Visibility != "public" {
		return nil, invalidArgument("publish", "Threads API posts are public")
	}
	if err := c.requireScope("publish", "threads_content_publish"); err != nil {
		return nil, err
	}
	form := url.Values{
		"media_type": {string(ContainerText)}, "text": {*input.Text}, "auto_publish_text": {"true"},
	}
	if input.ReplyToID != nil {
		if strings.TrimSpace(*input.ReplyToID) == "" {
			return nil, invalidArgument("publish", "reply target must not be empty")
		}
		form.Set("reply_to_id", *input.ReplyToID)
	}
	if input.QuotePostID != nil {
		if strings.TrimSpace(*input.QuotePostID) == "" {
			return nil, invalidArgument("publish", "quote target must not be empty")
		}
		form.Set("quote_post_id", *input.QuotePostID)
	}
	post, err := c.publishText(ctx, form, *input.Text, options...)
	if err != nil {
		return nil, err
	}
	if input.ReplyToID != nil {
		post.Relations = append(post.Relations, socialhub.PostRelation{Type: socialhub.RelationReply, PostID: *input.ReplyToID})
	}
	if input.QuotePostID != nil {
		post.Relations = append(post.Relations, socialhub.PostRelation{Type: socialhub.RelationQuote, PostID: *input.QuotePostID})
	}
	return post, nil
}

func (c *Client) publishText(ctx context.Context, form url.Values, text string, options ...socialhub.CallOption) (*socialhub.Post, error) {
	var response idResponse
	if err := c.form(ctx, http.MethodPost, "/me/threads", form, &response, options...); err != nil {
		return nil, err
	}
	if response.ID == "" {
		return nil, platformError("publish", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	now := c.clock.Now()
	return &socialhub.Post{
		Platform: "threads", AccountID: c.accountID, ID: response.ID, AuthorID: stringPointer(c.userID),
		Text: stringPointer(text), CreatedAt: &now, Visibility: stringPointer("public"),
		Status: &socialhub.PublishStatus{ID: response.ID, State: socialhub.PublishStatePublished, UpdatedAt: &now},
	}, nil
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
		return invalidArgument("delete_post", "post ID is required")
	}
	if err := c.requireScope("delete_post", "threads_delete"); err != nil {
		return err
	}
	var response successResponse
	if err := c.form(ctx, http.MethodDelete, "/"+url.PathEscape(postID), nil, &response, options...); err != nil {
		return err
	}
	if !response.Success {
		return platformError("delete_post", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return nil
}

func (c *Client) React(context.Context, socialhub.ReactionRequest, ...socialhub.CallOption) error {
	return unsupported("react", "Threads API does not expose like mutation; use RepostWorkflow for repost creation")
}

func (c *Client) RemoveReaction(context.Context, socialhub.ReactionRequest, ...socialhub.CallOption) error {
	return unsupported("remove_reaction", "delete the ID returned by RepostWorkflow through DeletePost")
}

func (c *Client) Comment(ctx context.Context, input socialhub.CreateCommentRequest, options ...socialhub.CallOption) (*socialhub.Comment, error) {
	if strings.TrimSpace(input.PostID) == "" || strings.TrimSpace(input.Text) == "" {
		return nil, invalidArgument("comment", "post ID and reply text are required")
	}
	target := input.PostID
	if input.ParentID != nil {
		if strings.TrimSpace(*input.ParentID) == "" {
			return nil, invalidArgument("comment", "parent reply ID must not be empty")
		}
		target = *input.ParentID
	}
	if err := c.requireScope("comment", "threads_content_publish"); err != nil {
		return nil, err
	}
	post, err := c.publishText(ctx, url.Values{
		"media_type": {string(ContainerText)}, "text": {input.Text}, "reply_to_id": {target}, "auto_publish_text": {"true"},
	}, input.Text, options...)
	if err != nil {
		return nil, err
	}
	extension, _ := json.Marshal(map[string]string{"reply_to_id": target})
	return &socialhub.Comment{
		Platform: "threads", AccountID: c.accountID, ID: post.ID, PostID: input.PostID,
		AuthorID: post.AuthorID, ParentID: stringPointer(target), Text: input.Text, CreatedAt: post.CreatedAt,
		Extensions: map[string]json.RawMessage{"threads.reply": extension},
	}, nil
}

func (c *Client) DeleteComment(ctx context.Context, commentID string, options ...socialhub.CallOption) error {
	return c.DeletePost(ctx, commentID, options...)
}
