package marketing

import (
	"context"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	CapabilityMarketingManagement socialhub.Capability = "tiktok_business_marketing_management"
	CapabilityMarketingReports    socialhub.Capability = "tiktok_business_marketing_reports"
)

// Client exposes advertiser-scoped paid-media workflows. Organic capabilities
// are intentionally unavailable.
type Client struct {
	accountID    socialhub.AccountID
	advertiserID string
	api          *transport.Client
}

func (client *Client) Platform() socialhub.Platform { return platformName }
func (client *Client) Account() socialhub.AccountID { return client.accountID }

func (client *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	return socialhub.Capabilities{
		CapabilityMarketingManagement: {
			Capability: CapabilityMarketingManagement, Supported: true, Approval: socialhub.ApprovalUnknown,
			Reason: "advertiser, Campaign, Ad Group, and Ad reads and mutations; app review, scopes, and advertiser authorization apply",
			DocURL: documentationURL,
		},
		CapabilityMarketingReports: {
			Capability: CapabilityMarketingReports, Supported: true, Approval: socialhub.ApprovalUnknown,
			Reason: "synchronous integrated auction reports; reporting scope and advertiser authorization apply",
			DocURL: documentationURL,
		},
		socialhub.CapPublish: {Capability: socialhub.CapPublish, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "paid advertising resources are not social posts; use Campaigns(), AdGroups(), and Ads()"},
		socialhub.CapFetch:   {Capability: socialhub.CapFetch, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "advertising reads use typed TikTok Business resources"},
		socialhub.CapMedia:   {Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "initial adapter references asset-library video and image IDs"},
		socialhub.CapReact:   {Capability: socialhub.CapReact, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Marketing API is not an organic engagement product"},
		socialhub.CapMessage: {Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "use TikTok Business Messaging API rather than Marketing API"},
		socialhub.CapWebhook: {Capability: socialhub.CapWebhook, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Subscription API is outside the initial adapter contract"},
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

func (client *Client) Advertisers() AdvertiserWorkflow { return client }
func (client *Client) Campaigns() CampaignWorkflow     { return client }
func (client *Client) AdGroups() AdGroupWorkflow       { return client }
func (client *Client) Ads() AdWorkflow                 { return client }
func (client *Client) Reports() ReportWorkflow         { return client }

var _ socialhub.Client = (*Client)(nil)
