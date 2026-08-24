package viber

import (
	"context"
	"encoding/json"
	"math"
	"net/url"
	"strings"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

// GetAccountInfo retrieves the configured Bot account.
func (c *Client) GetAccountInfo(ctx context.Context, options ...socialhub.CallOption) (*AccountInfo, error) {
	var response AccountInfo
	if err := c.request(ctx, "/pa/get_account_info", map[string]any{}, &response, options...); err != nil {
		return nil, err
	}
	if !validOpaqueID(response.ID) || strings.TrimSpace(response.Name) == "" || utf8.RuneCountInString(response.Name) > 75 {
		return nil, platformError("get_account_info", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	if response.Location != nil && (math.IsNaN(response.Location.Latitude) || math.IsInf(response.Location.Latitude, 0) ||
		math.IsNaN(response.Location.Longitude) || math.IsInf(response.Location.Longitude, 0) ||
		response.Location.Latitude < -90 || response.Location.Latitude > 90 || response.Location.Longitude < -180 || response.Location.Longitude > 180) {
		return nil, platformError("get_account_info", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	for _, endpoint := range []string{response.Icon, response.Background, response.Webhook} {
		if endpoint != "" && !validRemoteURL(endpoint, 2000) {
			return nil, platformError("get_account_info", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
		}
	}
	return &response, nil
}

// GetUserDetails retrieves one subscribed user's profile. Viber limits this
// endpoint to two requests per user in each 12-hour period.
func (c *Client) GetUserDetails(ctx context.Context, userID string, options ...socialhub.CallOption) (*UserDetails, error) {
	userID = strings.TrimSpace(userID)
	if !validOpaqueID(userID) {
		return nil, invalidArgument("get_user_details", "a bounded Viber subscriber ID is required")
	}
	var response userDetailsResponse
	if err := c.request(ctx, "/pa/get_user_details", map[string]string{"id": userID}, &response, options...); err != nil {
		return nil, err
	}
	if response.User.ID != userID || strings.TrimSpace(response.User.Name) == "" ||
		(response.User.Avatar != "" && !validRemoteURL(response.User.Avatar, 2000)) {
		return nil, platformError("get_user_details", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return &response.User, nil
}

// GetOnline retrieves presence for at most 100 subscribed users.
func (c *Client) GetOnline(ctx context.Context, userIDs []string, options ...socialhub.CallOption) ([]OnlineStatus, error) {
	ids, err := validateRecipients(userIDs, 100, "get_online")
	if err != nil {
		return nil, err
	}
	var response onlineResponse
	if err := c.request(ctx, "/pa/get_online", map[string]any{"ids": ids}, &response, options...); err != nil {
		return nil, err
	}
	if len(response.Users) > len(ids) {
		return nil, platformError("get_online", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	requested := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		requested[id] = struct{}{}
	}
	for _, status := range response.Users {
		if _, ok := requested[status.ID]; !ok || status.State < Online || status.State > OnlineUnavailable || status.LastOnlineMillis < 0 {
			return nil, platformError("get_online", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
		}
	}
	return response.Users, nil
}

func (c *Client) GetUser(ctx context.Context, userID string, options ...socialhub.CallOption) (*socialhub.User, error) {
	if userID == "me" {
		account, err := c.GetAccountInfo(ctx, options...)
		if err != nil {
			return nil, err
		}
		return c.mapAccount(account), nil
	}
	user, err := c.GetUserDetails(ctx, userID, options...)
	if err != nil {
		return nil, err
	}
	return c.mapUser(user), nil
}

func (c *Client) GetPost(context.Context, string, ...socialhub.CallOption) (*socialhub.Post, error) {
	return nil, unsupported("get_post", "Viber Bot API does not expose social posts")
}

func (c *Client) ListPosts(context.Context, socialhub.ListPostsRequest, ...socialhub.CallOption) (socialhub.Page[socialhub.Post], error) {
	return socialhub.Page[socialhub.Post]{}, unsupported("list_posts", "Viber Bot API does not expose timelines or message history")
}

func (c *Client) ListComments(context.Context, socialhub.ListCommentsRequest, ...socialhub.CallOption) (socialhub.Page[socialhub.Comment], error) {
	return socialhub.Page[socialhub.Comment]{}, unsupported("list_comments", "Viber Bot API does not expose comments")
}

func (c *Client) mapAccount(account *AccountInfo) *socialhub.User {
	accountType := "bot"
	name, username := account.Name, account.URI
	var avatarURL, profileURL *string
	if account.Icon != "" {
		avatarURL = stringPointer(account.Icon)
	}
	if account.URI != "" {
		profile := "viber://pa?chatURI=" + url.QueryEscape(account.URI)
		profileURL = &profile
	}
	extension, _ := json.Marshal(struct {
		Category         string    `json:"category,omitempty"`
		Subcategory      string    `json:"subcategory,omitempty"`
		Country          string    `json:"country,omitempty"`
		Background       string    `json:"background,omitempty"`
		Webhook          string    `json:"webhook,omitempty"`
		EventTypes       []string  `json:"event_types,omitempty"`
		SubscribersCount int64     `json:"subscribers_count,omitempty"`
		Location         *Location `json:"location,omitempty"`
	}{
		Category: account.Category, Subcategory: account.Subcategory, Country: account.Country,
		Background: account.Background, Webhook: account.Webhook, EventTypes: account.EventTypes,
		SubscribersCount: account.SubscribersCount, Location: account.Location,
	})
	return &socialhub.User{
		Platform: "viber", AccountID: c.accountID, ID: account.ID, Username: optionalStringPointer(username),
		DisplayName: &name, AvatarURL: avatarURL, ProfileURL: profileURL, AccountType: &accountType,
		Extensions: map[string]json.RawMessage{"viber.account": extension},
	}
}

func (c *Client) mapUser(user *UserDetails) *socialhub.User {
	accountType, name := "subscriber", user.Name
	var avatarURL *string
	if user.Avatar != "" {
		avatarURL = stringPointer(user.Avatar)
	}
	extension, _ := json.Marshal(struct {
		Country         string `json:"country,omitempty"`
		Language        string `json:"language,omitempty"`
		PrimaryDeviceOS string `json:"primary_device_os,omitempty"`
		APIVersion      int    `json:"api_version,omitempty"`
		ViberVersion    string `json:"viber_version,omitempty"`
		MCC             int    `json:"mcc,omitempty"`
		MNC             int    `json:"mnc,omitempty"`
		DeviceType      string `json:"device_type,omitempty"`
	}{
		Country: user.Country, Language: user.Language, PrimaryDeviceOS: user.PrimaryDeviceOS,
		APIVersion: user.APIVersion, ViberVersion: user.ViberVersion, MCC: user.MCC, MNC: user.MNC, DeviceType: user.DeviceType,
	})
	return &socialhub.User{
		Platform: "viber", AccountID: c.accountID, ID: user.ID, DisplayName: &name,
		AvatarURL: avatarURL, AccountType: &accountType, Extensions: map[string]json.RawMessage{"viber.user": extension},
	}
}
