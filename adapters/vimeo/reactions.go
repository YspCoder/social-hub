package vimeo

import (
	"context"
	"net/http"
	"strings"

	"social-hub/pkg/socialhub"
)

func (c *Client) React(ctx context.Context, input socialhub.ReactionRequest, options ...socialhub.CallOption) error {
	return c.like(ctx, input, http.MethodPut, options...)
}

func (c *Client) RemoveReaction(ctx context.Context, input socialhub.ReactionRequest, options ...socialhub.CallOption) error {
	return c.like(ctx, input, http.MethodDelete, options...)
}

func (c *Client) like(ctx context.Context, input socialhub.ReactionRequest, method string, options ...socialhub.CallOption) error {
	if !validResourceID(input.TargetID) {
		return invalidArgument("like", "target video ID is required and must be valid")
	}
	if input.ActorID != "" && c.userID != "" && input.ActorID != c.userID {
		return invalidArgument("like", "actor must match the configured Vimeo user")
	}
	if input.Kind != socialhub.ReactionLike {
		return unsupported("like", "Vimeo common reactions map only LIKE")
	}
	if err := c.requireScopes("like", "interact"); err != nil {
		return err
	}
	return c.requestJSON(ctx, method, "/me/likes/"+escapedID(input.TargetID), nil, nil, nil, options...)
}

func (c *Client) Comment(ctx context.Context, input socialhub.CreateCommentRequest, options ...socialhub.CallOption) (*socialhub.Comment, error) {
	if !validResourceID(input.PostID) || strings.TrimSpace(input.Text) == "" {
		return nil, invalidArgument("comment", "video ID and non-empty comment text are required")
	}
	if err := c.requireScopes("comment", "interact"); err != nil {
		return nil, err
	}
	body := struct {
		Text string `json:"text"`
	}{Text: input.Text}
	path := "/videos/" + escapedID(input.PostID) + "/comments"
	var parentID *string
	if input.ParentID != nil {
		value := *input.ParentID
		if strings.Contains(value, "/") {
			videoID, commentID, err := splitCommentID(value)
			if err != nil || videoID != input.PostID {
				return nil, invalidArgument("comment", "parent comment ID is invalid or belongs to another video")
			}
			value = commentID
		} else if !validResourceID(value) {
			return nil, invalidArgument("comment", "parent comment ID is invalid")
		}
		parentID = &value
		path += "/" + escapedID(value) + "/replies"
	}
	var response vimeoComment
	if err := c.requestJSON(ctx, http.MethodPost, path, nil, body, &response, options...); err != nil {
		return nil, err
	}
	mapped, err := c.mapComment(input.PostID, parentID, response)
	if err != nil {
		return nil, err
	}
	return &mapped, nil
}

func (c *Client) DeleteComment(ctx context.Context, commentID string, options ...socialhub.CallOption) error {
	videoID, rawCommentID, err := splitCommentID(commentID)
	if err != nil {
		return invalidArgument("delete_comment", "comment ID must be the composite ID returned by this adapter")
	}
	if err := c.requireScopes("delete_comment", "delete"); err != nil {
		return err
	}
	path := "/videos/" + escapedID(videoID) + "/comments/" + escapedID(rawCommentID)
	return c.requestJSON(ctx, http.MethodDelete, path, nil, nil, nil, options...)
}
