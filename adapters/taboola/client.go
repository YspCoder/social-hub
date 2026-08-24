package taboola

import (
	"context"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	CapabilityCampaignManagement socialhub.Capability = "taboola_campaign_management"
	CapabilityReporting          socialhub.Capability = "taboola_campaign_reporting"
)

// Client exposes one Taboola advertiser account's paid-media workflows.
type Client struct {
	accountID    socialhub.AccountID
	advertiserID string
	api          *transport.Client
}

func (client *Client) Platform() socialhub.Platform { return platformName }
func (client *Client) Account() socialhub.AccountID { return client.accountID }

func (client *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	return socialhub.Capabilities{
		CapabilityCampaignManagement: {
			Capability: CapabilityCampaignManagement, Supported: true, Approval: socialhub.ApprovalUnknown,
			Reason: "advertiser account, paused-first Campaign, and Campaign Item workflows; credentials require Taboola account-manager approval",
			DocURL: documentationURL + "docs/campaigns-overview",
		},
		CapabilityReporting: {
			Capability: CapabilityReporting, Supported: true, Approval: socialhub.ApprovalUnknown,
			Reason: "Campaign Summary and Realtime Campaign reports; realtime reporting is limited to 10 requests per minute",
			DocURL: documentationURL + "docs/reporting-overview",
		},
		socialhub.CapPublish: {Capability: socialhub.CapPublish, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "paid campaigns are not organic social posts; use Campaigns() and Items()"},
		socialhub.CapFetch:   {Capability: socialhub.CapFetch, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "advertising reads use typed Backstage API resources"},
		socialhub.CapMedia:   {Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "the initial adapter creates static Campaign Items from destination URLs"},
		socialhub.CapReact:   {Capability: socialhub.CapReact, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Backstage API is not an organic engagement product"},
		socialhub.CapMessage: {Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Backstage API does not expose general messaging"},
		socialhub.CapWebhook: {Capability: socialhub.CapWebhook, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "webhooks are outside the initial adapter contract"},
	}, nil
}

func (client *Client) Publisher() (socialhub.Publisher, bool)         { return nil, false }
func (client *Client) Fetcher() (socialhub.Fetcher, bool)             { return nil, false }
func (client *Client) MediaUploader() (socialhub.MediaUploader, bool) { return nil, false }
func (client *Client) Reactor() (socialhub.Reactor, bool)             { return nil, false }
func (client *Client) Messenger() (socialhub.Messenger, bool)         { return nil, false }
func (client *Client) WebhookHandler() (socialhub.WebhookHandler, bool) {
	return nil, false
}
func (client *Client) Close() error { return nil }

func (client *Client) Accounts() AccountWorkflow   { return client }
func (client *Client) Campaigns() CampaignWorkflow { return client }
func (client *Client) Items() ItemWorkflow         { return client }
func (client *Client) Reports() ReportWorkflow     { return client }

var _ socialhub.Client = (*Client)(nil)
