package rakutenadvertising

import (
	"context"
	"net/http"
	"strings"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	CapabilityAdvertiserDiscovery  socialhub.Capability = "rakuten_advertising_advertiser_discovery"
	CapabilityPartnershipDiscovery socialhub.Capability = "rakuten_advertising_partnership_discovery"
	CapabilityProductSearch        socialhub.Capability = "rakuten_advertising_product_search"
	CapabilityDeepLink             socialhub.Capability = "rakuten_advertising_deep_link"
	CapabilityTransactionEvents    socialhub.Capability = "rakuten_advertising_transaction_events"
)

// Client exposes Affiliate APIs workflows for one Rakuten publisher account.
type Client struct {
	accountID    socialhub.AccountID
	publisherID  string
	api          *transport.Client
	httpClient   *http.Client
	decodeError  func(int, http.Header, []byte) error
	errorSecrets func() []string
	approval     socialhub.ApprovalConfig
	clock        socialhub.Clock
}

func (client *Client) Platform() socialhub.Platform { return platformName }
func (client *Client) Account() socialhub.AccountID { return client.accountID }

func (client *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	approval := socialhub.ApprovalUnknown
	if strings.TrimSpace(client.approval.AccountType) != "" {
		approval = socialhub.ApprovalGranted
	}
	return socialhub.Capabilities{
		CapabilityAdvertiserDiscovery: {
			Capability: CapabilityAdvertiserDiscovery, Supported: true, Approval: approval,
			Reason: "Rakuten Advertising network advertiser discovery", DocURL: documentationURL,
		},
		CapabilityPartnershipDiscovery: {
			Capability: CapabilityPartnershipDiscovery, Supported: true, Approval: approval,
			Reason: "publisher partnership discovery and status filtering", DocURL: documentationURL,
		},
		CapabilityProductSearch: {
			Capability: CapabilityProductSearch, Supported: true, Approval: approval,
			Reason: "partner-advertiser product-feed search", DocURL: documentationURL,
		},
		CapabilityDeepLink: {
			Capability: CapabilityDeepLink, Supported: true, Approval: approval,
			Reason: "single deep-link generation; advertiser partnership and deep-link support are required", DocURL: documentationURL,
		},
		CapabilityTransactionEvents: {
			Capability: CapabilityTransactionEvents, Supported: true, Approval: approval,
			Reason: "near-real-time Events transaction retrieval", DocURL: documentationURL,
		},
		socialhub.CapPublish: {Capability: socialhub.CapPublish, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Affiliate APIs are not an organic publishing product"},
		socialhub.CapFetch:   {Capability: socialhub.CapFetch, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "affiliate reads use typed Rakuten workflows"},
		socialhub.CapMedia:   {Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "product image URLs are read-only"},
		socialhub.CapReact:   {Capability: socialhub.CapReact, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Affiliate APIs expose no organic reactions"},
		socialhub.CapMessage: {Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Affiliate APIs are not a messaging product"},
		socialhub.CapWebhook: {Capability: socialhub.CapWebhook, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Events postback configuration is outside these pull workflows"},
	}, nil
}

func (client *Client) Publisher() (socialhub.Publisher, bool)           { return nil, false }
func (client *Client) Fetcher() (socialhub.Fetcher, bool)               { return nil, false }
func (client *Client) MediaUploader() (socialhub.MediaUploader, bool)   { return nil, false }
func (client *Client) Reactor() (socialhub.Reactor, bool)               { return nil, false }
func (client *Client) Messenger() (socialhub.Messenger, bool)           { return nil, false }
func (client *Client) WebhookHandler() (socialhub.WebhookHandler, bool) { return nil, false }
func (client *Client) Close() error                                     { return nil }

func (client *Client) Affiliate() PublisherWorkflow { return client }

var _ socialhub.Client = (*Client)(nil)
