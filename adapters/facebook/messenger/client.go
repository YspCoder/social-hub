package messenger

import (
	"context"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	CapabilityUserProfile  socialhub.Capability = "facebook_messenger_user_profile"
	CapabilityMediaMessage socialhub.Capability = "facebook_messenger_media_message"
)

// Client implements Page-scoped Messenger Platform workflows.
type Client struct {
	accountID     socialhub.AccountID
	pageID        string
	api           *transport.Client
	clock         socialhub.Clock
	webhookSecret string
	webhookToken  string
}

func (c *Client) Platform() socialhub.Platform { return "facebook" }
func (c *Client) Account() socialhub.AccountID { return c.accountID }

func (c *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	webhookEnabled := c.webhookSecret != ""
	webhookReason := "configure webhook.secret_ref with the Meta app secret"
	if webhookEnabled {
		webhookReason = "verifies X-Hub-Signature-256 and decodes Page messaging events"
	}
	return socialhub.Capabilities{
		socialhub.CapPublish: {Capability: socialhub.CapPublish, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Messenger messages are conversations, not Page posts"},
		socialhub.CapFetch: {
			Capability: socialhub.CapFetch, Supported: true, Approval: socialhub.ApprovalUnknown,
			Reason: "reads PSID-scoped profiles; posts, comments, and message history are unavailable through the common Fetcher",
			Scopes: []string{"business_asset_user_profile_access"}, DocURL: docURL + "/identity/user-profile",
		},
		socialhub.CapMedia: {Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "remote and reusable attachments use the typed MessageWorkflow"},
		socialhub.CapReact: {Capability: socialhub.CapReact, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Messenger reactions are inbound webhook events, not a documented outbound reaction API"},
		socialhub.CapMessage: {
			Capability: socialhub.CapMessage, Supported: true, Approval: socialhub.ApprovalUnknown,
			Scopes: []string{"pages_messaging"}, Reason: "sends Page-scoped replies inside Meta's permitted messaging window", DocURL: docURL + "/send-messages",
		},
		socialhub.CapWebhook: {
			Capability: socialhub.CapWebhook, Supported: webhookEnabled, Approval: socialhub.ApprovalUnknown,
			Scopes: []string{"pages_messaging", "pages_manage_metadata"}, Reason: webhookReason, DocURL: docURL + "/webhooks",
		},
		CapabilityUserProfile:  capability(CapabilityUserProfile, "reads the basic PSID-scoped user profile after feature approval", docURL+"/identity/user-profile"),
		CapabilityMediaMessage: capability(CapabilityMediaMessage, "sends typed remote or reusable media attachments", docURL+"/send-messages"),
	}, nil
}

func capability(name socialhub.Capability, reason, reference string) socialhub.CapabilityState {
	return socialhub.CapabilityState{Capability: name, Supported: true, Approval: socialhub.ApprovalUnknown, Reason: reason, DocURL: reference}
}

func (c *Client) Publisher() (socialhub.Publisher, bool)         { return nil, false }
func (c *Client) Fetcher() (socialhub.Fetcher, bool)             { return c, true }
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

// MessageWorkflow returns the Messenger-specific text and attachment sender.
func (c *Client) MessageWorkflow() MessageWorkflow { return c }

// ProfileWorkflow returns the PSID profile reader.
func (c *Client) ProfileWorkflow() ProfileWorkflow { return c }

var _ socialhub.Client = (*Client)(nil)
var _ socialhub.Fetcher = (*Client)(nil)
var _ socialhub.Messenger = (*Client)(nil)
var _ MessageWorkflow = (*Client)(nil)
var _ ProfileWorkflow = (*Client)(nil)
