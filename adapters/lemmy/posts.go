package lemmy

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"social-hub/pkg/socialhub"
)

type createPostPayload struct {
	Name            string `json:"name"`
	CommunityID     int64  `json:"community_id"`
	URL             string `json:"url,omitempty"`
	Body            string `json:"body,omitempty"`
	AltText         string `json:"alt_text,omitempty"`
	NSFW            bool   `json:"nsfw,omitempty"`
	LanguageID      *int64 `json:"language_id,omitempty"`
	CustomThumbnail string `json:"custom_thumbnail,omitempty"`
}

type updatePostPayload struct {
	PostID          int64   `json:"post_id"`
	Name            *string `json:"name,omitempty"`
	URL             *string `json:"url,omitempty"`
	Body            *string `json:"body,omitempty"`
	AltText         *string `json:"alt_text,omitempty"`
	NSFW            *bool   `json:"nsfw,omitempty"`
	LanguageID      *int64  `json:"language_id,omitempty"`
	CustomThumbnail *string `json:"custom_thumbnail,omitempty"`
}

func (client *Client) CreatePost(ctx context.Context, input CreatePostRequest, options ...socialhub.CallOption) (*Post, error) {
	payload, err := client.createPostPayload(input)
	if err != nil {
		return nil, err
	}
	var response postResponse
	if err := client.requestJSON(ctx, http.MethodPost, "/post", nil, payload, &response, options...); err != nil {
		return nil, err
	}
	if !validPostView(response.PostView) {
		return nil, platformError("create_post", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	mapped := client.mapPost(response.PostView)
	return &mapped, nil
}

func (client *Client) createPostPayload(input CreatePostRequest) (createPostPayload, error) {
	if !validTitle(input.Title) || !validID(input.CommunityID) {
		return createPostPayload{}, invalidArgument("create_post", "title and positive community_id are required")
	}
	if !validBody(input.Body, 50000) || !validBody(input.AltText, 1500) {
		return createPostPayload{}, invalidArgument("create_post", "body or alt_text exceeds the Lemmy 0.19 limit")
	}
	if input.URL != "" && input.MediaID != "" {
		return createPostPayload{}, invalidArgument("create_post", "url and media_id are mutually exclusive")
	}
	postURL := input.URL
	if input.MediaID != "" {
		client.uploadMu.Lock()
		media, found := client.media[input.MediaID]
		client.uploadMu.Unlock()
		if !found || media.URL == "" {
			return createPostPayload{}, invalidArgument("create_post", "media_id must identify an image completed by this client")
		}
		postURL = media.URL
	}
	if !validPostURL(postURL) {
		return createPostPayload{}, invalidArgument("create_post", "url must be an allowed HTTP(S) or magnet URL")
	}
	if input.CustomThumbnailURL != "" && !validHTTPURL(input.CustomThumbnailURL) {
		return createPostPayload{}, invalidArgument("create_post", "custom_thumbnail_url must be an absolute HTTP(S) URL")
	}
	languageID, err := optionalID(input.LanguageID, "create_post")
	if err != nil {
		return createPostPayload{}, err
	}
	return createPostPayload{
		Name: input.Title, CommunityID: mustID(input.CommunityID), URL: postURL, Body: input.Body,
		AltText: input.AltText, NSFW: input.NSFW, LanguageID: languageID, CustomThumbnail: input.CustomThumbnailURL,
	}, nil
}

func (client *Client) GetLemmyPost(ctx context.Context, postID string, options ...socialhub.CallOption) (*Post, error) {
	view, err := client.getPostView(ctx, postID, options...)
	if err != nil {
		return nil, err
	}
	mapped := client.mapPost(view)
	return &mapped, nil
}

func (client *Client) UpdatePost(ctx context.Context, postID string, input UpdatePostRequest, options ...socialhub.CallOption) (*Post, error) {
	if !validID(postID) {
		return nil, invalidArgument("update_post", "post ID must be a positive integer")
	}
	payload, err := updatePostPayloadFor(postID, input)
	if err != nil {
		return nil, err
	}
	var response postResponse
	if err := client.requestJSON(ctx, http.MethodPut, "/post", nil, payload, &response, options...); err != nil {
		return nil, err
	}
	if !validPostView(response.PostView) || response.PostView.Post.ID != mustID(postID) {
		return nil, platformError("update_post", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	mapped := client.mapPost(response.PostView)
	return &mapped, nil
}

func updatePostPayloadFor(postID string, input UpdatePostRequest) (updatePostPayload, error) {
	if input.Title == nil && input.URL == nil && input.Body == nil && input.AltText == nil && input.NSFW == nil &&
		input.LanguageID == nil && input.CustomThumbnailURL == nil {
		return updatePostPayload{}, invalidArgument("update_post", "at least one post field is required")
	}
	if input.Title != nil && !validTitle(*input.Title) {
		return updatePostPayload{}, invalidArgument("update_post", "title must contain 3-200 characters and no newline")
	}
	if input.URL != nil && !validPostURL(*input.URL) {
		return updatePostPayload{}, invalidArgument("update_post", "url must be empty or an allowed HTTP(S) or magnet URL")
	}
	if input.Body != nil && !validBody(*input.Body, 50000) || input.AltText != nil && !validBody(*input.AltText, 1500) {
		return updatePostPayload{}, invalidArgument("update_post", "body or alt_text exceeds the Lemmy 0.19 limit")
	}
	if input.CustomThumbnailURL != nil && *input.CustomThumbnailURL != "" && !validHTTPURL(*input.CustomThumbnailURL) {
		return updatePostPayload{}, invalidArgument("update_post", "custom_thumbnail_url must be empty or an absolute HTTP(S) URL")
	}
	var languageID *int64
	if input.LanguageID != nil {
		if !validID(*input.LanguageID) {
			return updatePostPayload{}, invalidArgument("update_post", "language_id must be a positive integer")
		}
		languageID = int64Pointer(mustID(*input.LanguageID))
	}
	return updatePostPayload{
		PostID: mustID(postID), Name: input.Title, URL: input.URL, Body: input.Body, AltText: input.AltText,
		NSFW: input.NSFW, LanguageID: languageID, CustomThumbnail: input.CustomThumbnailURL,
	}, nil
}

func (client *Client) DeletePost(ctx context.Context, postID string, options ...socialhub.CallOption) error {
	if !validID(postID) {
		return invalidArgument("delete_post", "post ID must be a positive integer")
	}
	payload := struct {
		PostID  int64 `json:"post_id"`
		Deleted bool  `json:"deleted"`
	}{PostID: mustID(postID), Deleted: true}
	var response postResponse
	if err := client.requestJSON(ctx, http.MethodPost, "/post/delete", nil, payload, &response, options...); err != nil {
		return err
	}
	if response.PostView.Post.ID != mustID(postID) || !response.PostView.Post.Deleted {
		return platformError("delete_post", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return nil
}

func (client *Client) ListFeed(ctx context.Context, input FeedRequest, options ...socialhub.CallOption) (socialhub.Page[Post], error) {
	query, err := feedQuery(input)
	if err != nil {
		return socialhub.Page[Post]{}, err
	}
	var response getPostsResponse
	if err := client.requestJSON(ctx, http.MethodGet, "/post/list", query, nil, &response, options...); err != nil {
		return socialhub.Page[Post]{}, err
	}
	result := socialhub.Page[Post]{Items: make([]Post, 0, len(response.Posts))}
	for _, view := range response.Posts {
		if !validPostView(view) {
			return socialhub.Page[Post]{}, platformError("list_feed", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
		}
		result.Items = append(result.Items, client.mapPost(view))
	}
	if response.NextPage != "" {
		if !validCursor(response.NextPage) {
			return socialhub.Page[Post]{}, platformError("list_feed", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
		}
		result.NextCursor, result.HasMore = stringPointer(response.NextPage), true
	}
	return result, nil
}

func feedQuery(input FeedRequest) (url.Values, error) {
	if input.MaxResults < 0 || input.MaxResults > 50 {
		return nil, invalidArgument("list_feed", "max_results must be between 0 and 50")
	}
	if input.Cursor != "" && !validCursor(input.Cursor) {
		return nil, invalidArgument("list_feed", "cursor is invalid")
	}
	if input.CommunityID != "" && input.CommunityName != "" {
		return nil, invalidArgument("list_feed", "community_id and community_name are mutually exclusive")
	}
	if input.CommunityID != "" && !validID(input.CommunityID) || input.CommunityName != "" && !validUsername(input.CommunityName) {
		return nil, invalidArgument("list_feed", "community selector is invalid")
	}
	listing := input.Listing
	if listing == "" {
		listing = ListingAll
	}
	if !validListing(listing) {
		return nil, invalidArgument("list_feed", "listing type is invalid")
	}
	sortType := input.Sort
	if sortType == "" {
		sortType = SortHot
	}
	if !validSort(sortType) {
		return nil, invalidArgument("list_feed", "sort type is invalid")
	}
	maximum := input.MaxResults
	if maximum == 0 {
		maximum = 20
	}
	query := url.Values{"type_": {string(listing)}, "sort": {string(sortType)}, "limit": {strconv.Itoa(maximum)}}
	if input.Cursor != "" {
		query.Set("page_cursor", input.Cursor)
	}
	if input.CommunityID != "" {
		query.Set("community_id", input.CommunityID)
	}
	if input.CommunityName != "" {
		query.Set("community_name", input.CommunityName)
	}
	for key, enabled := range map[string]bool{
		"saved_only": input.SavedOnly, "liked_only": input.LikedOnly, "disliked_only": input.DislikedOnly,
		"show_hidden": input.ShowHidden, "show_read": input.ShowRead, "show_nsfw": input.ShowNSFW,
	} {
		if enabled {
			query.Set(key, "true")
		}
	}
	return query, nil
}

func validListing(value ListingType) bool {
	switch value {
	case ListingAll, ListingLocal, ListingSubscribed, ListingModeratorView:
		return true
	default:
		return false
	}
}

func validSort(value SortType) bool {
	switch value {
	case SortActive, SortHot, SortNew, SortOld, SortTopDay, SortTopWeek, SortTopMonth, SortTopYear, SortTopAll,
		SortMostComments, SortNewComments, SortTopHour, SortTopSixHour, SortTopTwelveHour, SortTopThreeMonths,
		SortTopSixMonths, SortTopNineMonths, SortControversial, SortScaled:
		return true
	default:
		return false
	}
}

func optionalID(value, operation string) (*int64, error) {
	if strings.TrimSpace(value) == "" {
		return nil, nil
	}
	if !validID(value) {
		return nil, invalidArgument(operation, "ID must be a positive integer")
	}
	return int64Pointer(mustID(value)), nil
}
