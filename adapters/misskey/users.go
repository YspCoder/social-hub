package misskey

import (
	"context"

	"social-hub/pkg/socialhub"
)

func (c *Client) GetUser(ctx context.Context, userID string, options ...socialhub.CallOption) (*socialhub.User, error) {
	endpoint := "users/show"
	input := any(struct {
		UserID string `json:"userId"`
	}{UserID: userID})
	if userID == "" {
		if err := c.requirePermissions("get_user", "read:account"); err != nil {
			return nil, err
		}
		endpoint, input = "i", struct{}{}
	} else if !validID(userID) {
		return nil, invalidArgument("get_user", "user ID is invalid")
	}
	var response misskeyUser
	if err := c.post(ctx, endpoint, input, &response, options...); err != nil {
		return nil, err
	}
	if response.ID == "" || (userID != "" && response.ID != userID) || (c.userID != "" && userID == "" && response.ID != c.userID) {
		return nil, platformError("get_user", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return c.mapUser(response)
}

func (c *Client) GetPost(ctx context.Context, postID string, options ...socialhub.CallOption) (*socialhub.Post, error) {
	if !validID(postID) {
		return nil, invalidArgument("get_post", "note ID is invalid")
	}
	var response misskeyNote
	if err := c.post(ctx, "notes/show", struct {
		NoteID string `json:"noteId"`
	}{NoteID: postID}, &response, options...); err != nil {
		return nil, err
	}
	if response.ID != postID {
		return nil, platformError("get_post", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return c.mapNote(response)
}

func (c *Client) ListPosts(ctx context.Context, input socialhub.ListPostsRequest, options ...socialhub.CallOption) (socialhub.Page[socialhub.Post], error) {
	pagination, err := makePagination("list_posts", input.Cursor, input.MaxResults, input.StartTime, input.EndTime)
	if err != nil {
		return socialhub.Page[socialhub.Post]{}, err
	}
	userID := input.UserID
	if userID == "" {
		userID = c.userID
	}
	if userID == "" {
		user, err := c.GetUser(ctx, "", options...)
		if err != nil {
			return socialhub.Page[socialhub.Post]{}, err
		}
		userID = user.ID
	}
	if !validID(userID) {
		return socialhub.Page[socialhub.Post]{}, invalidArgument("list_posts", "user ID is invalid")
	}
	request := struct {
		paginationRequest
		UserID string `json:"userId"`
	}{paginationRequest: pagination, UserID: userID}
	var response []misskeyNote
	if err := c.post(ctx, "users/notes", request, &response, options...); err != nil {
		return socialhub.Page[socialhub.Post]{}, err
	}
	return c.mapNotePage(response, pagination.Limit)
}

func (c *Client) ListComments(ctx context.Context, input socialhub.ListCommentsRequest, options ...socialhub.CallOption) (socialhub.Page[socialhub.Comment], error) {
	if !validID(input.PostID) {
		return socialhub.Page[socialhub.Comment]{}, invalidArgument("list_comments", "note ID is invalid")
	}
	pagination, err := makePagination("list_comments", input.Cursor, input.MaxResults, nil, nil)
	if err != nil {
		return socialhub.Page[socialhub.Comment]{}, err
	}
	request := struct {
		paginationRequest
		NoteID string `json:"noteId"`
	}{paginationRequest: pagination, NoteID: input.PostID}
	var response []misskeyNote
	if err := c.post(ctx, "notes/replies", request, &response, options...); err != nil {
		return socialhub.Page[socialhub.Comment]{}, err
	}
	items := make([]socialhub.Comment, 0, len(response))
	for _, note := range response {
		comment, err := c.mapComment(input.PostID, note)
		if err != nil {
			return socialhub.Page[socialhub.Comment]{}, err
		}
		items = append(items, comment)
	}
	page := socialhub.Page[socialhub.Comment]{Items: items, HasMore: len(response) == pagination.Limit}
	if page.HasMore && len(response) > 0 {
		cursor := response[len(response)-1].ID
		page.NextCursor = &cursor
	}
	return page, nil
}

func (c *Client) HomeTimeline(ctx context.Context, input TimelineRequest, options ...socialhub.CallOption) (socialhub.Page[socialhub.Post], error) {
	if err := c.requirePermissions("home_timeline", "read:account"); err != nil {
		return socialhub.Page[socialhub.Post]{}, err
	}
	pagination, err := makePagination("home_timeline", input.Cursor, input.MaxResults, input.StartTime, input.EndTime)
	if err != nil {
		return socialhub.Page[socialhub.Post]{}, err
	}
	request := struct {
		paginationRequest
		WithFiles   bool  `json:"withFiles,omitempty"`
		WithRenotes *bool `json:"withRenotes,omitempty"`
	}{paginationRequest: pagination, WithFiles: input.WithFiles, WithRenotes: input.WithRenotes}
	var response []misskeyNote
	if err := c.post(ctx, "notes/timeline", request, &response, options...); err != nil {
		return socialhub.Page[socialhub.Post]{}, err
	}
	return c.mapNotePage(response, pagination.Limit)
}
