package instagram

import (
	"context"
	"net/http"
	"net/url"
	"strings"

	"social-hub/pkg/socialhub"
)

func (c *Client) React(context.Context, socialhub.ReactionRequest, ...socialhub.CallOption) error {
	return unsupported("react", "Instagram Login API does not expose media like mutation")
}

func (c *Client) RemoveReaction(context.Context, socialhub.ReactionRequest, ...socialhub.CallOption) error {
	return unsupported("remove_reaction", "Instagram Login API does not expose media like mutation")
}

func (c *Client) Comment(ctx context.Context, input socialhub.CreateCommentRequest, options ...socialhub.CallOption) (*socialhub.Comment, error) {
	if input.PostID == "" || input.ParentID == nil || *input.ParentID == "" || strings.TrimSpace(input.Text) == "" {
		return nil, invalidArgument("comment", "media ID, parent comment ID, and reply text are required")
	}
	if err := c.requireScope("comment", "instagram_business_manage_comments"); err != nil {
		return nil, err
	}
	var response idResponse
	if err := c.form(ctx, http.MethodPost, "/"+url.PathEscape(*input.ParentID)+"/replies", url.Values{"message": {input.Text}}, &response, options...); err != nil {
		return nil, err
	}
	if response.ID == "" {
		return nil, wrapError("comment", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return &socialhub.Comment{
		Platform: "instagram", AccountID: c.accountID, ID: response.ID, PostID: input.PostID,
		AuthorID: stringPointer(c.userID), ParentID: input.ParentID, Text: input.Text,
	}, nil
}

func (c *Client) DeleteComment(ctx context.Context, commentID string, options ...socialhub.CallOption) error {
	if commentID == "" {
		return invalidArgument("delete_comment", "comment ID is required")
	}
	if err := c.requireScope("delete_comment", "instagram_business_manage_comments"); err != nil {
		return err
	}
	var response successResponse
	if err := c.form(ctx, http.MethodDelete, "/"+url.PathEscape(commentID), nil, &response, options...); err != nil {
		return err
	}
	if !response.Success {
		return wrapError("delete_comment", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return nil
}
