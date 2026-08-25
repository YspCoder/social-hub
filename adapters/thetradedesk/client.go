package thetradedesk

import (
	"context"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	CapabilityAdvertiserRead      socialhub.Capability = "thetradedesk_advertiser_read"
	CapabilityCampaignRead        socialhub.Capability = "thetradedesk_campaign_read"
	CapabilityCampaignManagement  socialhub.Capability = "thetradedesk_campaign_management"
	CapabilityShortLivedTokenAuth socialhub.Capability = "thetradedesk_short_lived_token_auth"
)

// Client exposes one advertiser's Platform API workflows.
type Client struct {
	accountID    socialhub.AccountID
	advertiserID string
	api          *transport.Client
	approval     socialhub.ApprovalConfig
	managedAuth  bool
	tokens       closableTokenSource
	requestIDs   *requestIDFilter
}

func (client *Client) Platform() socialhub.Platform { return platformName }
func (client *Client) Account() socialhub.AccountID { return client.accountID }

func (client *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	approval := socialhub.ApprovalRequired
	if client.approval.AccountType == productName {
		approval = socialhub.ApprovalGranted
	}
	authApproval := approval
	authReason := "password-based token generation up to 1440 minutes; long-lived UI-generated tokens are recommended for production"
	if !client.managedAuth {
		authApproval = socialhub.ApprovalUnknown
		authReason = "configured account uses an externally managed long-lived token"
	}
	state := func(capability socialhub.Capability, reason, docURL string) socialhub.CapabilityState {
		return socialhub.CapabilityState{
			Capability: capability, Supported: true, Approval: approval,
			Reason: reason, DocURL: docURL,
		}
	}
	return socialhub.Capabilities{
		CapabilityAdvertiserRead: state(
			CapabilityAdvertiserRead,
			"Platform API account and entity access required; current REST operation is marked legacy with migration date 2027-01-11",
			documentationURL,
		),
		CapabilityCampaignRead: state(
			CapabilityCampaignRead,
			"advertiser-scoped Campaign lookup and bounded paginated query; current REST operations are marked legacy with migration date 2027-01-11",
			"https://partner.thetradedesk.com/v3/portal/api/ref/get-campaign-campaignid",
		),
		CapabilityCampaignManagement: state(
			CapabilityCampaignManagement,
			"single Campaign creation and partial updates; account permissions and billing setup required; current REST operations are marked legacy with migration date 2027-01-11",
			"https://partner.thetradedesk.com/v3/portal/api/doc/Campaigns",
		),
		socialhub.CapPublish: {Capability: socialhub.CapPublish, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "programmatic advertising is separate from organic publishing"},
		socialhub.CapFetch:   {Capability: socialhub.CapFetch, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "paid-media reads use typed advertiser-scoped workflows"},
		socialhub.CapMedia:   {Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Creative upload is outside this initial Platform API adapter"},
		socialhub.CapReact:   {Capability: socialhub.CapReact, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "The Trade Desk has no organic reaction surface"},
		socialhub.CapMessage: {Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "The Trade Desk has no general messaging surface"},
		socialhub.CapWebhook: {Capability: socialhub.CapWebhook, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "webhooks are outside these Platform API workflows"},
		CapabilityShortLivedTokenAuth: {
			Capability: CapabilityShortLivedTokenAuth, Supported: client.managedAuth, Approval: authApproval,
			Reason: authReason,
			DocURL: "https://partner.thetradedesk.com/v3/portal/api/doc/AuthenticationShortLive",
		},
	}, nil
}

func (client *Client) Publisher() (socialhub.Publisher, bool)           { return nil, false }
func (client *Client) Fetcher() (socialhub.Fetcher, bool)               { return nil, false }
func (client *Client) MediaUploader() (socialhub.MediaUploader, bool)   { return nil, false }
func (client *Client) Reactor() (socialhub.Reactor, bool)               { return nil, false }
func (client *Client) Messenger() (socialhub.Messenger, bool)           { return nil, false }
func (client *Client) WebhookHandler() (socialhub.WebhookHandler, bool) { return nil, false }
func (client *Client) Close() error {
	if client.tokens != nil {
		client.tokens.Close()
	}
	if client.requestIDs != nil {
		client.requestIDs.clear()
	}
	return nil
}

func (client *Client) Advertisers() AdvertiserWorkflow { return client }
func (client *Client) Campaigns() CampaignWorkflow     { return client }

var _ socialhub.Client = (*Client)(nil)
