package xiaomiglobalreporting

import (
	"context"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	CapabilityGlobalReporting socialhub.Capability = "xiaomi_global_reporting"
	CapabilityGlobalNameQuery socialhub.Capability = "xiaomi_global_reporting_names"
	CapabilityGlobalTokens    socialhub.Capability = "xiaomi_global_reporting_tokens"
)

// Client exposes one configured set of Xiaomi Global advertiser accounts.
type Client struct {
	accountID            socialhub.AccountID
	api                  *transport.Client
	clock                socialhub.Clock
	timestamps           *timestampSequence
	authorizedAccountIDs []int64
	authorizedAccounts   map[int64]struct{}
	redactionSecrets     []string
}

func (*Client) String() string   { return "xiaomiglobalreporting.Client(<redacted credentials>)" }
func (*Client) GoString() string { return "xiaomiglobalreporting.Client(<redacted credentials>)" }

func (client *Client) Platform() socialhub.Platform { return platformName }
func (client *Client) Account() socialhub.AccountID { return client.accountID }

func (client *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	return socialhub.Capabilities{
		CapabilityGlobalReporting: {
			Capability: CapabilityGlobalReporting, Supported: true, Approval: socialhub.ApprovalRequired,
			Reason: "daily UTC Effect and Brand delivery reports require AM-approved Reporting API access and authorized account IDs",
			DocURL: documentationURL,
		},
		CapabilityGlobalNameQuery: {
			Capability: CapabilityGlobalNameQuery, Supported: true, Approval: socialhub.ApprovalRequired,
			Reason: "account, campaign, ad group, and creative name lookup requires Xiaomi Global Reporting API approval",
			DocURL: documentationURL,
		},
		CapabilityGlobalTokens: {
			Capability: CapabilityGlobalTokens, Supported: true, Approval: socialhub.ApprovalRequired,
			Reason: "token creation and rotation require the appId and appKey issued after AM approval",
			DocURL: documentationURL,
		},
		socialhub.CapPublish: {Capability: socialhub.CapPublish, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "the Reporting API adapter is intentionally read-only"},
		socialhub.CapFetch:   {Capability: socialhub.CapFetch, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "paid-media data uses the typed Reports workflow"},
		socialhub.CapMedia:   {Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "creative upload belongs to the separate Marketing API"},
		socialhub.CapReact:   {Capability: socialhub.CapReact, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "advertising reports have no organic reaction surface"},
		socialhub.CapMessage: {Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "advertising reports have no messaging surface"},
		socialhub.CapWebhook: {Capability: socialhub.CapWebhook, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "the Reporting API does not expose webhooks"},
	}, nil
}

func (client *Client) Publisher() (socialhub.Publisher, bool)           { return nil, false }
func (client *Client) Fetcher() (socialhub.Fetcher, bool)               { return nil, false }
func (client *Client) MediaUploader() (socialhub.MediaUploader, bool)   { return nil, false }
func (client *Client) Reactor() (socialhub.Reactor, bool)               { return nil, false }
func (client *Client) Messenger() (socialhub.Messenger, bool)           { return nil, false }
func (client *Client) WebhookHandler() (socialhub.WebhookHandler, bool) { return nil, false }
func (client *Client) Close() error                                     { return nil }

func (client *Client) Reports() ReportsWorkflow { return client }

var _ socialhub.Client = (*Client)(nil)
