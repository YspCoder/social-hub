package taobaounion

import (
	"context"
	"strings"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	CapabilityMaterialDiscovery socialhub.Capability = "alimama_material_discovery"
	CapabilityLinkConversion    socialhub.Capability = "alimama_link_conversion"
	CapabilityTaoPassword       socialhub.Capability = "alimama_tao_password"
	CapabilityOrderAttribution  socialhub.Capability = "alimama_order_attribution"
)

// Client exposes Taobao Union workflows for one configured publisher.
type Client struct {
	accountID       socialhub.AccountID
	defaultAdzoneID string
	gatewayPath     string
	api             *transport.Client
	approval        socialhub.ApprovalConfig
	clock           socialhub.Clock
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
		CapabilityMaterialDiscovery: state(CapabilityMaterialDiscovery, "publisher material search and exact-value product discovery"),
		CapabilityLinkConversion:    state(CapabilityLinkConversion, "typed item and material affiliate-link conversion"),
		CapabilityTaoPassword:       state(CapabilityTaoPassword, "Tao Password generation from an official affiliate URL"),
		CapabilityOrderAttribution:  state(CapabilityOrderAttribution, "bounded publisher-order attribution with cursor pagination"),
		socialhub.CapPublish:        {Capability: socialhub.CapPublish, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "affiliate materials are not organic social posts"},
		socialhub.CapFetch:          {Capability: socialhub.CapFetch, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "commerce discovery uses typed Taobao Union workflows"},
		socialhub.CapMedia:          {Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Taobao Union returns hosted media URLs and does not upload media"},
		socialhub.CapReact:          {Capability: socialhub.CapReact, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "affiliate APIs have no organic reaction surface"},
		socialhub.CapMessage:        {Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Taobao Union is not a messaging product"},
		socialhub.CapWebhook:        {Capability: socialhub.CapWebhook, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "TOP callbacks are outside this adapter contract"},
	}, nil
}

func (client *Client) Publisher() (socialhub.Publisher, bool)           { return nil, false }
func (client *Client) Fetcher() (socialhub.Fetcher, bool)               { return nil, false }
func (client *Client) MediaUploader() (socialhub.MediaUploader, bool)   { return nil, false }
func (client *Client) Reactor() (socialhub.Reactor, bool)               { return nil, false }
func (client *Client) Messenger() (socialhub.Messenger, bool)           { return nil, false }
func (client *Client) WebhookHandler() (socialhub.WebhookHandler, bool) { return nil, false }
func (client *Client) Close() error                                     { return nil }

func (client *Client) Materials() MaterialWorkflow { return client }
func (client *Client) Items() ItemWorkflow         { return client }
func (client *Client) Links() LinkWorkflow         { return client }
func (client *Client) Orders() OrderWorkflow       { return client }

func (client *Client) adzoneID(operation, override string) (string, error) {
	value := override
	if value == "" {
		value = client.defaultAdzoneID
	}
	if !validNumericID(value, 20) {
		return "", invalidArgument(operation, "adzone_id or account.settings.default_adzone_id is required")
	}
	return value, nil
}

var _ socialhub.Client = (*Client)(nil)
