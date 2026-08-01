package vimeo

import (
	"context"
	"net/http"
	"net/url"
	"sync"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	CapabilityVideoUpload socialhub.Capability = "video_upload"
	CapabilityHomeFeed    socialhub.Capability = "home_feed"
)

// Client implements Vimeo's supported common and typed capabilities.
type Client struct {
	accountID  socialhub.AccountID
	userID     string
	api        *transport.Client
	apiBaseURL *url.URL
	httpClient *http.Client
	scopes     []string
	clock      socialhub.Clock
	uploadMu   sync.Mutex
	uploads    map[string]*videoUpload
	videos     *VideoUploadService
}

func (c *Client) Platform() socialhub.Platform { return "vimeo" }
func (c *Client) Account() socialhub.AccountID { return c.accountID }

func (c *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	return socialhub.Capabilities{
		socialhub.CapFetch:    capabilityState(socialhub.CapFetch, c.scopes, []string{"public"}, "users, videos, account videos, and comments", docURL),
		socialhub.CapReact:    capabilityState(socialhub.CapReact, c.scopes, []string{"interact", "delete"}, "video likes, comments, replies, and comment deletion", docURL),
		CapabilityHomeFeed:    capabilityState(CapabilityHomeFeed, c.scopes, []string{"public"}, "authenticated or public user feed", docURL),
		CapabilityVideoUpload: capabilityState(CapabilityVideoUpload, c.scopes, []string{"upload", "edit"}, "TUS video creation and upload; update and delete have separate scopes", "https://developer.vimeo.com/api/upload/videos"),
		socialhub.CapPublish:  {Capability: socialhub.CapPublish, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "a Vimeo video is created as part of its upload lifecycle; use VideoUploadWorkflow"},
		socialhub.CapMedia:    {Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "uploaded bytes are inseparable from a Vimeo video resource; use VideoUploadWorkflow"},
		socialhub.CapMessage:  {Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Vimeo API does not expose direct messaging"},
		socialhub.CapWebhook:  {Capability: socialhub.CapWebhook, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "signed webhook handling is not part of the public Vimeo API adapter contract"},
	}, nil
}

func capabilityState(capability socialhub.Capability, granted, required []string, reason, documentation string) socialhub.CapabilityState {
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
	return socialhub.CapabilityState{Capability: capability, Supported: true, Approval: approval, Scopes: append([]string(nil), required...), Reason: reason, DocURL: documentation}
}

func (c *Client) Publisher() (socialhub.Publisher, bool)           { return nil, false }
func (c *Client) Fetcher() (socialhub.Fetcher, bool)               { return c, true }
func (c *Client) MediaUploader() (socialhub.MediaUploader, bool)   { return nil, false }
func (c *Client) Reactor() (socialhub.Reactor, bool)               { return c, true }
func (c *Client) Messenger() (socialhub.Messenger, bool)           { return nil, false }
func (c *Client) WebhookHandler() (socialhub.WebhookHandler, bool) { return nil, false }
func (c *Client) Close() error                                     { return nil }

// VideoUploadWorkflow returns Vimeo's typed TUS upload flow.
func (c *Client) VideoUploadWorkflow() VideoUploadWorkflow { return c.videos }

// FeedWorkflow returns Vimeo's typed home-feed reader.
func (c *Client) FeedWorkflow() FeedWorkflow { return c }

func (c *Client) requireScopes(operation string, required ...string) error {
	if len(c.scopes) == 0 {
		return nil
	}
	missing := make([]string, 0, len(required))
	for _, scope := range required {
		if !contains(c.scopes, scope) {
			missing = append(missing, scope)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	return &socialhub.Error{
		Code: socialhub.CodeApprovalRequired, Class: socialhub.ClassUserAction,
		Platform: "vimeo", Product: productName, Op: operation, RequiredScopes: missing,
		ApprovalURL: "https://developer.vimeo.com/apps", PlatformMessage: "configured OAuth scopes are incomplete",
	}
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
var _ socialhub.Fetcher = (*Client)(nil)
var _ socialhub.Reactor = (*Client)(nil)
