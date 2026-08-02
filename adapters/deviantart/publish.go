package deviantart

import (
	"context"
	"net/url"
	"strings"

	"social-hub/pkg/socialhub"
)

func (client *Client) PostStatus(ctx context.Context, input StatusPostRequest, options ...socialhub.CallOption) (*StatusPublishResponse, error) {
	if strings.TrimSpace(input.Body) == "" && strings.TrimSpace(input.ShareID) == "" && strings.TrimSpace(input.StashID) == "" {
		return nil, invalidArgument("post_status", "body, share_id, or stash_id is required")
	}
	if input.ShareID != "" && !validResourceID(input.ShareID) || input.ShareParentID != "" && !validResourceID(input.ShareParentID) ||
		input.StashID != "" && !validResourceID(input.StashID) || input.ShareParentID != "" && input.ShareID == "" {
		return nil, invalidArgument("post_status", "share or Sta.sh identifiers are invalid")
	}
	if err := client.requireScopes("post_status", "user.manage"); err != nil {
		return nil, err
	}
	values := url.Values{}
	if input.Body != "" {
		values.Set("body", input.Body)
	}
	if input.ShareID != "" {
		values.Set("id", input.ShareID)
	}
	if input.ShareParentID != "" {
		values.Set("parentid", input.ShareParentID)
	}
	if input.StashID != "" {
		values.Set("stashid", input.StashID)
	}
	var response StatusPublishResponse
	if err := client.form(ctx, "/user/statuses/post", values, &response, options...); err != nil {
		return nil, err
	}
	if !validResourceID(response.StatusID) {
		return nil, platformError("post_status", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return &response, nil
}

func (client *Client) Publish(ctx context.Context, input socialhub.CreatePostRequest, options ...socialhub.CallOption) (*socialhub.Post, error) {
	if err := input.Validate(); err != nil {
		return nil, platformError("publish", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	if len(input.MediaIDs) > 0 || input.ReplyToID != nil || input.QuotePostID != nil || input.Visibility != nil {
		return nil, unsupported("publish", "common DeviantArt publishing supports text-only Status posts; use typed Sta.sh or Status workflows for platform-specific content")
	}
	if input.Text == nil || strings.TrimSpace(*input.Text) == "" {
		return nil, invalidArgument("publish", "Status text is required")
	}
	response, err := client.PostStatus(ctx, StatusPostRequest{Body: *input.Text}, options...)
	if err != nil {
		return nil, err
	}
	now := client.clock.Now()
	text := *input.Text
	return &socialhub.Post{
		Platform: "deviantart", AccountID: client.accountID, ID: response.StatusID, Text: &text,
		AuthorID: stringPointer(client.userID), CreatedAt: &now,
		Status: &socialhub.PublishStatus{ID: response.StatusID, State: socialhub.PublishStatePublished, UpdatedAt: &now},
	}, nil
}

func (client *Client) PublishStatus(context.Context, string, ...socialhub.CallOption) (*socialhub.PublishStatus, error) {
	return nil, unsupported("publish_status", "DeviantArt does not expose a Status publication-state endpoint")
}

func (client *Client) DeletePost(context.Context, string, ...socialhub.CallOption) error {
	return unsupported("delete_post", "DeviantArt API v1 does not expose Status deletion")
}
