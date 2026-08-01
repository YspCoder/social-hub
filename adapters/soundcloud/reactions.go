package soundcloud

import (
	"context"
	"net/http"
	"strings"

	"social-hub/pkg/socialhub"
)

func (c *Client) React(ctx context.Context, input socialhub.ReactionRequest, options ...socialhub.CallOption) error {
	if err := c.validateReaction(input); err != nil {
		return err
	}
	var path string
	switch input.Kind {
	case socialhub.ReactionLike:
		path = "/likes/tracks/" + escapedURN(input.TargetID)
	case socialhub.ReactionRepost:
		path = "/reposts/tracks/" + escapedURN(input.TargetID)
	default:
		return unsupported("react", "SoundCloud common reactions support LIKE and REPOST creation")
	}
	return c.requestJSON(ctx, http.MethodPost, path, nil, nil, nil, options...)
}

func (c *Client) RemoveReaction(ctx context.Context, input socialhub.ReactionRequest, options ...socialhub.CallOption) error {
	if err := c.validateReaction(input); err != nil {
		return err
	}
	if input.Kind == socialhub.ReactionRepost {
		return unsupported("remove_reaction", "SoundCloud's public track unrepost endpoint is deprecated")
	}
	if input.Kind != socialhub.ReactionLike {
		return unsupported("remove_reaction", "SoundCloud common reaction removal supports only LIKE")
	}
	return c.requestJSON(ctx, http.MethodDelete, "/likes/tracks/"+escapedURN(input.TargetID), nil, nil, nil, options...)
}

func (c *Client) Comment(ctx context.Context, input socialhub.CreateCommentRequest, options ...socialhub.CallOption) (*socialhub.Comment, error) {
	if !validURN(input.PostID, "tracks") || strings.TrimSpace(input.Text) == "" {
		return nil, invalidArgument("comment", "post ID must be a SoundCloud track URN and comment text must not be empty")
	}
	if input.ParentID != nil {
		return nil, unsupported("comment", "SoundCloud track comments are flat and do not support replies")
	}
	body := struct {
		Comment struct {
			Body string `json:"body"`
		} `json:"comment"`
	}{}
	body.Comment.Body = input.Text
	var response soundCloudComment
	path := "/tracks/" + escapedURN(input.PostID) + "/comments"
	if err := c.requestJSON(ctx, http.MethodPost, path, nil, body, &response, options...); err != nil {
		return nil, err
	}
	mapped, err := c.mapComment(input.PostID, response)
	if err != nil {
		return nil, err
	}
	return &mapped, nil
}

func (c *Client) DeleteComment(context.Context, string, ...socialhub.CallOption) error {
	return unsupported("delete_comment", "SoundCloud Public API does not expose comment deletion")
}

func (c *Client) validateReaction(input socialhub.ReactionRequest) error {
	if !validURN(input.TargetID, "tracks") {
		return invalidArgument("react", "target ID must be a SoundCloud track URN")
	}
	if input.ActorID != "" && c.userURN != "" && input.ActorID != c.userURN {
		return invalidArgument("react", "actor must match the configured SoundCloud user")
	}
	return nil
}
