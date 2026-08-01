package kakao

import (
	"context"
	"net/http"
	"net/url"
	"strconv"

	"social-hub/pkg/socialhub"
)

type friendResponse struct {
	Elements []struct {
		ID                    int64  `json:"id"`
		UUID                  string `json:"uuid"`
		ProfileNickname       string `json:"profile_nickname"`
		ProfileThumbnailImage string `json:"profile_thumbnail_image"`
		Favorite              bool   `json:"favorite"`
	} `json:"elements"`
	TotalCount    int    `json:"total_count"`
	BeforeURL     string `json:"before_url"`
	AfterURL      string `json:"after_url"`
	FavoriteCount int    `json:"favorite_count"`
}

func (c *Client) ListFriends(ctx context.Context, input ListFriendsRequest, options ...socialhub.CallOption) (FriendPage, error) {
	if err := c.requireFriendApproval("list_friends"); err != nil {
		return FriendPage{}, err
	}
	if input.Offset < 0 {
		return FriendPage{}, invalidArgument("list_friends", "offset must be non-negative")
	}
	limit := input.Limit
	if limit == 0 {
		limit = 10
	}
	if limit < 1 || limit > 100 {
		return FriendPage{}, invalidArgument("list_friends", "limit must be between 1 and 100")
	}
	order := input.Order
	if order == "" {
		order = FriendOrderAscending
	}
	if order != FriendOrderAscending && order != FriendOrderDescending {
		return FriendPage{}, invalidArgument("list_friends", "order must be asc or desc")
	}
	sort := input.Sort
	if sort == "" {
		sort = FriendSortFavorite
	}
	if sort != FriendSortFavorite && sort != FriendSortNickname {
		return FriendPage{}, invalidArgument("list_friends", "sort must be favorite or nickname")
	}
	query := url.Values{
		"offset": {strconv.Itoa(input.Offset)}, "limit": {strconv.Itoa(limit)},
		"order": {string(order)}, "friend_order": {string(sort)},
	}
	var response friendResponse
	if err := c.api.JSON(ctx, http.MethodGet, "/v1/api/talk/friends", query, nil, &response, options...); err != nil {
		return FriendPage{}, err
	}
	if response.TotalCount < 0 || response.FavoriteCount < 0 || response.FavoriteCount > response.TotalCount {
		return FriendPage{}, platformError("list_friends", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	items := make([]Friend, 0, len(response.Elements))
	for _, wire := range response.Elements {
		if wire.ID <= 0 || !validBoundedString(wire.UUID, 512) || !validOptionalString(wire.ProfileNickname, 2048) ||
			(wire.ProfileThumbnailImage != "" && !validHTTPURL(wire.ProfileThumbnailImage)) {
			return FriendPage{}, platformError("list_friends", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
		}
		items = append(items, Friend{
			ID: strconv.FormatInt(wire.ID, 10), UUID: wire.UUID, Nickname: wire.ProfileNickname,
			ThumbnailURL: wire.ProfileThumbnailImage, Favorite: wire.Favorite,
		})
	}
	page := FriendPage{
		Items: items, TotalCount: response.TotalCount, FavoriteCount: response.FavoriteCount,
		HasMore: response.AfterURL != "",
	}
	if page.HasMore {
		next := input.Offset + len(response.Elements)
		if next <= input.Offset {
			return FriendPage{}, platformError("list_friends", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
		}
		page.NextOffset = &next
	}
	return page, nil
}
