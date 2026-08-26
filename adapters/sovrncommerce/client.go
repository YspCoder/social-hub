package sovrncommerce

import (
	"context"
	"strings"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	CapabilityAffiliateLink        socialhub.Capability = "sovrn_affiliate_link"
	CapabilityTransactionReporting socialhub.Capability = "sovrn_transaction_reporting"
	CapabilityMerchantPerformance  socialhub.Capability = "sovrn_merchant_performance"
	CapabilityMerchantDiscovery    socialhub.Capability = "sovrn_merchant_discovery"
)

// Client exposes Commerce workflows for one Sovrn site.
type Client struct {
	accountID        socialhub.AccountID
	apiKey           string
	reportsAPI       *transport.Client
	merchantRatesAPI *transport.Client
	approval         socialhub.ApprovalConfig
	redactionSecrets []string
}

func (client *Client) Platform() socialhub.Platform { return platformName }
func (client *Client) Account() socialhub.AccountID { return client.accountID }

func (*Client) String() string   { return "sovrncommerce.Client(<redacted credentials>)" }
func (*Client) GoString() string { return "sovrncommerce.Client(<redacted credentials>)" }

func (client *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	approval := socialhub.ApprovalUnknown
	if strings.TrimSpace(client.approval.AccountType) != "" {
		approval = socialhub.ApprovalGranted
	}
	return socialhub.Capabilities{
		CapabilityAffiliateLink: {
			Capability: CapabilityAffiliateLink, Supported: true, Approval: approval,
			Reason: "programmatic Sovrn affiliate-link construction", DocURL: documentationURL,
		},
		CapabilityTransactionReporting: {
			Capability: CapabilityTransactionReporting, Supported: true, Approval: approval,
			Reason: "single-day commission-event and delta reporting", DocURL: documentationURL,
		},
		CapabilityMerchantPerformance: {
			Capability: CapabilityMerchantPerformance, Supported: true, Approval: approval,
			Reason: "merchant performance aggregation for an exclusive 31-day window", DocURL: documentationURL,
		},
		CapabilityMerchantDiscovery: {
			Capability: CapabilityMerchantDiscovery, Supported: true, Approval: approval,
			Reason: "approved merchant discovery with rates, filters, and pagination", DocURL: documentationURL,
		},
		socialhub.CapPublish: {Capability: socialhub.CapPublish, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Commerce is not an organic publishing product"},
		socialhub.CapFetch:   {Capability: socialhub.CapFetch, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "affiliate reads use typed Sovrn Commerce workflows"},
		socialhub.CapMedia:   {Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "merchant logos are read-only"},
		socialhub.CapReact:   {Capability: socialhub.CapReact, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Commerce exposes no organic reactions"},
		socialhub.CapMessage: {Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Commerce is not a messaging product"},
		socialhub.CapWebhook: {Capability: socialhub.CapWebhook, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "the implemented workflows are request/response based"},
	}, nil
}

func (client *Client) Publisher() (socialhub.Publisher, bool)           { return nil, false }
func (client *Client) Fetcher() (socialhub.Fetcher, bool)               { return nil, false }
func (client *Client) MediaUploader() (socialhub.MediaUploader, bool)   { return nil, false }
func (client *Client) Reactor() (socialhub.Reactor, bool)               { return nil, false }
func (client *Client) Messenger() (socialhub.Messenger, bool)           { return nil, false }
func (client *Client) WebhookHandler() (socialhub.WebhookHandler, bool) { return nil, false }
func (client *Client) Close() error                                     { return nil }

// Commerce returns the typed publisher affiliate workflow surface.
func (client *Client) Commerce() CommerceWorkflow { return client }

var _ socialhub.Client = (*Client)(nil)
