package dailymotion

import (
	"encoding/json"
	"errors"
	"net/url"
	"strconv"
	"strings"
	"time"

	"social-hub/pkg/socialhub"
)

func (c *Client) mapProfile(input Profile) (*socialhub.User, error) {
	if !validResourceID(input.ProfileID) {
		return nil, platformError("map_profile", socialhub.CodePlatformError, socialhub.ClassPermanent, errors.New("profile response is missing profile_id"))
	}
	displayName := input.Name
	if input.DisplayName != nil && *input.DisplayName != "" {
		displayName = *input.DisplayName
	}
	extensions := map[string]json.RawMessage{}
	addExtension(extensions, "description", input.Description)
	addExtension(extensions, "can_change_name", input.CanChangeName)
	addExtension(extensions, "social_links", input.SocialLinks)
	addExtension(extensions, "webhook", input.Webhook)
	return &socialhub.User{
		Platform: "dailymotion", AccountID: c.accountID, ID: input.ProfileID,
		Username: stringPointer(input.Name), DisplayName: stringPointer(displayName), Extensions: extensions,
	}, nil
}

func (c *Client) mapVideo(input Video) (*socialhub.Post, error) {
	if !validResourceID(input.VideoID) {
		return nil, platformError("map_video", socialhub.CodePlatformError, socialhub.ClassPermanent, errors.New("video response is missing video_id"))
	}
	text := input.Title
	if input.Description != nil && *input.Description != "" {
		text = *input.Description
	}
	mediaState := socialhub.MediaStateProcessing
	publishState := socialhub.PublishStatePending
	switch input.Processing.EncodingStatus {
	case "encoded":
		mediaState = socialhub.MediaStateReady
		if input.IsPublished {
			publishState = socialhub.PublishStatePublished
		}
	case "error":
		mediaState, publishState = socialhub.MediaStateFailed, socialhub.PublishStateFailed
	case "waiting_for_upload":
		mediaState = socialhub.MediaStateCreated
	case "uploaded":
		mediaState = socialhub.MediaStateUploading
	}
	media := socialhub.Media{ID: input.VideoID, Type: socialhub.MediaTypeVideo, State: mediaState}
	if input.Source.Width != nil {
		media.Width = input.Source.Width
	}
	if input.Source.Height != nil {
		media.Height = input.Source.Height
	}
	if input.Source.Duration != nil {
		duration := time.Duration(*input.Source.Duration * float64(time.Second))
		media.Duration = &duration
	}
	extensions := map[string]json.RawMessage{}
	addExtension(extensions, "title", input.Title)
	addExtension(extensions, "category", input.Category)
	addExtension(extensions, "is_for_kids", input.IsForKids)
	addExtension(extensions, "is_explicit", input.IsExplicit)
	addExtension(extensions, "processing", input.Processing)
	addExtension(extensions, "tags", input.Tags)
	addExtension(extensions, "hashtags", input.Hashtags)
	addExtension(extensions, "thumbnail", input.Thumbnail)
	post := &socialhub.Post{
		Platform: "dailymotion", AccountID: c.accountID, ID: input.VideoID, Text: &text,
		Media: []socialhub.Media{media}, CreatedAt: &input.CreatedAt, Visibility: stringPointer(input.Visibility),
		Status: &socialhub.PublishStatus{ID: input.VideoID, State: publishState, UpdatedAt: &input.UpdatedAt}, Extensions: extensions,
	}
	if input.Profile.ProfileID != "" {
		post.AuthorID = stringPointer(input.Profile.ProfileID)
	}
	if input.VideoURL != "" {
		post.URL = stringPointer(input.VideoURL)
	}
	return post, nil
}

func addExtension(target map[string]json.RawMessage, key string, value any) {
	encoded, err := json.Marshal(value)
	if err == nil && string(encoded) != "null" {
		target[key] = encoded
	}
}

func stringPointer(value string) *string { return &value }

func pageQuery(operation, cursor string, maximum int) (url.Values, error) {
	if maximum < 0 {
		return nil, invalidArgument(operation, "max results must not be negative")
	}
	query := url.Values{}
	if cursor != "" {
		page, err := strconv.Atoi(cursor)
		if err != nil || page < 1 || page > 1_000_000 {
			return nil, invalidArgument(operation, "cursor must be a positive decimal page number")
		}
		query.Set("page", strconv.Itoa(page))
	}
	if maximum > 0 {
		query.Set("page_size", strconv.Itoa(min(maximum, 100)))
	}
	return query, nil
}

func mapPage[T any](c *Client, input apiPage[T]) (socialhub.Page[T], error) {
	next, err := pageCursor(input.Pagination.Next, c.apiBaseURL)
	if err != nil {
		return socialhub.Page[T]{}, err
	}
	previous, err := pageCursor(input.Pagination.Previous, c.apiBaseURL)
	if err != nil {
		return socialhub.Page[T]{}, err
	}
	return socialhub.Page[T]{Items: input.Data, NextCursor: next, PrevCursor: previous, HasMore: next != nil}, nil
}

func pageCursor(value *string, base *url.URL) (*string, error) {
	if value == nil || strings.TrimSpace(*value) == "" {
		return nil, nil
	}
	parsed, err := url.Parse(*value)
	if err != nil {
		return nil, platformError("pagination", socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	if (parsed.Scheme != "" || parsed.Host != "") && (base == nil || !strings.EqualFold(parsed.Scheme, base.Scheme) || !strings.EqualFold(parsed.Host, base.Host)) {
		return nil, platformError("pagination", socialhub.CodePlatformError, socialhub.ClassPermanent, errors.New("pagination URL changed origin"))
	}
	page := parsed.Query().Get("page")
	number, err := strconv.Atoi(page)
	if err != nil || number < 1 {
		return nil, platformError("pagination", socialhub.CodePlatformError, socialhub.ClassPermanent, errors.New("pagination URL has an invalid page"))
	}
	return &page, nil
}
