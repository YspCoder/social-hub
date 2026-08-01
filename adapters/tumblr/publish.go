package tumblr

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
	if input.Text == nil || strings.TrimSpace(*input.Text) == "" {
		return nil, invalidArgument("publish", "common Tumblr publishing requires non-empty text")
	}
	if len(input.MediaIDs) > 0 {
		return nil, unsupported("publish", "Tumblr uploads media inline with NPFWorkflow rather than reusable media IDs")
	}
	if input.ReplyToID != nil || input.QuotePostID != nil {
		return nil, unsupported("publish", "Tumblr replies and reblogs require platform-specific context; use NPFWorkflow")
	}
	if input.Visibility != nil && *input.Visibility != "public" {
		return nil, unsupported("publish", "common publishing supports public posts; use NPFWorkflow for private, draft, or queued states")
	}
	block, _ := json.Marshal(map[string]string{"type": "text", "text": *input.Text})
	result, err := c.CreateNPF(ctx, "", NPFPostRequest{Content: []json.RawMessage{block}, State: NPFPublished}, options...)
	if err != nil {
		return nil, err
	}
	now := c.clock.Now()
	visibility := "published"
	return &socialhub.Post{
		Platform: "tumblr", AccountID: c.accountID, ID: result.ID, Text: input.Text,
		Visibility: &visibility, CreatedAt: &now,
		Status: &socialhub.PublishStatus{ID: result.ID, State: socialhub.PublishStatePublished, UpdatedAt: &now},
	}, nil
}

func (c *Client) PublishStatus(ctx context.Context, postID string, options ...socialhub.CallOption) (*socialhub.PublishStatus, error) {
	post, err := c.GetNPF(ctx, "", postID, options...)
	if err != nil {
		return nil, err
	}
	state := socialhub.PublishStatePending
	if post.State == NPFPublished || post.State == NPFPrivate {
		state = socialhub.PublishStatePublished
	}
	return &socialhub.PublishStatus{ID: post.ID, State: state, UpdatedAt: post.Timestamp}, nil
}

func (c *Client) DeletePost(ctx context.Context, postID string, options ...socialhub.CallOption) error {
	if !validPostID(postID) {
		return invalidArgument("delete_post", "numeric post ID is required")
	}
	user, err := c.requireUser("delete_post")
	if err != nil {
		return err
	}
	if err := c.requireScopes("delete_post", "write"); err != nil {
		return err
	}
	body := map[string]string{"id": postID}
	return c.request(ctx, user, http.MethodPost, "/blog/"+url.PathEscape(c.blogIdentifier)+"/post/delete", nil, body, nil, options...)
}
