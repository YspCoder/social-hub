package applovinconversion

import (
	"context"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const CapabilityConversionEvents socialhub.Capability = "applovin_growth_conversion_events"

// Client exposes one Event Key's server-to-server conversion workflow.
type Client struct {
	accountID socialhub.AccountID
	eventKey  string
	policy    AccountPolicy
	api       *transport.Client
}

func (client *Client) Platform() socialhub.Platform { return platformName }
func (client *Client) Account() socialhub.AccountID { return client.accountID }

func (client *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	docURL := documentationURL
	reason := "server-to-server standard conversion events with atomic local batch validation"
	if client.policy == PolicyLeadGen {
		docURL = leadGenDocumentationURL
		reason = "lead-generation page_view, generate_lead, and mobile app_open events"
	}
	if client.policy == PolicyRestrictedLeadGen {
		docURL = restrictedLeadGenDocumentationURL
		reason = "restricted lead-generation events with origin-only URLs and deny-by-default user data"
	}
	return socialhub.Capabilities{
		CapabilityConversionEvents: {
			Capability: CapabilityConversionEvents, Supported: true, Approval: socialhub.ApprovalUnknown,
			Reason: reason, DocURL: docURL,
		},
		socialhub.CapPublish: {Capability: socialhub.CapPublish, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "conversion events are paid-media telemetry, not organic posts"},
		socialhub.CapFetch:   {Capability: socialhub.CapFetch, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Conversion API v1 is write-only"},
		socialhub.CapMedia:   {Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Conversion API v1 does not upload media"},
		socialhub.CapReact:   {Capability: socialhub.CapReact, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Conversion API v1 has no organic engagement surface"},
		socialhub.CapMessage: {Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Conversion API v1 has no messaging surface"},
		socialhub.CapWebhook: {Capability: socialhub.CapWebhook, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Conversion API v1 does not expose webhooks"},
	}, nil
}

func (client *Client) Publisher() (socialhub.Publisher, bool)           { return nil, false }
func (client *Client) Fetcher() (socialhub.Fetcher, bool)               { return nil, false }
func (client *Client) MediaUploader() (socialhub.MediaUploader, bool)   { return nil, false }
func (client *Client) Reactor() (socialhub.Reactor, bool)               { return nil, false }
func (client *Client) Messenger() (socialhub.Messenger, bool)           { return nil, false }
func (client *Client) WebhookHandler() (socialhub.WebhookHandler, bool) { return nil, false }
func (client *Client) Close() error                                     { return nil }

func (client *Client) Conversions() ConversionWorkflow { return client }

var _ socialhub.Client = (*Client)(nil)
