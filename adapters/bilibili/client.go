package bilibili

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

// Client implements the supported Bilibili Open Platform capabilities.
type Client struct {
	accountID     socialhub.AccountID
	openID        string
	baseURL       string
	transport     *transport.Client
	httpClient    *http.Client
	uploadBaseURL string
	token         socialhub.Token
	signer        *requestSigner
	clock         socialhub.Clock
	scopes        []string
	defaults      accountSettings

	uploadMu    sync.Mutex
	uploads     map[string]*uploadState
	submissions *SubmissionService
	videos      *VideoService
}

func (c *Client) Platform() socialhub.Platform { return "bilibili" }
func (c *Client) Account() socialhub.AccountID { return c.accountID }

func (c *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	return socialhub.Capabilities{
		socialhub.CapPublish: capabilityState(socialhub.CapPublish, true, c.scopes, []string{"ARC_BASE"}, "video submission requires approved institutional access and Bilibili-specific metadata", "https://open.bilibili.com/doc/4/f7fc57dd-55a1-5cb1-cba4-61fb2994bf0f"),
		socialhub.CapFetch:   capabilityState(socialhub.CapFetch, true, c.scopes, []string{"USER_INFO", "ARC_BASE"}, "authorized user and archive management reads are supported", "https://open.bilibili.com/doc/4/feb66f99-7d87-c206-00e7-d84164cd701c"),
		socialhub.CapMedia:   capabilityState(socialhub.CapMedia, true, c.scopes, []string{"ARC_BASE"}, "single video files up to 100 MiB and cover images up to 5 MiB are supported", "https://open.bilibili.com/doc/4/f22a0eee-c92d-0f1d-f69c-be170cf533c7"),
		socialhub.CapReact:   {Capability: socialhub.CapReact, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "the Open Platform archive product does not expose comment or reaction mutation"},
		socialhub.CapMessage: {Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "private messages are outside the approved archive-management product"},
		socialhub.CapWebhook: {Capability: socialhub.CapWebhook, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "message-push callbacks use separately approved event products not included in this adapter"},
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

func (c *Client) Publisher() (socialhub.Publisher, bool)           { return c, true }
func (c *Client) Fetcher() (socialhub.Fetcher, bool)               { return c, true }
func (c *Client) MediaUploader() (socialhub.MediaUploader, bool)   { return c, true }
func (c *Client) Reactor() (socialhub.Reactor, bool)               { return nil, false }
func (c *Client) Messenger() (socialhub.Messenger, bool)           { return nil, false }
func (c *Client) WebhookHandler() (socialhub.WebhookHandler, bool) { return nil, false }
func (c *Client) Close() error                                     { return nil }

// SubmissionWorkflow returns Bilibili's typed archive submission service.
func (c *Client) SubmissionWorkflow() SubmissionWorkflow { return c.submissions }

// VideoWorkflow adapts the account's default submission metadata to video.Workflow.
func (c *Client) VideoWorkflow() video.Workflow { return c.videos }

func (c *Client) Publish(ctx context.Context, input socialhub.CreatePostRequest, options ...socialhub.CallOption) (*socialhub.Post, error) {
	if input.Text == nil || strings.TrimSpace(*input.Text) == "" {
		return nil, invalidArgument("publish", "a non-empty archive title is required")
	}
	if c.defaults.DefaultTID <= 0 || len(c.defaults.DefaultTags) == 0 {
		return nil, invalidArgument("publish", "account default_tid and default_tags are required; otherwise use SubmissionWorkflow")
	}
	if input.ReplyToID != nil || input.QuotePostID != nil || input.Visibility != nil {
		return nil, &socialhub.Error{Code: socialhub.CodeUnsupported, Class: socialhub.ClassPermanent, Platform: "bilibili", Product: "open-platform", Op: "publish", PlatformMessage: "reply, quote, and visibility overrides are not archive submission fields"}
	}
	videoID, coverID, err := c.publicationMedia(input.MediaIDs)
	if err != nil {
		return nil, err
	}
	return c.submissions.Publish(ctx, SubmissionRequest{
		UploadToken: videoID, CoverID: coverID, Title: *input.Text, TID: c.defaults.DefaultTID,
		Tags: append([]string(nil), c.defaults.DefaultTags...), Copyright: c.defaults.DefaultCopyright,
		Source: c.defaults.DefaultSource, NoReprint: c.defaults.NoReprint,
	}, options...)
}

func (c *Client) PublishStatus(ctx context.Context, postID string, options ...socialhub.CallOption) (*socialhub.PublishStatus, error) {
	post, err := c.GetPost(ctx, postID, options...)
	if err != nil {
		return nil, err
	}
	return post.Status, nil
}

func (c *Client) DeletePost(ctx context.Context, postID string, options ...socialhub.CallOption) error {
	if postID == "" {
		return invalidArgument("delete_post", "post ID is required")
	}
	if err := c.requireScope("delete_post", "ARC_BASE"); err != nil {
		return err
	}
	var response responseEnvelope[jsonEmpty]
	if err := c.transport.JSON(ctx, http.MethodPost, "/arcopen/fn/archive/delete", nil, map[string]string{"resource_id": postID}, &response, options...); err != nil {
		return err
	}
	return response.Err("delete_post", http.StatusOK, nil)
}

func (c *Client) GetUser(ctx context.Context, userID string, options ...socialhub.CallOption) (*socialhub.User, error) {
	if userID == "" || userID != c.openID {
		return nil, invalidArgument("get_user", "user ID must be the configured app-scoped open_id")
	}
	if err := c.requireScope("get_user", "USER_INFO"); err != nil {
		return nil, err
	}
	var response responseEnvelope[userInfo]
	if err := c.transport.JSON(ctx, http.MethodGet, "/arcopen/fn/user/account/info", nil, nil, &response, options...); err != nil {
		return nil, err
	}
	if err := response.Err("get_user", http.StatusOK, nil); err != nil {
		return nil, err
	}
	if response.Data.OpenID == "" || response.Data.OpenID != c.openID {
		return nil, wrapError("get_user", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return mapUser(c.accountID, response.Data), nil
}

func (c *Client) GetPost(ctx context.Context, postID string, options ...socialhub.CallOption) (*socialhub.Post, error) {
	if postID == "" {
		return nil, invalidArgument("get_post", "post ID is required")
	}
	if err := c.requireScope("get_post", "ARC_BASE"); err != nil {
		return nil, err
	}
	var response responseEnvelope[archive]
	query := url.Values{"resource_id": {postID}}
	if err := c.transport.JSON(ctx, http.MethodGet, "/arcopen/fn/archive/view", query, nil, &response, options...); err != nil {
		return nil, err
	}
	if err := response.Err("get_post", http.StatusOK, nil); err != nil {
		return nil, err
	}
	return mapArchive(c.accountID, c.openID, response.Data), nil
}

func (c *Client) ListPosts(ctx context.Context, input socialhub.ListPostsRequest, options ...socialhub.CallOption) (socialhub.Page[socialhub.Post], error) {
	if input.UserID != "" && input.UserID != c.openID {
		return socialhub.Page[socialhub.Post]{}, invalidArgument("list_posts", "user ID must be the configured app-scoped open_id")
	}
	if input.StartTime != nil || input.EndTime != nil {
		return socialhub.Page[socialhub.Post]{}, &socialhub.Error{Code: socialhub.CodeUnsupported, Class: socialhub.ClassPermanent, Platform: "bilibili", Product: "open-platform", Op: "list_posts", PlatformMessage: "archive viewlist does not support time filters"}
	}
	if err := c.requireScope("list_posts", "ARC_BASE"); err != nil {
		return socialhub.Page[socialhub.Post]{}, err
	}
	pageNumber := 1
	if input.Cursor != "" {
		parsed, err := strconv.Atoi(input.Cursor)
		if err != nil || parsed < 1 {
			return socialhub.Page[socialhub.Post]{}, invalidArgument("list_posts", "cursor must be a positive page number returned by Bilibili")
		}
		pageNumber = parsed
	}
	pageSize := input.MaxResults
	if pageSize <= 0 || pageSize > 50 {
		pageSize = 20
	}
	query := url.Values{"pn": {strconv.Itoa(pageNumber)}, "ps": {strconv.Itoa(pageSize)}, "status": {"all"}}
	var response responseEnvelope[archiveList]
	if err := c.transport.JSON(ctx, http.MethodGet, "/arcopen/fn/archive/viewlist", query, nil, &response, options...); err != nil {
		return socialhub.Page[socialhub.Post]{}, err
	}
	if err := response.Err("list_posts", http.StatusOK, nil); err != nil {
		return socialhub.Page[socialhub.Post]{}, err
	}
	items := make([]socialhub.Post, 0, len(response.Data.List))
	for _, item := range response.Data.List {
		items = append(items, *mapArchive(c.accountID, c.openID, item))
	}
	var next *string
	if response.Data.Page.Number*response.Data.Page.Size < response.Data.Page.Total {
		value := strconv.Itoa(response.Data.Page.Number + 1)
		next = &value
	}
	return socialhub.Page[socialhub.Post]{Items: items, NextCursor: next, HasMore: next != nil}, nil
}

func (c *Client) ListComments(context.Context, socialhub.ListCommentsRequest, ...socialhub.CallOption) (socialhub.Page[socialhub.Comment], error) {
	return socialhub.Page[socialhub.Comment]{}, socialhub.UnsupportedError("bilibili", socialhub.CapReact)
}

func (c *Client) requireScope(operation, scope string) error {
	if len(c.scopes) == 0 || contains(c.scopes, scope) {
		return nil
	}
	return &socialhub.Error{Code: socialhub.CodeApprovalRequired, Class: socialhub.ClassUserAction, Platform: "bilibili", Product: "open-platform", Op: operation, RequiredScopes: []string{scope}, ApprovalURL: "https://openhome.bilibili.com/", PlatformMessage: "configured approval scopes do not include " + scope}
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

type jsonEmpty struct{}

var _ socialhub.Client = (*Client)(nil)
var _ socialhub.Publisher = (*Client)(nil)
var _ socialhub.Fetcher = (*Client)(nil)
var _ socialhub.MediaUploader = (*Client)(nil)
var _ video.Provider = (*Client)(nil)
