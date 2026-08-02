package viber

import (
	"context"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	CapabilityTypedMessages     socialhub.Capability = "viber_typed_messages"
	CapabilityBroadcast         socialhub.Capability = "viber_broadcast"
	CapabilityProfiles          socialhub.Capability = "viber_profiles"
	CapabilityPresence          socialhub.Capability = "viber_presence"
	CapabilityWebhookManagement socialhub.Capability = "viber_webhook_management"
)

// Client implements the Viber Bot API workflows for one configured bot.
type Client struct {
	accountID socialhub.AccountID
	api       *transport.Client
	authToken string
	sender    Sender
	clock     socialhub.Clock
}

func (c *Client) Platform() socialhub.Platform { return "viber" }
func (c *Client) Account() socialhub.AccountID { return c.accountID }

func (c *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	return socialhub.Capabilities{
		socialhub.CapPublish:        {Capability: socialhub.CapPublish, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Viber Bot API exposes conversations, not generic social posts"},
		socialhub.CapFetch:          capability(socialhub.CapFetch, "retrieves the bot account and subscribed-user details; posts, comments, and message history are unavailable"),
		socialhub.CapMedia:          {Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "outbound Viber media uses caller-hosted URLs rather than an upload API"},
		socialhub.CapReact:          {Capability: socialhub.CapReact, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Bot API exposes delivery events but no generic reactions"},
		socialhub.CapMessage:        capability(socialhub.CapMessage, "sends text through the common Messenger; the typed workflow supports Viber media and interactive message types"),
		socialhub.CapWebhook:        capability(socialhub.CapWebhook, "verifies HMAC-SHA256 callbacks and decodes message, subscription, and delivery events"),
		CapabilityTypedMessages:     capability(CapabilityTypedMessages, "typed text, picture, video, file, contact, location, URL, and sticker messages"),
		CapabilityBroadcast:         capability(CapabilityBroadcast, "broadcasts one typed message to up to 300 subscribed users"),
		CapabilityProfiles:          capability(CapabilityProfiles, "reads bot account and subscribed-user details"),
		CapabilityPresence:          capability(CapabilityPresence, "reads online state for up to 100 subscribed users"),
		CapabilityWebhookManagement: capability(CapabilityWebhookManagement, "registers, filters, and removes the bot webhook"),
	}, nil
}

func capability(name socialhub.Capability, reason string) socialhub.CapabilityState {
	return socialhub.CapabilityState{
		Capability: name, Supported: true, Approval: socialhub.ApprovalUnknown,
		Reason: reason + "; new bots require Viber commercial provisioning", DocURL: docURL,
	}
}

func (c *Client) Publisher() (socialhub.Publisher, bool)         { return nil, false }
func (c *Client) Fetcher() (socialhub.Fetcher, bool)             { return c, true }
func (c *Client) MediaUploader() (socialhub.MediaUploader, bool) { return nil, false }
func (c *Client) Reactor() (socialhub.Reactor, bool)             { return nil, false }
func (c *Client) Messenger() (socialhub.Messenger, bool)         { return c, true }
func (c *Client) WebhookHandler() (socialhub.WebhookHandler, bool) {
	return c, true
}
func (c *Client) Close() error { return nil }

func (c *Client) MessageWorkflow() MessageWorkflow { return c }
func (c *Client) AccountWorkflow() AccountWorkflow { return c }
func (c *Client) WebhookWorkflow() WebhookWorkflow { return c }

var _ socialhub.Client = (*Client)(nil)
var _ socialhub.Fetcher = (*Client)(nil)
var _ socialhub.Messenger = (*Client)(nil)
var _ socialhub.WebhookHandler = (*Client)(nil)
