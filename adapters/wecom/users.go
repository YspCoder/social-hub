package wecom

import (
	"context"
	"net/http"
	"net/url"

	"social-hub/pkg/socialhub"
)

type memberResponse struct {
	APIResponse
	UserID           string  `json:"userid"`
	Name             string  `json:"name"`
	Mobile           string  `json:"mobile"`
	Department       []int64 `json:"department"`
	Order            []int64 `json:"order"`
	Position         string  `json:"position"`
	Gender           string  `json:"gender"`
	Email            string  `json:"email"`
	IsLeaderInDept   []int64 `json:"is_leader_in_dept"`
	Avatar           string  `json:"avatar"`
	ThumbAvatar      string  `json:"thumb_avatar"`
	Telephone        string  `json:"telephone"`
	Alias            string  `json:"alias"`
	Status           int     `json:"status"`
	QRCode           string  `json:"qr_code"`
	ExternalPosition string  `json:"external_position"`
	Address          string  `json:"address"`
	MainDepartment   int64   `json:"main_department"`
}

func (c *Client) GetUser(ctx context.Context, userID string, options ...socialhub.CallOption) (*socialhub.User, error) {
	if !validBoundedValue(userID, 64, false) || userID == "@all" {
		return nil, invalidArgument("get_user", "userid must contain 1 to 64 bytes without separators")
	}
	var response memberResponse
	if err := c.api.JSON(ctx, http.MethodGet, "/cgi-bin/user/get", url.Values{"userid": {userID}}, nil, &response, options...); err != nil {
		return nil, err
	}
	if err := c.responseError(ctx, "get_user", response.APIResponse); err != nil {
		return nil, err
	}
	if response.UserID == "" {
		return nil, platformError("get_user", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	accountType := "wecom_member"
	return &socialhub.User{
		Platform: "wecom", AccountID: c.accountID, ID: response.UserID,
		Username: stringPointer(response.Alias), DisplayName: stringPointer(response.Name),
		AvatarURL: stringPointer(response.Avatar), AccountType: &accountType,
		Extensions: jsonExtension(response),
	}, nil
}

func (c *Client) GetPost(context.Context, string, ...socialhub.CallOption) (*socialhub.Post, error) {
	return nil, unsupported("get_post", "WeCom self-built applications do not expose social posts")
}

func (c *Client) ListPosts(context.Context, socialhub.ListPostsRequest, ...socialhub.CallOption) (socialhub.Page[socialhub.Post], error) {
	return socialhub.Page[socialhub.Post]{}, unsupported("list_posts", "WeCom self-built applications do not expose a social feed")
}

func (c *Client) ListComments(context.Context, socialhub.ListCommentsRequest, ...socialhub.CallOption) (socialhub.Page[socialhub.Comment], error) {
	return socialhub.Page[socialhub.Comment]{}, unsupported("list_comments", "WeCom self-built applications do not expose social comments")
}
