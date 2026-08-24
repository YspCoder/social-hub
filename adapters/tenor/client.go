package tenor

import (
	"context"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	CapabilityMediaDiscovery socialhub.Capability = "tenor_media_discovery"
	CapabilityCategoryRead   socialhub.Capability = "tenor_category_read"
	CapabilityPostLookup     socialhub.Capability = "tenor_post_lookup"
)

// Client exposes typed, read-only Tenor discovery workflows for an existing
// API key. Google stopped accepting new Tenor API clients in January 2026.
type Client struct {
	accountID socialhub.AccountID
	api       *transport.Client
}

func (client *Client) Platform() socialhub.Platform { return platformName }
func (client *Client) Account() socialhub.AccountID { return client.accountID }

func (client *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	return socialhub.Capabilities{
		CapabilityMediaDiscovery: {
			Capability: CapabilityMediaDiscovery, Supported: true, Approval: socialhub.ApprovalGranted,
			Reason: "GIF and sticker Search and Featured reads for an existing Tenor API key", DocURL: documentationURL,
		},
		CapabilityCategoryRead: {
			Capability: CapabilityCategoryRead, Supported: true, Approval: socialhub.ApprovalGranted,
			Reason: "localized featured and trending Tenor categories", DocURL: documentationURL,
		},
		CapabilityPostLookup: {
			Capability: CapabilityPostLookup, Supported: true, Approval: socialhub.ApprovalGranted,
			Reason: "lookup of up to 50 Tenor response-object IDs", DocURL: documentationURL,
		},
		socialhub.CapPublish: {Capability: socialhub.CapPublish, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Tenor API v2 discovery is read-only in this adapter"},
		socialhub.CapFetch:   {Capability: socialhub.CapFetch, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Tenor media discovery uses its typed workflow rather than social timelines"},
		socialhub.CapMedia:   {Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "media upload is outside the implemented Tenor API v2 surface"},
		socialhub.CapReact:   {Capability: socialhub.CapReact, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Register Share is a write and is intentionally excluded"},
		socialhub.CapMessage: {Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Tenor API v2 does not provide direct messaging"},
		socialhub.CapWebhook: {Capability: socialhub.CapWebhook, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "these Tenor API v2 resources do not define signed webhooks"},
	}, nil
}

func (client *Client) Publisher() (socialhub.Publisher, bool)           { return nil, false }
func (client *Client) Fetcher() (socialhub.Fetcher, bool)               { return nil, false }
func (client *Client) MediaUploader() (socialhub.MediaUploader, bool)   { return nil, false }
func (client *Client) Reactor() (socialhub.Reactor, bool)               { return nil, false }
func (client *Client) Messenger() (socialhub.Messenger, bool)           { return nil, false }
func (client *Client) WebhookHandler() (socialhub.WebhookHandler, bool) { return nil, false }
func (client *Client) Close() error                                     { return nil }

// Discovery returns the bounded Tenor API v2 read workflow.
func (client *Client) Discovery() DiscoveryWorkflow { return client }

var _ socialhub.Client = (*Client)(nil)
