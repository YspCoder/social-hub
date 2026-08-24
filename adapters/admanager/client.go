package admanager

import (
	"context"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const CapabilityAdManager socialhub.Capability = "google_ad_manager_inventory_delivery_reporting"

// Client exposes one network's inventory, delivery, and reporting workflows.
type Client struct {
	accountID   socialhub.AccountID
	networkCode string
	api         *transport.Client
	scopes      []string
}

func (client *Client) Platform() socialhub.Platform { return platformName }
func (client *Client) Account() socialhub.AccountID { return client.accountID }

func (client *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	return socialhub.Capabilities{
		CapabilityAdManager: {
			Capability: CapabilityAdManager, Supported: true, Approval: socialhub.ApprovalUnknown,
			Reason: "Network-bound inventory and delivery reads plus hidden Interactive Report creation, execution, and row retrieval",
			DocURL: documentationURL,
		},
		socialhub.CapPublish: {Capability: socialhub.CapPublish, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Ad Manager trafficking is not organic social publishing"},
		socialhub.CapFetch:   {Capability: socialhub.CapFetch, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "publisher resources use typed Ad Manager workflows"},
		socialhub.CapMedia:   {Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "creative upload is outside the initial v1 contract"},
		socialhub.CapReact:   {Capability: socialhub.CapReact, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Ad Manager is not an organic engagement product"},
		socialhub.CapMessage: {Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Ad Manager does not provide social messaging"},
		socialhub.CapWebhook: {Capability: socialhub.CapWebhook, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Ad Manager v1 does not expose these workflows as social webhooks"},
	}, nil
}

func (client *Client) Publisher() (socialhub.Publisher, bool)           { return nil, false }
func (client *Client) Fetcher() (socialhub.Fetcher, bool)               { return nil, false }
func (client *Client) MediaUploader() (socialhub.MediaUploader, bool)   { return nil, false }
func (client *Client) Reactor() (socialhub.Reactor, bool)               { return nil, false }
func (client *Client) Messenger() (socialhub.Messenger, bool)           { return nil, false }
func (client *Client) WebhookHandler() (socialhub.WebhookHandler, bool) { return nil, false }
func (client *Client) Close() error                                     { return nil }

func (client *Client) Inventory() InventoryWorkflow { return client }
func (client *Client) Delivery() DeliveryWorkflow   { return client }
func (client *Client) Reporting() ReportingWorkflow { return client }

func (client *Client) requireAnyScope(operation string, allowed ...string) error {
	if len(client.scopes) == 0 {
		return nil
	}
	for _, configured := range client.scopes {
		for _, candidate := range allowed {
			if configured == candidate {
				return nil
			}
		}
	}
	return &socialhub.Error{
		Code: socialhub.CodeApprovalRequired, Class: socialhub.ClassUserAction,
		Platform: platformName, Product: productName, Op: operation,
		RequiredScopes: append([]string(nil), allowed...), ApprovalURL: defaultAuthURL,
		PlatformMessage: "configured approval scopes do not authorize this Google Ad Manager workflow",
	}
}

func (client *Client) networkName() string { return "networks/" + client.networkCode }

var _ socialhub.Client = (*Client)(nil)
