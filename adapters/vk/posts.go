package vk

import (
	"context"
	"net/url"
	"strconv"
	"strings"

	"social-hub/pkg/socialhub"
)

func (c *Client) Publish(ctx context.Context, input socialhub.CreatePostRequest, options ...socialhub.CallOption) (*socialhub.Post, error) {
	if input.ReplyToID != nil {
		return nil, unsupported("publish", "VK wall replies are comments, not reply posts")
	}
	if input.QuotePostID != nil {
		if len(input.MediaIDs) > 0 || input.Visibility != nil || strings.TrimSpace(*input.QuotePostID) == "" {
			return nil, invalidArgument("publish", "a VK repost requires a target ID and cannot include common media or visibility")
		}
		message := ""
		if input.Text != nil {
			message = *input.Text
		}
		return c.Repost(ctx, RepostRequest{Object: *input.QuotePostID, Message: message}, options...)
	}
	request := WallPostRequest{OwnerID: c.ownerID, Attachments: input.MediaIDs}
	if input.Text != nil {
		request.Message = *input.Text
	}
	if input.Visibility != nil {
		switch *input.Visibility {
		case "public":
		case "friends":
			request.FriendsOnly = true
		default:
			return nil, invalidArgument("publish", "visibility must be public or friends")
		}
	}
	if c.ownerID < 0 {
		request.FromGroup = true
	}
	return c.CreateWallPost(ctx, request, options...)
}

func (c *Client) CreateWallPost(ctx context.Context, input WallPostRequest, options ...socialhub.CallOption) (*socialhub.Post, error) {
	if c.tokenKind != TokenUser {
		return nil, tokenPermission("wall.post", "VK wall publishing requires a user access token")
	}
	ownerID := input.OwnerID
	if ownerID == 0 {
		ownerID = c.ownerID
	}
	if strings.TrimSpace(input.Message) == "" && len(input.Attachments) == 0 {
		return nil, invalidArgument("wall.post", "message or at least one attachment is required")
	}
	if ownerID == 0 || len(input.Attachments) > 10 {
		return nil, invalidArgument("wall.post", "a non-zero owner and at most ten attachments are required")
	}
	for _, attachment := range input.Attachments {
		if !validAttachment(attachment) {
			return nil, invalidArgument("wall.post", "attachments must use VK attachment identifiers")
		}
	}
	if input.FriendsOnly && ownerID < 0 {
		return nil, invalidArgument("wall.post", "friends-only visibility is unavailable on community walls")
	}
	if input.FromGroup && ownerID > 0 {
		return nil, invalidArgument("wall.post", "from_group is available only on community walls")
	}
	values := url.Values{"owner_id": {strconv.FormatInt(ownerID, 10)}}
	if input.Message != "" {
		values.Set("message", input.Message)
	}
	if len(input.Attachments) > 0 {
		values.Set("attachments", strings.Join(input.Attachments, ","))
	}
	setBool(values, "from_group", input.FromGroup)
	setBool(values, "friends_only", input.FriendsOnly)
	setBool(values, "signed", input.Signed)
	setBool(values, "close_comments", input.CloseComments)
	setBool(values, "mute_notifications", input.MuteNotifications)
	postType, date := "post", c.clock.Now()
	if input.PublishAt != nil {
		if !input.PublishAt.After(c.clock.Now()) {
			return nil, invalidArgument("wall.post", "publish_at must be in the future")
		}
		date, postType = input.PublishAt.UTC(), "postpone"
		values.Set("publish_date", strconv.FormatInt(date.Unix(), 10))
	}
	resolved, err := socialhub.ResolveCallOptions(options...)
	if err != nil {
		return nil, err
	}
	if resolved.IdempotencyKey != "" {
		if !validOpaque(resolved.IdempotencyKey, 64) {
			return nil, invalidArgument("wall.post", "idempotency key must be a bounded opaque VK guid")
		}
		values.Set("guid", resolved.IdempotencyKey)
	}
	var response struct {
		PostID int64 `json:"post_id"`
	}
	if err := c.method(ctx, "wall.post", values, &response, options...); err != nil {
		return nil, err
	}
	if response.PostID <= 0 {
		return nil, platformError("wall.post", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	post := mapPost(c.accountID, wirePost{
		ID: response.PostID, OwnerID: ownerID, FromID: ownerID, Date: date.Unix(), Text: input.Message,
		FriendsOnly: boolInt(input.FriendsOnly), PostType: postType,
	}, c.clock.Now())
	return post, nil
}

func (c *Client) Repost(ctx context.Context, input RepostRequest, options ...socialhub.CallOption) (*socialhub.Post, error) {
	if c.tokenKind != TokenUser {
		return nil, tokenPermission("wall.repost", "VK reposting requires a user access token")
	}
	sourceOwner, sourcePost, err := parseCompositeID(input.Object, "wall.repost")
	if err != nil {
		return nil, err
	}
	destination := input.DestinationOwnerID
	if destination == 0 {
		destination = c.ownerID
	}
	if destination == 0 {
		return nil, invalidArgument("wall.repost", "destination owner is required")
	}
	if destination > 0 && destination != c.ownerID {
		return nil, invalidArgument("wall.repost", "a personal repost destination must match the configured owner_id")
	}
	values := url.Values{"object": {"wall" + compositeID(sourceOwner, sourcePost)}}
	if input.Message != "" {
		values.Set("message", input.Message)
	}
	if destination < 0 {
		values.Set("group_id", strconv.FormatInt(-destination, 10))
	}
	var response struct {
		Success int   `json:"success"`
		PostID  int64 `json:"post_id"`
	}
	if err := c.method(ctx, "wall.repost", values, &response, options...); err != nil {
		return nil, err
	}
	if response.Success != 1 || response.PostID <= 0 {
		return nil, platformError("wall.repost", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	post := mapPost(c.accountID, wirePost{
		ID: response.PostID, OwnerID: destination, FromID: destination, Date: c.clock.Now().Unix(), Text: input.Message,
		PostType: "post", CopyHistory: []wirePost{{ID: sourcePost, OwnerID: sourceOwner}},
	}, c.clock.Now())
	return post, nil
}

func (c *Client) PublishStatus(ctx context.Context, postID string, options ...socialhub.CallOption) (*socialhub.PublishStatus, error) {
	post, err := c.GetPost(ctx, postID, options...)
	if err != nil {
		return nil, err
	}
	if post.Status == nil {
		return nil, platformError("publish_status", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return post.Status, nil
}

func (c *Client) DeletePost(ctx context.Context, postID string, options ...socialhub.CallOption) error {
	if c.tokenKind != TokenUser {
		return tokenPermission("wall.delete", "VK wall deletion requires a user access token")
	}
	ownerID, itemID, err := parseCompositeID(postID, "wall.delete")
	if err != nil {
		return err
	}
	var response int
	if err := c.method(ctx, "wall.delete", url.Values{
		"owner_id": {strconv.FormatInt(ownerID, 10)}, "post_id": {strconv.FormatInt(itemID, 10)},
	}, &response, options...); err != nil {
		return err
	}
	if response != 1 {
		return platformError("wall.delete", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return nil
}

func setBool(values url.Values, key string, value bool) {
	if value {
		values.Set(key, "1")
	}
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func validAttachment(value string) bool {
	value = strings.TrimSpace(value)
	prefixes := []string{"photo", "video", "audio", "doc", "poll", "page", "album"}
	prefix := ""
	for _, candidate := range prefixes {
		if strings.HasPrefix(value, candidate) {
			prefix = candidate
			break
		}
	}
	if prefix == "" {
		return false
	}
	parts := strings.Split(strings.TrimPrefix(value, prefix), "_")
	if len(parts) < 2 || len(parts) > 3 {
		return false
	}
	ownerID, ownerErr := strconv.ParseInt(parts[0], 10, 64)
	itemID, itemErr := strconv.ParseInt(parts[1], 10, 64)
	if ownerErr != nil || itemErr != nil || ownerID == 0 || itemID <= 0 {
		return false
	}
	return len(parts) == 2 || validOpaque(parts[2], 256)
}

func validOpaque(value string, maximum int) bool {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > maximum {
		return false
	}
	for _, character := range value {
		if character <= 0x20 || character == 0x7f {
			return false
		}
	}
	return true
}
