package pddunion

import (
	"context"
	"strings"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	CapabilityGoodsDiscovery   socialhub.Capability = "pinduoduo_goods_discovery"
	CapabilityLinkGeneration   socialhub.Capability = "pinduoduo_link_generation"
	CapabilityOrderAttribution socialhub.Capability = "pinduoduo_order_attribution"
)

// Client exposes Duoduo Jinbao workflows for one configured publisher.
type Client struct {
	accountID   socialhub.AccountID
	defaultPID  string
	gatewayPath string
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
	state := func(capability socialhub.Capability, reason string) socialhub.CapabilityState {
		return socialhub.CapabilityState{
			Capability: capability, Supported: true, Approval: approval,
			Reason: reason, DocURL: documentationURL,
		}
	}
	return socialhub.Capabilities{
		CapabilityGoodsDiscovery:   state(CapabilityGoodsDiscovery, "channel recommendations and signed-goods detail with exact-value commerce data"),
		CapabilityLinkGeneration:   state(CapabilityLinkGeneration, "promotion links for web, mobile, mini-program, schema, and approved social surfaces"),
		CapabilityOrderAttribution: state(CapabilityOrderAttribution, "incremental publisher-order attribution by last update time"),
		socialhub.CapPublish:       {Capability: socialhub.CapPublish, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "affiliate goods are not organic social posts"},
		socialhub.CapFetch:         {Capability: socialhub.CapFetch, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "commerce discovery uses typed Duoduo Jinbao workflows"},
		socialhub.CapMedia:         {Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "this adapter returns hosted commerce media and does not upload filing assets"},
		socialhub.CapReact:         {Capability: socialhub.CapReact, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "affiliate APIs have no organic reaction surface"},
		socialhub.CapMessage:       {Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Duoduo Jinbao is not a messaging product"},
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

func (client *Client) requiredPID(operation, override string) (string, error) {
	value := override
	if value == "" {
		value = client.defaultPID
	}
	if !validPID(value) {
		return "", invalidArgument(operation, "pid or account.settings.default_pid is required")
	}
	return value, nil
}

func (client *Client) optionalPID(override string) string {
	if override != "" {
		return override
	}
	return client.defaultPID
}

var _ socialhub.Client = (*Client)(nil)
