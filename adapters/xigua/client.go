package xigua

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"sync"

	"social-hub/extensions/video"
	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

// Client implements the public Xigua user and video capabilities.
type Client struct {
	accountID socialhub.AccountID
	openID    string
	api       *transport.Client
	identity  *transport.Client
	clock     socialhub.Clock
	scopes    []string

	uploadMu sync.Mutex
	uploads  map[string]*uploadState
	videos   *VideoService

	submittedMu sync.Mutex
	submitted   map[string]socialhub.PublishStatus
}

func (c *Client) Platform() socialhub.Platform { return platformName }
func (c *Client) Account() socialhub.AccountID { return c.accountID }

func (c *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	return socialhub.Capabilities{
		socialhub.CapPublish: capabilityState(socialhub.CapPublish, c.scopes, []string{"xigua.video.create"}, "small-video publication is asynchronous and subject to review", "https://open.douyin.com/platform/resource/docs/openapi/video-management/xigua/create-video/publish-video/"),
		socialhub.CapFetch:   capabilityState(socialhub.CapFetch, c.scopes, []string{"user_info", "xigua.video.data"}, "authorized profile and owned public video data", "https://open.douyin.com/platform/resource/docs/openapi/video-management/xigua/search-video/account-video-list/"),
		socialhub.CapMedia:   capabilityState(socialhub.CapMedia, c.scopes, []string{"xigua.video.create"}, "direct and multipart video upload up to the documented limits", "https://open.douyin.com/platform/resource/docs/openapi/video-management/xigua/create-video/upload-video"),
		socialhub.CapReact:   {Capability: socialhub.CapReact, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "the public Xigua OpenAPI does not document reaction mutation or comment APIs"},
		socialhub.CapMessage: {Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "the public Xigua OpenAPI does not document direct messaging"},
		socialhub.CapWebhook: {Capability: socialhub.CapWebhook, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "no signed Xigua webhook contract is included in this adapter version"},
	}, nil
}

func capabilityState(capability socialhub.Capability, granted, required []string, reason, docURL string) socialhub.CapabilityState {
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
	return socialhub.CapabilityState{Capability: capability, Supported: true, Approval: approval, Scopes: required, Reason: reason, DocURL: docURL}
}

func (c *Client) Publisher() (socialhub.Publisher, bool)           { return c, true }
func (c *Client) Fetcher() (socialhub.Fetcher, bool)               { return c, true }
func (c *Client) MediaUploader() (socialhub.MediaUploader, bool)   { return c, true }
func (c *Client) Reactor() (socialhub.Reactor, bool)               { return nil, false }
func (c *Client) Messenger() (socialhub.Messenger, bool)           { return nil, false }
func (c *Client) WebhookHandler() (socialhub.WebhookHandler, bool) { return nil, false }
func (c *Client) Close() error                                     { return nil }

// VideoWorkflow returns the typed short-video upload and publication workflow.
func (c *Client) VideoWorkflow() video.Workflow { return c.videos }

// PublishVideoRequest preserves Xigua-specific publication metadata.
type PublishVideoRequest struct {
	VideoID              string
	Title                string
	Summary              string
	CoverTimestampMillis *int64
	DeclareOriginal      *bool
	EnableReward         *bool
}

func (c *Client) Publish(ctx context.Context, input socialhub.CreatePostRequest, options ...socialhub.CallOption) (*socialhub.Post, error) {
	if err := c.requireScope("publish", "xigua.video.create"); err != nil {
		return nil, err
	}
	if len(input.MediaIDs) != 1 || !validOpaque(input.MediaIDs[0], maxOpaqueLength) {
		return nil, invalidArgument("publish", "exactly one uploaded video ID is required")
	}
	if input.ReplyToID != nil || input.QuotePostID != nil || input.Visibility != nil {
		return nil, unsupported("publish", "reply, quote, and visibility overrides are not supported by xigua.video.create")
	}
	if input.Text == nil {
		return nil, invalidArgument("publish", "a Xigua title is required")
	}
	return c.PublishVideo(ctx, PublishVideoRequest{VideoID: input.MediaIDs[0], Title: *input.Text}, options...)
}

// PublishVideo publishes a completed upload with Xigua-specific metadata.
func (c *Client) PublishVideo(ctx context.Context, input PublishVideoRequest, options ...socialhub.CallOption) (*socialhub.Post, error) {
	if err := c.requireScope("publish_video", "xigua.video.create"); err != nil {
		return nil, err
	}
	if !validOpaque(input.VideoID, maxOpaqueLength) {
		return nil, invalidArgument("publish_video", "a valid uploaded video ID is required")
	}
	if !validTitle(input.Title) {
		return nil, invalidArgument("publish_video", "title must contain 5 to 30 Unicode characters")
	}
	if !validSummary(input.Summary) {
		return nil, invalidArgument("publish_video", "summary must contain at most 400 Unicode characters")
	}
	if input.CoverTimestampMillis != nil && *input.CoverTimestampMillis < 0 {
		return nil, invalidArgument("publish_video", "cover timestamp must be non-negative")
	}
	body := map[string]string{"video_id": input.VideoID, "text": input.Title}
	if input.Summary != "" {
		body["abstract"] = input.Summary
	}
	if input.CoverTimestampMillis != nil {
		body["cover_tsp"] = strconv.FormatInt(*input.CoverTimestampMillis, 10)
	}
	if input.DeclareOriginal != nil {
		body["claim_origin"] = strconv.FormatBool(*input.DeclareOriginal)
	}
	if input.EnableReward != nil {
		body["praise"] = strconv.FormatBool(*input.EnableReward)
	}
	var response createVideoEnvelope
	if err := c.api.JSON(ctx, http.MethodPost, "/xigua/video/create/", c.openIDQuery(), body, &response, options...); err != nil {
		return nil, err
	}
	if err := responseError(response.Data.apiResponse, response.Extra, "publish_video", http.StatusOK, nil); err != nil {
		return nil, err
	}
	if !validOpaque(response.Data.ItemID, maxOpaqueLength) {
		return nil, invalidPlatformResponse("publish_video", "response omitted a valid item_id")
	}
	status := socialhub.PublishStatus{ID: response.Data.ItemID, State: socialhub.PublishStatePending, Message: "submitted for Xigua review"}
	c.submittedMu.Lock()
	c.submitted[response.Data.ItemID] = status
	c.submittedMu.Unlock()
	return &socialhub.Post{
		Platform: platformName, AccountID: c.accountID, ID: response.Data.ItemID,
		AuthorID: stringPointer(c.openID), Text: stringPointer(input.Title),
		Media:      []socialhub.Media{{ID: input.VideoID, Type: socialhub.MediaTypeVideo, State: socialhub.MediaStateProcessing}},
		Status:     &status,
		Extensions: publishExtensions(input),
	}, nil
}

func publishExtensions(input PublishVideoRequest) map[string]json.RawMessage {
	extension, _ := json.Marshal(map[string]any{
		"summary": input.Summary, "cover_timestamp_ms": input.CoverTimestampMillis,
		"declare_original": input.DeclareOriginal, "enable_reward": input.EnableReward,
	})
	return map[string]json.RawMessage{"xigua.publish": extension}
}

func (c *Client) PublishStatus(ctx context.Context, postID string, options ...socialhub.CallOption) (*socialhub.PublishStatus, error) {
	if !validOpaque(postID, maxOpaqueLength) {
		return nil, invalidArgument("publish_status", "post ID is required")
	}
	post, err := c.GetPost(ctx, postID, options...)
	if err != nil {
		if errors.Is(err, socialhub.ErrNotFound) {
			c.submittedMu.Lock()
			status, found := c.submitted[postID]
			c.submittedMu.Unlock()
			if found {
				copy := status
				return &copy, nil
			}
		}
		return nil, err
	}
	c.submittedMu.Lock()
	delete(c.submitted, postID)
	c.submittedMu.Unlock()
	if post.Status != nil {
		copy := *post.Status
		return &copy, nil
	}
	return &socialhub.PublishStatus{ID: postID, State: socialhub.PublishStatePublished, UpdatedAt: post.CreatedAt}, nil
}

func (c *Client) DeletePost(_ context.Context, postID string, _ ...socialhub.CallOption) error {
	if !validOpaque(postID, maxOpaqueLength) {
		return invalidArgument("delete_post", "post ID is required")
	}
	return unsupported("delete_post", "no public Xigua video deletion endpoint is documented")
}

func (c *Client) GetUser(ctx context.Context, userID string, options ...socialhub.CallOption) (*socialhub.User, error) {
	if userID != c.openID {
		return nil, invalidArgument("get_user", "user ID must be the configured app-scoped open_id")
	}
	if err := c.requireScope("get_user", "user_info"); err != nil {
		return nil, err
	}
	var response userEnvelope
	if err := c.identity.JSON(ctx, http.MethodPost, "/oauth/userinfo/", c.openIDQuery(), map[string]string{}, &response, options...); err != nil {
		return nil, err
	}
	if err := responseError(response.Data.apiResponse, response.Extra, "get_user", http.StatusOK, nil); err != nil {
		return nil, err
	}
	if response.Data.OpenID != c.openID {
		return nil, invalidPlatformResponse("get_user", "response open_id did not match the configured account")
	}
	return mapUser(c.accountID, response.Data), nil
}

func (c *Client) GetPost(ctx context.Context, postID string, options ...socialhub.CallOption) (*socialhub.Post, error) {
	if !validOpaque(postID, maxOpaqueLength) {
		return nil, invalidArgument("get_post", "post ID is required")
	}
	if err := c.requireScope("get_post", "xigua.video.data"); err != nil {
		return nil, err
	}
	var response videoListEnvelope
	if err := c.api.JSON(ctx, http.MethodPost, "/xigua/video/data/", c.openIDQuery(), map[string][]string{"item_ids": {postID}}, &response, options...); err != nil {
		return nil, err
	}
	if err := responseError(response.Data.apiResponse, response.Extra, "get_post", http.StatusOK, nil); err != nil {
		return nil, err
	}
	if len(response.Data.List) == 0 {
		return nil, platformError("get_post", socialhub.CodeNotFound, socialhub.ClassPermanent, nil)
	}
	if len(response.Data.List) != 1 || response.Data.List[0].ItemID != postID || !validVideo(response.Data.List[0]) {
		return nil, invalidPlatformResponse("get_post", "response did not contain the requested video with valid identifiers")
	}
	return mapVideo(c.accountID, c.openID, response.Data.List[0], c.clock.Now()), nil
}

func (c *Client) ListPosts(ctx context.Context, input socialhub.ListPostsRequest, options ...socialhub.CallOption) (socialhub.Page[socialhub.Post], error) {
	if input.UserID != "" && input.UserID != c.openID {
		return socialhub.Page[socialhub.Post]{}, invalidArgument("list_posts", "user ID must be the configured app-scoped open_id")
	}
	if input.StartTime != nil || input.EndTime != nil {
		return socialhub.Page[socialhub.Post]{}, unsupported("list_posts", "xigua.video.list does not document time filters")
	}
	if err := c.requireScope("list_posts", "xigua.video.data"); err != nil {
		return socialhub.Page[socialhub.Post]{}, err
	}
	cursor, err := parseCursor(input.Cursor)
	if err != nil {
		return socialhub.Page[socialhub.Post]{}, err
	}
	count, err := pageSize(input.MaxResults)
	if err != nil {
		return socialhub.Page[socialhub.Post]{}, err
	}
	query := c.openIDQuery()
	query.Set("cursor", strconv.FormatInt(cursor, 10))
	query.Set("count", strconv.Itoa(count))
	var response videoListEnvelope
	if err := c.api.JSON(ctx, http.MethodGet, "/xigua/video/list/", query, nil, &response, options...); err != nil {
		return socialhub.Page[socialhub.Post]{}, err
	}
	if err := responseError(response.Data.apiResponse, response.Extra, "list_posts", http.StatusOK, nil); err != nil {
		return socialhub.Page[socialhub.Post]{}, err
	}
	items := make([]socialhub.Post, 0, len(response.Data.List))
	observedAt := c.clock.Now()
	for _, item := range response.Data.List {
		if !validVideo(item) {
			return socialhub.Page[socialhub.Post]{}, invalidPlatformResponse("list_posts", "response contained invalid video identifiers")
		}
		items = append(items, *mapVideo(c.accountID, c.openID, item, observedAt))
	}
	var next *string
	if response.Data.HasMore {
		if response.Data.Cursor < 0 {
			return socialhub.Page[socialhub.Post]{}, invalidPlatformResponse("list_posts", "response contained a negative cursor")
		}
		value := strconv.FormatInt(int64(response.Data.Cursor), 10)
		next = &value
	}
	return socialhub.Page[socialhub.Post]{Items: items, NextCursor: next, HasMore: next != nil}, nil
}

func (c *Client) ListComments(_ context.Context, input socialhub.ListCommentsRequest, _ ...socialhub.CallOption) (socialhub.Page[socialhub.Comment], error) {
	if !validOpaque(input.PostID, maxOpaqueLength) {
		return socialhub.Page[socialhub.Comment]{}, invalidArgument("list_comments", "post ID is required")
	}
	return socialhub.Page[socialhub.Comment]{}, unsupported("list_comments", "no public Xigua comment-list endpoint is documented")
}

func (c *Client) openIDQuery() url.Values { return url.Values{"open_id": {c.openID}} }

func validVideo(input xiguaVideo) bool {
	return validOpaque(input.ItemID, maxOpaqueLength) && (input.VideoID == "" || validOpaque(input.VideoID, maxOpaqueLength))
}

func (c *Client) requireScope(operation, scope string) error {
	if len(c.scopes) == 0 || contains(c.scopes, scope) {
		return nil
	}
	return &socialhub.Error{
		Code: socialhub.CodeApprovalRequired, Class: socialhub.ClassUserAction,
		Platform: platformName, Product: productName, Op: operation,
		RequiredScopes: []string{scope}, ApprovalURL: "https://open.douyin.com/platform/management/",
		PlatformMessage: "configured approval scopes do not include " + scope,
	}
}

var _ socialhub.Client = (*Client)(nil)
var _ socialhub.Publisher = (*Client)(nil)
var _ socialhub.Fetcher = (*Client)(nil)
var _ socialhub.MediaUploader = (*Client)(nil)
var _ video.Provider = (*Client)(nil)
