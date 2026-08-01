package xiaohongshu

import (
	"context"
	"net/http"
	"sync"

	"social-hub/pkg/socialhub"
)

// Client exposes the official Share JS SDK handoff without pretending it is a
// server-side social content API.
type Client struct {
	accountID  socialhub.AccountID
	appKey     string
	appSecret  string
	baseURL    string
	httpClient *http.Client
	clock      socialhub.Clock
	tokenStore socialhub.TokenStore
	approved   bool

	tokenMu sync.Mutex
	token   socialhub.Token
	shares  *ShareService
}

func (c *Client) Platform() socialhub.Platform { return "xiaohongshu" }
func (c *Client) Account() socialhub.AccountID { return c.accountID }

func (c *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	approval := socialhub.ApprovalRequired
	reason := "Share Open Platform onboarding is currently paused; an existing approved app is required"
	if c.approved {
		approval = socialhub.ApprovalGranted
		reason = "approved apps can prepare a media-only handoff to the Xiaohongshu client Share JS SDK"
	}
	return socialhub.Capabilities{
		CapabilityShare:      {Capability: CapabilityShare, Supported: true, Approval: approval, Reason: reason, DocURL: docURL},
		socialhub.CapPublish: {Capability: socialhub.CapPublish, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "the public Share SDK requires an interactive client-side handoff and does not expose server-side note publication"},
		socialhub.CapFetch:   {Capability: socialhub.CapFetch, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "the Share Open Platform does not expose user, note, or feed reads"},
		socialhub.CapMedia:   {Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "share media must already be hosted at application-controlled URLs"},
		socialhub.CapReact:   {Capability: socialhub.CapReact, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "reactions and comments are not Share SDK capabilities"},
		socialhub.CapMessage: {Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "messages are not Share SDK capabilities"},
		socialhub.CapWebhook: {Capability: socialhub.CapWebhook, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "the Share JS SDK documents no server webhook contract"},
	}, nil
}

func (c *Client) Publisher() (socialhub.Publisher, bool)           { return nil, false }
func (c *Client) Fetcher() (socialhub.Fetcher, bool)               { return nil, false }
func (c *Client) MediaUploader() (socialhub.MediaUploader, bool)   { return nil, false }
func (c *Client) Reactor() (socialhub.Reactor, bool)               { return nil, false }
func (c *Client) Messenger() (socialhub.Messenger, bool)           { return nil, false }
func (c *Client) WebhookHandler() (socialhub.WebhookHandler, bool) { return nil, false }
func (c *Client) Close() error                                     { return nil }

// ShareWorkflow returns the typed client-side share handoff service.
func (c *Client) ShareWorkflow() ShareWorkflow { return c.shares }

var _ socialhub.Client = (*Client)(nil)
