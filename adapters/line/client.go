package line

import (
	"context"
	"net/http"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	CapabilityTypedMessages socialhub.Capability = "line_messages"
	CapabilityProfiles      socialhub.Capability = "line_profiles"
	CapabilityContent       socialhub.Capability = "line_message_content"
	CapabilityQuota         socialhub.Capability = "line_message_quota"
)

// Client implements LINE messaging, profile, content, quota, and webhook
// workflows for one Messaging API channel.
type Client struct {
	accountID     socialhub.AccountID
	botUserID     string
	api           *transport.Client
	data          *transport.Client
	httpClient    *http.Client
	channelSecret string
	clock         socialhub.Clock
}

func (c *Client) Platform() socialhub.Platform { return "line" }
func (c *Client) Account() socialhub.AccountID { return c.accountID }

func (c *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	webhookReason := "configure secret_ref with the channel secret for X-Line-Signature verification"
	if c.channelSecret != "" {
		webhookReason = "signed Messaging API webhook events"
	}
	return socialhub.Capabilities{
		socialhub.CapPublish:    {Capability: socialhub.CapPublish, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "LINE Official Accounts expose conversations, not generic social posts"},
		socialhub.CapFetch:      {Capability: socialhub.CapFetch, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Messaging API does not expose arbitrary message history, posts, or comments"},
		socialhub.CapMedia:      {Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "outbound media uses HTTPS URLs and inbound content is download-only"},
		socialhub.CapReact:      {Capability: socialhub.CapReact, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Messaging API does not expose generic post reactions"},
		socialhub.CapMessage:    capability(socialhub.CapMessage, "text push messages through the common Messenger; use MessageWorkflow for reply, multicast, broadcast, and rich media"),
		socialhub.CapWebhook:    {Capability: socialhub.CapWebhook, Supported: c.channelSecret != "", Approval: socialhub.ApprovalUnknown, Reason: webhookReason, DocURL: docURL + "#webhooks"},
		CapabilityTypedMessages: capability(CapabilityTypedMessages, "typed reply, push, multicast, broadcast, and rich media messages"),
		CapabilityProfiles:      capability(CapabilityProfiles, "user, group-member, and room-member profiles"),
		CapabilityContent:       capability(CapabilityContent, "streaming inbound message content and preview downloads"),
		CapabilityQuota:         capability(CapabilityQuota, "monthly message quota and consumption"),
	}, nil
}

func capability(name socialhub.Capability, reason string) socialhub.CapabilityState {
	return socialhub.CapabilityState{
		Capability: name, Supported: true, Approval: socialhub.ApprovalUnknown,
		Reason: reason, DocURL: docURL,
	}
}

func (c *Client) Publisher() (socialhub.Publisher, bool)         { return nil, false }
func (c *Client) Fetcher() (socialhub.Fetcher, bool)             { return nil, false }
func (c *Client) MediaUploader() (socialhub.MediaUploader, bool) { return nil, false }
func (c *Client) Reactor() (socialhub.Reactor, bool)             { return nil, false }
func (c *Client) Messenger() (socialhub.Messenger, bool)         { return c, true }
func (c *Client) WebhookHandler() (socialhub.WebhookHandler, bool) {
	if c.channelSecret == "" {
		return nil, false
	}
	return c, true
}
func (c *Client) Close() error { return nil }

func (c *Client) MessageWorkflow() MessageWorkflow { return c }
func (c *Client) ProfileWorkflow() ProfileWorkflow { return c }
func (c *Client) ContentWorkflow() ContentWorkflow { return c }
func (c *Client) QuotaWorkflow() QuotaWorkflow     { return c }

var _ socialhub.Client = (*Client)(nil)
var _ socialhub.Messenger = (*Client)(nil)
