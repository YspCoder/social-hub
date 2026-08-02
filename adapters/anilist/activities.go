package anilist

import (
	"context"

	"social-hub/pkg/socialhub"
)

const activitiesQuery = `
query Activities($page: Int!, $perPage: Int!, $userId: Int, $mediaId: Int, $typeIn: [ActivityType], $isFollowing: Boolean, $sort: [ActivitySort]) {
  Page(page: $page, perPage: $perPage) {
    pageInfo { currentPage perPage hasNextPage }
    activities(userId: $userId, mediaId: $mediaId, type_in: $typeIn, isFollowing: $isFollowing, sort: $sort) {` + activityFields + `}
  }
}`

const saveTextActivityMutation = `
mutation SaveTextActivity($id: Int, $text: String, $locked: Boolean) {
  SaveTextActivity(id: $id, text: $text, locked: $locked) { __typename ` + textActivityFields + ` }
}`

const deleteActivityMutation = `
mutation DeleteActivity($id: Int!) {
  DeleteActivity(id: $id) { deleted }
}`

const saveActivityReplyMutation = `
mutation SaveActivityReply($activityId: Int!, $text: String!) {
  SaveActivityReply(activityId: $activityId, text: $text) {` + activityReplyFields + `}
}`

const deleteActivityReplyMutation = `
mutation DeleteActivityReply($id: Int!) {
  DeleteActivityReply(id: $id) { deleted }
}`

const toggleLikeMutation = `
mutation ToggleLike($id: Int!, $type: LikeableType!) {
  ToggleLikeV2(id: $id, type: $type) {
    __typename
    ... on TextActivity { id likeCount isLiked }
    ... on ListActivity { id likeCount isLiked }
    ... on MessageActivity { id likeCount isLiked }
    ... on ActivityReply { id likeCount isLiked }
  }
}`

func (c *Client) ListActivities(ctx context.Context, input ListActivitiesRequest, options ...socialhub.CallOption) (socialhub.Page[Activity], error) {
	if input.UserID < 0 || input.MediaID < 0 || (input.UserID > 0 && !validID(input.UserID)) ||
		(input.MediaID > 0 && !validID(input.MediaID)) || !validActivityTypes(input.Types) {
		return socialhub.Page[Activity]{}, invalidArgument("list_activities", "user, media, or activity types are invalid")
	}
	if input.Following {
		if err := c.requireUser("list_activities"); err != nil {
			return socialhub.Page[Activity]{}, err
		}
	}
	page, variables, err := pageVariables(input.Cursor, input.Limit)
	if err != nil {
		return socialhub.Page[Activity]{}, err
	}
	if input.UserID > 0 {
		variables["userId"] = input.UserID
	}
	if input.MediaID > 0 {
		variables["mediaId"] = input.MediaID
	}
	types := input.Types
	if len(types) == 0 {
		types = []ActivityType{ActivityText, ActivityAnimeList, ActivityMangaList}
	}
	variables["typeIn"], variables["sort"] = types, []string{"ID_DESC"}
	if input.Following {
		variables["isFollowing"] = true
	}
	var response struct {
		Page *struct {
			PageInfo   pageInfo   `json:"pageInfo"`
			Activities []Activity `json:"activities"`
		} `json:"Page"`
	}
	if err := c.requestGraphQL(ctx, "list_activities", activitiesQuery, variables, &response, options...); err != nil {
		return socialhub.Page[Activity]{}, err
	}
	if response.Page == nil {
		return socialhub.Page[Activity]{}, platformError("list_activities", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	for _, activity := range response.Page.Activities {
		if !validID(activity.ID) || (activity.Typename != "TextActivity" && activity.Typename != "ListActivity") {
			return socialhub.Page[Activity]{}, platformError("list_activities", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
		}
	}
	return toPage(response.Page.Activities, response.Page.PageInfo, page)
}

func (c *Client) SaveTextActivity(ctx context.Context, input SaveTextActivityRequest, options ...socialhub.CallOption) (*Activity, error) {
	if err := c.requireUser("save_text_activity"); err != nil {
		return nil, err
	}
	if input.ID < 0 || (input.ID > 0 && !validID(input.ID)) || !validText(input.Text) {
		return nil, invalidArgument("save_text_activity", "activity ID or text is invalid")
	}
	variables := map[string]any{"text": input.Text}
	if input.ID > 0 {
		variables["id"] = input.ID
	}
	setOptional(variables, "locked", input.Locked)
	var response struct {
		Activity *Activity `json:"SaveTextActivity"`
	}
	if err := c.requestGraphQL(ctx, "save_text_activity", saveTextActivityMutation, variables, &response, options...); err != nil {
		return nil, err
	}
	if response.Activity == nil || !validID(response.Activity.ID) || response.Activity.Typename != "TextActivity" {
		return nil, platformError("save_text_activity", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return response.Activity, nil
}

func (c *Client) DeleteActivity(ctx context.Context, activityID int64, options ...socialhub.CallOption) error {
	if err := c.requireUser("delete_activity"); err != nil {
		return err
	}
	return c.deleteObject(ctx, "delete_activity", deleteActivityMutation, "DeleteActivity", activityID, options...)
}

func (c *Client) ReplyActivity(ctx context.Context, activityID int64, text string, options ...socialhub.CallOption) (*ActivityReply, error) {
	if err := c.requireUser("reply_activity"); err != nil {
		return nil, err
	}
	if !validID(activityID) || !validText(text) {
		return nil, invalidArgument("reply_activity", "activity ID or text is invalid")
	}
	var response struct {
		Reply *ActivityReply `json:"SaveActivityReply"`
	}
	if err := c.requestGraphQL(ctx, "reply_activity", saveActivityReplyMutation,
		map[string]any{"activityId": activityID, "text": text}, &response, options...); err != nil {
		return nil, err
	}
	if response.Reply == nil || !validID(response.Reply.ID) || response.Reply.ActivityID != activityID {
		return nil, platformError("reply_activity", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return response.Reply, nil
}

func (c *Client) DeleteActivityReply(ctx context.Context, replyID int64, options ...socialhub.CallOption) error {
	if err := c.requireUser("delete_activity_reply"); err != nil {
		return err
	}
	return c.deleteObject(ctx, "delete_activity_reply", deleteActivityReplyMutation, "DeleteActivityReply", replyID, options...)
}

func (c *Client) ToggleLike(ctx context.Context, targetID int64, kind LikeableType, options ...socialhub.CallOption) (*LikeResult, error) {
	if err := c.requireUser("toggle_like"); err != nil {
		return nil, err
	}
	if !validID(targetID) || !validLikeableType(kind) {
		return nil, invalidArgument("toggle_like", "target ID or likeable type is invalid")
	}
	var response struct {
		Result *LikeResult `json:"ToggleLikeV2"`
	}
	if err := c.requestGraphQL(ctx, "toggle_like", toggleLikeMutation,
		map[string]any{"id": targetID, "type": kind}, &response, options...); err != nil {
		return nil, err
	}
	if response.Result == nil || response.Result.ID != targetID || response.Result.Typename == "" {
		return nil, platformError("toggle_like", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return response.Result, nil
}

func (c *Client) deleteObject(ctx context.Context, operation, query, field string, objectID int64, options ...socialhub.CallOption) error {
	if !validID(objectID) {
		return invalidArgument(operation, "object ID must be a positive GraphQL Int")
	}
	var response map[string]*struct {
		Deleted bool `json:"deleted"`
	}
	if err := c.requestGraphQL(ctx, operation, query, map[string]any{"id": objectID}, &response, options...); err != nil {
		return err
	}
	deleted := response[field]
	if deleted == nil || !deleted.Deleted {
		return platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return nil
}
