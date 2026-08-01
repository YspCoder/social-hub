package kakao

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"social-hub/pkg/socialhub"
)

type userResponse struct {
	ID           int64  `json:"id"`
	ConnectedAt  string `json:"connected_at"`
	SynchedAt    string `json:"synched_at"`
	HasSignedUp  *bool  `json:"has_signed_up"`
	KakaoAccount struct {
		ProfileNicknameNeedsAgreement bool   `json:"profile_nickname_needs_agreement"`
		ProfileImageNeedsAgreement    bool   `json:"profile_image_needs_agreement"`
		EmailNeedsAgreement           bool   `json:"email_needs_agreement"`
		IsEmailValid                  bool   `json:"is_email_valid"`
		IsEmailVerified               bool   `json:"is_email_verified"`
		Email                         string `json:"email"`
		Profile                       *struct {
			Nickname          string `json:"nickname"`
			ThumbnailImageURL string `json:"thumbnail_image_url"`
			ProfileImageURL   string `json:"profile_image_url"`
			IsDefaultNickname bool   `json:"is_default_nickname"`
		} `json:"profile"`
	} `json:"kakao_account"`
	ForPartner struct {
		UUID string `json:"uuid"`
	} `json:"for_partner"`
}

func (c *Client) Me(ctx context.Context, options ...socialhub.CallOption) (*socialhub.User, error) {
	var response userResponse
	query := url.Values{"secure_resource": {"true"}}
	if err := c.api.JSON(ctx, http.MethodGet, "/v2/user/me", query, nil, &response, options...); err != nil {
		return nil, err
	}
	if response.ID <= 0 || strconv.FormatInt(response.ID, 10) != c.userID {
		return nil, platformError("get_user", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	if !validOptionalString(response.ConnectedAt, 128) || !validOptionalString(response.SynchedAt, 128) ||
		!validOptionalString(response.KakaoAccount.Email, 512) ||
		(response.ForPartner.UUID != "" && !validBoundedString(response.ForPartner.UUID, 512)) {
		return nil, platformError("get_user", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	if response.KakaoAccount.Profile != nil {
		if !validOptionalString(response.KakaoAccount.Profile.Nickname, 2048) {
			return nil, platformError("get_user", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
		}
		for _, imageURL := range []string{response.KakaoAccount.Profile.ProfileImageURL, response.KakaoAccount.Profile.ThumbnailImageURL} {
			if imageURL != "" && !validHTTPURL(imageURL) {
				return nil, platformError("get_user", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
			}
		}
	}
	return c.mapUser(response), nil
}

func (c *Client) GetUser(ctx context.Context, userID string, options ...socialhub.CallOption) (*socialhub.User, error) {
	if userID != "me" && userID != c.userID {
		return nil, unsupported("get_user", "a user access token can retrieve only its authorized Kakao Login user")
	}
	return c.Me(ctx, options...)
}

func (c *Client) GetPost(context.Context, string, ...socialhub.CallOption) (*socialhub.Post, error) {
	return nil, unsupported("get_post", "Kakao Login and Talk Message APIs do not expose social posts")
}

func (c *Client) ListPosts(context.Context, socialhub.ListPostsRequest, ...socialhub.CallOption) (socialhub.Page[socialhub.Post], error) {
	return socialhub.Page[socialhub.Post]{}, unsupported("list_posts", "Kakao Login and Talk Message APIs do not expose timelines")
}

func (c *Client) ListComments(context.Context, socialhub.ListCommentsRequest, ...socialhub.CallOption) (socialhub.Page[socialhub.Comment], error) {
	return socialhub.Page[socialhub.Comment]{}, unsupported("list_comments", "Kakao Login and Talk Message APIs do not expose comments")
}

func (c *Client) mapUser(response userResponse) *socialhub.User {
	var displayName, avatarURL *string
	if profile := response.KakaoAccount.Profile; profile != nil {
		if strings.TrimSpace(profile.Nickname) != "" {
			value := profile.Nickname
			displayName = &value
		}
		if profile.ProfileImageURL != "" {
			value := profile.ProfileImageURL
			avatarURL = &value
		}
	}
	accountType := "kakao_login_user"
	extension, _ := json.Marshal(struct {
		ConnectedAt                   string `json:"connected_at,omitempty"`
		SynchedAt                     string `json:"synched_at,omitempty"`
		HasSignedUp                   *bool  `json:"has_signed_up,omitempty"`
		FriendUUID                    string `json:"friend_uuid,omitempty"`
		Email                         string `json:"email,omitempty"`
		EmailNeedsAgreement           bool   `json:"email_needs_agreement,omitempty"`
		IsEmailValid                  bool   `json:"is_email_valid,omitempty"`
		IsEmailVerified               bool   `json:"is_email_verified,omitempty"`
		ProfileNicknameNeedsAgreement bool   `json:"profile_nickname_needs_agreement,omitempty"`
		ProfileImageNeedsAgreement    bool   `json:"profile_image_needs_agreement,omitempty"`
		ProfileThumbnailImageURL      string `json:"profile_thumbnail_image_url,omitempty"`
		ProfileUsesDefaultNickname    bool   `json:"profile_uses_default_nickname,omitempty"`
	}{
		ConnectedAt: response.ConnectedAt, SynchedAt: response.SynchedAt, HasSignedUp: response.HasSignedUp,
		FriendUUID: response.ForPartner.UUID, Email: response.KakaoAccount.Email,
		EmailNeedsAgreement: response.KakaoAccount.EmailNeedsAgreement,
		IsEmailValid:        response.KakaoAccount.IsEmailValid, IsEmailVerified: response.KakaoAccount.IsEmailVerified,
		ProfileNicknameNeedsAgreement: response.KakaoAccount.ProfileNicknameNeedsAgreement,
		ProfileImageNeedsAgreement:    response.KakaoAccount.ProfileImageNeedsAgreement,
		ProfileThumbnailImageURL:      profileThumbnail(response), ProfileUsesDefaultNickname: profileDefaultNickname(response),
	})
	return &socialhub.User{
		Platform: "kakao", AccountID: c.accountID, ID: strconv.FormatInt(response.ID, 10), DisplayName: displayName,
		AvatarURL: avatarURL, AccountType: &accountType, Extensions: map[string]json.RawMessage{"kakao.login": extension},
	}
}

func profileThumbnail(response userResponse) string {
	if response.KakaoAccount.Profile == nil {
		return ""
	}
	return response.KakaoAccount.Profile.ThumbnailImageURL
}

func profileDefaultNickname(response userResponse) bool {
	return response.KakaoAccount.Profile != nil && response.KakaoAccount.Profile.IsDefaultNickname
}
