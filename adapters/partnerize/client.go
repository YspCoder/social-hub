package partnerize

import (
	"context"
	"strings"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	CapabilityPartnerProfile        socialhub.Capability = "partnerize_partner_profile"
	CapabilityCampaignDiscovery     socialhub.Capability = "partnerize_campaign_discovery"
	CapabilityCreativeDiscovery     socialhub.Capability = "partnerize_creative_discovery"
	CapabilityTrackingLink          socialhub.Capability = "partnerize_tracking_link"
	CapabilityConversionAttribution socialhub.Capability = "partnerize_conversion_attribution"
)

// Client exposes Partnerize workflows for one partner account.
type Client struct {
	accountID   socialhub.AccountID
	publisherID string
	api         *transport.Client
	approval    socialhub.ApprovalConfig
	clock       socialhub.Clock
}

func (client *Client) Platform() socialhub.Platform { return platformName }
func (client *Client) Account() socialhub.AccountID { return client.accountID }

func (client *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	approval := socialhub.ApprovalUnknown
	if strings.TrimSpace(client.approval.AccountType) != "" {
		approval = socialhub.ApprovalGranted
	}
	return socialhub.Capabilities{
		CapabilityPartnerProfile: {
			Capability: CapabilityPartnerProfile, Supported: true, Approval: approval,
			Reason: "partner profile, website, and database retrieval", DocURL: documentationURL,
		},
		CapabilityCampaignDiscovery: {
			Capability: CapabilityCampaignDiscovery, Supported: true, Approval: approval,
			Reason: "campaign relationship discovery by partner status", DocURL: documentationURL,
		},
		CapabilityCreativeDiscovery: {
			Capability: CapabilityCreativeDiscovery, Supported: true, Approval: approval,
			Reason: "campaign creative and creative-item discovery", DocURL: documentationURL,
		},
		CapabilityTrackingLink: {
			Capability: CapabilityTrackingLink, Supported: true, Approval: approval,
			Reason: "v2 partner tracking-link creation", DocURL: documentationURL,
		},
		CapabilityConversionAttribution: {
			Capability: CapabilityConversionAttribution, Supported: true, Approval: approval,
			Reason: "partner conversion and commission reporting", DocURL: documentationURL,
		},
		socialhub.CapPublish: {Capability: socialhub.CapPublish, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Partners API is not an organic publishing product"},
		socialhub.CapFetch:   {Capability: socialhub.CapFetch, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "affiliate reads use typed Partnerize workflows"},
		socialhub.CapMedia:   {Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "creative media is read-only"},
		socialhub.CapReact:   {Capability: socialhub.CapReact, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Partners API exposes no organic reactions"},
		socialhub.CapMessage: {Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Partners API is not a messaging product"},
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

// Partnerize returns the typed affiliate workflow surface.
func (client *Client) Partnerize() PartnerWorkflow { return client }

var _ socialhub.Client = (*Client)(nil)
