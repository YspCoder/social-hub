package kick

import (
	"context"
	"net/http"
	"net/url"
	"strings"

	"social-hub/pkg/socialhub"
)

func (client *Client) ListUsers(ctx context.Context, userIDs []string, options ...socialhub.CallOption) ([]User, error) {
	if client.tokenType == "user" {
		if err := client.requireScope("list_users", "user:read"); err != nil {
			return nil, err
		}
	}
	query := make(url.Values)
	if err := addPositiveIDs(query, "id", userIDs, 0); err != nil {
		return nil, err
	}
	var response responseEnvelope[[]User]
	if err := client.request(ctx, http.MethodGet, "/public/v1/users", query, nil, &response, options...); err != nil {
		return nil, err
	}
	return response.Data, nil
}

func (client *Client) ListChannels(ctx context.Context, input ChannelListRequest, options ...socialhub.CallOption) ([]Channel, error) {
	if len(input.BroadcasterUserIDs) == 0 && len(input.Slugs) == 0 {
		if client.broadcasterUserID != "" {
			input.BroadcasterUserIDs = []string{client.broadcasterUserID}
		} else if client.channelSlug != "" {
			input.Slugs = []string{client.channelSlug}
		}
	}
	if len(input.BroadcasterUserIDs) != 0 && len(input.Slugs) != 0 {
		return nil, invalidArgument("list_channels", "broadcaster_user_ids and slugs cannot be mixed")
	}
	if len(input.BroadcasterUserIDs) > 50 || len(input.Slugs) > 50 {
		return nil, invalidArgument("list_channels", "at most 50 broadcaster IDs or slugs are allowed")
	}
	if client.tokenType == "user" {
		if err := client.requireScope("list_channels", "channel:read"); err != nil {
			return nil, err
		}
	}
	query := make(url.Values)
	if err := addPositiveIDs(query, "broadcaster_user_id", input.BroadcasterUserIDs, 50); err != nil {
		return nil, err
	}
	for _, slug := range input.Slugs {
		if !validSlug(slug) {
			return nil, invalidArgument("list_channels", "slugs must be 1-25 safe characters")
		}
		query.Add("slug", slug)
	}
	var response responseEnvelope[[]Channel]
	if err := client.request(ctx, http.MethodGet, "/public/v1/channels", query, nil, &response, options...); err != nil {
		return nil, err
	}
	return response.Data, nil
}

func (client *Client) UpdateChannel(ctx context.Context, input UpdateChannelRequest, options ...socialhub.CallOption) error {
	if err := client.requireUserToken("update_channel"); err != nil {
		return err
	}
	if err := client.requireScope("update_channel", "channel:write"); err != nil {
		return err
	}
	if input.StreamTitle == nil && input.CategoryID == nil && input.CustomTags == nil {
		return invalidArgument("update_channel", "at least one channel field is required")
	}
	if input.StreamTitle != nil && strings.TrimSpace(*input.StreamTitle) == "" {
		return invalidArgument("update_channel", "stream title must not be empty")
	}
	if input.CategoryID != nil && *input.CategoryID <= 0 {
		return invalidArgument("update_channel", "category ID must be positive")
	}
	if input.CustomTags != nil {
		if len(*input.CustomTags) > 10 {
			return invalidArgument("update_channel", "custom tags must not exceed 10 items")
		}
		for _, tag := range *input.CustomTags {
			if !validFilterValue(tag, 100, true) {
				return invalidArgument("update_channel", "custom tags must be non-empty bounded values")
			}
		}
	}
	body := struct {
		StreamTitle *string   `json:"stream_title,omitempty"`
		CategoryID  *int64    `json:"category_id,omitempty"`
		CustomTags  *[]string `json:"custom_tags,omitempty"`
	}{StreamTitle: input.StreamTitle, CategoryID: input.CategoryID, CustomTags: input.CustomTags}
	return client.request(ctx, http.MethodPatch, "/public/v1/channels", nil, body, nil, options...)
}

var _ UserWorkflow = (*Client)(nil)
var _ ChannelWorkflow = (*Client)(nil)
