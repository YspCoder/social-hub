package outbrain

import (
	"context"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	CapabilityCampaignManagement socialhub.Capability = "outbrain_campaign_management"
	CapabilityReporting          socialhub.Capability = "outbrain_campaign_reporting"
)

// Client exposes one Outbrain marketer's paid-media workflows.
type Client struct {
	accountID  socialhub.AccountID
	marketerID string
	api        *transport.Client
}

func (client *Client) Platform() socialhub.Platform { return platformName }
func (client *Client) Account() socialhub.AccountID { return client.accountID }

func (client *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	return socialhub.Capabilities{
		CapabilityCampaignManagement: {
			Capability: CapabilityCampaignManagement, Supported: true, Approval: socialhub.ApprovalUnknown,
			Reason: "marketer, Budget, paused-first Campaign, and PromotedLink workflows; Amplify API access requires Outbrain approval",
			DocURL: documentationURL + "#reference/overview-and-entities",
		},
		CapabilityReporting: {
			Capability: CapabilityReporting, Supported: true, Approval: socialhub.ApprovalUnknown,
			Reason: "Campaign, Promoted Content, and periodic performance reports",
			DocURL: documentationURL + "#reference/performance-reporting",
		},
		socialhub.CapPublish: {Capability: socialhub.CapPublish, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "paid promotion is not organic publishing; use Campaigns() and PromotedLinks()"},
		socialhub.CapFetch:   {Capability: socialhub.CapFetch, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "advertising reads use typed Amplify API resources"},
		socialhub.CapMedia:   {Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "the initial adapter creates PromotedLinks from hosted image URLs"},
		socialhub.CapReact:   {Capability: socialhub.CapReact, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Amplify API is not an organic engagement product"},
		socialhub.CapMessage: {Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Amplify API does not expose general messaging"},
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

func (client *Client) Marketers() MarketerWorkflow         { return client }
func (client *Client) Budgets() BudgetWorkflow             { return client }
func (client *Client) Campaigns() CampaignWorkflow         { return client }
func (client *Client) PromotedLinks() PromotedLinkWorkflow { return client }
func (client *Client) Reports() ReportWorkflow             { return client }

var _ socialhub.Client = (*Client)(nil)
