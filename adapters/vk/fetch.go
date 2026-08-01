package vk

import (
	"context"
	"net/url"
	"strconv"
	"strings"

	"social-hub/pkg/socialhub"
)

func (c *Client) GetUser(ctx context.Context, userID string, options ...socialhub.CallOption) (*socialhub.User, error) {
	id, err := strconv.ParseInt(strings.TrimSpace(userID), 10, 64)
	if err != nil || id == 0 {
		return nil, invalidArgument("get_user", "VK numeric user or negative community ID is required")
	}
	if id > 0 {
		var response []wireUser
		if err := c.method(ctx, "users.get", url.Values{
			"user_ids": {strconv.FormatInt(id, 10)}, "fields": {"screen_name,domain,photo_200,deactivated,is_closed,can_access_closed"},
		}, &response, options...); err != nil {
			return nil, err
		}
		if len(response) != 1 || response[0].ID != id {
			return nil, platformError("users.get", socialhub.CodeNotFound, socialhub.ClassPermanent, nil)
		}
		return mapUser(c.accountID, response[0]), nil
	}
	var response struct {
		Groups []wireGroup `json:"groups"`
	}
	if err := c.method(ctx, "groups.getById", url.Values{
		"group_id": {strconv.FormatInt(-id, 10)}, "fields": {"photo_200,description,members_count,status"},
	}, &response, options...); err != nil {
		return nil, err
	}
	if len(response.Groups) != 1 || response.Groups[0].ID != -id {
		return nil, platformError("groups.getById", socialhub.CodeNotFound, socialhub.ClassPermanent, nil)
	}
	return mapGroup(c.accountID, response.Groups[0]), nil
}

func (c *Client) GetPost(ctx context.Context, postID string, options ...socialhub.CallOption) (*socialhub.Post, error) {
	ownerID, itemID, err := parseCompositeID(postID, "get_post")
	if err != nil {
		return nil, err
	}
	var response struct {
		Items []wirePost `json:"items"`
	}
	if err := c.method(ctx, "wall.getById", url.Values{"posts": {compositeID(ownerID, itemID)}}, &response, options...); err != nil {
		return nil, err
	}
	if len(response.Items) != 1 || response.Items[0].OwnerID != ownerID || response.Items[0].ID != itemID {
		return nil, platformError("wall.getById", socialhub.CodeNotFound, socialhub.ClassPermanent, nil)
	}
	return mapPost(c.accountID, response.Items[0], c.clock.Now()), nil
}

func (c *Client) ListPosts(ctx context.Context, input socialhub.ListPostsRequest, options ...socialhub.CallOption) (socialhub.Page[socialhub.Post], error) {
	if input.StartTime != nil || input.EndTime != nil {
		return socialhub.Page[socialhub.Post]{}, unsupported("list_posts", "wall.get does not accept common time-range filters")
	}
	ownerID := c.ownerID
	if strings.TrimSpace(input.UserID) != "" {
		parsed, err := strconv.ParseInt(strings.TrimSpace(input.UserID), 10, 64)
		if err != nil || parsed == 0 {
			return socialhub.Page[socialhub.Post]{}, invalidArgument("list_posts", "user_id must be a non-zero numeric VK owner ID")
		}
		ownerID = parsed
	}
	offset, count, err := pageParameters(input.Cursor, input.MaxResults, 20, 100)
	if err != nil {
		return socialhub.Page[socialhub.Post]{}, err
	}
	var response struct {
		Count int        `json:"count"`
		Items []wirePost `json:"items"`
	}
	if err := c.method(ctx, "wall.get", url.Values{
		"owner_id": {strconv.FormatInt(ownerID, 10)}, "offset": {strconv.Itoa(offset)}, "count": {strconv.Itoa(count)}, "filter": {"all"},
	}, &response, options...); err != nil {
		return socialhub.Page[socialhub.Post]{}, err
	}
	items := make([]socialhub.Post, 0, len(response.Items))
	observedAt := c.clock.Now()
	for _, post := range response.Items {
		items = append(items, *mapPost(c.accountID, post, observedAt))
	}
	return paged(items, offset, count, response.Count), nil
}

func (c *Client) ListComments(ctx context.Context, input socialhub.ListCommentsRequest, options ...socialhub.CallOption) (socialhub.Page[socialhub.Comment], error) {
	ownerID, postID, err := parseCompositeID(input.PostID, "list_comments")
	if err != nil {
		return socialhub.Page[socialhub.Comment]{}, err
	}
	offset, count, err := pageParameters(input.Cursor, input.MaxResults, 20, 100)
	if err != nil {
		return socialhub.Page[socialhub.Comment]{}, err
	}
	var response struct {
		Count int           `json:"count"`
		Items []wireComment `json:"items"`
	}
	if err := c.method(ctx, "wall.getComments", url.Values{
		"owner_id": {strconv.FormatInt(ownerID, 10)}, "post_id": {strconv.FormatInt(postID, 10)},
		"offset": {strconv.Itoa(offset)}, "count": {strconv.Itoa(count)}, "need_likes": {"1"}, "extended": {"0"},
	}, &response, options...); err != nil {
		return socialhub.Page[socialhub.Comment]{}, err
	}
	items := make([]socialhub.Comment, 0, len(response.Items))
	observedAt := c.clock.Now()
	for _, comment := range response.Items {
		items = append(items, mapComment(c.accountID, input.PostID, ownerID, comment, observedAt))
	}
	return paged(items, offset, count, response.Count), nil
}

func pageParameters(cursor string, requested, fallback, maximum int) (int, int, error) {
	offset := 0
	if strings.TrimSpace(cursor) != "" {
		parsed, err := strconv.Atoi(cursor)
		if err != nil || parsed < 0 {
			return 0, 0, invalidArgument("pagination", "cursor must be a non-negative numeric offset")
		}
		offset = parsed
	}
	count := requested
	if count <= 0 {
		count = fallback
	}
	if count > maximum {
		count = maximum
	}
	return offset, count, nil
}

func paged[T any](items []T, offset, count, total int) socialhub.Page[T] {
	var next, previous *string
	if offset+len(items) < total {
		value := strconv.Itoa(offset + count)
		next = &value
	}
	if offset > 0 {
		value := strconv.Itoa(max(0, offset-count))
		previous = &value
	}
	return socialhub.Page[T]{Items: items, NextCursor: next, PrevCursor: previous, HasMore: next != nil}
}
