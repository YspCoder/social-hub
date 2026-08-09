package criteo

import (
	"context"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	CapabilityAdvertiserAccess   socialhub.Capability = "criteo_advertiser_access"
	CapabilityCampaignManagement socialhub.Capability = "criteo_campaign_management"
	CapabilityReporting          socialhub.Capability = "criteo_statistics_reporting"
)

// Client exposes one advertiser's Marketing Solutions workflows.
type Client struct {
	accountID    socialhub.AccountID
	advertiserID string
	api          *transport.Client
}

func (client *Client) Platform() socialhub.Platform { return platformName }
func (client *Client) Account() socialhub.AccountID { return client.accountID }

func (client *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	return socialhub.Capabilities{
		CapabilityAdvertiserAccess: {
			Capability: CapabilityAdvertiserAccess, Supported: true, Approval: socialhub.ApprovalUnknown,
			Reason: "portfolio lookup and configured advertiser ownership validation",
			DocURL: documentationURL + "docs/getting-started-1",
		},
		CapabilityCampaignManagement: {
			Capability: CapabilityCampaignManagement, Supported: true, Approval: socialhub.ApprovalUnknown,
			Reason: "Campaign and off-first Ad Set create, search, patch, start, and stop workflows; Campaign Read/Manage scopes required",
			DocURL: documentationURL + "docs/campaign",
		},
		CapabilityReporting: {
			Capability: CapabilityReporting, Supported: true, Approval: socialhub.ApprovalUnknown,
			Reason: "advertiser-scoped JSON Statistics reports; Analytics Read scope required",
			DocURL: documentationURL + "docs/analytics",
		},
		socialhub.CapPublish: {Capability: socialhub.CapPublish, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Criteo Marketing Solutions is paid media, not organic publishing"},
		socialhub.CapFetch:   {Capability: socialhub.CapFetch, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "advertising reads use typed Criteo resources"},
		socialhub.CapMedia:   {Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "media and creative endpoints are outside this adapter's 2026-01 scope"},
		socialhub.CapReact:   {Capability: socialhub.CapReact, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Criteo does not expose organic reactions"},
		socialhub.CapMessage: {Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Criteo does not expose general messaging"},
		socialhub.CapWebhook: {Capability: socialhub.CapWebhook, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "these Marketing Solutions workflows do not expose webhooks"},
	}, nil
}

func (client *Client) Publisher() (socialhub.Publisher, bool)           { return nil, false }
func (client *Client) Fetcher() (socialhub.Fetcher, bool)               { return nil, false }
func (client *Client) MediaUploader() (socialhub.MediaUploader, bool)   { return nil, false }
func (client *Client) Reactor() (socialhub.Reactor, bool)               { return nil, false }
func (client *Client) Messenger() (socialhub.Messenger, bool)           { return nil, false }
func (client *Client) WebhookHandler() (socialhub.WebhookHandler, bool) { return nil, false }
func (client *Client) Close() error                                     { return nil }

func (client *Client) Advertisers() AdvertiserWorkflow { return client }
func (client *Client) Campaigns() CampaignWorkflow     { return client }
func (client *Client) AdSets() AdSetWorkflow           { return client }
func (client *Client) Statistics() StatisticsWorkflow  { return client }

var _ socialhub.Client = (*Client)(nil)
