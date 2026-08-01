package reddit

import (
	"context"
	"net/url"
	"strings"

	"social-hub/pkg/socialhub"
)

func (c *Client) React(ctx context.Context, input socialhub.ReactionRequest, options ...socialhub.CallOption) error {
	return c.vote(ctx, input, "1", options...)
}

func (c *Client) RemoveReaction(ctx context.Context, input socialhub.ReactionRequest, options ...socialhub.CallOption) error {
	return c.vote(ctx, input, "0", options...)
}

func (c *Client) vote(ctx context.Context, input socialhub.ReactionRequest, direction string, options ...socialhub.CallOption) error {
	if input.ActorID != "" && input.ActorID != c.userID {
		return invalidArgument("vote", "actor must be the configured Reddit user fullname")
	}
	if !validThingFullname(input.TargetID) || input.Kind != socialhub.ReactionLike {
		return invalidArgument("vote", "target must be a submission/comment fullname and reaction must be LIKE")
	}
	if err := c.requireScopes("vote", "vote"); err != nil {
		return err
	}
	return c.form(ctx, "/api/vote", url.Values{"id": {input.TargetID}, "dir": {direction}}, nil, options...)
}

func (c *Client) Comment(ctx context.Context, input socialhub.CreateCommentRequest, options ...socialhub.CallOption) (*socialhub.Comment, error) {
	postID := fullname(input.PostID, "t3_")
	if !validFullname(postID, "t3_") || strings.TrimSpace(input.Text) == "" {
		return nil, invalidArgument("comment", "submission fullname and comment text are required")
	}
	parent := postID
	if input.ParentID != nil {
		if !validFullname(*input.ParentID, "t1_") {
			return nil, invalidArgument("comment", "parent comment must be a t1_ fullname")
		}
		parent = *input.ParentID
	}
	if err := c.requireScopes("comment", "submit"); err != nil {
		return nil, err
	}
	var response redditAPIResponse
	if err := c.form(ctx, "/api/comment", url.Values{"api_type": {"json"}, "thing_id": {parent}, "text": {input.Text}, "raw_json": {"1"}}, &response, options...); err != nil {
		return nil, err
	}
	if err := checkAPIResponse("comment", response); err != nil {
		return nil, err
	}
	if len(response.JSON.Data.Things) == 0 || response.JSON.Data.Things[0].Kind != "t1" {
		return nil, platformError("comment", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	comments := mapComments(c.accountID, postID, response.JSON.Data.Things[:1], c.clock.Now())
	if len(comments) != 1 {
		return nil, platformError("comment", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return &comments[0], nil
}

func (c *Client) DeleteComment(ctx context.Context, commentID string, options ...socialhub.CallOption) error {
	if !validFullname(commentID, "t1_") {
		return invalidArgument("delete_comment", "comment t1_ fullname is required")
	}
	if err := c.requireScopes("delete_comment", "edit"); err != nil {
		return err
	}
	return c.form(ctx, "/api/del", url.Values{"id": {commentID}}, nil, options...)
}
