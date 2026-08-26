package admitadpublisher

import (
	"context"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	CapabilityProgramDiscovery socialhub.Capability = "admitad_program_discovery"
	CapabilityDeeplink         socialhub.Capability = "admitad_deeplink"
	CapabilityCouponDiscovery  socialhub.Capability = "admitad_coupon_discovery"
	CapabilityPublisherReport  socialhub.Capability = "admitad_publisher_report"
)

// Client exposes Publisher API workflows for one Admitad application or
// externally managed access token.
type Client struct {
	accountID socialhub.AccountID
	api       *transport.Client
	scopes    []string
}

func (client *Client) Platform() socialhub.Platform { return platformName }
func (client *Client) Account() socialhub.AccountID { return client.accountID }

func (client *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	return socialhub.Capabilities{
		CapabilityProgramDiscovery: capabilityState(CapabilityProgramDiscovery, scopePrograms, client.scopes, "ad-space program discovery and connection status"),
		CapabilityDeeplink:         capabilityState(CapabilityDeeplink, scopeDeeplinks, client.scopes, "batch affiliate deeplink generation"),
		CapabilityCouponDiscovery:  capabilityState(CapabilityCouponDiscovery, scopeCoupons, client.scopes, "ad-space coupon and promocode discovery"),
		CapabilityPublisherReport:  capabilityState(CapabilityPublisherReport, scopeStatistics, client.scopes, "campaign-level publisher statistics"),
		socialhub.CapPublish:       {Capability: socialhub.CapPublish, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Publisher API is not an organic publishing product"},
		socialhub.CapFetch:         {Capability: socialhub.CapFetch, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "affiliate reads use typed Admitad workflows"},
		socialhub.CapMedia:         {Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Publisher API exposes hosted commerce assets only"},
		socialhub.CapReact:         {Capability: socialhub.CapReact, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Publisher API exposes no organic reactions"},
		socialhub.CapMessage:       {Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Publisher API is not a messaging product"},
		socialhub.CapWebhook:       {Capability: socialhub.CapWebhook, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "this surface is polling-only; postback URLs are a separate workflow"},
	}, nil
}

func capabilityState(capability socialhub.Capability, scope string, configured []string, reason string) socialhub.CapabilityState {
	approval := socialhub.ApprovalUnknown
	if len(configured) > 0 {
		approval = socialhub.ApprovalRequired
		if containsScope(configured, scope) {
			approval = socialhub.ApprovalGranted
		}
	}
	return socialhub.CapabilityState{
		Capability: capability, Supported: true, Approval: approval, Scopes: []string{scope},
		Reason: reason, DocURL: documentationURL,
	}
}

func (client *Client) Publisher() (socialhub.Publisher, bool)           { return nil, false }
func (client *Client) Fetcher() (socialhub.Fetcher, bool)               { return nil, false }
func (client *Client) MediaUploader() (socialhub.MediaUploader, bool)   { return nil, false }
func (client *Client) Reactor() (socialhub.Reactor, bool)               { return nil, false }
func (client *Client) Messenger() (socialhub.Messenger, bool)           { return nil, false }
func (client *Client) WebhookHandler() (socialhub.WebhookHandler, bool) { return nil, false }
func (client *Client) Close() error                                     { return nil }

func (client *Client) PublisherAPI() PublisherWorkflow { return client }

var _ socialhub.Client = (*Client)(nil)
