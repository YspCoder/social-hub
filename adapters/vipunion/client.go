package vipunion

import (
	"context"
	"strings"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	CapabilityGoodsDiscovery   socialhub.Capability = "vipshop_goods_discovery"
	CapabilityLinkGeneration   socialhub.Capability = "vipshop_link_generation"
	CapabilityOrderAttribution socialhub.Capability = "vipshop_order_attribution"
)

// Client exposes Vipshop Union workflows for one configured publisher.
type Client struct {
	accountID      socialhub.AccountID
	defaultChanTag string
	defaultOpenID  string
	defaultAdCode  string
	api            *transport.Client
	approval       socialhub.ApprovalConfig
	clock          socialhub.Clock
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
		CapabilityGoodsDiscovery:   state(CapabilityGoodsDiscovery, "keyword search, batch goods detail, and channel-aware marketing detail"),
		CapabilityLinkGeneration:   state(CapabilityLinkGeneration, "CPS web, deep-link, mini-program, quick-app, and command outputs"),
		CapabilityOrderAttribution: state(CapabilityOrderAttribution, "V2 attributed orders by order time, update time, or order number"),
		socialhub.CapPublish:       {Capability: socialhub.CapPublish, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "affiliate goods are not organic social posts"},
		socialhub.CapFetch:         {Capability: socialhub.CapFetch, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "commerce discovery uses typed Vipshop Union workflows"},
		socialhub.CapMedia:         {Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "this adapter returns hosted commerce media and does not upload assets"},
		socialhub.CapReact:         {Capability: socialhub.CapReact, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "affiliate APIs have no organic reaction surface"},
		socialhub.CapMessage:       {Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Vipshop Union is not a messaging product"},
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

func (client *Client) Goods() GoodsWorkflow  { return client }
func (client *Client) Links() LinkWorkflow   { return client }
func (client *Client) Orders() OrderWorkflow { return client }

func (client *Client) chanTag(override string) string {
	if override != "" {
		return override
	}
	if client.defaultChanTag != "" {
		return client.defaultChanTag
	}
	return "default_pid"
}

func (client *Client) openID(override string) string {
	if override != "" {
		return override
	}
	if client.defaultOpenID != "" {
		return client.defaultOpenID
	}
	return "default_open_id"
}

func (client *Client) adCode(override string) string {
	if override != "" {
		return override
	}
	if client.defaultAdCode != "" {
		return client.defaultAdCode
	}
	return "unionapi"
}

var _ socialhub.Client = (*Client)(nil)
