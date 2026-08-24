package ironsourcereporting

import (
	"context"
	"net/http"
	"net/url"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	CapabilityAdvertiserReporting socialhub.Capability = "ironsource_advertiser_reporting"
	CapabilityCostReporting       socialhub.Capability = "ironsource_cost_reporting"
	CapabilitySKANReporting       socialhub.Capability = "ironsource_skan_reporting"
)

// Client exposes one ironSource advertiser account's v4 reporting workflows.
type Client struct {
	accountID  socialhub.AccountID
	api        *transport.Client
	httpClient *http.Client
	baseURL    *url.URL
	clock      socialhub.Clock
}

func (client *Client) Platform() socialhub.Platform { return platformName }
func (client *Client) Account() socialhub.AccountID { return client.accountID }

func (client *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	return socialhub.Capabilities{
		CapabilityAdvertiserReporting: {
			Capability: CapabilityAdvertiserReporting, Supported: true, Approval: socialhub.ApprovalRequired,
			Reason: "campaign delivery and MMP-reported install metrics; ironSource advertiser API access and a Bearer token are required",
			DocURL: documentationURL,
		},
		CapabilityCostReporting: {
			Capability: CapabilityCostReporting, Supported: true, Approval: socialhub.ApprovalRequired,
			Reason: "billable installs, spend, and eCPI are intentionally separate from MMP-reported advertiser statistics",
			DocURL: costDocumentationURL,
		},
		CapabilitySKANReporting: {
			Capability: CapabilitySKANReporting, Supported: true, Approval: socialhub.ApprovalRequired,
			Reason: "SKAdNetwork delivery and dedicated conversion-value reports for authorized iOS campaigns",
			DocURL: skanDocumentationURL,
		},
		socialhub.CapPublish: {Capability: socialhub.CapPublish, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "advertiser reporting v4 is read-only"},
		socialhub.CapFetch:   {Capability: socialhub.CapFetch, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "paid-media reports use the typed Reports workflow"},
		socialhub.CapMedia:   {Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "advertiser reporting v4 does not upload creative assets"},
		socialhub.CapReact:   {Capability: socialhub.CapReact, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "advertising reports have no organic engagement surface"},
		socialhub.CapMessage: {Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "advertising reports have no messaging surface"},
		socialhub.CapWebhook: {Capability: socialhub.CapWebhook, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "advertiser reporting v4 does not expose webhooks"},
	}, nil
}

func (client *Client) Publisher() (socialhub.Publisher, bool)           { return nil, false }
func (client *Client) Fetcher() (socialhub.Fetcher, bool)               { return nil, false }
func (client *Client) MediaUploader() (socialhub.MediaUploader, bool)   { return nil, false }
func (client *Client) Reactor() (socialhub.Reactor, bool)               { return nil, false }
func (client *Client) Messenger() (socialhub.Messenger, bool)           { return nil, false }
func (client *Client) WebhookHandler() (socialhub.WebhookHandler, bool) { return nil, false }
func (client *Client) Close() error                                     { return nil }

func (client *Client) Reports() ReportsWorkflow { return client }

var _ socialhub.Client = (*Client)(nil)
