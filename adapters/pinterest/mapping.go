package pinterest

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"social-hub/pkg/socialhub"
)

func mapAccount(accountID socialhub.AccountID, input pinterestAccount) *socialhub.User {
	statistics, _ := json.Marshal(struct {
		About          string `json:"about,omitempty"`
		WebsiteURL     string `json:"website_url,omitempty"`
		FollowerCount  *int64 `json:"follower_count,omitempty"`
		FollowingCount *int64 `json:"following_count,omitempty"`
		MonthlyViews   *int64 `json:"monthly_views,omitempty"`
		PinCount       *int64 `json:"pin_count,omitempty"`
		BoardCount     *int64 `json:"board_count,omitempty"`
	}{input.About, input.WebsiteURL, input.FollowerCount, input.FollowingCount, input.MonthlyViews, input.PinCount, input.BoardCount})
	displayName := input.BusinessName
	if displayName == "" {
		displayName = input.Username
	}
	return &socialhub.User{
		Platform: "pinterest", AccountID: accountID, ID: input.ID,
		Username: stringPointer(input.Username), DisplayName: stringPointer(displayName),
		AvatarURL: stringPointer(input.ProfileImage), ProfileURL: stringPointer("https://www.pinterest.com/" + input.Username + "/"),
		AccountType: stringPointer(input.AccountType), Extensions: map[string]json.RawMessage{"pinterest.account": statistics},
	}
}

func mapPin(accountID socialhub.AccountID, userID string, input pinterestPin, observedAt time.Time) *socialhub.Post {
	details, _ := json.Marshal(struct {
		Title           string `json:"title,omitempty"`
		AltText         string `json:"alt_text,omitempty"`
		Link            string `json:"link,omitempty"`
		BoardID         string `json:"board_id,omitempty"`
		BoardSectionID  string `json:"board_section_id,omitempty"`
		CreativeType    string `json:"creative_type,omitempty"`
		DominantColor   string `json:"dominant_color,omitempty"`
		HasBeenPromoted bool   `json:"has_been_promoted,omitempty"`
		IsOwner         bool   `json:"is_owner,omitempty"`
		IsProduct       bool   `json:"is_product,omitempty"`
		IsStandard      bool   `json:"is_standard,omitempty"`
	}{input.Title, input.AltText, input.Link, input.BoardID, input.BoardSectionID, input.CreativeType, input.DominantColor, input.HasBeenPromoted, input.IsOwner, input.IsProduct, input.IsStandard})
	post := &socialhub.Post{
		Platform: "pinterest", AccountID: accountID, ID: input.ID, AuthorID: stringPointer(userID),
		Text: stringPointer(firstNonEmpty(input.Description, input.Title)), CreatedAt: input.CreatedAt,
		URL:    stringPointer("https://www.pinterest.com/pin/" + input.ID + "/"),
		Status: &socialhub.PublishStatus{ID: input.ID, State: socialhub.PublishStatePublished, UpdatedAt: input.CreatedAt},
		Media:  mapPinMedia(input.ID, input.Media), Extensions: map[string]json.RawMessage{"pinterest.pin": details},
	}
	if input.ParentPinID != "" {
		post.Relations = []socialhub.PostRelation{{Type: socialhub.RelationRepost, PostID: input.ParentPinID}}
	}
	for window, metrics := range input.PinMetrics {
		for name, value := range metrics {
			post.Metrics = append(post.Metrics, socialhub.Metric{
				Name: name, Value: value, AsOf: observedAt, Window: window,
				Definition: "Pinterest pin_metrics " + name,
			})
		}
	}
	return post
}

func mapPinMedia(pinID string, input pinterestPinMedia) []socialhub.Media {
	extension, _ := json.Marshal(input)
	mediaExtension := map[string]json.RawMessage{"pinterest.media": extension}
	switch input.MediaType {
	case "image":
		asset := largestAsset(input.Images)
		return []socialhub.Media{{ID: pinID, URL: asset.URL, Type: socialhub.MediaTypeImage, Width: intPointer(asset.Width), Height: intPointer(asset.Height), State: socialhub.MediaStateReady, Extensions: mediaExtension}}
	case "video":
		url := firstNonEmpty(input.VideoURL, input.VideoURLHLS, input.CoverImageURL, largestAsset(input.Images).URL)
		return []socialhub.Media{{ID: pinID, URL: url, Type: socialhub.MediaTypeVideo, Width: input.Width, Height: input.Height, Duration: durationPointer(input.DurationMS), State: socialhub.MediaStateReady, Extensions: mediaExtension}}
	case "multiple_images", "multiple_videos", "multiple_mixed":
		items := make([]socialhub.Media, 0, len(input.Items))
		for index, raw := range input.Items {
			var item pinterestMediaItem
			if json.Unmarshal(raw, &item) != nil {
				continue
			}
			asset := largestAsset(item.Images)
			mediaType := socialhub.MediaTypeImage
			url := firstNonEmpty(item.URL, asset.URL)
			width, height := intPointer(firstPositive(item.Width, asset.Width)), intPointer(firstPositive(item.Height, asset.Height))
			var duration *time.Duration
			if item.ItemType == "video" || input.MediaType == "multiple_videos" {
				mediaType = socialhub.MediaTypeVideo
				url = firstNonEmpty(item.VideoURL, item.URL, asset.URL)
				duration = durationPointer(&item.DurationMS)
			}
			items = append(items, socialhub.Media{ID: pinID + ":" + strconv.Itoa(index), URL: url, Type: mediaType, Width: width, Height: height, Duration: duration, State: socialhub.MediaStateReady})
		}
		return items
	default:
		return nil
	}
}

func largestAsset(images map[string]pinterestAsset) pinterestAsset {
	var best pinterestAsset
	for _, image := range images {
		if image.Width*image.Height > best.Width*best.Height || (best.URL == "" && image.URL != "") {
			best = image
		}
	}
	return best
}

func durationPointer(milliseconds *float64) *time.Duration {
	if milliseconds == nil || *milliseconds <= 0 {
		return nil
	}
	duration := time.Duration(*milliseconds * float64(time.Millisecond))
	return &duration
}

func firstPositive(values ...int) int {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}

func stringPointer(value string) *string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	copy := value
	return &copy
}

func intPointer(value int) *int {
	if value <= 0 {
		return nil
	}
	copy := value
	return &copy
}
