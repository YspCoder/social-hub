package youtube

import (
	"context"
	"net/http"
	"sync"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

// Client implements the common capabilities supported by YouTube Data API.
type Client struct {
	accountID       socialhub.AccountID
	channelID       string
	transport       *transport.Client
	uploadTransport *transport.Client
	httpClient      *http.Client
	accessToken     string
	scopes          []string
	clock           socialhub.Clock
	uploadMu        sync.Mutex
	uploads         map[string]*videoUpload
	videos          *VideoUploadService
}

func (c *Client) Platform() socialhub.Platform { return "youtube" }
func (c *Client) Account() socialhub.AccountID { return c.accountID }

func (c *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	return socialhub.Capabilities{
		CapabilityVideoUpload: capabilityState(CapabilityVideoUpload, true, c.scopes, []string{
			"https://www.googleapis.com/auth/youtube.upload",
			"https://www.googleapis.com/auth/youtube.readonly",
			"https://www.googleapis.com/auth/youtube.force-ssl",
		}, "upload uses youtube.upload; status and deletion require readonly/force-ssl", docURL+"guides/using_resumable_upload_protocol"),
		socialhub.CapPublish: {Capability: socialhub.CapPublish, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "YouTube publication requires video metadata plus a resumable media upload"},
		socialhub.CapFetch:   capabilityState(socialhub.CapFetch, true, c.scopes, []string{"https://www.googleapis.com/auth/youtube.readonly"}, "reads cover the configured channel, videos, search results, and comment threads", docURL+"docs"),
		socialhub.CapMedia:   {Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "uploaded media is inseparable from a YouTube video resource; use VideoUploadWorkflow"},
		socialhub.CapReact:   capabilityState(socialhub.CapReact, true, c.scopes, []string{"https://www.googleapis.com/auth/youtube.force-ssl"}, "LIKE rating and comment create/delete are supported", docURL+"docs/videos/rate"),
		socialhub.CapMessage: {Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "YouTube Data API does not expose direct messaging"},
		socialhub.CapWebhook: {Capability: socialhub.CapWebhook, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "YouTube push notifications use a separate WebSub subscription and Atom feed contract"},
	}, nil
}

func capabilityState(capability socialhub.Capability, supported bool, granted, required []string, reason, documentation string) socialhub.CapabilityState {
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
	return socialhub.CapabilityState{Capability: capability, Supported: supported, Approval: approval, Scopes: required, Reason: reason, DocURL: documentation}
}

func (c *Client) Publisher() (socialhub.Publisher, bool)           { return nil, false }
func (c *Client) Fetcher() (socialhub.Fetcher, bool)               { return c, true }
func (c *Client) MediaUploader() (socialhub.MediaUploader, bool)   { return nil, false }
func (c *Client) Reactor() (socialhub.Reactor, bool)               { return c, true }
func (c *Client) Messenger() (socialhub.Messenger, bool)           { return nil, false }
func (c *Client) WebhookHandler() (socialhub.WebhookHandler, bool) { return nil, false }
func (c *Client) Close() error                                     { return nil }

// VideoUploadWorkflow returns YouTube's typed resumable upload flow.
func (c *Client) VideoUploadWorkflow() VideoUploadWorkflow { return c.videos }

func (c *Client) requireScope(operation, scope string) error {
	if len(c.scopes) == 0 || contains(c.scopes, scope) || contains(c.scopes, "https://www.googleapis.com/auth/youtube") {
		return nil
	}
	return &socialhub.Error{
		Code: socialhub.CodeApprovalRequired, Class: socialhub.ClassUserAction, Platform: "youtube", Product: "youtube-data", Op: operation,
		RequiredScopes: []string{scope}, ApprovalURL: "https://console.cloud.google.com/apis/credentials/consent", PlatformMessage: "configured approval scopes do not include " + scope,
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
