package adsense

import (
	"context"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const CapabilityAdSense socialhub.Capability = "google_adsense_account_inventory_compliance_reporting"

// Client exposes one publisher account's read-only AdSense workflows.
type Client struct {
	accountID   socialhub.AccountID
	publisherID string
	api         *transport.Client
	scopes      []string
}

func (client *Client) Platform() socialhub.Platform { return platformName }
func (client *Client) Account() socialhub.AccountID { return client.accountID }

func (client *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	return socialhub.Capabilities{
		CapabilityAdSense: {
			Capability: CapabilityAdSense, Supported: true, Approval: socialhub.ApprovalUnknown,
			Reason: "Publisher-bound account, inventory, compliance, payments, and JSON reporting reads; restricted AdSense for Platforms mutation methods are intentionally omitted",
			DocURL: documentationURL,
		},
		socialhub.CapPublish: {Capability: socialhub.CapPublish, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "AdSense inventory is not organic social publishing"},
		socialhub.CapFetch:   {Capability: socialhub.CapFetch, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "publisher resources use typed AdSense workflows"},
		socialhub.CapMedia:   {Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "AdSense Management API does not upload social media"},
		socialhub.CapReact:   {Capability: socialhub.CapReact, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "AdSense has no organic engagement workflow"},
		socialhub.CapMessage: {Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "AdSense has no social messaging"},
		socialhub.CapWebhook: {Capability: socialhub.CapWebhook, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "AdSense Management API v2 does not expose these resources as social webhooks"},
	}, nil
}

func (client *Client) Publisher() (socialhub.Publisher, bool)           { return nil, false }
func (client *Client) Fetcher() (socialhub.Fetcher, bool)               { return nil, false }
func (client *Client) MediaUploader() (socialhub.MediaUploader, bool)   { return nil, false }
func (client *Client) Reactor() (socialhub.Reactor, bool)               { return nil, false }
func (client *Client) Messenger() (socialhub.Messenger, bool)           { return nil, false }
func (client *Client) WebhookHandler() (socialhub.WebhookHandler, bool) { return nil, false }
func (client *Client) Close() error                                     { return nil }

func (client *Client) Accounts() AccountsWorkflow     { return client }
func (client *Client) Inventory() InventoryWorkflow   { return client }
func (client *Client) Compliance() ComplianceWorkflow { return client }
func (client *Client) Reporting() ReportingWorkflow   { return client }

func (client *Client) requireReadScope(operation string) error {
	if len(client.scopes) == 0 {
		return nil
	}
	for _, configured := range client.scopes {
		if configured == fullScope || configured == readOnlyScope {
			return nil
		}
	}
	return &socialhub.Error{
		Code: socialhub.CodeApprovalRequired, Class: socialhub.ClassUserAction,
		Platform: platformName, Product: productName, Op: operation,
		RequiredScopes: []string{readOnlyScope, fullScope}, ApprovalURL: defaultAuthURL,
		PlatformMessage: "configured approval scopes do not authorize this AdSense Management API workflow",
	}
}

func (client *Client) accountName() string { return "accounts/" + client.publisherID }

var _ socialhub.Client = (*Client)(nil)
