package snapchat

import (
	"bytes"
	"encoding/json"
	"strconv"
	"time"
)

// SearchCategory is a Snapchat creator-search category filter.
type SearchCategory string

const (
	SearchCategoryPerson   SearchCategory = "CATEGORY_PERSON"
	SearchCategoryBusiness SearchCategory = "CATEGORY_BUSINESS"
)

// SearchTier is a Snapchat creator-search profile tier filter.
type SearchTier string

const (
	SearchTierPublic         SearchTier = "TIER_PUBLIC"
	SearchTierPublicOfficial SearchTier = "TIER_PUBLIC_OFFICIAL"
)

// ProfileSearchRequest describes the public creator-search endpoint.
type ProfileSearchRequest struct {
	Query           string
	Limit           int
	Category        SearchCategory
	Tier            SearchTier
	IncludeStandard bool
}

// SpotlightListRequest describes an authorized profile Spotlight page.
// ProfileID defaults to the configured account profile.
type SpotlightListRequest struct {
	ProfileID string
	Cursor    string
	Limit     int
}

type responseMeta struct {
	RequestID        string `json:"request_id"`
	RequestStatus    string `json:"request_status"`
	DebugMessage     string `json:"debug_message"`
	DisplayMessage   string `json:"display_message"`
	ErrorCode        string `json:"error_code"`
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

type paging struct {
	NextPageID string `json:"next_page_id"`
	NextLink   string `json:"next_link"`
}

type profileEnvelope struct {
	responseMeta
	PublicProfiles []profileSubResponse `json:"public_profiles"`
	PublicProfile  *snapProfile         `json:"public_profile"`
	Paging         paging               `json:"paging"`
}

type profileSubResponse struct {
	SubRequestStatus      string      `json:"sub_request_status"`
	SubRequestErrorReason string      `json:"sub_request_error_reason"`
	PublicProfile         snapProfile `json:"public_profile"`
}

type snapProfile struct {
	ID                           string        `json:"id"`
	OrganizationID               string        `json:"organization_id"`
	DisplayName                  string        `json:"display_name"`
	Description                  string        `json:"description"`
	Category                     string        `json:"category"`
	Subcategory                  string        `json:"subcategory"`
	Email                        string        `json:"email"`
	PhoneNumber                  string        `json:"phone_number"`
	Website                      string        `json:"website"`
	LogoURLs                     logoURLs      `json:"logo_urls"`
	AddressLine1                 string        `json:"address_line_1"`
	AddressLine2                 string        `json:"address_line_2"`
	AddressLine3                 string        `json:"address_line_3"`
	Locality                     string        `json:"locality"`
	AdministrativeDistrictLevel1 string        `json:"administrative_district_level_1"`
	AdministrativeDistrictLevel2 string        `json:"administrative_district_level_2"`
	PostalCode                   string        `json:"postal_code"`
	Country                      string        `json:"country"`
	CountryCode90Days            string        `json:"l_90_country_code"`
	SnapUsername                 string        `json:"snap_user_name"`
	StoreID                      string        `json:"store_id"`
	ProfileType                  string        `json:"profile_type"`
	ProfileTier                  string        `json:"profile_tier"`
	ProfileIconPrimaryColorHex   string        `json:"profile_icon_primary_color_hex"`
	InternalProfileCategory      string        `json:"internal_profile_category"`
	SubscriberCount              flexibleInt64 `json:"subscriber_count"`
	ShareAuthorizedData          bool          `json:"share_authorized_data_with_api_partners"`
	ShareInsights                bool          `json:"share_insights"`
	ShareExpiredStories          bool          `json:"share_expired_stories"`
}

type logoURLs struct {
	OriginalLogoURL      string `json:"original_logo_url"`
	DiscoverFeedLogoURL  string `json:"discover_feed_logo_url"`
	MegaProfileLogoURL   string `json:"mega_profile_logo_url"`
	ManageProfileLogoURL string `json:"manage_profile_logo_url"`
}

type spotlightEnvelope struct {
	responseMeta
	Spotlights []spotlightSubResponse `json:"spotlights"`
	Paging     paging                 `json:"paging"`
}

type spotlightSubResponse struct {
	SubRequestStatus      string        `json:"sub_request_status"`
	SubRequestErrorReason string        `json:"sub_request_error_reason"`
	Spotlight             snapSpotlight `json:"spotlight"`
}

type snapSpotlight struct {
	ID           string            `json:"id"`
	ProfileID    string            `json:"profile_id"`
	ThumbnailURL string            `json:"thumbnail_url"`
	MediaURL     string            `json:"media_url"`
	CreatedAt    *time.Time        `json:"created_at"`
	Status       string            `json:"status"`
	Duration     float64           `json:"duration"`
	Title        string            `json:"title"`
	Caption      string            `json:"caption"`
	Link         string            `json:"link"`
	Hashtags     []string          `json:"hashtags"`
	Sponsor      *spotlightSponsor `json:"sponsor"`
	MLTags       []string          `json:"ml_tags"`
}

type spotlightSponsor struct {
	ProfileID     string `json:"profile_id"`
	DisplayName   string `json:"display_name"`
	SponsorStatus string `json:"sponsor_status"`
}

type flexibleInt64 struct {
	Value int64
	Set   bool
}

func (v *flexibleInt64) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if bytes.Equal(data, []byte("null")) || len(data) == 0 {
		return nil
	}
	if data[0] == '"' {
		var value string
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		parsed, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return err
		}
		v.Value, v.Set = parsed, true
		return nil
	}
	parsed, err := strconv.ParseInt(string(data), 10, 64)
	if err != nil {
		return err
	}
	v.Value, v.Set = parsed, true
	return nil
}
