package strava

import (
	"context"
	"net/http"
	"net/url"
	"strconv"

	"social-hub/pkg/socialhub"
)

const defaultPageSize = 30
const maxPageSize = 200

func (client *Client) GetUser(ctx context.Context, userID string, options ...socialhub.CallOption) (*socialhub.User, error) {
	if userID != "" && userID != "me" && userID != client.athleteID {
		return nil, invalidArgument("get_user", "Strava API v3 can only fetch the authenticated athlete")
	}
	if err := client.requireScopes("get_user", "read"); err != nil {
		return nil, err
	}
	var response athleteWire
	if err := client.api.JSON(ctx, http.MethodGet, "/athlete", nil, nil, &response, options...); err != nil {
		return nil, err
	}
	if string(response.ID) != client.athleteID {
		return nil, platformError("get_user", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return mapUser(client.accountID, response), nil
}

func (client *Client) GetPost(ctx context.Context, postID string, options ...socialhub.CallOption) (*socialhub.Post, error) {
	if !validResourceID(postID) {
		return nil, invalidArgument("get_post", "activity ID must be a positive decimal Strava ID")
	}
	if err := client.requireScopes("get_post", "activity:read"); err != nil {
		return nil, err
	}
	var response activityWire
	if err := client.api.JSON(ctx, http.MethodGet, "/activities/"+url.PathEscape(postID), nil, nil, &response, options...); err != nil {
		return nil, err
	}
	activity, err := client.validateActivity("get_post", response, postID)
	if err != nil {
		return nil, err
	}
	return mapPost(client.accountID, activity, client.clock.Now()), nil
}

func (client *Client) ListPosts(ctx context.Context, input socialhub.ListPostsRequest, options ...socialhub.CallOption) (socialhub.Page[socialhub.Post], error) {
	if input.UserID != "" && input.UserID != client.athleteID {
		return socialhub.Page[socialhub.Post]{}, invalidArgument("list_posts", "user filter must match the configured Strava athlete")
	}
	if input.StartTime != nil && input.EndTime != nil && !input.StartTime.Before(*input.EndTime) {
		return socialhub.Page[socialhub.Post]{}, invalidArgument("list_posts", "start_time must be before end_time")
	}
	page, pageSize, err := pageValues(input.Cursor, input.MaxResults)
	if err != nil {
		return socialhub.Page[socialhub.Post]{}, err
	}
	if err := client.requireScopes("list_posts", "activity:read"); err != nil {
		return socialhub.Page[socialhub.Post]{}, err
	}
	query := url.Values{"page": {strconv.Itoa(page)}, "per_page": {strconv.Itoa(pageSize)}}
	if input.StartTime != nil {
		query.Set("after", strconv.FormatInt(input.StartTime.Unix(), 10))
	}
	if input.EndTime != nil {
		query.Set("before", strconv.FormatInt(input.EndTime.Unix(), 10))
	}
	var response []activityWire
	if err := client.api.JSON(ctx, http.MethodGet, "/athlete/activities", query, nil, &response, options...); err != nil {
		return socialhub.Page[socialhub.Post]{}, err
	}
	items := make([]socialhub.Post, 0, len(response))
	observedAt := client.clock.Now()
	for _, wire := range response {
		activity, err := client.validateActivity("list_posts", wire, "")
		if err != nil {
			return socialhub.Page[socialhub.Post]{}, err
		}
		items = append(items, *mapPost(client.accountID, activity, observedAt))
	}
	result := socialhub.Page[socialhub.Post]{Items: items}
	if len(response) == pageSize {
		next := strconv.Itoa(page + 1)
		result.NextCursor, result.HasMore = &next, true
	}
	return result, nil
}

func (client *Client) ListComments(ctx context.Context, input socialhub.ListCommentsRequest, options ...socialhub.CallOption) (socialhub.Page[socialhub.Comment], error) {
	if !validResourceID(input.PostID) {
		return socialhub.Page[socialhub.Comment]{}, invalidArgument("list_comments", "activity ID must be a positive decimal Strava ID")
	}
	pageSize, err := validPageSize(input.MaxResults)
	if err != nil {
		return socialhub.Page[socialhub.Comment]{}, err
	}
	if input.Cursor != "" && !validOpaque(input.Cursor, 4096) {
		return socialhub.Page[socialhub.Comment]{}, invalidArgument("list_comments", "after cursor is invalid")
	}
	if err := client.requireScopes("list_comments", "activity:read"); err != nil {
		return socialhub.Page[socialhub.Comment]{}, err
	}
	query := url.Values{"page_size": {strconv.Itoa(pageSize)}}
	if input.Cursor != "" {
		query.Set("after_cursor", input.Cursor)
	}
	var response []commentWire
	path := "/activities/" + url.PathEscape(input.PostID) + "/comments"
	if err := client.api.JSON(ctx, http.MethodGet, path, query, nil, &response, options...); err != nil {
		return socialhub.Page[socialhub.Comment]{}, err
	}
	items := make([]socialhub.Comment, 0, len(response))
	for _, comment := range response {
		if comment.ID == "" || string(comment.ActivityID) != input.PostID || !validText(comment.Text, 100_000, true) {
			return socialhub.Page[socialhub.Comment]{}, platformError("list_comments", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
		}
		items = append(items, mapComment(client.accountID, comment))
	}
	result := socialhub.Page[socialhub.Comment]{Items: items}
	if len(response) == pageSize {
		next := response[len(response)-1].Cursor
		if !validOpaque(next, 4096) {
			return socialhub.Page[socialhub.Comment]{}, platformError("list_comments", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
		}
		result.NextCursor, result.HasMore = &next, true
	}
	return result, nil
}

func (client *Client) validateActivity(operation string, input activityWire, expectedID string) (*Activity, error) {
	activity := typedActivity(input)
	if !validResourceID(activity.ID) || expectedID != "" && activity.ID != expectedID || activity.AthleteID != client.athleteID ||
		!validText(activity.Name, 4096, false) || !validSportType(activity.SportType) {
		return nil, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return activity, nil
}

func pageValues(cursor string, maximum int) (int, int, error) {
	pageSize, err := validPageSize(maximum)
	if err != nil {
		return 0, 0, err
	}
	page := 1
	if cursor != "" {
		parsed, err := strconv.Atoi(cursor)
		if err != nil || parsed <= 0 || parsed == int(^uint(0)>>1) {
			return 0, 0, invalidArgument("list_posts", "cursor must be a positive page number")
		}
		page = parsed
	}
	return page, pageSize, nil
}

func validPageSize(value int) (int, error) {
	if value < 0 || value > maxPageSize {
		return 0, invalidArgument("pagination", "max_results must be between 0 and 200")
	}
	if value == 0 {
		value = defaultPageSize
	}
	return value, nil
}
