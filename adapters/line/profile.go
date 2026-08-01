package line

import (
	"context"
	"net/http"
	"net/url"
	"strings"

	"social-hub/pkg/socialhub"
)

func (c *Client) GetProfile(ctx context.Context, userID string, options ...socialhub.CallOption) (*Profile, error) {
	userID = strings.TrimSpace(userID)
	if !validLINEID(userID, 'U') {
		return nil, invalidArgument("get_profile", "LINE user ID is required")
	}
	return c.profile(ctx, "/v2/bot/profile/"+url.PathEscape(userID), userID, "get_profile", options...)
}

func (c *Client) GetGroupMemberProfile(ctx context.Context, groupID, userID string, options ...socialhub.CallOption) (*Profile, error) {
	groupID, userID = strings.TrimSpace(groupID), strings.TrimSpace(userID)
	if !validLINEID(groupID, 'C') || !validLINEID(userID, 'U') {
		return nil, invalidArgument("get_group_member_profile", "LINE group and user IDs are required")
	}
	path := "/v2/bot/group/" + url.PathEscape(groupID) + "/member/" + url.PathEscape(userID)
	return c.profile(ctx, path, userID, "get_group_member_profile", options...)
}

func (c *Client) GetRoomMemberProfile(ctx context.Context, roomID, userID string, options ...socialhub.CallOption) (*Profile, error) {
	roomID, userID = strings.TrimSpace(roomID), strings.TrimSpace(userID)
	if !validLINEID(roomID, 'R') || !validLINEID(userID, 'U') {
		return nil, invalidArgument("get_room_member_profile", "LINE room and user IDs are required")
	}
	path := "/v2/bot/room/" + url.PathEscape(roomID) + "/member/" + url.PathEscape(userID)
	return c.profile(ctx, path, userID, "get_room_member_profile", options...)
}

func (c *Client) profile(ctx context.Context, path, expectedUserID, operation string, options ...socialhub.CallOption) (*Profile, error) {
	var profile Profile
	if err := c.request(ctx, c.api, http.MethodGet, path, nil, nil, &profile, false, options...); err != nil {
		return nil, err
	}
	if profile.UserID != expectedUserID || strings.TrimSpace(profile.DisplayName) == "" {
		return nil, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return &profile, nil
}
