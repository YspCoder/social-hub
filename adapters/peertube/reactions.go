package peertube

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

func (c *Client) React(ctx context.Context, input socialhub.ReactionRequest, options ...socialhub.CallOption) error {
	return c.rate(ctx, input, "like", options...)
}

func (c *Client) RemoveReaction(ctx context.Context, input socialhub.ReactionRequest, options ...socialhub.CallOption) error {
	return c.rate(ctx, input, "none", options...)
}

func (c *Client) rate(ctx context.Context, input socialhub.ReactionRequest, rating string, options ...socialhub.CallOption) error {
	if !validResourceID(input.TargetID) {
		return invalidArgument("rate", "a valid target video ID is required")
	}
	if input.ActorID != "" && c.accountName != "" && input.ActorID != c.accountName {
		return invalidArgument("rate", "actor must match the configured PeerTube account name")
	}
	if input.Kind != socialhub.ReactionLike {
		return unsupported("rate", "PeerTube common reactions map only LIKE")
	}
	if err := c.requireUser("rate"); err != nil {
		return err
	}
	body := struct {
		Rating string `json:"rating"`
	}{Rating: rating}
	return c.transport.JSON(ctx, http.MethodPut, "/videos/"+url.PathEscape(input.TargetID)+"/rate", nil, body, nil, options...)
}

func (c *Client) Comment(ctx context.Context, input socialhub.CreateCommentRequest, options ...socialhub.CallOption) (*socialhub.Comment, error) {
	if !validResourceID(input.PostID) || strings.TrimSpace(input.Text) == "" || utf8.RuneCountInString(input.Text) > 10000 {
		return nil, invalidArgument("comment", "a valid video ID and comment text of at most 10,000 characters are required")
	}
	if err := c.requireUser("comment"); err != nil {
		return nil, err
	}
	path := "/videos/" + url.PathEscape(input.PostID) + "/comment-threads"
	if input.ParentID != nil {
		if !validResourceID(*input.ParentID) {
			return nil, invalidArgument("comment", "parent comment ID is invalid")
		}
		path = "/videos/" + url.PathEscape(input.PostID) + "/comments/" + url.PathEscape(*input.ParentID)
	}
	body := struct {
		Text string `json:"text"`
	}{Text: input.Text}
	var response commentPostResponse
	if err := c.transport.JSON(ctx, http.MethodPost, path, nil, body, &response, options...); err != nil {
		return nil, err
	}
	comment, err := c.mapComment(input.PostID, response.Comment)
	if err != nil {
		return nil, err
	}
	if input.ParentID != nil && comment.ParentID == nil {
		comment.ParentID = input.ParentID
	}
	return &comment, nil
}

func (c *Client) DeleteComment(context.Context, string, ...socialhub.CallOption) error {
	return unsupported("delete_comment", "PeerTube requires both video ID and comment ID; use CommentWorkflow.DeleteVideoComment")
}

func (c *Client) DeleteVideoComment(ctx context.Context, videoID, commentID string, options ...socialhub.CallOption) error {
	if !validResourceID(videoID) || !validResourceID(commentID) {
		return invalidArgument("delete_video_comment", "valid video and comment IDs are required")
	}
	if err := c.requireUser("delete_video_comment"); err != nil {
		return err
	}
	path := "/videos/" + url.PathEscape(videoID) + "/comments/" + url.PathEscape(commentID)
	return c.transport.JSON(ctx, http.MethodDelete, path, nil, nil, nil, options...)
}

func (c *Client) GetCommentThread(ctx context.Context, videoID, threadID string, options ...socialhub.CallOption) (*VideoCommentThread, error) {
	if !validResourceID(videoID) || !validResourceID(threadID) {
		return nil, invalidArgument("get_comment_thread", "valid video and thread IDs are required")
	}
	path := "/videos/" + url.PathEscape(videoID) + "/comment-threads/" + url.PathEscape(threadID)
	var response VideoCommentThread
	if err := c.transport.JSON(ctx, http.MethodGet, path, nil, nil, &response, options...); err != nil {
		return nil, err
	}
	if response.Comment.ID < 1 {
		return nil, platformError("get_comment_thread", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return &response, nil
}
