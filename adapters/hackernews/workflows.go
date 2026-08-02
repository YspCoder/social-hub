package hackernews

import (
	"context"
	"errors"
	"net/url"
	"slices"
	"strconv"

	"social-hub/pkg/socialhub"
)

func (c *Client) GetItem(ctx context.Context, id int64, options ...socialhub.CallOption) (*Item, error) {
	return c.getItem(ctx, "get_item", id, options...)
}

func (c *Client) getItem(ctx context.Context, operation string, id int64, options ...socialhub.CallOption) (*Item, error) {
	if id <= 0 {
		return nil, invalidArgument(operation, "item ID must be positive")
	}
	var item *Item
	path := "/item/" + strconv.FormatInt(id, 10) + ".json"
	if err := c.getJSON(ctx, operation, path, &item, options...); err != nil {
		return nil, err
	}
	if item == nil {
		return nil, notFound(operation)
	}
	if item.ID != id || !validItemType(item.Type) || item.Time < 0 || item.Parent < 0 || item.Poll < 0 ||
		!validIDs(item.Kids, maxSubmittedIDs) || !validIDs(item.Parts, maxFeedIDs) {
		return nil, invalidPlatformResponse(operation, "item response fields are invalid")
	}
	copy := *item
	copy.Kids, copy.Parts = slices.Clone(item.Kids), slices.Clone(item.Parts)
	return &copy, nil
}

func (c *Client) ListChildren(ctx context.Context, request ChildrenRequest, options ...socialhub.CallOption) (socialhub.Page[Item], error) {
	if request.ParentID <= 0 {
		return socialhub.Page[Item]{}, invalidArgument("list_children", "parent ID must be positive")
	}
	size, err := pageSize(request.MaxResults)
	if err != nil {
		return socialhub.Page[Item]{}, operationError(err, "list_children")
	}
	parent, err := c.getItem(ctx, "list_children", request.ParentID, options...)
	if err != nil {
		return socialhub.Page[Item]{}, err
	}
	return c.listChildrenFromParent(ctx, parent, request.Cursor, size, "list_children", options...)
}

func (c *Client) MaxItemID(ctx context.Context, options ...socialhub.CallOption) (int64, error) {
	var id int64
	if err := c.getJSON(ctx, "max_item_id", "/maxitem.json", &id, options...); err != nil {
		return 0, err
	}
	if id <= 0 {
		return 0, invalidPlatformResponse("max_item_id", "maximum item ID is invalid")
	}
	return id, nil
}

func (c *Client) ListFeed(ctx context.Context, request FeedRequest, options ...socialhub.CallOption) (socialhub.Page[Item], error) {
	if !validFeed(request.Feed) {
		return socialhub.Page[Item]{}, invalidArgument("list_feed", "feed is invalid")
	}
	size, err := pageSize(request.MaxResults)
	if err != nil {
		return socialhub.Page[Item]{}, operationError(err, "list_feed")
	}
	var ids []int64
	if err := c.getJSON(ctx, "list_feed", "/"+string(request.Feed)+".json", &ids, options...); err != nil {
		return socialhub.Page[Item]{}, err
	}
	if !validIDs(ids, maxFeedIDs) {
		return socialhub.Page[Item]{}, invalidPlatformResponse("list_feed", "feed ID list is invalid")
	}
	offset, err := pageOffset(request.Cursor, len(ids))
	if err != nil {
		return socialhub.Page[Item]{}, operationError(err, "list_feed")
	}
	end := min(offset+size, len(ids))
	items := make([]Item, 0, end-offset)
	for _, id := range ids[offset:end] {
		item, err := c.getItem(ctx, "list_feed", id, options...)
		if err != nil {
			return socialhub.Page[Item]{}, err
		}
		if !postItem(item.Type) {
			return socialhub.Page[Item]{}, invalidPlatformResponse("list_feed", "feed contained a non-post item")
		}
		items = append(items, *item)
	}
	next, more := nextPageCursor(offset, end-offset, len(ids))
	return socialhub.Page[Item]{Items: items, NextCursor: next, HasMore: more}, nil
}

func (c *Client) GetUserProfile(ctx context.Context, username string, options ...socialhub.CallOption) (*User, error) {
	if !validUsername(username) {
		return nil, invalidArgument("get_user_profile", "username is invalid")
	}
	var user *User
	path := "/user/" + url.PathEscape(username) + ".json"
	if err := c.getJSON(ctx, "get_user_profile", path, &user, options...); err != nil {
		return nil, err
	}
	if user == nil {
		return nil, notFound("get_user_profile")
	}
	if user.ID != username || user.Created < 0 || user.Karma < 0 || !validIDs(user.Submitted, maxSubmittedIDs) {
		return nil, invalidPlatformResponse("get_user_profile", "user response fields are invalid")
	}
	copy := *user
	copy.Submitted = slices.Clone(user.Submitted)
	return &copy, nil
}

func (c *Client) GetUpdates(ctx context.Context, options ...socialhub.CallOption) (*Updates, error) {
	var updates *Updates
	if err := c.getJSON(ctx, "get_updates", "/updates.json", &updates, options...); err != nil {
		return nil, err
	}
	if updates == nil || !validIDs(updates.Items, maxUpdateItems) || len(updates.Profiles) > maxUpdateItems {
		return nil, invalidPlatformResponse("get_updates", "updates response is invalid")
	}
	for _, profile := range updates.Profiles {
		if !validUsername(profile) {
			return nil, invalidPlatformResponse("get_updates", "updates profile is invalid")
		}
	}
	return &Updates{Items: slices.Clone(updates.Items), Profiles: slices.Clone(updates.Profiles)}, nil
}

func (c *Client) GetUser(ctx context.Context, username string, options ...socialhub.CallOption) (*socialhub.User, error) {
	user, err := c.GetUserProfile(ctx, username, options...)
	if err != nil {
		return nil, err
	}
	return c.mapUser(user), nil
}

func (c *Client) GetPost(ctx context.Context, postID string, options ...socialhub.CallOption) (*socialhub.Post, error) {
	id, err := parseItemID(postID, "get_post")
	if err != nil {
		return nil, err
	}
	item, err := c.getItem(ctx, "get_post", id, options...)
	if err != nil {
		return nil, err
	}
	if !postItem(item.Type) {
		return nil, invalidArgument("get_post", "item is not a story, job, or poll")
	}
	return c.mapPost(item), nil
}

func (c *Client) ListPosts(ctx context.Context, request socialhub.ListPostsRequest, options ...socialhub.CallOption) (socialhub.Page[socialhub.Post], error) {
	if request.StartTime != nil || request.EndTime != nil {
		return socialhub.Page[socialhub.Post]{}, unsupported("list_posts", "API v0 feeds do not support server-side time filtering")
	}
	if request.UserID != "" {
		return c.listUserPosts(ctx, request, options...)
	}
	page, err := c.ListFeed(ctx, FeedRequest{Feed: c.defaultFeed, Cursor: request.Cursor, MaxResults: request.MaxResults}, options...)
	if err != nil {
		return socialhub.Page[socialhub.Post]{}, operationError(err, "list_posts")
	}
	posts := make([]socialhub.Post, 0, len(page.Items))
	for index := range page.Items {
		posts = append(posts, *c.mapPost(&page.Items[index]))
	}
	return socialhub.Page[socialhub.Post]{Items: posts, NextCursor: page.NextCursor, HasMore: page.HasMore}, nil
}

func (c *Client) listUserPosts(ctx context.Context, request socialhub.ListPostsRequest, options ...socialhub.CallOption) (socialhub.Page[socialhub.Post], error) {
	profile, err := c.GetUserProfile(ctx, request.UserID, options...)
	if err != nil {
		return socialhub.Page[socialhub.Post]{}, operationError(err, "list_posts")
	}
	size, err := pageSize(request.MaxResults)
	if err != nil {
		return socialhub.Page[socialhub.Post]{}, operationError(err, "list_posts")
	}
	offset, err := pageOffset(request.Cursor, len(profile.Submitted))
	if err != nil {
		return socialhub.Page[socialhub.Post]{}, operationError(err, "list_posts")
	}
	posts := make([]socialhub.Post, 0, size)
	scanned := 0
	for offset+scanned < len(profile.Submitted) && scanned < maxUserScanPerPage && len(posts) < size {
		id := profile.Submitted[offset+scanned]
		scanned++
		item, err := c.getItem(ctx, "list_posts", id, options...)
		if errors.Is(err, socialhub.ErrNotFound) {
			continue
		}
		if err != nil {
			return socialhub.Page[socialhub.Post]{}, err
		}
		if postItem(item.Type) {
			posts = append(posts, *c.mapPost(item))
		}
	}
	next, more := nextPageCursor(offset, scanned, len(profile.Submitted))
	return socialhub.Page[socialhub.Post]{Items: posts, NextCursor: next, HasMore: more}, nil
}

func (c *Client) ListComments(ctx context.Context, request socialhub.ListCommentsRequest, options ...socialhub.CallOption) (socialhub.Page[socialhub.Comment], error) {
	postID, err := parseItemID(request.PostID, "list_comments")
	if err != nil {
		return socialhub.Page[socialhub.Comment]{}, err
	}
	parent, err := c.getItem(ctx, "list_comments", postID, options...)
	if err != nil {
		return socialhub.Page[socialhub.Comment]{}, err
	}
	if !postItem(parent.Type) {
		return socialhub.Page[socialhub.Comment]{}, invalidArgument("list_comments", "item is not a story, job, or poll")
	}
	size, err := pageSize(request.MaxResults)
	if err != nil {
		return socialhub.Page[socialhub.Comment]{}, operationError(err, "list_comments")
	}
	page, err := c.listChildrenFromParent(ctx, parent, request.Cursor, size, "list_comments", options...)
	if err != nil {
		return socialhub.Page[socialhub.Comment]{}, err
	}
	comments := make([]socialhub.Comment, 0, len(page.Items))
	for index := range page.Items {
		if page.Items[index].Type != ItemComment {
			return socialhub.Page[socialhub.Comment]{}, invalidPlatformResponse("list_comments", "post child was not a comment")
		}
		comments = append(comments, *c.mapComment(&page.Items[index], request.PostID))
	}
	return socialhub.Page[socialhub.Comment]{Items: comments, NextCursor: page.NextCursor, HasMore: page.HasMore}, nil
}

func (c *Client) listChildrenFromParent(ctx context.Context, parent *Item, cursor string, size int, operation string, options ...socialhub.CallOption) (socialhub.Page[Item], error) {
	offset, err := pageOffset(cursor, len(parent.Kids))
	if err != nil {
		return socialhub.Page[Item]{}, operationError(err, operation)
	}
	end := min(offset+size, len(parent.Kids))
	items := make([]Item, 0, end-offset)
	for _, id := range parent.Kids[offset:end] {
		item, err := c.getItem(ctx, operation, id, options...)
		if err != nil {
			return socialhub.Page[Item]{}, err
		}
		items = append(items, *item)
	}
	next, more := nextPageCursor(offset, end-offset, len(parent.Kids))
	return socialhub.Page[Item]{Items: items, NextCursor: next, HasMore: more}, nil
}

func operationError(err error, operation string) error {
	if platformErr, ok := err.(*socialhub.Error); ok {
		platformErr.Op = operation
	}
	return err
}
