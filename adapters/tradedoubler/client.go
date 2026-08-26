package tradedoubler

import (
	"context"
	"strings"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	CapabilityProductSearch          socialhub.Capability = "tradedoubler_product_search"
	CapabilityProductFeedDiscovery   socialhub.Capability = "tradedoubler_product_feed_discovery"
	CapabilityFeedProgramSummary     socialhub.Capability = "tradedoubler_feed_program_summary"
	CapabilityUnlimitedFeedFreshness socialhub.Capability = "tradedoubler_unlimited_feed_freshness"
	CapabilityTrackedProductURL      socialhub.Capability = "tradedoubler_tracked_product_url"
)

// Client exposes Products API publisher workflows for one token.
type Client struct {
	accountID    socialhub.AccountID
	api          *transport.Client
	errorSecrets func() []string
	approval     socialhub.ApprovalConfig
}

func (client *Client) Platform() socialhub.Platform { return platformName }
func (client *Client) Account() socialhub.AccountID { return client.accountID }

func (client *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	approval := socialhub.ApprovalUnknown
	if strings.TrimSpace(client.approval.AccountType) != "" {
		approval = socialhub.ApprovalGranted
	}
	return socialhub.Capabilities{
		CapabilityProductSearch: {
			Capability: CapabilityProductSearch, Supported: true, Approval: approval,
			Reason: "product search across advertiser feeds connected to the publisher token", DocURL: documentationURL,
		},
		CapabilityProductFeedDiscovery: {
			Capability: CapabilityProductFeedDiscovery, Supported: true, Approval: approval,
			Reason: "product-feed metadata connected to the publisher token", DocURL: documentationURL,
		},
		CapabilityFeedProgramSummary: {
			Capability: CapabilityFeedProgramSummary, Supported: true, Approval: approval,
			Reason: "program summaries embedded in product-feed metadata", DocURL: documentationURL,
		},
		CapabilityUnlimitedFeedFreshness: {
			Capability: CapabilityUnlimitedFeedFreshness, Supported: true, Approval: approval,
			Reason: "unlimited-feed last-updated preflight without downloading the feed", DocURL: documentationURL,
		},
		CapabilityTrackedProductURL: {
			Capability: CapabilityTrackedProductURL, Supported: true, Approval: approval,
			Reason: "commission-tracked product URLs returned inside product offers", DocURL: documentationURL,
		},
		socialhub.CapPublish: {Capability: socialhub.CapPublish, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Products API is not an organic publishing product"},
		socialhub.CapFetch:   {Capability: socialhub.CapFetch, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "commerce reads use typed Tradedoubler product workflows"},
		socialhub.CapMedia:   {Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "product media URLs are read-only"},
		socialhub.CapReact:   {Capability: socialhub.CapReact, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Products API exposes no organic reactions"},
		socialhub.CapMessage: {Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Products API is not a messaging product"},
		socialhub.CapWebhook: {Capability: socialhub.CapWebhook, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Products API workflows are request/response based"},
	}, nil
}

func (client *Client) Publisher() (socialhub.Publisher, bool)           { return nil, false }
func (client *Client) Fetcher() (socialhub.Fetcher, bool)               { return nil, false }
func (client *Client) MediaUploader() (socialhub.MediaUploader, bool)   { return nil, false }
func (client *Client) Reactor() (socialhub.Reactor, bool)               { return nil, false }
func (client *Client) Messenger() (socialhub.Messenger, bool)           { return nil, false }
func (client *Client) WebhookHandler() (socialhub.WebhookHandler, bool) { return nil, false }
func (client *Client) Close() error                                     { return nil }

func (client *Client) Tradedoubler() ProductsWorkflow { return client }

var _ socialhub.Client = (*Client)(nil)
