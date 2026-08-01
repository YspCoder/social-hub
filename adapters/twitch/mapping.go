package twitch

import (
	"encoding/json"
	"strings"
	"time"

	"social-hub/pkg/socialhub"
)

func mapUser(accountID socialhub.AccountID, input twitchUser) (*socialhub.User, error) {
	if strings.TrimSpace(input.ID) == "" || strings.TrimSpace(input.Login) == "" {
		return nil, platformError("map_user", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	extension, _ := json.Marshal(input)
	accountType := firstNonEmpty(input.BroadcasterType, input.Type, "user")
	return &socialhub.User{
		Platform: "twitch", AccountID: accountID, ID: input.ID, Username: stringPointer(input.Login),
		DisplayName: stringPointer(input.DisplayName), AvatarURL: stringPointer(input.ProfileImageURL),
		ProfileURL: stringPointer("https://www.twitch.tv/" + input.Login), AccountType: stringPointer(accountType),
		Extensions: map[string]json.RawMessage{"twitch.user": extension},
	}, nil
}

func mapVideo(accountID socialhub.AccountID, input twitchVideo, observedAt time.Time) (*socialhub.Post, error) {
	if strings.TrimSpace(input.ID) == "" {
		return nil, platformError("map_video", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	createdAt := input.PublishedAt
	if createdAt.IsZero() {
		createdAt = input.CreatedAt
	}
	statusTime := createdAt
	metrics := []socialhub.Metric{{
		Name: "views", Value: float64(input.ViewCount), AsOf: observedAt,
		Definition: "Twitch VOD view_count returned by Helix",
	}}
	extension, _ := json.Marshal(input)
	post := &socialhub.Post{
		Platform: "twitch", AccountID: accountID, ID: input.ID, AuthorID: stringPointer(input.UserID),
		Text: stringPointer(input.Title), CreatedAt: timePointer(createdAt), URL: stringPointer(input.URL),
		Visibility: stringPointer(input.Viewable), Metrics: metrics,
		Status:     &socialhub.PublishStatus{ID: input.ID, State: socialhub.PublishStatePublished, UpdatedAt: timePointer(statusTime)},
		Extensions: map[string]json.RawMessage{"twitch.video": extension},
	}
	return post, nil
}

func mapVideoPage(accountID socialhub.AccountID, response videoPage, observedAt time.Time) (socialhub.Page[socialhub.Post], error) {
	items := make([]socialhub.Post, 0, len(response.Data))
	for _, item := range response.Data {
		post, err := mapVideo(accountID, item, observedAt)
		if err != nil {
			return socialhub.Page[socialhub.Post]{}, err
		}
		items = append(items, *post)
	}
	next := stringPointer(response.Pagination.Cursor)
	return socialhub.Page[socialhub.Post]{Items: items, NextCursor: next, HasMore: next != nil}, nil
}

func stringPointer(value string) *string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	copy := value
	return &copy
}

func timePointer(value time.Time) *time.Time {
	if value.IsZero() {
		return nil
	}
	copy := value
	return &copy
}
