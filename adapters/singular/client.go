package singular

import (
	"context"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	CapabilityEvents    socialhub.Capability = "singular_s2s_events_v2"
	CapabilityWebEvents socialhub.Capability = "singular_web_s2s_events_v2"
	CapabilityRevenue   socialhub.Capability = "singular_s2s_revenue"
	CapabilityAdRevenue socialhub.Capability = "singular_s2s_ad_revenue"
)

// Client exposes Singular EVENT v2 for one configured app.
type Client struct {
	accountID socialhub.AccountID
	appID     string
	sdkKey    string
	api       *transport.Client
}

func (client *Client) Platform() socialhub.Platform { return platformName }
func (client *Client) Account() socialhub.AccountID { return client.accountID }

func (*Client) String() string   { return "singular.Client(<redacted credentials>)" }
func (*Client) GoString() string { return "singular.Client(<redacted credentials>)" }

func (client *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	return socialhub.Capabilities{
		CapabilityEvents: {
			Capability: CapabilityEvents, Supported: true, Approval: socialhub.ApprovalUnknown,
			Reason: "SDID-based EVENT v2 for mobile, Web, PC, console, Meta Quest, and CTV", DocURL: documentationURL,
		},
		CapabilityWebEvents: {
			Capability: CapabilityWebEvents, Supported: true, Approval: socialhub.ApprovalUnknown,
			Reason: "Web S2S page-visit conversion and engagement events", DocURL: "https://support.singular.net/hc/en-us/articles/52863243353627-Server-to-Server-Web-S2S-Implementation-Guide",
		},
		CapabilityRevenue: {
			Capability: CapabilityRevenue, Supported: true, Approval: socialhub.ApprovalUnknown,
			Reason: "business revenue and optional app-store validation fields", DocURL: documentationURL,
		},
		CapabilityAdRevenue: {
			Capability: CapabilityAdRevenue, Supported: true, Approval: socialhub.ApprovalUnknown,
			Reason: "impression-level ad-monetization revenue events", DocURL: documentationURL,
		},
		socialhub.CapPublish: {Capability: socialhub.CapPublish, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "attribution events are not organic posts"},
		socialhub.CapFetch:   {Capability: socialhub.CapFetch, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "EVENT v2 is an ingestion product"},
		socialhub.CapMedia:   {Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "EVENT v2 does not upload media"},
		socialhub.CapReact:   {Capability: socialhub.CapReact, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "EVENT v2 has no engagement surface"},
		socialhub.CapMessage: {Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "EVENT v2 has no messaging surface"},
		socialhub.CapWebhook: {Capability: socialhub.CapWebhook, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Singular postbacks are a separate product surface"},
	}, nil
}

func (client *Client) Publisher() (socialhub.Publisher, bool)           { return nil, false }
func (client *Client) Fetcher() (socialhub.Fetcher, bool)               { return nil, false }
func (client *Client) MediaUploader() (socialhub.MediaUploader, bool)   { return nil, false }
func (client *Client) Reactor() (socialhub.Reactor, bool)               { return nil, false }
func (client *Client) Messenger() (socialhub.Messenger, bool)           { return nil, false }
func (client *Client) WebhookHandler() (socialhub.WebhookHandler, bool) { return nil, false }
func (client *Client) Close() error                                     { return nil }

func (client *Client) Events() EventWorkflow { return client }

var _ socialhub.Client = (*Client)(nil)
