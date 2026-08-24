package messenger

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"

	"social-hub/pkg/socialhub"
)

// GetUserProfile reads the basic profile fields for one Page-scoped ID.
func (c *Client) GetUserProfile(ctx context.Context, psid string, options ...socialhub.CallOption) (*UserProfile, error) {
	psid = strings.TrimSpace(psid)
	if !validNumericID(psid) {
		return nil, invalidArgument("get_user_profile", "PSID must be a decimal Page-scoped ID")
	}
	query := url.Values{"fields": {"id,name,first_name,last_name,profile_pic"}}
	var profile UserProfile
	if err := c.api.JSON(ctx, http.MethodGet, "/"+url.PathEscape(psid), query, nil, &profile, options...); err != nil {
		return nil, err
	}
	if profile.ID == "" {
		return nil, approvalRequired("get_user_profile", "Meta returned an empty profile; request Business Asset User Profile Access and ensure the user authorized profile access")
	}
	if profile.ID != psid {
		return nil, platformError("get_user_profile", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return &profile, nil
}

func (c *Client) GetUser(ctx context.Context, psid string, options ...socialhub.CallOption) (*socialhub.User, error) {
	profile, err := c.GetUserProfile(ctx, psid, options...)
	if err != nil {
		return nil, err
	}
	displayName := strings.TrimSpace(profile.Name)
	if displayName == "" {
		displayName = strings.TrimSpace(profile.FirstName + " " + profile.LastName)
	}
	extension, _ := json.Marshal(profile)
	return &socialhub.User{
		Platform: "facebook", AccountID: c.accountID, ID: profile.ID,
		DisplayName: stringPointer(displayName), AvatarURL: stringPointer(profile.ProfilePic),
		Extensions: map[string]json.RawMessage{"facebook.messenger_profile": extension},
	}, nil
}

func (c *Client) GetPost(context.Context, string, ...socialhub.CallOption) (*socialhub.Post, error) {
	return nil, unsupported("get_post", "Messenger Platform does not expose Page posts through this product adapter")
}

func (c *Client) ListPosts(context.Context, socialhub.ListPostsRequest, ...socialhub.CallOption) (socialhub.Page[socialhub.Post], error) {
	return socialhub.Page[socialhub.Post]{}, unsupported("list_posts", "Messenger Platform does not expose a post feed")
}

func (c *Client) ListComments(context.Context, socialhub.ListCommentsRequest, ...socialhub.CallOption) (socialhub.Page[socialhub.Comment], error) {
	return socialhub.Page[socialhub.Comment]{}, unsupported("list_comments", "Messenger Platform does not expose Page comments through this product adapter")
}
