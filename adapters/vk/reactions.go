package vk

import (
	"context"
	"net/url"
	"strconv"
	"strings"

	"social-hub/pkg/socialhub"
)

func (c *Client) React(ctx context.Context, input socialhub.ReactionRequest, options ...socialhub.CallOption) error {
	return c.setReaction(ctx, input, false, options...)
}

func (c *Client) RemoveReaction(ctx context.Context, input socialhub.ReactionRequest, options ...socialhub.CallOption) error {
	return c.setReaction(ctx, input, true, options...)
}

func (c *Client) setReaction(ctx context.Context, input socialhub.ReactionRequest, remove bool, options ...socialhub.CallOption) error {
	if input.Kind != socialhub.ReactionLike {
		return unsupported("react", "VK reposts are independent wall posts; use WallWorkflow.Repost")
	}
	if c.tokenKind != TokenUser {
		return tokenPermission("react", "VK likes require a user access token")
	}
	ownerID, itemID, err := parseCompositeID(input.TargetID, "react")
	if err != nil {
		return err
	}
	method := "likes.add"
	if remove {
		method = "likes.delete"
	}
	var response struct {
		Likes int `json:"likes"`
	}
	return c.method(ctx, method, url.Values{
		"type": {"post"}, "owner_id": {strconv.FormatInt(ownerID, 10)}, "item_id": {strconv.FormatInt(itemID, 10)},
	}, &response, options...)
}

func (c *Client) Comment(ctx context.Context, input socialhub.CreateCommentRequest, options ...socialhub.CallOption) (*socialhub.Comment, error) {
	if c.tokenKind == TokenService {
		return nil, tokenPermission("wall.createComment", "service tokens cannot create wall comments")
	}
	ownerID, postID, err := parseCompositeID(input.PostID, "wall.createComment")
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(input.Text) == "" {
		return nil, invalidArgument("wall.createComment", "comment text is required")
	}
	values := url.Values{
		"owner_id": {strconv.FormatInt(ownerID, 10)}, "post_id": {strconv.FormatInt(postID, 10)}, "message": {input.Text},
	}
	if c.tokenKind == TokenCommunity {
		values.Set("from_group", "1")
	}
	parentID := int64(0)
	if input.ParentID != nil {
		parentOwner, parsed, err := parseCompositeID(*input.ParentID, "wall.createComment")
		if err != nil {
			return nil, err
		}
		if parentOwner != ownerID {
			return nil, invalidArgument("wall.createComment", "parent comment must belong to the same wall owner")
		}
		parentID = parsed
		values.Set("reply_to_comment", strconv.FormatInt(parentID, 10))
	}
	var response struct {
		CommentID int64 `json:"comment_id"`
	}
	if err := c.method(ctx, "wall.createComment", values, &response, options...); err != nil {
		return nil, err
	}
	if response.CommentID <= 0 {
		return nil, platformError("wall.createComment", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	comment := mapComment(c.accountID, input.PostID, ownerID, wireComment{
		ID: response.CommentID, FromID: c.ownerID, Date: c.clock.Now().Unix(), Text: input.Text, ReplyToComment: parentID,
	}, c.clock.Now())
	return &comment, nil
}

func (c *Client) DeleteComment(ctx context.Context, commentID string, options ...socialhub.CallOption) error {
	if c.tokenKind != TokenUser {
		return tokenPermission("wall.deleteComment", "VK comment deletion requires a user access token")
	}
	ownerID, itemID, err := parseCompositeID(commentID, "wall.deleteComment")
	if err != nil {
		return err
	}
	var response int
	if err := c.method(ctx, "wall.deleteComment", url.Values{
		"owner_id": {strconv.FormatInt(ownerID, 10)}, "comment_id": {strconv.FormatInt(itemID, 10)},
	}, &response, options...); err != nil {
		return err
	}
	if response != 1 {
		return platformError("wall.deleteComment", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return nil
}
