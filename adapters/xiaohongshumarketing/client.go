package xiaohongshumarketing

import (
	"context"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	CapabilitySpotlightCampaignManagement socialhub.Capability = "xiaohongshu_spotlight_campaign_management"
	CapabilitySpotlightUnitManagement     socialhub.Capability = "xiaohongshu_spotlight_unit_management"
	CapabilitySpotlightCreativeManagement socialhub.Capability = "xiaohongshu_spotlight_creative_management"
)

// Client exposes one Spotlight advertiser's typed management workflows.
type Client struct {
	accountID    socialhub.AccountID
	advertiserID uint64
	api          *transport.Client
	clock        socialhub.Clock
}

func (client *Client) Platform() socialhub.Platform { return platformName }
func (client *Client) Account() socialhub.AccountID { return client.accountID }

func (client *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	return socialhub.Capabilities{
		CapabilitySpotlightCampaignManagement: {
			Capability: CapabilitySpotlightCampaignManagement, Supported: true, Approval: socialhub.ApprovalRequired,
			Reason: "campaign query, current cascade editing for product-seeding and lead-generation campaigns, and explicit status changes require ad_query/ad_manage approval",
			DocURL: documentationURL,
		},
		CapabilitySpotlightUnitManagement: {
			Capability: CapabilitySpotlightUnitManagement, Supported: true, Approval: socialhub.ApprovalRequired,
			Reason: "unit query and explicit status changes require an approved Spotlight application and ad_query/ad_manage scopes",
			DocURL: "https://ad-market.xiaohongshu.com/docs-center?bizType=943&articleId=3044",
		},
		CapabilitySpotlightCreativeManagement: {
			Capability: CapabilitySpotlightCreativeManagement, Supported: true, Approval: socialhub.ApprovalRequired,
			Reason: "creative query and explicit status changes require an approved Spotlight application and ad_query/ad_manage scopes",
			DocURL: "https://ad-market.xiaohongshu.com/docs-center?bizType=943&articleId=3158",
		},
		socialhub.CapPublish: {Capability: socialhub.CapPublish, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "paid-media management is not organic note publishing"},
		socialhub.CapFetch:   {Capability: socialhub.CapFetch, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "advertising reads use typed Spotlight workflows"},
		socialhub.CapMedia:   {Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "creative asset upload and note creation are outside this adapter"},
		socialhub.CapReact:   {Capability: socialhub.CapReact, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Spotlight management has no organic reaction surface"},
		socialhub.CapMessage: {Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Spotlight management has no messaging surface"},
		socialhub.CapWebhook: {Capability: socialhub.CapWebhook, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Spotlight management does not expose a webhook workflow"},
	}, nil
}

func (client *Client) Publisher() (socialhub.Publisher, bool)           { return nil, false }
func (client *Client) Fetcher() (socialhub.Fetcher, bool)               { return nil, false }
func (client *Client) MediaUploader() (socialhub.MediaUploader, bool)   { return nil, false }
func (client *Client) Reactor() (socialhub.Reactor, bool)               { return nil, false }
func (client *Client) Messenger() (socialhub.Messenger, bool)           { return nil, false }
func (client *Client) WebhookHandler() (socialhub.WebhookHandler, bool) { return nil, false }
func (client *Client) Close() error                                     { return nil }

func (client *Client) Campaigns() CampaignWorkflow { return client }
func (client *Client) Units() UnitWorkflow         { return client }
func (client *Client) Creatives() CreativeWorkflow { return client }

var _ socialhub.Client = (*Client)(nil)
