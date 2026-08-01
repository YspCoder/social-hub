package douyin

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"

	"social-hub/extensions/video"
	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

// Client implements the supported Douyin OpenAPI capability interfaces.
type Client struct {
	accountID     socialhub.AccountID
	clientKey     string
	openID        string
	transport     *transport.Client
	clock         socialhub.Clock
	webhookSecret string
	scopes        []string

	uploadMu sync.Mutex
	uploads  map[string]*uploadState
	videos   *VideoService
}

func (c *Client) Platform() socialhub.Platform { return "douyin" }
func (c *Client) Account() socialhub.AccountID { return c.accountID }

func (c *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	webhookSupported := c.webhookSecret != ""
	webhookReason := ""
	if !webhookSupported {
		webhookReason = "secret_ref is not configured"
	}
	return socialhub.Capabilities{
		socialhub.CapPublish: capabilityState(socialhub.CapPublish, true, c.scopes, []string{"video.create"}, "video publication is asynchronous and subject to review", "https://open.douyin.com/platform/resource/docs/openapi/video-management/douyin/create/create-video"),
		socialhub.CapFetch:   capabilityState(socialhub.CapFetch, true, c.scopes, []string{"user_info", "video.list", "video.data"}, "user and video visibility follows granted scopes", "https://open.douyin.com/platform/resource/docs/openapi/video-management/douyin/search-video/account-video-list"),
		socialhub.CapMedia:   capabilityState(socialhub.CapMedia, true, c.scopes, []string{"video.create"}, "direct and multipart video upload are supported", "https://open.douyin.com/platform/resource/docs/openapi/video-management/douyin/create/upload/"),
		socialhub.CapReact:   capabilityState(socialhub.CapReact, true, c.scopes, []string{"item.comment"}, "comment listing and replies are supported; like mutation is not public", "https://open.douyin.com/platform/resource/docs/openapi/interaction-management/comment-management-user/comment-list/"),
		socialhub.CapMessage: {Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "direct-message APIs are limited to separately approved enterprise products"},
		socialhub.CapWebhook: {Capability: socialhub.CapWebhook, Supported: webhookSupported, Approval: socialhub.ApprovalUnknown, Reason: webhookReason, DocURL: "https://open.douyin.com/platform/resource/docs/develop/webhooks/summarize/"},
	}, nil
}

func capabilityState(capability socialhub.Capability, supported bool, granted, required []string, reason, docURL string) socialhub.CapabilityState {
	approval := socialhub.ApprovalUnknown
	if len(granted) > 0 {
		approval = socialhub.ApprovalGranted
		for _, scope := range required {
			if !contains(granted, scope) {
				approval = socialhub.ApprovalRequired
				break
			}
		}
	}
	return socialhub.CapabilityState{Capability: capability, Supported: supported, Approval: approval, Scopes: required, Reason: reason, DocURL: docURL}
}

func (c *Client) Publisher() (socialhub.Publisher, bool)         { return c, true }
func (c *Client) Fetcher() (socialhub.Fetcher, bool)             { return c, true }
func (c *Client) MediaUploader() (socialhub.MediaUploader, bool) { return c, true }
func (c *Client) Reactor() (socialhub.Reactor, bool)             { return c, true }
func (c *Client) Messenger() (socialhub.Messenger, bool)         { return nil, false }
func (c *Client) WebhookHandler() (socialhub.WebhookHandler, bool) {
	if c.webhookSecret == "" {
		return nil, false
	}
	return c, true
}
func (c *Client) Close() error { return nil }

// VideoWorkflow returns the typed short-video workflow.
func (c *Client) VideoWorkflow() video.Workflow { return c.videos }

func (c *Client) Publish(ctx context.Context, input socialhub.CreatePostRequest, options ...socialhub.CallOption) (*socialhub.Post, error) {
	if err := c.requireScope("publish", "video.create"); err != nil {
		return nil, err
	}
	if len(input.MediaIDs) != 1 || input.MediaIDs[0] == "" {
		return nil, invalidArgument("publish", "exactly one uploaded video ID is required")
	}
	if input.ReplyToID != nil || input.QuotePostID != nil || input.Visibility != nil {
		return nil, &socialhub.Error{Code: socialhub.CodeUnsupported, Class: socialhub.ClassPermanent, Platform: "douyin", Product: "openapi", Op: "publish", PlatformMessage: "reply, quote, and visibility overrides are not supported by video.create"}
	}
	body := map[string]any{"video_id": input.MediaIDs[0]}
	if input.Text != nil && *input.Text != "" {
		body["text"] = *input.Text
	}
	var response createVideoResponse
	if err := c.transport.JSON(ctx, http.MethodPost, "/video/create/", c.openIDQuery(), body, &response, options...); err != nil {
		return nil, err
	}
	if err := responseError(response.Data.APIResponse, response.Extra, "publish", http.StatusOK, nil); err != nil {
		return nil, err
	}
	if response.Data.ItemID == "" {
		return nil, wrapError("publish", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	status := &socialhub.PublishStatus{ID: response.Data.ItemID, State: socialhub.PublishStatePending, Message: "submitted for Douyin review"}
	post := &socialhub.Post{Platform: "douyin", AccountID: c.accountID, ID: response.Data.ItemID, AuthorID: stringPointer(c.openID), Text: input.Text, Media: []socialhub.Media{{ID: input.MediaIDs[0], Type: socialhub.MediaTypeVideo, State: socialhub.MediaStateProcessing}}, Status: status}
	return post, nil
}

func (c *Client) PublishStatus(ctx context.Context, postID string, options ...socialhub.CallOption) (*socialhub.PublishStatus, error) {
	if postID == "" {
		return nil, invalidArgument("publish_status", "post ID is required")
	}
	post, err := c.GetPost(ctx, postID, options...)
	if err != nil {
		return nil, err
	}
	state := socialhub.PublishStatePublished
	message := ""
	if post.Status != nil {
		state = post.Status.State
		message = post.Status.Message
	}
	return &socialhub.PublishStatus{ID: postID, State: state, Message: message, UpdatedAt: post.CreatedAt}, nil
}

func (c *Client) DeletePost(ctx context.Context, postID string, options ...socialhub.CallOption) error {
	if postID == "" {
		return invalidArgument("delete_post", "post ID is required")
	}
	if err := c.requireScope("delete_post", "video.delete"); err != nil {
		return err
	}
	var response baseEnvelope
	if err := c.transport.JSON(ctx, http.MethodPost, "/video/delete/", c.openIDQuery(), map[string]string{"item_id": postID}, &response, options...); err != nil {
		return err
	}
	return responseError(response.Data, response.Extra, "delete_post", http.StatusOK, nil)
}

func (c *Client) GetUser(ctx context.Context, userID string, options ...socialhub.CallOption) (*socialhub.User, error) {
	if userID == "" || userID != c.openID {
		return nil, invalidArgument("get_user", "user ID must be the configured app-scoped open_id")
	}
	if err := c.requireScope("get_user", "user_info"); err != nil {
		return nil, err
	}
	var response userEnvelope
	if err := c.transport.JSON(ctx, http.MethodPost, "/oauth/userinfo/", c.openIDQuery(), map[string]string{}, &response, options...); err != nil {
		return nil, err
	}
	if err := responseError(response.Data.APIResponse, response.Extra, "get_user", http.StatusOK, nil); err != nil {
		return nil, err
	}
	return mapUser(c.accountID, response.Data), nil
}

func (c *Client) GetPost(ctx context.Context, postID string, options ...socialhub.CallOption) (*socialhub.Post, error) {
	if postID == "" {
		return nil, invalidArgument("get_post", "post ID is required")
	}
	if err := c.requireScope("get_post", "video.data"); err != nil {
		return nil, err
	}
	var response videoListEnvelope
	if err := c.transport.JSON(ctx, http.MethodPost, "/video/data/", c.openIDQuery(), map[string][]string{"item_ids": {postID}}, &response, options...); err != nil {
		return nil, err
	}
	if err := responseError(response.Data.APIResponse, response.Extra, "get_post", http.StatusOK, nil); err != nil {
		return nil, err
	}
	if len(response.Data.List) == 0 {
		return nil, wrapError("get_post", socialhub.CodeNotFound, socialhub.ClassPermanent, nil)
	}
	return mapVideo(c.accountID, c.openID, response.Data.List[0], c.clock.Now()), nil
}

func (c *Client) ListPosts(ctx context.Context, input socialhub.ListPostsRequest, options ...socialhub.CallOption) (socialhub.Page[socialhub.Post], error) {
	if input.UserID != "" && input.UserID != c.openID {
		return socialhub.Page[socialhub.Post]{}, invalidArgument("list_posts", "user ID must be the configured app-scoped open_id")
	}
	if input.StartTime != nil || input.EndTime != nil {
		return socialhub.Page[socialhub.Post]{}, &socialhub.Error{Code: socialhub.CodeUnsupported, Class: socialhub.ClassPermanent, Platform: "douyin", Product: "openapi", Op: "list_posts", PlatformMessage: "video.list does not support time filters"}
	}
	if err := c.requireScope("list_posts", "video.list"); err != nil {
		return socialhub.Page[socialhub.Post]{}, err
	}
	cursor, err := parseCursor(input.Cursor)
	if err != nil {
		return socialhub.Page[socialhub.Post]{}, err
	}
	count := input.MaxResults
	if count <= 0 || count > 20 {
		count = 20
	}
	query := c.openIDQuery()
	query.Set("cursor", strconv.FormatInt(cursor, 10))
	query.Set("count", strconv.Itoa(count))
	var response videoListEnvelope
	if err := c.transport.JSON(ctx, http.MethodGet, "/video/list/", query, nil, &response, options...); err != nil {
		return socialhub.Page[socialhub.Post]{}, err
	}
	if err := responseError(response.Data.APIResponse, response.Extra, "list_posts", http.StatusOK, nil); err != nil {
		return socialhub.Page[socialhub.Post]{}, err
	}
	items := make([]socialhub.Post, 0, len(response.Data.List))
	observedAt := c.clock.Now()
	for _, item := range response.Data.List {
		items = append(items, *mapVideo(c.accountID, c.openID, item, observedAt))
	}
	var next *string
	if response.Data.HasMore {
		value := strconv.FormatInt(int64(response.Data.Cursor), 10)
		next = &value
	}
	return socialhub.Page[socialhub.Post]{Items: items, NextCursor: next, HasMore: next != nil}, nil
}

func (c *Client) ListComments(ctx context.Context, input socialhub.ListCommentsRequest, options ...socialhub.CallOption) (socialhub.Page[socialhub.Comment], error) {
	if input.PostID == "" {
		return socialhub.Page[socialhub.Comment]{}, invalidArgument("list_comments", "post ID is required")
	}
	if err := c.requireScope("list_comments", "item.comment"); err != nil {
		return socialhub.Page[socialhub.Comment]{}, err
	}
	cursor, err := parseCursor(input.Cursor)
	if err != nil {
		return socialhub.Page[socialhub.Comment]{}, err
	}
	count := input.MaxResults
	if count <= 0 || count > 20 {
		count = 20
	}
	query := c.openIDQuery()
	query.Set("item_id", input.PostID)
	query.Set("cursor", strconv.FormatInt(cursor, 10))
	query.Set("count", strconv.Itoa(count))
	var response commentListEnvelope
	if err := c.transport.JSON(ctx, http.MethodGet, "/item/comment/list/", query, nil, &response, options...); err != nil {
		return socialhub.Page[socialhub.Comment]{}, err
	}
	if err := responseError(response.Data.APIResponse, response.Extra, "list_comments", http.StatusOK, nil); err != nil {
		return socialhub.Page[socialhub.Comment]{}, err
	}
	items := make([]socialhub.Comment, 0, len(response.Data.List))
	observedAt := c.clock.Now()
	for _, item := range response.Data.List {
		items = append(items, mapComment(c.accountID, input.PostID, item, observedAt))
	}
	var next *string
	if response.Data.HasMore {
		value := strconv.FormatInt(int64(response.Data.Cursor), 10)
		next = &value
	}
	return socialhub.Page[socialhub.Comment]{Items: items, NextCursor: next, HasMore: next != nil}, nil
}

func (c *Client) React(context.Context, socialhub.ReactionRequest, ...socialhub.CallOption) error {
	return &socialhub.Error{Code: socialhub.CodeUnsupported, Class: socialhub.ClassPermanent, Platform: "douyin", Product: "openapi", Op: "react", PlatformMessage: "the public user OpenAPI does not expose like mutation"}
}

func (c *Client) RemoveReaction(context.Context, socialhub.ReactionRequest, ...socialhub.CallOption) error {
	return &socialhub.Error{Code: socialhub.CodeUnsupported, Class: socialhub.ClassPermanent, Platform: "douyin", Product: "openapi", Op: "remove_reaction", PlatformMessage: "the public user OpenAPI does not expose like mutation"}
}

func (c *Client) Comment(ctx context.Context, input socialhub.CreateCommentRequest, options ...socialhub.CallOption) (*socialhub.Comment, error) {
	if input.PostID == "" || input.ParentID == nil || *input.ParentID == "" || strings.TrimSpace(input.Text) == "" {
		return nil, invalidArgument("comment", "post ID, parent comment ID, and text are required")
	}
	if err := c.requireScope("comment", "item.comment"); err != nil {
		return nil, err
	}
	body := map[string]string{"item_id": input.PostID, "comment_id": *input.ParentID, "content": input.Text}
	var response commentReplyEnvelope
	if err := c.transport.JSON(ctx, http.MethodPost, "/item/comment/reply/", c.openIDQuery(), body, &response, options...); err != nil {
		return nil, err
	}
	if err := responseError(response.Data.APIResponse, response.Extra, "comment", http.StatusOK, nil); err != nil {
		return nil, err
	}
	return &socialhub.Comment{Platform: "douyin", AccountID: c.accountID, ID: response.Data.CommentID, PostID: input.PostID, ParentID: input.ParentID, AuthorID: stringPointer(c.openID), Text: input.Text}, nil
}

func (c *Client) DeleteComment(context.Context, string, ...socialhub.CallOption) error {
	return &socialhub.Error{Code: socialhub.CodeUnsupported, Class: socialhub.ClassPermanent, Platform: "douyin", Product: "openapi", Op: "delete_comment", PlatformMessage: "no public comment deletion endpoint is documented"}
}

func (c *Client) SendMessage(context.Context, socialhub.SendMessageRequest, ...socialhub.CallOption) (*socialhub.Message, error) {
	return nil, socialhub.UnsupportedError("douyin", socialhub.CapMessage)
}

func (c *Client) GetMessage(context.Context, string, ...socialhub.CallOption) (*socialhub.Message, error) {
	return nil, socialhub.UnsupportedError("douyin", socialhub.CapMessage)
}

func (c *Client) openIDQuery() url.Values { return url.Values{"open_id": {c.openID}} }

func (c *Client) requireScope(operation, scope string) error {
	if len(c.scopes) == 0 || contains(c.scopes, scope) {
		return nil
	}
	return &socialhub.Error{Code: socialhub.CodeApprovalRequired, Class: socialhub.ClassUserAction, Platform: "douyin", Product: "openapi", Op: operation, RequiredScopes: []string{scope}, ApprovalURL: "https://open.douyin.com/platform/management/", PlatformMessage: "configured approval scopes do not include " + scope}
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func parseCursor(value string) (int64, error) {
	if value == "" {
		return 0, nil
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed < 0 {
		return 0, invalidArgument("pagination", "cursor must be a non-negative integer returned by Douyin")
	}
	return parsed, nil
}

func stringPointer(value string) *string {
	if value == "" {
		return nil
	}
	copy := value
	return &copy
}

var _ socialhub.Client = (*Client)(nil)
var _ socialhub.Publisher = (*Client)(nil)
var _ socialhub.Fetcher = (*Client)(nil)
var _ socialhub.MediaUploader = (*Client)(nil)
var _ socialhub.Reactor = (*Client)(nil)
var _ video.Provider = (*Client)(nil)
