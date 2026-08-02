package anilist

import (
	"context"

	"social-hub/pkg/socialhub"
)

const viewerQuery = `
query Viewer {
  Viewer {` + userFields + `}
}`

const userQuery = `
query User($id: Int, $name: String) {
  User(id: $id, name: $name) {` + userFields + `}
}`

const mediaListQuery = `
query MediaList($page: Int!, $perPage: Int!, $userId: Int, $userName: String, $type: MediaType!, $status: MediaListStatus, $sort: [MediaListSort]) {
  Page(page: $page, perPage: $perPage) {
    pageInfo { currentPage perPage hasNextPage }
    mediaList(userId: $userId, userName: $userName, type: $type, status: $status, sort: $sort) {` + mediaListFields + `}
  }
}`

const saveMediaListMutation = `
mutation SaveMediaListEntry(
  $id: Int, $mediaId: Int, $status: MediaListStatus, $score: Float, $progress: Int,
  $progressVolumes: Int, $repeat: Int, $priority: Int, $private: Boolean, $notes: String,
  $hiddenFromStatusLists: Boolean, $customLists: [String], $startedAt: FuzzyDateInput, $completedAt: FuzzyDateInput
) {
  SaveMediaListEntry(
    id: $id, mediaId: $mediaId, status: $status, score: $score, progress: $progress,
    progressVolumes: $progressVolumes, repeat: $repeat, priority: $priority, private: $private,
    notes: $notes, hiddenFromStatusLists: $hiddenFromStatusLists, customLists: $customLists,
    startedAt: $startedAt, completedAt: $completedAt
  ) {` + mediaListFields + `}
}`

const deleteMediaListMutation = `
mutation DeleteMediaListEntry($id: Int!) {
  DeleteMediaListEntry(id: $id) { deleted }
}`

func (c *Client) GetViewer(ctx context.Context, options ...socialhub.CallOption) (*User, error) {
	if err := c.requireUser("get_viewer"); err != nil {
		return nil, err
	}
	var response struct {
		Viewer *User `json:"Viewer"`
	}
	if err := c.requestGraphQL(ctx, "get_viewer", viewerQuery, nil, &response, options...); err != nil {
		return nil, err
	}
	if response.Viewer == nil {
		return nil, platformError("get_viewer", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return response.Viewer, nil
}

func (c *Client) GetUser(ctx context.Context, input UserLookup, options ...socialhub.CallOption) (*User, error) {
	if (input.ID == 0) == (input.Name == "") || input.ID < 0 || (input.ID > 0 && !validID(input.ID)) || (input.Name != "" && !validUsername(input.Name)) {
		return nil, invalidArgument("get_user", "exactly one valid user ID or name is required")
	}
	variables := map[string]any{}
	if input.ID > 0 {
		variables["id"] = input.ID
	} else {
		variables["name"] = input.Name
	}
	var response struct {
		User *User `json:"User"`
	}
	if err := c.requestGraphQL(ctx, "get_user", userQuery, variables, &response, options...); err != nil {
		return nil, err
	}
	if response.User == nil {
		return nil, platformError("get_user", socialhub.CodeNotFound, socialhub.ClassPermanent, nil)
	}
	return response.User, nil
}

func (c *Client) ListMediaList(ctx context.Context, input ListMediaListRequest, options ...socialhub.CallOption) (socialhub.Page[MediaListEntry], error) {
	if (input.UserID == 0) == (input.Username == "") || input.UserID < 0 ||
		(input.UserID > 0 && !validID(input.UserID)) || (input.Username != "" && !validUsername(input.Username)) ||
		!validMediaType(input.Type) || !validMediaListStatus(input.Status) || !validMediaListSort(input.Sort) {
		return socialhub.Page[MediaListEntry]{}, invalidArgument("media_list", "user, media type, status, or sort is invalid")
	}
	page, variables, err := pageVariables(input.Cursor, input.Limit)
	if err != nil {
		return socialhub.Page[MediaListEntry]{}, err
	}
	if input.UserID > 0 {
		variables["userId"] = input.UserID
	} else {
		variables["userName"] = input.Username
	}
	variables["type"] = input.Type
	if input.Status != "" {
		variables["status"] = input.Status
	}
	sort := input.Sort
	if sort == "" {
		sort = MediaListSortUpdatedDesc
	}
	variables["sort"] = []MediaListSort{sort}
	var response struct {
		Page *struct {
			PageInfo pageInfo         `json:"pageInfo"`
			Entries  []MediaListEntry `json:"mediaList"`
		} `json:"Page"`
	}
	if err := c.requestGraphQL(ctx, "media_list", mediaListQuery, variables, &response, options...); err != nil {
		return socialhub.Page[MediaListEntry]{}, err
	}
	if response.Page == nil {
		return socialhub.Page[MediaListEntry]{}, platformError("media_list", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return toPage(response.Page.Entries, response.Page.PageInfo, page)
}

func (c *Client) SaveMediaListEntry(ctx context.Context, input SaveMediaListEntryRequest, options ...socialhub.CallOption) (*MediaListEntry, error) {
	if err := c.requireUser("save_media_list"); err != nil {
		return nil, err
	}
	if (input.ID == 0) == (input.MediaID == 0) || input.ID < 0 || input.MediaID < 0 ||
		(input.ID > 0 && !validID(input.ID)) || (input.MediaID > 0 && !validID(input.MediaID)) ||
		(input.Status != nil && !validConcreteMediaListStatus(*input.Status)) || !validScore(input.Score) ||
		!validCount(input.Progress) || !validCount(input.ProgressVolumes) || !validCount(input.Repeat) ||
		!validPriority(input.Priority) || !validOptionalText(input.Notes) || !validCustomLists(input.CustomLists) ||
		(input.StartedAt != nil && !validFuzzyDate(*input.StartedAt)) ||
		(input.CompletedAt != nil && !validFuzzyDate(*input.CompletedAt)) {
		return nil, invalidArgument("save_media_list", "media list entry is invalid")
	}
	if input.ID > 0 && !hasMediaListUpdate(input) {
		return nil, invalidArgument("save_media_list", "an existing entry requires at least one update field")
	}
	variables := map[string]any{}
	if input.ID > 0 {
		variables["id"] = input.ID
	} else {
		variables["mediaId"] = input.MediaID
	}
	setOptional(variables, "status", input.Status)
	setOptional(variables, "score", input.Score)
	setOptional(variables, "progress", input.Progress)
	setOptional(variables, "progressVolumes", input.ProgressVolumes)
	setOptional(variables, "repeat", input.Repeat)
	setOptional(variables, "priority", input.Priority)
	setOptional(variables, "private", input.Private)
	setOptional(variables, "notes", input.Notes)
	setOptional(variables, "hiddenFromStatusLists", input.HiddenFromStatusLists)
	setOptional(variables, "startedAt", input.StartedAt)
	setOptional(variables, "completedAt", input.CompletedAt)
	if input.CustomLists != nil {
		variables["customLists"] = input.CustomLists
	}
	var response struct {
		Entry *MediaListEntry `json:"SaveMediaListEntry"`
	}
	if err := c.requestGraphQL(ctx, "save_media_list", saveMediaListMutation, variables, &response, options...); err != nil {
		return nil, err
	}
	if response.Entry == nil {
		return nil, platformError("save_media_list", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return response.Entry, nil
}

func (c *Client) DeleteMediaListEntry(ctx context.Context, entryID int64, options ...socialhub.CallOption) error {
	if err := c.requireUser("delete_media_list"); err != nil {
		return err
	}
	if !validID(entryID) {
		return invalidArgument("delete_media_list", "entry ID must be a positive GraphQL Int")
	}
	var response struct {
		Deleted *struct {
			Deleted bool `json:"deleted"`
		} `json:"DeleteMediaListEntry"`
	}
	if err := c.requestGraphQL(ctx, "delete_media_list", deleteMediaListMutation, map[string]any{"id": entryID}, &response, options...); err != nil {
		return err
	}
	if response.Deleted == nil || !response.Deleted.Deleted {
		return platformError("delete_media_list", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return nil
}

func hasMediaListUpdate(input SaveMediaListEntryRequest) bool {
	return input.Status != nil || input.Score != nil || input.Progress != nil || input.ProgressVolumes != nil ||
		input.Repeat != nil || input.Priority != nil || input.Private != nil || input.Notes != nil ||
		input.HiddenFromStatusLists != nil || input.CustomLists != nil || input.StartedAt != nil || input.CompletedAt != nil
}

func setOptional[T any](values map[string]any, name string, value *T) {
	if value != nil {
		values[name] = *value
	}
}
