package snapchat

import (
	"encoding/json"
	"net/url"
	"strings"
	"time"

	"social-hub/pkg/socialhub"
)

func mapProfile(accountID socialhub.AccountID, input snapProfile) *socialhub.User {
	var subscriberCount *int64
	if input.SubscriberCount.Set {
		value := input.SubscriberCount.Value
		subscriberCount = &value
	}
	extension, _ := json.Marshal(struct {
		OrganizationID             string `json:"organization_id,omitempty"`
		Description                string `json:"description,omitempty"`
		Category                   string `json:"category,omitempty"`
		Subcategory                string `json:"subcategory,omitempty"`
		Email                      string `json:"email,omitempty"`
		PhoneNumber                string `json:"phone_number,omitempty"`
		Website                    string `json:"website,omitempty"`
		AddressLine1               string `json:"address_line_1,omitempty"`
		AddressLine2               string `json:"address_line_2,omitempty"`
		AddressLine3               string `json:"address_line_3,omitempty"`
		Locality                   string `json:"locality,omitempty"`
		AdministrativeDistrict1    string `json:"administrative_district_level_1,omitempty"`
		AdministrativeDistrict2    string `json:"administrative_district_level_2,omitempty"`
		PostalCode                 string `json:"postal_code,omitempty"`
		Country                    string `json:"country,omitempty"`
		CountryCode90Days          string `json:"l_90_country_code,omitempty"`
		StoreID                    string `json:"store_id,omitempty"`
		ProfileType                string `json:"profile_type,omitempty"`
		ProfileTier                string `json:"profile_tier,omitempty"`
		ProfileIconPrimaryColorHex string `json:"profile_icon_primary_color_hex,omitempty"`
		InternalProfileCategory    string `json:"internal_profile_category,omitempty"`
		SubscriberCount            *int64 `json:"subscriber_count,omitempty"`
		ShareAuthorizedData        bool   `json:"share_authorized_data_with_api_partners,omitempty"`
		ShareInsights              bool   `json:"share_insights,omitempty"`
		ShareExpiredStories        bool   `json:"share_expired_stories,omitempty"`
	}{
		input.OrganizationID, input.Description, input.Category, input.Subcategory, input.Email, input.PhoneNumber,
		input.Website, input.AddressLine1, input.AddressLine2, input.AddressLine3, input.Locality,
		input.AdministrativeDistrictLevel1, input.AdministrativeDistrictLevel2, input.PostalCode, input.Country,
		input.CountryCode90Days, input.StoreID, input.ProfileType, input.ProfileTier, input.ProfileIconPrimaryColorHex,
		input.InternalProfileCategory, subscriberCount, input.ShareAuthorizedData, input.ShareInsights, input.ShareExpiredStories,
	})
	return &socialhub.User{
		Platform: "snapchat", AccountID: accountID, ID: input.ID, Username: stringPointer(input.SnapUsername),
		DisplayName: stringPointer(input.DisplayName), AvatarURL: stringPointer(input.LogoURLs.OriginalLogoURL),
		ProfileURL: profileURL(input.SnapUsername), AccountType: stringPointer(strings.ToLower(firstNonEmpty(input.ProfileType, input.ProfileTier))),
		Extensions: map[string]json.RawMessage{"snapchat.public_profile": extension},
	}
}

func mapSpotlight(accountID socialhub.AccountID, input snapSpotlight) *socialhub.Post {
	var duration *time.Duration
	if input.Duration > 0 {
		value := time.Duration(input.Duration * float64(time.Second))
		duration = &value
	}
	mediaState := socialhub.MediaStateProcessing
	publishState := socialhub.PublishStatePending
	switch input.Status {
	case "LIVE":
		mediaState, publishState = socialhub.MediaStateReady, socialhub.PublishStatePublished
	case "REJECTED":
		mediaState, publishState = socialhub.MediaStateFailed, socialhub.PublishStateFailed
	}
	mediaExtension, _ := json.Marshal(struct {
		ThumbnailURL string `json:"thumbnail_url,omitempty"`
	}{input.ThumbnailURL})
	postExtension, _ := json.Marshal(struct {
		Title    string            `json:"title,omitempty"`
		Caption  string            `json:"caption,omitempty"`
		Link     string            `json:"link,omitempty"`
		Status   string            `json:"status,omitempty"`
		Hashtags []string          `json:"hashtags,omitempty"`
		Sponsor  *spotlightSponsor `json:"sponsor,omitempty"`
		MLTags   []string          `json:"ml_tags,omitempty"`
	}{input.Title, input.Caption, input.Link, input.Status, input.Hashtags, input.Sponsor, input.MLTags})
	return &socialhub.Post{
		Platform: "snapchat", AccountID: accountID, ID: input.ID, AuthorID: stringPointer(input.ProfileID),
		Text: stringPointer(firstNonEmpty(input.Caption, input.Title)), CreatedAt: input.CreatedAt, Visibility: stringPointer("public"),
		Media: []socialhub.Media{{
			ID: input.ID, URL: input.MediaURL, Type: socialhub.MediaTypeVideo, Duration: duration, State: mediaState,
			Extensions: map[string]json.RawMessage{"snapchat.spotlight_media": mediaExtension},
		}},
		Status:     &socialhub.PublishStatus{ID: input.ID, State: publishState, UpdatedAt: input.CreatedAt},
		Extensions: map[string]json.RawMessage{"snapchat.spotlight": postExtension},
	}
}

func profileURL(username string) *string {
	if strings.TrimSpace(username) == "" {
		return nil
	}
	value := "https://www.snapchat.com/add/" + url.PathEscape(username)
	return &value
}

func stringPointer(value string) *string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	copy := value
	return &copy
}
