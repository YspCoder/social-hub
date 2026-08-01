package linkedin

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"social-hub/pkg/socialhub"
)

func (c *Client) ListComments(ctx context.Context, input socialhub.ListCommentsRequest, options ...socialhub.CallOption) (socialhub.Page[socialhub.Comment], error) {
	if !validURN(input.PostID) {
		return socialhub.Page[socialhub.Comment]{}, invalidArgument("list_comments", "post ID must be a LinkedIn URN")
	}
	if err := c.requireScopes("list_comments", c.socialScope(false)); err != nil {
		return socialhub.Page[socialhub.Comment]{}, err
	}
	start, err := parseCursor(input.Cursor)
	if err != nil {
		return socialhub.Page[socialhub.Comment]{}, err
	}
	query := url.Values{"start": {strconv.Itoa(start)}}
	if input.MaxResults > 0 {
		limit := input.MaxResults
		if limit > 100 {
			limit = 100
		}
		query.Set("count", strconv.Itoa(limit))
	}
	var response commentPage
	if err := c.transport.JSON(ctx, http.MethodGet, "/rest/socialActions/"+input.PostID+"/comments", query, nil, &response, options...); err != nil {
		return socialhub.Page[socialhub.Comment]{}, err
	}
	items := make([]socialhub.Comment, 0, len(response.Elements))
	for _, comment := range response.Elements {
		items = append(items, mapComment(c.accountID, input.PostID, comment))
	}
	next := nextCursor(response.Paging, len(items))
	return socialhub.Page[socialhub.Comment]{Items: items, NextCursor: next, HasMore: next != nil}, nil
}

func (c *Client) React(ctx context.Context, input socialhub.ReactionRequest, options ...socialhub.CallOption) error {
	return c.setReaction(ctx, input, false, options...)
}

func (c *Client) RemoveReaction(ctx context.Context, input socialhub.ReactionRequest, options ...socialhub.CallOption) error {
	return c.setReaction(ctx, input, true, options...)
}

func (c *Client) setReaction(ctx context.Context, input socialhub.ReactionRequest, remove bool, options ...socialhub.CallOption) error {
	if input.ActorID != c.authorURN || !validURN(input.TargetID) {
		return invalidArgument("react", "actor must match the configured author URN and target must be a LinkedIn URN")
	}
	if input.Kind != socialhub.ReactionLike {
		return unsupported("react", "the common adapter maps only LinkedIn LIKE reactions")
	}
	if err := c.requireScopes("react", c.socialScope(true)); err != nil {
		return err
	}
	if remove {
		path := "/rest/reactions/(actor:" + input.ActorID + ",entity:" + input.TargetID + ")"
		return c.transport.JSON(ctx, http.MethodDelete, path, nil, nil, nil, options...)
	}
	body := struct {
		Root         string `json:"root"`
		ReactionType string `json:"reactionType"`
	}{Root: input.TargetID, ReactionType: "LIKE"}
	return c.transport.JSON(ctx, http.MethodPost, "/rest/reactions", url.Values{"actor": {input.ActorID}}, body, nil, options...)
}

func (c *Client) Comment(ctx context.Context, input socialhub.CreateCommentRequest, options ...socialhub.CallOption) (*socialhub.Comment, error) {
	if !validURN(input.PostID) || strings.TrimSpace(input.Text) == "" {
		return nil, invalidArgument("comment", "post ID must be a LinkedIn URN and text is required")
	}
	if input.ParentID != nil && !validURN(*input.ParentID) {
		return nil, invalidArgument("comment", "parent comment ID must be a LinkedIn comment URN")
	}
	if err := c.requireScopes("comment", c.socialScope(true)); err != nil {
		return nil, err
	}
	body := struct {
		Actor         string  `json:"actor"`
		Object        string  `json:"object"`
		ParentComment *string `json:"parentComment,omitempty"`
		Message       struct {
			Text string `json:"text"`
		} `json:"message"`
	}{Actor: c.authorURN, Object: input.PostID, ParentComment: input.ParentID}
	body.Message.Text = input.Text
	pathTarget := input.PostID
	if input.ParentID != nil {
		pathTarget = *input.ParentID
	}
	encoded, err := json.Marshal(body)
	if err != nil {
		return nil, platformError("comment", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	request, err := c.transport.NewRequest(ctx, http.MethodPost, "/rest/socialActions/"+pathTarget+"/comments", nil, bytes.NewReader(encoded), options...)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/json")
	var response linkedInComment
	metadata, err := c.transport.DoWithMetadata(request, &response)
	if err != nil {
		return nil, err
	}
	id := string(response.ID)
	if id == "" {
		id = metadata.Header.Get("X-RestLi-Id")
	}
	if id == "" {
		return nil, platformError("comment", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return &socialhub.Comment{
		Platform: "linkedin", AccountID: c.accountID, ID: id, PostID: input.PostID,
		AuthorID: stringPointer(c.authorURN), ParentID: input.ParentID, Text: input.Text,
	}, nil
}

func (c *Client) DeleteComment(context.Context, string, ...socialhub.CallOption) error {
	return unsupported("delete_comment", "LinkedIn deletion requires the root post URN and actor, which the common DeleteComment signature does not carry")
}
