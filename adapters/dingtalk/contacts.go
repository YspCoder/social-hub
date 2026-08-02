package dingtalk

import (
	"context"
	"net/url"

	"social-hub/pkg/socialhub"
)

type userDetailResponse struct {
	apiError
	UserDetail
}

func (c *Client) GetUserByUnionID(ctx context.Context, unionID string, options ...socialhub.CallOption) (*UserDetail, error) {
	if !validOpaque(unionID, 256) {
		return nil, invalidArgument("get_user_by_union_id", "UnionID must contain 1 to 256 bytes")
	}
	var response userDetailResponse
	path := "/v1.0/contact/users/" + url.PathEscape(unionID)
	if err := c.get(ctx, "get_user_by_union_id", path, &response, options...); err != nil {
		return nil, err
	}
	if err := c.responseError(ctx, "get_user_by_union_id", response.apiError); err != nil {
		return nil, err
	}
	if !validOpaque(response.UnionID, 256) {
		return nil, platformError("get_user_by_union_id", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	result := response.UserDetail
	return &result, nil
}

func (c *Client) GetUser(ctx context.Context, unionID string, options ...socialhub.CallOption) (*socialhub.User, error) {
	detail, err := c.GetUserByUnionID(ctx, unionID, options...)
	if err != nil {
		return nil, err
	}
	accountType := "dingtalk_internal_user"
	return &socialhub.User{
		Platform: "dingtalk", AccountID: c.accountID, ID: detail.UnionID,
		Username: stringPointer(detail.OpenID), DisplayName: stringPointer(detail.Nick),
		AvatarURL: stringPointer(detail.AvatarURL), AccountType: &accountType,
		Extensions: jsonExtension("dingtalk.contact_user", detail),
	}, nil
}

func (c *Client) GetPost(context.Context, string, ...socialhub.CallOption) (*socialhub.Post, error) {
	return nil, unsupported("get_post", "DingTalk internal applications do not expose social posts")
}

func (c *Client) ListPosts(context.Context, socialhub.ListPostsRequest, ...socialhub.CallOption) (socialhub.Page[socialhub.Post], error) {
	return socialhub.Page[socialhub.Post]{}, unsupported("list_posts", "DingTalk internal applications do not expose a social feed")
}

func (c *Client) ListComments(context.Context, socialhub.ListCommentsRequest, ...socialhub.CallOption) (socialhub.Page[socialhub.Comment], error) {
	return socialhub.Page[socialhub.Comment]{}, unsupported("list_comments", "DingTalk application bot messages do not expose social comments")
}
