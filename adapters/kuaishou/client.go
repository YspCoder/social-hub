package kuaishou

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"social-hub/extensions/video"
	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

// Client implements the supported Kuaishou Open Platform capabilities.
type Client struct {
	accountID          socialhub.AccountID
	appID              string
	openID             string
	transport          *transport.Client
	httpClient         *http.Client
	clock              socialhub.Clock
	uploadScheme       string
	allowedUploadHosts []string
	scopes             []string

	uploadMu sync.Mutex
	uploads  map[string]*uploadState
	statuses map[string]socialhub.PublishStatus
	videos   *VideoService
}

func (c *Client) Platform() socialhub.Platform { return "kuaishou" }
func (c *Client) Account() socialhub.AccountID { return c.accountID }

func (c *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	return socialhub.Capabilities{
		socialhub.CapPublish: capabilityState(socialhub.CapPublish, true, c.scopes, []string{"user_video_publish"}, "video publication requires a separately uploaded JPEG cover and is asynchronous", "https://open.kuaishou.com/platform/openApi?menu=20"),
		socialhub.CapFetch:   capabilityState(socialhub.CapFetch, true, c.scopes, []string{"user_info"}, "the initial adapter fetches the authorized user's public profile; post and comment reads are explicit unsupported operations", "https://open.kuaishou.com/platformDocs/openAbility/userInformation/publicInformation"),
		socialhub.CapMedia:   capabilityState(socialhub.CapMedia, true, c.scopes, []string{"user_video_publish"}, "direct and fragment video upload plus bounded local cover staging are supported", "https://open.kuaishou.com/platform/openApi?menu=20"),
		socialhub.CapReact:   {Capability: socialhub.CapReact, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "no stable public interaction mutation is included in this product integration"},
		socialhub.CapMessage: {Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "direct messaging is not exposed by the public website-app OpenAPI product"},
		socialhub.CapWebhook: {Capability: socialhub.CapWebhook, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "no callback contract is documented for the implemented user and video APIs"},
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
func (c *Client) Reactor() (socialhub.Reactor, bool)             { return nil, false }
func (c *Client) Messenger() (socialhub.Messenger, bool)         { return nil, false }
func (c *Client) WebhookHandler() (socialhub.WebhookHandler, bool) {
	return nil, false
}
func (c *Client) Close() error { return nil }

// VideoWorkflow returns the typed short-video workflow.
func (c *Client) VideoWorkflow() video.Workflow { return c.videos }

func (c *Client) Publish(ctx context.Context, input socialhub.CreatePostRequest, options ...socialhub.CallOption) (*socialhub.Post, error) {
	if err := c.requireScope("publish", "user_video_publish"); err != nil {
		return nil, err
	}
	if input.Text == nil || strings.TrimSpace(*input.Text) == "" {
		return nil, invalidArgument("publish", "a non-empty caption is required")
	}
	if len(input.MediaIDs) != 2 {
		return nil, invalidArgument("publish", "one completed video and one completed cover image are required")
	}
	if input.ReplyToID != nil || input.QuotePostID != nil || input.Visibility != nil {
		return nil, &socialhub.Error{Code: socialhub.CodeUnsupported, Class: socialhub.ClassPermanent, Platform: "kuaishou", Product: "openapi", Op: "publish", PlatformMessage: "reply, quote, and visibility overrides are not supported by photo/publish"}
	}
	videoState, coverState, err := c.publicationMedia(input.MediaIDs)
	if err != nil {
		return nil, err
	}
	query := c.appQuery()
	query.Set("upload_token", videoState.media.ID)
	var response publishEnvelope
	if err := c.multipartPublish(ctx, query, *input.Text, coverState, &response, options...); err != nil {
		return nil, err
	}
	if err := resultError(response.Result, response.ErrorMessage, "publish", http.StatusOK, nil); err != nil {
		return nil, err
	}
	if response.Video.PhotoID == "" {
		return nil, wrapError("publish", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	post := mapVideo(c.accountID, c.openID, response.Video, c.clock.Now())
	post.Media[0].ID = videoState.media.ID
	post.Media[0].MIME = videoState.request.MIME
	post.Media = append(post.Media, socialhub.Media{ID: coverState.media.ID, URL: response.Video.Cover, MIME: coverState.request.MIME, Type: socialhub.MediaTypeImage, State: socialhub.MediaStateReady})
	c.uploadMu.Lock()
	c.statuses[post.ID] = *post.Status
	c.uploadMu.Unlock()
	return post, nil
}

func (c *Client) PublishStatus(_ context.Context, postID string, _ ...socialhub.CallOption) (*socialhub.PublishStatus, error) {
	if postID == "" {
		return nil, invalidArgument("publish_status", "post ID is required")
	}
	c.uploadMu.Lock()
	defer c.uploadMu.Unlock()
	status, ok := c.statuses[postID]
	if !ok {
		return nil, wrapError("publish_status", socialhub.CodeNotFound, socialhub.ClassPermanent, nil)
	}
	copy := status
	return &copy, nil
}

func (c *Client) DeletePost(context.Context, string, ...socialhub.CallOption) error {
	return &socialhub.Error{Code: socialhub.CodeUnsupported, Class: socialhub.ClassPermanent, Platform: "kuaishou", Product: "openapi", Op: "delete_post", PlatformMessage: "video deletion is not included in the documented public website-app API"}
}

func (c *Client) GetUser(ctx context.Context, userID string, options ...socialhub.CallOption) (*socialhub.User, error) {
	if userID == "" || userID != c.openID {
		return nil, invalidArgument("get_user", "user ID must be the configured app-scoped open_id")
	}
	if err := c.requireScope("get_user", "user_info"); err != nil {
		return nil, err
	}
	var response userEnvelope
	if err := c.transport.JSON(ctx, http.MethodGet, "/openapi/user_info", c.appQuery(), nil, &response, options...); err != nil {
		return nil, err
	}
	if err := resultError(response.Result, response.ErrorMessage, "get_user", http.StatusOK, nil); err != nil {
		return nil, err
	}
	return mapUser(c.accountID, c.openID, response.User, c.clock.Now()), nil
}

func (c *Client) GetPost(context.Context, string, ...socialhub.CallOption) (*socialhub.Post, error) {
	return nil, &socialhub.Error{Code: socialhub.CodeUnsupported, Class: socialhub.ClassPermanent, Platform: "kuaishou", Product: "openapi", Op: "get_post", PlatformMessage: "the initial adapter does not bind an undocumented video-info endpoint"}
}

func (c *Client) ListPosts(context.Context, socialhub.ListPostsRequest, ...socialhub.CallOption) (socialhub.Page[socialhub.Post], error) {
	return socialhub.Page[socialhub.Post]{}, &socialhub.Error{Code: socialhub.CodeUnsupported, Class: socialhub.ClassPermanent, Platform: "kuaishou", Product: "openapi", Op: "list_posts", PlatformMessage: "the initial adapter does not bind an undocumented video-list endpoint"}
}

func (c *Client) ListComments(context.Context, socialhub.ListCommentsRequest, ...socialhub.CallOption) (socialhub.Page[socialhub.Comment], error) {
	return socialhub.Page[socialhub.Comment]{}, socialhub.UnsupportedError("kuaishou", socialhub.CapReact)
}

func (c *Client) appQuery() url.Values { return url.Values{"app_id": {c.appID}} }

func (c *Client) requireScope(operation, scope string) error {
	if len(c.scopes) == 0 || contains(c.scopes, scope) {
		return nil
	}
	return &socialhub.Error{Code: socialhub.CodeApprovalRequired, Class: socialhub.ClassUserAction, Platform: "kuaishou", Product: "openapi", Op: operation, RequiredScopes: []string{scope}, ApprovalURL: "https://open.kuaishou.com/platform/", PlatformMessage: "configured approval scopes do not include " + scope}
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

var _ socialhub.Client = (*Client)(nil)
var _ socialhub.Publisher = (*Client)(nil)
var _ socialhub.Fetcher = (*Client)(nil)
var _ socialhub.MediaUploader = (*Client)(nil)
var _ video.Provider = (*Client)(nil)
