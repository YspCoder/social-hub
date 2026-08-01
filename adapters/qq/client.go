package qq

import (
	"context"
	"errors"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	// CapabilityTypedMessages exposes scene-aware QQ message operations.
	CapabilityTypedMessages socialhub.Capability = "qq_typed_messages"
	// CapabilityURLMedia exposes scene-bound URL media uploads.
	CapabilityURLMedia socialhub.Capability = "qq_url_media"
)

type tokenInvalidator interface {
	Invalidate(context.Context)
}

// Client implements the supported QQ Bot capabilities for one bot account.
type Client struct {
	accountID     socialhub.AccountID
	appID         string
	api           *transport.Client
	clock         socialhub.Clock
	invalidator   tokenInvalidator
	webhookSecret string
}

func (c *Client) Platform() socialhub.Platform { return "qq" }
func (c *Client) Account() socialhub.AccountID { return c.accountID }

func (c *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	webhook := c.webhookSecret != ""
	webhookReason := "configure secret_ref or webhook.secret_ref with the QQ bot AppSecret"
	if webhook {
		webhookReason = "verifies Ed25519-signed QQ event callbacks and validation payloads"
	}
	return socialhub.Capabilities{
		socialhub.CapPublish:    {Capability: socialhub.CapPublish, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "QQ bot messages are not social posts"},
		socialhub.CapFetch:      {Capability: socialhub.CapFetch, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "QQ Bot OpenAPI does not expose arbitrary user or message history"},
		socialhub.CapMedia:      {Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "media uploads are bound to one C2C or group target"},
		socialhub.CapReact:      {Capability: socialhub.CapReact, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "channel reactions require channel and message identifiers outside the common target contract"},
		socialhub.CapMessage:    {Capability: socialhub.CapMessage, Supported: true, Approval: socialhub.ApprovalUnknown, Reason: "sends text to C2C, group, or channel targets", DocURL: docURL + "server-inter/message/overview.html"},
		socialhub.CapWebhook:    {Capability: socialhub.CapWebhook, Supported: webhook, Approval: socialhub.ApprovalUnknown, Reason: webhookReason, DocURL: docURL + "dev-prepare/event-emit/webhook.html"},
		CapabilityTypedMessages: {Capability: CapabilityTypedMessages, Supported: true, Approval: socialhub.ApprovalUnknown, Reason: "preserves QQ scene, reply, wakeup, Markdown, and media semantics", DocURL: docURL + "server-inter/message/overview.html"},
		CapabilityURLMedia:      {Capability: CapabilityURLMedia, Supported: true, Approval: socialhub.ApprovalUnknown, Reason: "uploads public media URLs into one C2C or group scene", DocURL: docURL + "server-inter/message/rich-media.html"},
	}, nil
}

func (c *Client) Publisher() (socialhub.Publisher, bool)         { return nil, false }
func (c *Client) Fetcher() (socialhub.Fetcher, bool)             { return nil, false }
func (c *Client) MediaUploader() (socialhub.MediaUploader, bool) { return nil, false }
func (c *Client) Reactor() (socialhub.Reactor, bool)             { return nil, false }
func (c *Client) Messenger() (socialhub.Messenger, bool)         { return c, true }
func (c *Client) WebhookHandler() (socialhub.WebhookHandler, bool) {
	if c.webhookSecret == "" {
		return nil, false
	}
	return c, true
}
func (c *Client) Close() error { return nil }

// MessageWorkflow returns QQ scene-aware send and retract operations.
func (c *Client) MessageWorkflow() MessageWorkflow { return c }

// MediaWorkflow returns QQ C2C/group URL media upload operations.
func (c *Client) MediaWorkflow() URLMediaWorkflow { return c }

// WebhookWorkflow returns QQ's POST validation-response helper.
func (c *Client) WebhookWorkflow() WebhookWorkflow { return c }

func (c *Client) responseError(ctx context.Context, operation string, response APIError) error {
	if response.EffectiveCode() == 0 {
		return nil
	}
	err := response.Err(operation)
	var platformErr *socialhub.Error
	if c.invalidator != nil && errors.As(err, &platformErr) && platformErr.Code == socialhub.CodeUnauthenticated {
		c.invalidator.Invalidate(ctx)
	}
	return err
}

var _ socialhub.Client = (*Client)(nil)
var _ socialhub.Messenger = (*Client)(nil)
