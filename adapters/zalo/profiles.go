package zalo

import (
	"context"
	"encoding/json"
	"net/url"
	"strings"

	"social-hub/pkg/socialhub"
)

func (c *Client) GetOA(ctx context.Context, options ...socialhub.CallOption) (*OAProfile, error) {
	profile, err := get[OAProfile](ctx, c, "/v2.0/oa/getoa", nil, "get_oa", options...)
	if err != nil {
		return nil, err
	}
	if !validNumericID(profile.ID) || strings.TrimSpace(profile.Name) == "" {
		return nil, platformError("get_oa", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	if c.oaID != "" && profile.ID != c.oaID {
		return nil, platformError("get_oa", socialhub.CodeConflict, socialhub.ClassUserAction, nil)
	}
	return &profile, nil
}

func (c *Client) GetUserProfile(ctx context.Context, userID string, options ...socialhub.CallOption) (*UserProfile, error) {
	userID = strings.TrimSpace(userID)
	if !validNumericID(userID) {
		return nil, invalidArgument("get_user_profile", "a decimal Zalo user ID is required")
	}
	data, _ := json.Marshal(map[string]string{"user_id": userID})
	profile, err := get[UserProfile](ctx, c, "/v3.0/oa/user/detail", url.Values{"data": {string(data)}}, "get_user_profile", options...)
	if err != nil {
		return nil, err
	}
	if profile.UserID != userID {
		return nil, platformError("get_user_profile", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return &profile, nil
}

func (c *Client) GetUser(ctx context.Context, userID string, options ...socialhub.CallOption) (*socialhub.User, error) {
	profile, err := c.GetUserProfile(ctx, userID, options...)
	if err != nil {
		return nil, err
	}
	extension, _ := json.Marshal(profile)
	result := &socialhub.User{
		Platform: "zalo", AccountID: c.accountID, ID: profile.UserID,
		Extensions: map[string]json.RawMessage{"zalo.user_profile": extension},
	}
	if profile.DisplayName != "" {
		result.DisplayName = &profile.DisplayName
	}
	if profile.Alias != "" {
		result.Username = &profile.Alias
	}
	if profile.Avatar != "" {
		result.AvatarURL = &profile.Avatar
	}
	return result, nil
}

func (c *Client) GetPost(context.Context, string, ...socialhub.CallOption) (*socialhub.Post, error) {
	return nil, unsupported("get_post", "Zalo Article API is not exposed through the common Fetcher")
}

func (c *Client) ListPosts(context.Context, socialhub.ListPostsRequest, ...socialhub.CallOption) (socialhub.Page[socialhub.Post], error) {
	return socialhub.Page[socialhub.Post]{}, unsupported("list_posts", "Zalo Article API is not exposed through the common Fetcher")
}

func (c *Client) ListComments(context.Context, socialhub.ListCommentsRequest, ...socialhub.CallOption) (socialhub.Page[socialhub.Comment], error) {
	return socialhub.Page[socialhub.Comment]{}, unsupported("list_comments", "Zalo OA OpenAPI does not expose article comments through this adapter")
}
