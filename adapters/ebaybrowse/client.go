package ebaybrowse

import (
	"context"
	"strings"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	CapabilityItemDiscovery        socialhub.Capability = "ebay_item_discovery"
	CapabilityAffiliateAttribution socialhub.Capability = "ebay_affiliate_attribution"
)

// Client exposes eBay Browse workflows for one configured application account.
type Client struct {
	accountID           socialhub.AccountID
	api                 *transport.Client
	approval            socialhub.ApprovalConfig
	marketplaceID       string
	affiliateCampaignID string
	acceptLanguage      string
}

func (client *Client) Platform() socialhub.Platform { return platformName }
func (client *Client) Account() socialhub.AccountID { return client.accountID }

func (client *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	discoveryApproval := socialhub.ApprovalUnknown
	if strings.TrimSpace(client.approval.AccountType) != "" {
		discoveryApproval = socialhub.ApprovalGranted
	}
	affiliateApproval := discoveryApproval
	if client.affiliateCampaignID == "" {
		affiliateApproval = socialhub.ApprovalRequired
	}
	return socialhub.Capabilities{
		CapabilityItemDiscovery: {
			Capability: CapabilityItemDiscovery, Supported: true, Approval: discoveryApproval,
			Scopes: []string{applicationScope}, Reason: "keyword, GTIN, ePID, legacy-ID, and item-group discovery",
			DocURL: documentationURL,
		},
		CapabilityAffiliateAttribution: {
			Capability: CapabilityAffiliateAttribution, Supported: true, Approval: affiliateApproval,
			Scopes: []string{applicationScope}, Reason: "EPN context header and attributed itemAffiliateWebUrl retrieval",
			DocURL: documentationURL,
		},
		socialhub.CapPublish: {Capability: socialhub.CapPublish, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "eBay Browse is a buyer discovery API, not organic publishing"},
		socialhub.CapFetch:   {Capability: socialhub.CapFetch, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "commerce reads use typed eBay Browse workflows"},
		socialhub.CapMedia:   {Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Browse returns hosted listing media and does not upload assets"},
		socialhub.CapReact:   {Capability: socialhub.CapReact, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "eBay Browse exposes no organic reactions"},
		socialhub.CapMessage: {Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "eBay Browse is not a messaging product"},
		socialhub.CapWebhook: {Capability: socialhub.CapWebhook, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "these read-only Browse workflows do not expose webhooks"},
	}, nil
}

func (client *Client) Publisher() (socialhub.Publisher, bool)           { return nil, false }
func (client *Client) Fetcher() (socialhub.Fetcher, bool)               { return nil, false }
func (client *Client) MediaUploader() (socialhub.MediaUploader, bool)   { return nil, false }
func (client *Client) Reactor() (socialhub.Reactor, bool)               { return nil, false }
func (client *Client) Messenger() (socialhub.Messenger, bool)           { return nil, false }
func (client *Client) WebhookHandler() (socialhub.WebhookHandler, bool) { return nil, false }
func (client *Client) Close() error                                     { return nil }

func (client *Client) Browse() BrowseWorkflow { return client }

var _ socialhub.Client = (*Client)(nil)
