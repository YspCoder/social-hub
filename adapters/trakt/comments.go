package trakt

import (
	"context"
	"net/http"
	"net/url"
	"strconv"

	"social-hub/pkg/socialhub"
)

func (c *Client) ListComments(ctx context.Context, input CommentActivityRequest, options ...socialhub.CallOption) (socialhub.Page[Comment], error) {
	page, err := validatePage(input.Cursor, input.MaxResults)
	if err != nil || !validCommentActivity(input.Activity) || !validCommentType(input.CommentType) || !validCommentMediaType(input.MediaType) {
		if err != nil {
			return socialhub.Page[Comment]{}, err
		}
		return socialhub.Page[Comment]{}, invalidArgument("list_comments", "comment activity filters are invalid")
	}
	query := url.Values{}
	setPage(query, page, input.MaxResults)
	if input.IncludeReplies {
		query.Set("include_replies", "true")
	}
	path := "/comments/" + input.Activity + "/" + input.CommentType + "/" + input.MediaType
	var response []Comment
	metadata, err := c.requestJSON(ctx, http.MethodGet, path, query, nil, &response, options...)
	if err != nil {
		return socialhub.Page[Comment]{}, err
	}
	return pageFromMetadata(response, page, metadata), nil
}

func (c *Client) GetComment(ctx context.Context, id int64, options ...socialhub.CallOption) (*Comment, error) {
	if id <= 0 {
		return nil, invalidArgument("get_comment", "comment ID must be positive")
	}
	var response Comment
	if _, err := c.requestJSON(ctx, http.MethodGet, "/comments/"+strconv.FormatInt(id, 10), nil, nil, &response, options...); err != nil {
		return nil, err
	}
	return &response, nil
}

func (c *Client) ListReplies(ctx context.Context, id int64, input PageRequest, options ...socialhub.CallOption) (socialhub.Page[Comment], error) {
	page, err := validatePage(input.Cursor, input.MaxResults)
	if err != nil || id <= 0 {
		if err != nil {
			return socialhub.Page[Comment]{}, err
		}
		return socialhub.Page[Comment]{}, invalidArgument("list_replies", "comment ID must be positive")
	}
	query := url.Values{}
	setPage(query, page, input.MaxResults)
	var response []Comment
	metadata, err := c.requestJSON(ctx, http.MethodGet, "/comments/"+strconv.FormatInt(id, 10)+"/replies", query, nil, &response, options...)
	if err != nil {
		return socialhub.Page[Comment]{}, err
	}
	return pageFromMetadata(response, page, metadata), nil
}

func (c *Client) PostComment(ctx context.Context, input CreateCommentRequest, options ...socialhub.CallOption) (*Comment, error) {
	if err := c.requireOAuth("post_comment"); err != nil {
		return nil, err
	}
	if !validCommentTarget(input.Target) || !validComment(input.Text) {
		return nil, invalidArgument("post_comment", "comment target or text is invalid")
	}
	body := map[string]any{
		"comment": input.Text, "spoiler": input.Spoiler,
		string(input.Target.Type): map[string]any{"ids": map[string]int64{"trakt": input.Target.TraktID}},
	}
	var response Comment
	if _, err := c.requestJSON(ctx, http.MethodPost, "/comments", nil, body, &response, options...); err != nil {
		return nil, err
	}
	return &response, nil
}

func (c *Client) ReplyComment(ctx context.Context, id int64, text string, spoiler bool, options ...socialhub.CallOption) (*Comment, error) {
	return c.writeComment(ctx, http.MethodPost, "/comments/"+strconv.FormatInt(id, 10)+"/replies", id, text, spoiler, options...)
}

func (c *Client) UpdateComment(ctx context.Context, input EditCommentRequest, options ...socialhub.CallOption) (*Comment, error) {
	return c.writeComment(ctx, http.MethodPut, "/comments/"+strconv.FormatInt(input.ID, 10), input.ID, input.Text, input.Spoiler, options...)
}

func (c *Client) DeleteComment(ctx context.Context, id int64, options ...socialhub.CallOption) error {
	return c.commentAction(ctx, http.MethodDelete, id, "", options...)
}

func (c *Client) LikeComment(ctx context.Context, id int64, options ...socialhub.CallOption) error {
	return c.commentAction(ctx, http.MethodPost, id, "/like", options...)
}

func (c *Client) UnlikeComment(ctx context.Context, id int64, options ...socialhub.CallOption) error {
	return c.commentAction(ctx, http.MethodDelete, id, "/like", options...)
}

func (c *Client) writeComment(ctx context.Context, method, path string, id int64, text string, spoiler bool, options ...socialhub.CallOption) (*Comment, error) {
	if err := c.requireOAuth("write_comment"); err != nil {
		return nil, err
	}
	if id <= 0 || !validComment(text) {
		return nil, invalidArgument("write_comment", "comment ID and text are required")
	}
	var response Comment
	if _, err := c.requestJSON(ctx, method, path, nil, map[string]any{"comment": text, "spoiler": spoiler}, &response, options...); err != nil {
		return nil, err
	}
	return &response, nil
}

func (c *Client) commentAction(ctx context.Context, method string, id int64, suffix string, options ...socialhub.CallOption) error {
	if err := c.requireOAuth("comment_action"); err != nil {
		return err
	}
	if id <= 0 {
		return invalidArgument("comment_action", "comment ID must be positive")
	}
	_, err := c.requestJSON(ctx, method, "/comments/"+strconv.FormatInt(id, 10)+suffix, nil, nil, nil, options...)
	return err
}

func validCommentActivity(value string) bool {
	return value == "recent" || value == "trending" || value == "updates"
}

func validCommentType(value string) bool {
	return value == "all" || value == "comments" || value == "reviews"
}

func validCommentMediaType(value string) bool {
	switch value {
	case "all", "movies", "shows", "seasons", "episodes", "lists":
		return true
	default:
		return false
	}
}

func validCommentTarget(target CommentTarget) bool {
	if target.TraktID <= 0 {
		return false
	}
	switch target.Type {
	case MediaMovie, MediaShow, MediaSeason, MediaEpisode, MediaList:
		return true
	default:
		return false
	}
}
