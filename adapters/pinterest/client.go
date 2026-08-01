package pinterest

import (
	"context"
	"net/http"
	"sync"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

// Client implements the common capabilities supported by Pinterest API v5.
type Client struct {
	accountID  socialhub.AccountID
	userID     string
	transport  *transport.Client
	httpClient *http.Client
	scopes     []string
	clock      socialhub.Clock
	pins       *PinService
	uploadMu   sync.Mutex
	uploads    map[string]*videoUpload
}

func (c *Client) Platform() socialhub.Platform { return "pinterest" }
func (c *Client) Account() socialhub.AccountID { return c.accountID }

func (c *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	return socialhub.Capabilities{
		CapabilityPinWorkflow: capabilityState(CapabilityPinWorkflow, true, c.scopes, []string{"boards:read", "boards:write", "pins:read", "pins:write"}, "board-aware image and video Pin creation uses PinWorkflow", docURL+"pins-create/"),
		socialhub.CapPublish:  {Capability: socialhub.CapPublish, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "use PinWorkflow; common posts cannot express Pinterest board and media-source requirements"},
		socialhub.CapFetch:    capabilityState(socialhub.CapFetch, true, c.scopes, []string{"user_accounts:read", "boards:read", "pins:read"}, "authorized account and owned Pins are readable; Pin comments are not exposed", docURL+"pins-list/"),
		socialhub.CapMedia:    {Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "video registration and signed multipart upload are part of PinWorkflow"},
		socialhub.CapReact:    {Capability: socialhub.CapReact, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Pinterest API v5 does not expose common like or comment mutation endpoints"},
		socialhub.CapMessage:  {Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Pinterest API v5 does not expose direct messaging"},
		socialhub.CapWebhook:  {Capability: socialhub.CapWebhook, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "organic Pin webhooks are not part of the public v5 contract"},
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

func (c *Client) Publisher() (socialhub.Publisher, bool)           { return nil, false }
func (c *Client) Fetcher() (socialhub.Fetcher, bool)               { return c, true }
func (c *Client) MediaUploader() (socialhub.MediaUploader, bool)   { return nil, false }
func (c *Client) Reactor() (socialhub.Reactor, bool)               { return nil, false }
func (c *Client) Messenger() (socialhub.Messenger, bool)           { return nil, false }
func (c *Client) WebhookHandler() (socialhub.WebhookHandler, bool) { return nil, false }
func (c *Client) Close() error                                     { return nil }

// PinWorkflow returns Pinterest's typed Pin and video-upload workflow.
func (c *Client) PinWorkflow() PinWorkflow { return c.pins }

func (c *Client) requireScopes(operation string, scopes ...string) error {
	if len(c.scopes) == 0 {
		return nil
	}
	var missing []string
	for _, scope := range scopes {
		if !contains(c.scopes, scope) {
			missing = append(missing, scope)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	return &socialhub.Error{
		Code: socialhub.CodeApprovalRequired, Class: socialhub.ClassUserAction, Platform: "pinterest", Product: "pinterest-rest", Op: operation,
		RequiredScopes: missing, ApprovalURL: "https://developers.pinterest.com/apps/", PlatformMessage: "configured approval scopes are incomplete",
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
