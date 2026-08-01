package wecom

import (
	"context"
	"sync"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	// CapabilityApplicationMessages exposes typed WeCom application messages.
	CapabilityApplicationMessages socialhub.Capability = "wecom_application_messages"
	// CapabilityTemporaryMedia exposes three-day WeCom temporary media uploads.
	CapabilityTemporaryMedia socialhub.Capability = "wecom_temporary_media"
)

type tokenInvalidator interface {
	Invalidate(context.Context)
}

// Client implements the supported WeCom capability interfaces for one app.
type Client struct {
	accountID socialhub.AccountID
	corpID    string
	agentID   int64
	api       *transport.Client
	clock     socialhub.Clock

	invalidator  tokenInvalidator
	webhookToken string
	aesKey       string
	defaults     RecipientSet

	uploadMu sync.Mutex
	uploads  map[string]*uploadState
}

func (c *Client) Platform() socialhub.Platform { return "wecom" }
func (c *Client) Account() socialhub.AccountID { return c.accountID }

func (c *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	webhook := c.webhookToken != "" && c.aesKey != ""
	webhookReason := ""
	if !webhook {
		webhookReason = "configure webhook.token_ref and webhook.aes_key_ref together"
	}
	return socialhub.Capabilities{
		socialhub.CapPublish:          {Capability: socialhub.CapPublish, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "WeCom application messages are not social posts"},
		socialhub.CapFetch:            {Capability: socialhub.CapFetch, Supported: true, Approval: socialhub.ApprovalUnknown, Reason: "reads members visible to the self-built application", DocURL: "https://developer.work.weixin.qq.com/document/path/90196"},
		socialhub.CapMedia:            {Capability: socialhub.CapMedia, Supported: true, Approval: socialhub.ApprovalUnknown, Reason: "uploads three-day temporary image, voice, video, and file media", DocURL: "https://developer.work.weixin.qq.com/document/path/90253"},
		socialhub.CapReact:            {Capability: socialhub.CapReact, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "the self-built application API has no common social reaction contract"},
		socialhub.CapMessage:          {Capability: socialhub.CapMessage, Supported: true, Approval: socialhub.ApprovalUnknown, Reason: "common Messenger sends text; typed workflow supports richer application messages", DocURL: "https://developer.work.weixin.qq.com/document/path/90236"},
		socialhub.CapWebhook:          {Capability: socialhub.CapWebhook, Supported: webhook, Approval: socialhub.ApprovalUnknown, Reason: webhookReason, DocURL: "https://developer.work.weixin.qq.com/document/path/90930"},
		CapabilityApplicationMessages: {Capability: CapabilityApplicationMessages, Supported: true, Approval: socialhub.ApprovalUnknown, Reason: "typed recipients and content for self-built application messages", DocURL: "https://developer.work.weixin.qq.com/document/path/90236"},
		CapabilityTemporaryMedia:      {Capability: CapabilityTemporaryMedia, Supported: true, Approval: socialhub.ApprovalUnknown, Reason: "temporary media IDs expire after three days", DocURL: "https://developer.work.weixin.qq.com/document/path/90253"},
	}, nil
}

func (c *Client) Publisher() (socialhub.Publisher, bool)         { return nil, false }
func (c *Client) Fetcher() (socialhub.Fetcher, bool)             { return c, true }
func (c *Client) MediaUploader() (socialhub.MediaUploader, bool) { return c, true }
func (c *Client) Reactor() (socialhub.Reactor, bool)             { return nil, false }
func (c *Client) Messenger() (socialhub.Messenger, bool)         { return c, true }
func (c *Client) WebhookHandler() (socialhub.WebhookHandler, bool) {
	if c.webhookToken == "" || c.aesKey == "" {
		return nil, false
	}
	return c, true
}
func (c *Client) Close() error { return nil }

// ApplicationMessages returns WeCom-specific typed message operations.
func (c *Client) ApplicationMessages() ApplicationMessageWorkflow { return c }

func (c *Client) responseError(ctx context.Context, operation string, response APIResponse) error {
	if response.ErrCode == 0 {
		return nil
	}
	if isTokenError(response.ErrCode) && c.invalidator != nil {
		c.invalidator.Invalidate(ctx)
	}
	return response.Err(operation)
}

var _ socialhub.Client = (*Client)(nil)
var _ socialhub.Fetcher = (*Client)(nil)
var _ socialhub.MediaUploader = (*Client)(nil)
var _ socialhub.Messenger = (*Client)(nil)
