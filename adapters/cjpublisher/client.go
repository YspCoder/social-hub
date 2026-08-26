package cjpublisher

import (
	"context"
	"net/http"
	"strings"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	CapabilityProductCatalog        socialhub.Capability = "cj_product_catalog"
	CapabilityLinkDiscovery         socialhub.Capability = "cj_link_discovery"
	CapabilityCommissionAttribution socialhub.Capability = "cj_commission_attribution"
	CapabilityProgramTerms          socialhub.Capability = "cj_program_terms"
)

// Client exposes CJ publisher workflows for one company ID.
type Client struct {
	accountID   socialhub.AccountID
	publisherID string
	websiteID   string

	productsAPI    *transport.Client
	commissionsAPI *transport.Client
	programsAPI    *transport.Client
	linksAPI       *transport.Client
	httpClient     *http.Client
	errorDecoder   transport.ErrorDecoder
	approval       socialhub.ApprovalConfig
}

func (client *Client) Platform() socialhub.Platform { return platformName }
func (client *Client) Account() socialhub.AccountID { return client.accountID }

func (client *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	approval := socialhub.ApprovalRequired
	if strings.TrimSpace(client.approval.AccountType) != "" {
		approval = socialhub.ApprovalGranted
	}
	return socialhub.Capabilities{
		CapabilityProductCatalog: {
			Capability: CapabilityProductCatalog, Supported: true, Approval: approval,
			Reason: "publisher product-feed summary and product-detail search", DocURL: documentationURL,
		},
		CapabilityLinkDiscovery: {
			Capability: CapabilityLinkDiscovery, Supported: true, Approval: approval,
			Reason: "Link Search v2 creative and deep-link eligibility discovery", DocURL: documentationURL,
		},
		CapabilityCommissionAttribution: {
			Capability: CapabilityCommissionAttribution, Supported: true, Approval: approval,
			Reason: "near-real-time publisher commissions and correction deltas", DocURL: documentationURL,
		},
		CapabilityProgramTerms: {
			Capability: CapabilityProgramTerms, Supported: true, Approval: approval,
			Reason: "active, pending, and expired publisher program terms", DocURL: documentationURL,
		},
		socialhub.CapPublish: {Capability: socialhub.CapPublish, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "CJ Publisher APIs are not organic publishing products"},
		socialhub.CapFetch:   {Capability: socialhub.CapFetch, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "affiliate reads use typed CJ workflows"},
		socialhub.CapMedia:   {Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "catalog media URLs are read-only"},
		socialhub.CapReact:   {Capability: socialhub.CapReact, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "CJ Publisher APIs expose no organic reactions"},
		socialhub.CapMessage: {Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "CJ Publisher APIs are not messaging products"},
		socialhub.CapWebhook: {Capability: socialhub.CapWebhook, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "these workflows are request/response based"},
	}, nil
}

func (client *Client) Publisher() (socialhub.Publisher, bool)           { return nil, false }
func (client *Client) Fetcher() (socialhub.Fetcher, bool)               { return nil, false }
func (client *Client) MediaUploader() (socialhub.MediaUploader, bool)   { return nil, false }
func (client *Client) Reactor() (socialhub.Reactor, bool)               { return nil, false }
func (client *Client) Messenger() (socialhub.Messenger, bool)           { return nil, false }
func (client *Client) WebhookHandler() (socialhub.WebhookHandler, bool) { return nil, false }
func (client *Client) Close() error                                     { return nil }

// CJ returns the typed affiliate workflow surface.
func (client *Client) CJ() PublisherWorkflow { return client }

var _ socialhub.Client = (*Client)(nil)
