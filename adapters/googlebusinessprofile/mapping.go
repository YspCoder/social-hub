package googlebusinessprofile

import (
	"encoding/json"
	"strings"

	"social-hub/pkg/socialhub"
)

func mapLocation(accountID socialhub.AccountID, location *Location) *socialhub.User {
	accountType := "business_location"
	extension := append(json.RawMessage(nil), location.Raw...)
	if len(extension) == 0 {
		extension, _ = json.Marshal(location)
	}
	return &socialhub.User{
		Platform: platformName, AccountID: accountID, ID: resourceID(location.Name),
		Username: stringPointer(location.StoreCode), DisplayName: stringPointer(location.LocationName),
		ProfileURL: stringPointer(firstNonEmpty(location.Metadata.MapsURL, location.WebsiteURL)), AccountType: &accountType,
		Extensions: map[string]json.RawMessage{"google_business_profile.location": extension},
	}
}

func mapPost(accountID socialhub.AccountID, locationID string, post *LocalPost) *socialhub.Post {
	extension := append(json.RawMessage(nil), post.Raw...)
	if len(extension) == 0 {
		extension, _ = json.Marshal(post)
	}
	visibility := "public"
	return &socialhub.Post{
		Platform: platformName, AccountID: accountID, ID: post.ID, AuthorID: stringPointer(locationID),
		Text: stringPointer(post.Summary), Media: mapMedia(post.Media), CreatedAt: post.CreateTime,
		URL: stringPointer(post.SearchURL), Visibility: &visibility, Status: mapPublishStatus(post),
		Extensions: map[string]json.RawMessage{"google_business_profile.local_post": extension},
	}
}

func mapMedia(items []LocalPostMedia) []socialhub.Media {
	mapped := make([]socialhub.Media, 0, len(items))
	for _, item := range items {
		mediaType := socialhub.MediaTypeImage
		if item.MediaFormat == MediaFormatVideo {
			mediaType = socialhub.MediaTypeVideo
		}
		raw, _ := json.Marshal(item)
		mapped = append(mapped, socialhub.Media{
			ID: resourceID(item.Name), URL: firstNonEmpty(item.GoogleURL, item.SourceURL, item.ThumbnailURL),
			Type: mediaType, State: socialhub.MediaStateReady,
			Extensions: map[string]json.RawMessage{"google_business_profile.media": raw},
		})
	}
	return mapped
}

func mapPublishStatus(post *LocalPost) *socialhub.PublishStatus {
	state := socialhub.PublishStatePending
	switch post.State {
	case LocalPostLive, LocalPostRecurring:
		state = socialhub.PublishStatePublished
	case LocalPostRejected:
		state = socialhub.PublishStateFailed
	}
	updatedAt := post.UpdateTime
	if updatedAt == nil {
		updatedAt = post.CreateTime
	}
	return &socialhub.PublishStatus{ID: post.ID, State: state, Message: string(post.State), UpdatedAt: updatedAt}
}

func (client *Client) validateLocation(operation string, location *Location) error {
	if location == nil || location.Name != client.locationResource() || resourceID(location.Name) != client.locationID || !validRequiredText(location.LocationName, 4096) {
		return platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return nil
}

func (client *Client) validateLocalPost(operation string, post *LocalPost, expectedID string) error {
	prefix := client.locationResource() + "/localPosts/"
	if post == nil || !strings.HasPrefix(post.Name, prefix) {
		return platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	post.ID = strings.TrimPrefix(post.Name, prefix)
	if !validResourceSegment(post.ID) || expectedID != "" && post.ID != expectedID || post.TopicType == "" || !validOptionalText(post.Summary, 100_000) {
		return platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return nil
}

func (client *Client) validateReview(operation string, review *Review, expectedID string) error {
	prefix := client.locationResource() + "/reviews/"
	if review == nil || !strings.HasPrefix(review.Name, prefix) || !validResourceSegment(review.ID) ||
		strings.TrimPrefix(review.Name, prefix) != review.ID || expectedID != "" && review.ID != expectedID || !validStarRating(review.StarRating) {
		return platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return nil
}

func resourceID(resourceName string) string {
	index := strings.LastIndex(resourceName, "/")
	if index < 0 {
		return resourceName
	}
	return resourceName[index+1:]
}

func stringPointer(value string) *string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	copy := value
	return &copy
}
