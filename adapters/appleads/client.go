package appleads

import (
	"context"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	CapabilityAccountAccess      socialhub.Capability = "appleads_account_access"
	CapabilityCampaignManagement socialhub.Capability = "appleads_campaign_management"
	CapabilityCreativeManagement socialhub.Capability = "appleads_creative_management"
	CapabilityReporting          socialhub.Capability = "appleads_campaign_reporting"
)

// Client exposes one Apple Ads organization's paid-media workflows.
type Client struct {
	accountID socialhub.AccountID
	orgID     int64
	api       *transport.Client
}

func (client *Client) Platform() socialhub.Platform { return platformName }
func (client *Client) Account() socialhub.AccountID { return client.accountID }

func (client *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	return socialhub.Capabilities{
		CapabilityAccountAccess: {
			Capability: CapabilityAccountAccess, Supported: true, Approval: socialhub.ApprovalUnknown,
			Reason: "organization ACL and API-role validation; Apple Ads Advanced account access is required",
			DocURL: documentationURL + "/calling-the-apple-search-ads-api",
		},
		CapabilityCampaignManagement: {
			Capability: CapabilityCampaignManagement, Supported: true, Approval: socialhub.ApprovalUnknown,
			Reason: "paused-first Campaign, Ad Group, targeting Keyword, and Ad assignment workflows",
			DocURL: documentationURL + "/campaigns",
		},
		CapabilityCreativeManagement: {
			Capability: CapabilityCreativeManagement, Supported: true, Approval: socialhub.ApprovalUnknown,
			Reason: "custom and default product page Creative lookup and creation",
			DocURL: documentationURL + "/creatives",
		},
		CapabilityReporting: {
			Capability: CapabilityReporting, Supported: true, Approval: socialhub.ApprovalUnknown,
			Reason: "Campaign, Ad Group, Keyword, and Ad performance reports",
			DocURL: documentationURL + "/reports",
		},
		socialhub.CapPublish: {Capability: socialhub.CapPublish, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Apple Ads is paid acquisition, not organic publishing"},
		socialhub.CapFetch:   {Capability: socialhub.CapFetch, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "advertising reads use typed Apple Ads resources"},
		socialhub.CapMedia:   {Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "creatives reference App Store product pages; media upload is outside this API"},
		socialhub.CapReact:   {Capability: socialhub.CapReact, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Apple Ads does not expose organic reactions"},
		socialhub.CapMessage: {Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Apple Ads does not expose general messaging"},
		socialhub.CapWebhook: {Capability: socialhub.CapWebhook, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Campaign Management API does not expose webhooks"},
	}, nil
}

func (client *Client) Publisher() (socialhub.Publisher, bool)           { return nil, false }
func (client *Client) Fetcher() (socialhub.Fetcher, bool)               { return nil, false }
func (client *Client) MediaUploader() (socialhub.MediaUploader, bool)   { return nil, false }
func (client *Client) Reactor() (socialhub.Reactor, bool)               { return nil, false }
func (client *Client) Messenger() (socialhub.Messenger, bool)           { return nil, false }
func (client *Client) WebhookHandler() (socialhub.WebhookHandler, bool) { return nil, false }
func (client *Client) Close() error                                     { return nil }

func (client *Client) ACL() ACLWorkflow            { return client }
func (client *Client) Campaigns() CampaignWorkflow { return client }
func (client *Client) AdGroups() AdGroupWorkflow   { return client }
func (client *Client) Keywords() KeywordWorkflow   { return client }
func (client *Client) Creatives() CreativeWorkflow { return client }
func (client *Client) Ads() AdWorkflow             { return client }
func (client *Client) Reports() ReportWorkflow     { return client }

var _ socialhub.Client = (*Client)(nil)
