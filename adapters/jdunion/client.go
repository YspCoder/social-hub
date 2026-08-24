package jdunion

import (
	"context"
	"strings"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	CapabilityGoodsDiscovery   socialhub.Capability = "jd_union_goods_discovery"
	CapabilityLinkConversion   socialhub.Capability = "jd_union_link_conversion"
	CapabilityOrderAttribution socialhub.Capability = "jd_union_order_attribution"
)

// Client exposes JD Union workflows for one configured publisher.
type Client struct {
	accountID     socialhub.AccountID
	defaultSiteID string
	gatewayPath   string
	api           *transport.Client
	approval      socialhub.ApprovalConfig
	clock         socialhub.Clock
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
		CapabilityGoodsDiscovery:   state(CapabilityGoodsDiscovery, "Jingfen channel discovery with exact-value product and commission data"),
		CapabilityLinkConversion:   state(CapabilityLinkConversion, "website or app affiliate-link conversion with optional JD command generation"),
		CapabilityOrderAttribution: state(CapabilityOrderAttribution, "bounded order-row attribution by created, completed, or updated time"),
		socialhub.CapPublish:       {Capability: socialhub.CapPublish, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "affiliate goods are not organic social posts"},
		socialhub.CapFetch:         {Capability: socialhub.CapFetch, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "commerce discovery uses typed JD Union workflows"},
		socialhub.CapMedia:         {Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "JD Union returns hosted media URLs and does not upload media"},
		socialhub.CapReact:         {Capability: socialhub.CapReact, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "affiliate APIs have no organic reaction surface"},
		socialhub.CapMessage:       {Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "JD Union is not a messaging product"},
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

func (client *Client) Goods() GoodsWorkflow          { return client }
func (client *Client) Promotions() PromotionWorkflow { return client }
func (client *Client) Orders() OrderWorkflow         { return client }

func (client *Client) siteID(operation, override string) (string, error) {
	value := override
	if value == "" {
		value = client.defaultSiteID
	}
	if !validIdentifier(value, 256) {
		return "", invalidArgument(operation, "site_id or account.settings.default_site_id is required")
	}
	return value, nil
}

var _ socialhub.Client = (*Client)(nil)
