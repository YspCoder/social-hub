package amazoncreators

import (
	"context"
	"strings"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	CapabilityCatalogDiscovery     socialhub.Capability = "amazon_catalog_discovery"
	CapabilityAffiliateAttribution socialhub.Capability = "amazon_affiliate_attribution"
)

// Client exposes Creators API Catalog workflows for one Associates store.
type Client struct {
	accountID   socialhub.AccountID
	api         *transport.Client
	approval    socialhub.ApprovalConfig
	marketplace string
	partnerTag  string
}

func (client *Client) Platform() socialhub.Platform { return platformName }
func (client *Client) Account() socialhub.AccountID { return client.accountID }

func (client *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	approval := socialhub.ApprovalUnknown
	if strings.TrimSpace(client.approval.AccountType) != "" {
		approval = socialhub.ApprovalGranted
	}
	return socialhub.Capabilities{
		CapabilityCatalogDiscovery: {
			Capability: CapabilityCatalogDiscovery, Supported: true, Approval: approval,
			Scopes: []string{oauthScope}, Reason: "item search, batch item detail, variations, and browse nodes",
			DocURL: documentationURL,
		},
		CapabilityAffiliateAttribution: {
			Capability: CapabilityAffiliateAttribution, Supported: true, Approval: approval,
			Scopes: []string{oauthScope}, Reason: "Partner Tag-attributed detailPageURL and searchURL retrieval",
			DocURL: documentationURL,
		},
		socialhub.CapPublish: {Capability: socialhub.CapPublish, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Creators API Catalog is a commerce discovery API"},
		socialhub.CapFetch:   {Capability: socialhub.CapFetch, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "commerce reads use typed Amazon Catalog workflows"},
		socialhub.CapMedia:   {Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Catalog returns hosted product media and does not upload assets"},
		socialhub.CapReact:   {Capability: socialhub.CapReact, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Creators API exposes no organic reactions"},
		socialhub.CapMessage: {Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Creators API is not a messaging product"},
		socialhub.CapWebhook: {Capability: socialhub.CapWebhook, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Catalog workflows do not expose webhooks"},
	}, nil
}

func (client *Client) Publisher() (socialhub.Publisher, bool)           { return nil, false }
func (client *Client) Fetcher() (socialhub.Fetcher, bool)               { return nil, false }
func (client *Client) MediaUploader() (socialhub.MediaUploader, bool)   { return nil, false }
func (client *Client) Reactor() (socialhub.Reactor, bool)               { return nil, false }
func (client *Client) Messenger() (socialhub.Messenger, bool)           { return nil, false }
func (client *Client) WebhookHandler() (socialhub.WebhookHandler, bool) { return nil, false }
func (client *Client) Close() error                                     { return nil }

func (client *Client) Catalog() CatalogWorkflow { return client }

var _ socialhub.Client = (*Client)(nil)
