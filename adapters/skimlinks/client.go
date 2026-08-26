package skimlinks

import (
	"context"
	"strings"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	CapabilityMerchantDiscovery    socialhub.Capability = "skimlinks_merchant_discovery"
	CapabilityLinkWrapper          socialhub.Capability = "skimlinks_link_wrapper"
	CapabilityCommissionReporting  socialhub.Capability = "skimlinks_commission_reporting"
	CapabilityPerformanceReporting socialhub.Capability = "skimlinks_performance_reporting"
)

// Client exposes Skimlinks publisher workflows for one configured account.
type Client struct {
	accountID         socialhub.AccountID
	publisherID       int64
	publisherDomainID int64
	siteID            string
	linkBaseURL       string
	merchantAPI       *transport.Client
	reportingAPI      *transport.Client
	errorSecrets      func() []string
	approval          socialhub.ApprovalConfig
}

func (client *Client) Platform() socialhub.Platform { return platformName }
func (client *Client) Account() socialhub.AccountID { return client.accountID }

func (client *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	approval := socialhub.ApprovalUnknown
	if strings.TrimSpace(client.approval.AccountType) != "" {
		approval = socialhub.ApprovalGranted
	}
	return socialhub.Capabilities{
		CapabilityMerchantDiscovery: {
			Capability: CapabilityMerchantDiscovery, Supported: true, Approval: approval,
			Reason: "site-specific Merchant API v4 discovery", DocURL: documentationURL,
		},
		CapabilityLinkWrapper: {
			Capability: CapabilityLinkWrapper, Supported: true, Approval: approval,
			Reason: "registered-site Link Wrapper URL construction", DocURL: documentationURL,
		},
		CapabilityCommissionReporting: {
			Capability: CapabilityCommissionReporting, Supported: true, Approval: approval,
			Reason: "individual publisher commission reporting", DocURL: documentationURL,
		},
		CapabilityPerformanceReporting: {
			Capability: CapabilityPerformanceReporting, Supported: true, Approval: approval,
			Reason: "aggregated publisher performance reporting", DocURL: documentationURL,
		},
		socialhub.CapPublish: {Capability: socialhub.CapPublish, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Skimlinks publisher APIs are not an organic publishing product"},
		socialhub.CapFetch:   {Capability: socialhub.CapFetch, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "affiliate reads use typed Skimlinks workflows"},
		socialhub.CapMedia:   {Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "the implemented APIs do not upload media"},
		socialhub.CapReact:   {Capability: socialhub.CapReact, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "the implemented APIs expose no reactions"},
		socialhub.CapMessage: {Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "the implemented APIs expose no messaging"},
		socialhub.CapWebhook: {Capability: socialhub.CapWebhook, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "the implemented APIs are request/response and redirect workflows"},
	}, nil
}

func (client *Client) Publisher() (socialhub.Publisher, bool)           { return nil, false }
func (client *Client) Fetcher() (socialhub.Fetcher, bool)               { return nil, false }
func (client *Client) MediaUploader() (socialhub.MediaUploader, bool)   { return nil, false }
func (client *Client) Reactor() (socialhub.Reactor, bool)               { return nil, false }
func (client *Client) Messenger() (socialhub.Messenger, bool)           { return nil, false }
func (client *Client) WebhookHandler() (socialhub.WebhookHandler, bool) { return nil, false }
func (client *Client) Close() error                                     { return nil }

// Skimlinks returns the bounded publisher workflow surface.
func (client *Client) Skimlinks() PublisherWorkflow { return client }

var _ socialhub.Client = (*Client)(nil)
