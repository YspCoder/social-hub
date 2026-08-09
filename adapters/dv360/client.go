package dv360

import (
	"context"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const CapabilityAdsManagement socialhub.Capability = "dv360_ads_management"

// Client exposes advertiser-scoped DV360 paid-media workflows. Organic social
// capabilities are intentionally unavailable.
type Client struct {
	accountID    socialhub.AccountID
	advertiserID string
	partnerID    string
	api          *transport.Client
	scopes       []string
}

func (client *Client) Platform() socialhub.Platform { return platformName }
func (client *Client) Account() socialhub.AccountID { return client.accountID }

func (client *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	return socialhub.Capabilities{
		CapabilityAdsManagement: {
			Capability: CapabilityAdsManagement, Supported: true, Approval: socialhub.ApprovalUnknown,
			Reason: "Advertiser reads and paused/draft-first Campaign, Insertion Order, and standard RTB Line Item workflows; Google account access applies",
			DocURL: documentationURL,
		},
		socialhub.CapPublish: {Capability: socialhub.CapPublish, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "paid ads are not organic social posts; use Campaigns(), InsertionOrders(), and LineItems()"},
		socialhub.CapFetch:   {Capability: socialhub.CapFetch, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "advertising reads use typed DV360 resources"},
		socialhub.CapMedia:   {Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "creative and asset upload are outside the initial adapter contract"},
		socialhub.CapReact:   {Capability: socialhub.CapReact, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "DV360 is not an organic engagement product"},
		socialhub.CapMessage: {Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "DV360 does not provide social messaging"},
		socialhub.CapWebhook: {Capability: socialhub.CapWebhook, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "webhooks are outside the initial adapter contract"},
	}, nil
}

func (client *Client) Publisher() (socialhub.Publisher, bool)           { return nil, false }
func (client *Client) Fetcher() (socialhub.Fetcher, bool)               { return nil, false }
func (client *Client) MediaUploader() (socialhub.MediaUploader, bool)   { return nil, false }
func (client *Client) Reactor() (socialhub.Reactor, bool)               { return nil, false }
func (client *Client) Messenger() (socialhub.Messenger, bool)           { return nil, false }
func (client *Client) WebhookHandler() (socialhub.WebhookHandler, bool) { return nil, false }
func (client *Client) Close() error                                     { return nil }

func (client *Client) Advertisers() AdvertiserWorkflow         { return client }
func (client *Client) Campaigns() CampaignWorkflow             { return client }
func (client *Client) InsertionOrders() InsertionOrderWorkflow { return client }
func (client *Client) LineItems() LineItemWorkflow             { return client }

func (client *Client) requireAccess(operation string) error {
	if len(client.scopes) == 0 {
		return nil
	}
	for _, scope := range client.scopes {
		if scope == displayVideoScope {
			return nil
		}
	}
	return &socialhub.Error{
		Code: socialhub.CodeApprovalRequired, Class: socialhub.ClassUserAction,
		Platform: platformName, Product: productName, Op: operation,
		RequiredScopes: []string{displayVideoScope}, ApprovalURL: defaultAuthURL,
		PlatformMessage: "configured approval scopes do not authorize Display & Video 360 API access",
	}
}

var _ socialhub.Client = (*Client)(nil)
