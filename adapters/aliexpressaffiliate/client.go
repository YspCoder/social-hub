package aliexpressaffiliate

import (
	"context"
	"strings"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	CapabilityProductDiscovery socialhub.Capability = "aliexpress_affiliate_product_discovery"
	CapabilityLinkGeneration   socialhub.Capability = "aliexpress_affiliate_link_generation"
	CapabilityOrderAttribution socialhub.Capability = "aliexpress_affiliate_order_attribution"
)

// Client exposes AliExpress Affiliate workflows for one configured publisher.
type Client struct {
	accountID           socialhub.AccountID
	defaultTrackingID   string
	defaultAppSignature string
	gatewayPath         string
	api                 *transport.Client
	approval            socialhub.ApprovalConfig
	clock               socialhub.Clock
}

func (client *Client) Platform() socialhub.Platform { return platformName }
func (client *Client) Account() socialhub.AccountID { return client.accountID }

func (client *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	approval := socialhub.ApprovalUnknown
	if strings.TrimSpace(client.approval.AccountType) != "" {
		approval = socialhub.ApprovalGranted
	}
	state := func(capability socialhub.Capability, reason string) socialhub.CapabilityState {
		return socialhub.CapabilityState{
			Capability: capability, Supported: true, Approval: approval,
			Reason: reason, DocURL: documentationURL,
		}
	}
	return socialhub.Capabilities{
		CapabilityProductDiscovery: state(CapabilityProductDiscovery, "commissionable product search and batch product detail"),
		CapabilityLinkGeneration:   state(CapabilityLinkGeneration, "standard and hot-product publisher link generation"),
		CapabilityOrderAttribution: state(CapabilityOrderAttribution, "paged order attribution and sub-order detail reconciliation"),
		socialhub.CapPublish:       {Capability: socialhub.CapPublish, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "affiliate products are not organic social posts"},
		socialhub.CapFetch:         {Capability: socialhub.CapFetch, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "commerce discovery uses typed AliExpress Affiliate workflows"},
		socialhub.CapMedia:         {Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "the Affiliate API returns hosted media URLs and does not upload media"},
		socialhub.CapReact:         {Capability: socialhub.CapReact, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "the Affiliate API has no organic reaction surface"},
		socialhub.CapMessage:       {Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "the Affiliate API is not a messaging product"},
		socialhub.CapWebhook:       {Capability: socialhub.CapWebhook, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "order reconciliation is pull-based in this adapter"},
	}, nil
}

func (client *Client) Publisher() (socialhub.Publisher, bool)           { return nil, false }
func (client *Client) Fetcher() (socialhub.Fetcher, bool)               { return nil, false }
func (client *Client) MediaUploader() (socialhub.MediaUploader, bool)   { return nil, false }
func (client *Client) Reactor() (socialhub.Reactor, bool)               { return nil, false }
func (client *Client) Messenger() (socialhub.Messenger, bool)           { return nil, false }
func (client *Client) WebhookHandler() (socialhub.WebhookHandler, bool) { return nil, false }
func (client *Client) Close() error                                     { return nil }

func (client *Client) Products() ProductWorkflow { return client }
func (client *Client) Links() LinkWorkflow       { return client }
func (client *Client) Orders() OrderWorkflow     { return client }

func (client *Client) trackingID(operation, override string, required bool) (string, error) {
	value := override
	if value == "" {
		value = client.defaultTrackingID
	}
	if value == "" && !required {
		return "", nil
	}
	if !validCSVValue(value, 512) {
		return "", invalidArgument(operation, "tracking_id or account.settings.default_tracking_id is required and must not contain a comma")
	}
	return value, nil
}

func (client *Client) appSignature(override string) string {
	if override != "" {
		return override
	}
	return client.defaultAppSignature
}

var _ socialhub.Client = (*Client)(nil)
