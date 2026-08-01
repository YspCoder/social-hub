package instagram

import (
	"encoding/json"
	"strings"
	"time"

	"social-hub/pkg/socialhub"
)

func mapUser(accountID socialhub.AccountID, input instagramUser) *socialhub.User {
	id := firstNonEmpty(input.UserID, input.ID)
	var profileURL *string
	if input.Username != "" {
		profileURL = stringPointer("https://www.instagram.com/" + input.Username + "/")
	}
	extension, _ := json.Marshal(struct {
		GraphID        string `json:"graph_id,omitempty"`
		FollowersCount int64  `json:"followers_count,omitempty"`
		MediaCount     int64  `json:"media_count,omitempty"`
	}{GraphID: input.ID, FollowersCount: input.FollowersCount, MediaCount: input.MediaCount})
	return &socialhub.User{
		Platform: "instagram", AccountID: accountID, ID: id, Username: stringPointer(input.Username),
		DisplayName: stringPointer(input.Name), AvatarURL: stringPointer(input.ProfilePictureURL), ProfileURL: profileURL,
		AccountType: stringPointer(strings.ToLower(input.AccountType)), Extensions: map[string]json.RawMessage{"instagram.user": extension},
	}
}

func mapMediaPost(accountID socialhub.AccountID, authorID string, input instagramMedia) *socialhub.Post {
	post := &socialhub.Post{
		Platform: "instagram", AccountID: accountID, ID: input.ID, AuthorID: stringPointer(authorID),
		Text: stringPointer(input.Caption), CreatedAt: input.Timestamp, URL: stringPointer(input.Permalink),
		Status: &socialhub.PublishStatus{ID: input.ID, State: socialhub.PublishStatePublished, UpdatedAt: input.Timestamp},
	}
	if input.MediaURL != "" || input.ThumbnailURL != "" {
		post.Media = append(post.Media, mapInstagramMedia(input))
	}
	for _, child := range input.Children.Data {
		post.Media = append(post.Media, mapInstagramMedia(child))
	}
	extension, _ := json.Marshal(struct {
		MediaType        string `json:"media_type"`
		MediaProductType string `json:"media_product_type,omitempty"`
		Username         string `json:"username,omitempty"`
	}{MediaType: input.MediaType, MediaProductType: input.MediaProductType, Username: input.Username})
	post.Extensions = map[string]json.RawMessage{"instagram.media": extension}
	return post
}

func mapInstagramMedia(input instagramMedia) socialhub.Media {
	mediaType := socialhub.MediaTypeDocument
	switch input.MediaType {
	case "IMAGE", "CAROUSEL_ALBUM":
		mediaType = socialhub.MediaTypeImage
	case "VIDEO":
		mediaType = socialhub.MediaTypeVideo
	}
	url := firstNonEmpty(input.MediaURL, input.ThumbnailURL)
	extension, _ := json.Marshal(struct {
		MediaProductType string `json:"media_product_type,omitempty"`
		ThumbnailURL     string `json:"thumbnail_url,omitempty"`
	}{MediaProductType: input.MediaProductType, ThumbnailURL: input.ThumbnailURL})
	return socialhub.Media{ID: input.ID, URL: url, Type: mediaType, State: socialhub.MediaStateReady, Extensions: map[string]json.RawMessage{"instagram.media": extension}}
}

func mapMediaPage(accountID socialhub.AccountID, authorID string, input instagramMediaList) socialhub.Page[socialhub.Post] {
	items := make([]socialhub.Post, 0, len(input.Data))
	for _, media := range input.Data {
		items = append(items, *mapMediaPost(accountID, authorID, media))
	}
	return socialhub.Page[socialhub.Post]{Items: items, NextCursor: input.Paging.Cursors.After, PrevCursor: input.Paging.Cursors.Before, HasMore: input.Paging.Next != ""}
}

func mapComment(accountID socialhub.AccountID, postID string, input instagramComment, observedAt time.Time) socialhub.Comment {
	authorID := input.From.ID
	if authorID == "" {
		authorID = input.Username
	}
	var metrics []socialhub.Metric
	if input.LikeCount > 0 {
		metrics = append(metrics, socialhub.Metric{Name: "likes", Value: float64(input.LikeCount), AsOf: observedAt, Definition: "Instagram comment like_count"})
	}
	return socialhub.Comment{
		Platform: "instagram", AccountID: accountID, ID: input.ID, PostID: postID, AuthorID: stringPointer(authorID),
		ParentID: stringPointer(input.ParentID), Text: input.Text, CreatedAt: input.Timestamp, Metrics: metrics,
	}
}

func mapCommentPage(accountID socialhub.AccountID, postID string, input instagramCommentList, observedAt time.Time) socialhub.Page[socialhub.Comment] {
	items := make([]socialhub.Comment, 0, len(input.Data))
	for _, comment := range input.Data {
		items = append(items, mapComment(accountID, postID, comment, observedAt))
	}
	return socialhub.Page[socialhub.Comment]{Items: items, NextCursor: input.Paging.Cursors.After, PrevCursor: input.Paging.Cursors.Before, HasMore: input.Paging.Next != ""}
}

func stringPointer(value string) *string {
	if value == "" {
		return nil
	}
	copy := value
	return &copy
}
