package dailymotion

import (
	"context"
	"net/http"
	"net/url"

	"social-hub/pkg/socialhub"
)

const profileFields = "profile_id,name,display_name,description,created_at,can_change_name,social_links,webhook"

func (c *Client) CurrentAccount(ctx context.Context, options ...socialhub.CallOption) (*Account, error) {
	if err := c.requireScopes("current_account", ScopeAccountRead); err != nil {
		return nil, err
	}
	var response Account
	if err := c.requestJSON(ctx, http.MethodGet, "/me", url.Values{"fields": {"user_id,username,profiles"}}, nil, &response, options...); err != nil {
		return nil, err
	}
	if !validResourceID(response.UserID) || response.Username == "" {
		return nil, platformError("current_account", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return &response, nil
}

func (c *Client) GetProfile(ctx context.Context, profileID string, options ...socialhub.CallOption) (*Profile, error) {
	if err := c.requireScopes("get_profile", ScopeProfileRead); err != nil {
		return nil, err
	}
	if !validResourceID(profileID) {
		return nil, invalidArgument("get_profile", "a valid profile ID is required")
	}
	var response Profile
	if err := c.requestJSON(ctx, http.MethodGet, "/profiles/"+escapedID(profileID), url.Values{"fields": {profileFields}}, nil, &response, options...); err != nil {
		return nil, err
	}
	if response.ProfileID != profileID {
		return nil, platformError("get_profile", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return &response, nil
}

func (c *Client) UpdateProfile(ctx context.Context, profileID string, input UpdateProfileRequest, options ...socialhub.CallOption) error {
	if err := c.requireScopes("update_profile", ScopeProfileManage); err != nil {
		return err
	}
	if !validResourceID(profileID) {
		return invalidArgument("update_profile", "a valid profile ID is required")
	}
	if err := validateProfileUpdate(input); err != nil {
		return err
	}
	body := struct {
		DisplayName *string          `json:"display_name,omitempty"`
		Description *string          `json:"description,omitempty"`
		SocialLinks *SocialLinks     `json:"social_links,omitempty"`
		Webhook     *WebhookSettings `json:"webhook,omitempty"`
	}{input.DisplayName, input.Description, input.SocialLinks, input.Webhook}
	return c.requestJSON(ctx, http.MethodPatch, "/profiles/"+escapedID(profileID), nil, body, nil, options...)
}

func (c *Client) GetUser(ctx context.Context, userID string, options ...socialhub.CallOption) (*socialhub.User, error) {
	profile, err := c.GetProfile(ctx, userID, options...)
	if err != nil {
		return nil, err
	}
	return c.mapProfile(*profile)
}

var _ ProfileWorkflow = (*Client)(nil)
