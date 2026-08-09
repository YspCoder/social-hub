package admob

import (
	"context"
	"net/http"
	"slices"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const CapabilityAdMob socialhub.Capability = "google_admob_account_inventory_reporting"

// Client exposes one publisher account's read-only AdMob workflows.
type Client struct {
	accountID   socialhub.AccountID
	publisherID string
	api         *transport.Client
	httpClient  *http.Client
	scopes      []string
}

func (client *Client) Platform() socialhub.Platform { return platformName }
func (client *Client) Account() socialhub.AccountID { return client.accountID }

func (client *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	return socialhub.Capabilities{
		CapabilityAdMob: {
			Capability: CapabilityAdMob, Supported: true, Approval: socialhub.ApprovalUnknown,
			Reason: "Publisher-bound account and inventory reads plus AdMob Network and third-party Mediation reports",
			DocURL: documentationURL,
		},
		socialhub.CapPublish: {Capability: socialhub.CapPublish, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "AdMob inventory is not organic social publishing"},
		socialhub.CapFetch:   {Capability: socialhub.CapFetch, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "publisher resources use typed AdMob workflows"},
		socialhub.CapMedia:   {Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "AdMob API v1 does not upload social media"},
		socialhub.CapReact:   {Capability: socialhub.CapReact, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "AdMob has no organic engagement workflow"},
		socialhub.CapMessage: {Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "AdMob has no social messaging"},
		socialhub.CapWebhook: {Capability: socialhub.CapWebhook, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "AdMob API v1 does not expose these resources as webhooks"},
	}, nil
}

func (client *Client) Publisher() (socialhub.Publisher, bool)           { return nil, false }
func (client *Client) Fetcher() (socialhub.Fetcher, bool)               { return nil, false }
func (client *Client) MediaUploader() (socialhub.MediaUploader, bool)   { return nil, false }
func (client *Client) Reactor() (socialhub.Reactor, bool)               { return nil, false }
func (client *Client) Messenger() (socialhub.Messenger, bool)           { return nil, false }
func (client *Client) WebhookHandler() (socialhub.WebhookHandler, bool) { return nil, false }
func (client *Client) Close() error                                     { return nil }

func (client *Client) Accounts() AccountsWorkflow   { return client }
func (client *Client) Inventory() InventoryWorkflow { return client }
func (client *Client) Reporting() ReportingWorkflow { return client }

func (client *Client) requireScope(operation string, allowed ...string) error {
	if len(client.scopes) == 0 {
		return nil
	}
	for _, configured := range client.scopes {
		if slices.Contains(allowed, configured) {
			return nil
		}
	}
	return &socialhub.Error{
		Code: socialhub.CodeApprovalRequired, Class: socialhub.ClassUserAction,
		Platform: platformName, Product: productName, Op: operation,
		RequiredScopes: append([]string(nil), allowed...), ApprovalURL: defaultAuthURL,
		PlatformMessage: "configured approval scopes do not authorize this AdMob API workflow",
	}
}

func (client *Client) accountName() string { return "accounts/" + client.publisherID }

var _ socialhub.Client = (*Client)(nil)
