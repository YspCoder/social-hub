package impactpartner

import (
	"context"
	"strings"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	CapabilityProgramDiscovery       socialhub.Capability = "impact_program_discovery"
	CapabilityCatalogDiscovery       socialhub.Capability = "impact_catalog_discovery"
	CapabilityTrackingLink           socialhub.Capability = "impact_tracking_link"
	CapabilityTransactionAttribution socialhub.Capability = "impact_transaction_attribution"
)

// Client exposes Partner API workflows for one impact.com partner account.
type Client struct {
	accountID  socialhub.AccountID
	accountSID string
	api        *transport.Client
	approval   socialhub.ApprovalConfig
	clock      socialhub.Clock
}

func (client *Client) Platform() socialhub.Platform { return platformName }
func (client *Client) Account() socialhub.AccountID { return client.accountID }

func (client *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	approval := socialhub.ApprovalUnknown
	if strings.TrimSpace(client.approval.AccountType) != "" {
		approval = socialhub.ApprovalGranted
	}
	return socialhub.Capabilities{
		CapabilityProgramDiscovery: {
			Capability: CapabilityProgramDiscovery, Supported: true, Approval: approval,
			Reason: "joined impact.com program discovery", DocURL: documentationURL,
		},
		CapabilityCatalogDiscovery: {
			Capability: CapabilityCatalogDiscovery, Supported: true, Approval: approval,
			Reason: "cross-catalog item search and item detail", DocURL: documentationURL,
		},
		CapabilityTrackingLink: {
			Capability: CapabilityTrackingLink, Supported: true, Approval: approval,
			Reason: "regular, vanity, and deep tracking-link generation", DocURL: documentationURL,
		},
		CapabilityTransactionAttribution: {
			Capability: CapabilityTransactionAttribution, Supported: true, Approval: approval,
			Reason: "attributed action and commission retrieval", DocURL: documentationURL,
		},
		socialhub.CapPublish: {Capability: socialhub.CapPublish, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Partner API is not an organic publishing product"},
		socialhub.CapFetch:   {Capability: socialhub.CapFetch, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "commerce reads use typed impact.com Partner workflows"},
		socialhub.CapMedia:   {Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "catalog image URLs are read-only"},
		socialhub.CapReact:   {Capability: socialhub.CapReact, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Partner API exposes no organic reactions"},
		socialhub.CapMessage: {Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Partner API is not a messaging product"},
		socialhub.CapWebhook: {Capability: socialhub.CapWebhook, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "this surface polls Actions; lifecycle postbacks are a separate workflow"},
	}, nil
}

func (client *Client) Publisher() (socialhub.Publisher, bool)           { return nil, false }
func (client *Client) Fetcher() (socialhub.Fetcher, bool)               { return nil, false }
func (client *Client) MediaUploader() (socialhub.MediaUploader, bool)   { return nil, false }
func (client *Client) Reactor() (socialhub.Reactor, bool)               { return nil, false }
func (client *Client) Messenger() (socialhub.Messenger, bool)           { return nil, false }
func (client *Client) WebhookHandler() (socialhub.WebhookHandler, bool) { return nil, false }
func (client *Client) Close() error                                     { return nil }

func (client *Client) Partner() PartnerWorkflow { return client }

var _ socialhub.Client = (*Client)(nil)
