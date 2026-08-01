package flickr

import (
	"context"
	"net/http"
	"net/url"

	"social-hub/pkg/socialhub"
)

func (c *Client) React(ctx context.Context, input socialhub.ReactionRequest, options ...socialhub.CallOption) error {
	if err := c.requirePermission("favorite_add", PermissionWrite); err != nil {
		return err
	}
	if input.Kind != socialhub.ReactionLike || !validResourceID(input.TargetID) || input.ActorID != "" && input.ActorID != c.userID {
		return invalidArgument("favorite_add", "Flickr reactions support favorites by the configured member")
	}
	return c.call(ctx, http.MethodPost, "flickr.favorites.add", url.Values{"photo_id": {input.TargetID}}, true, nil, options...)
}

func (c *Client) RemoveReaction(ctx context.Context, input socialhub.ReactionRequest, options ...socialhub.CallOption) error {
	if err := c.requirePermission("favorite_remove", PermissionWrite); err != nil {
		return err
	}
	if input.Kind != socialhub.ReactionLike || !validResourceID(input.TargetID) || input.ActorID != "" && input.ActorID != c.userID {
		return invalidArgument("favorite_remove", "Flickr reactions support favorites by the configured member")
	}
	return c.call(ctx, http.MethodPost, "flickr.favorites.remove", url.Values{"photo_id": {input.TargetID}}, true, nil, options...)
}

func (c *Client) ListComments(ctx context.Context, input socialhub.ListCommentsRequest, options ...socialhub.CallOption) (socialhub.Page[socialhub.Comment], error) {
	if !validResourceID(input.PostID) || input.Cursor != "" || input.MaxResults < 0 {
		return socialhub.Page[socialhub.Comment]{}, invalidArgument("list_comments", "valid photo ID, empty cursor, and non-negative max results are required")
	}
	var response commentsResponse
	if err := c.call(ctx, http.MethodGet, "flickr.photos.comments.getList", url.Values{"photo_id": {input.PostID}}, c.signed != nil, &response, options...); err != nil {
		return socialhub.Page[socialhub.Comment]{}, err
	}
	if response.Comments.PhotoID != "" && response.Comments.PhotoID != input.PostID {
		return socialhub.Page[socialhub.Comment]{}, platformError("list_comments", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	maximum := len(response.Comments.Items)
	if input.MaxResults > 0 && input.MaxResults < maximum {
		maximum = input.MaxResults
	}
	items := make([]socialhub.Comment, 0, maximum)
	for _, item := range response.Comments.Items[:maximum] {
		comment, err := c.mapComment(input.PostID, item)
		if err != nil {
			return socialhub.Page[socialhub.Comment]{}, err
		}
		items = append(items, *comment)
	}
	return socialhub.Page[socialhub.Comment]{Items: items}, nil
}

func (c *Client) Comment(ctx context.Context, input socialhub.CreateCommentRequest, options ...socialhub.CallOption) (*socialhub.Comment, error) {
	if err := c.requirePermission("comment_add", PermissionWrite); err != nil {
		return nil, err
	}
	if !validResourceID(input.PostID) || input.ParentID != nil || !validText(input.Text, 65_536) {
		return nil, invalidArgument("comment_add", "Flickr supports a non-empty flat comment on a valid photo")
	}
	var response commentResponse
	if err := c.call(ctx, http.MethodPost, "flickr.photos.comments.addComment", url.Values{"photo_id": {input.PostID}, "comment_text": {input.Text}}, true, &response, options...); err != nil {
		return nil, err
	}
	if !validResourceID(response.Comment.ID) {
		return nil, platformError("comment_add", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	now := c.clock.Now().UTC()
	return &socialhub.Comment{Platform: "flickr", AccountID: c.accountID, ID: response.Comment.ID, PostID: input.PostID, AuthorID: pointer(c.userID), Text: input.Text, CreatedAt: &now}, nil
}

func (c *Client) DeleteComment(ctx context.Context, commentID string, options ...socialhub.CallOption) error {
	if err := c.requirePermission("comment_delete", PermissionWrite); err != nil {
		return err
	}
	if !validResourceID(commentID) {
		return invalidArgument("comment_delete", "a valid comment ID is required")
	}
	return c.call(ctx, http.MethodPost, "flickr.photos.comments.deleteComment", url.Values{"comment_id": {commentID}}, true, nil, options...)
}
