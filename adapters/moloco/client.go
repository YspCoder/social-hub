package moloco

import (
	"context"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const CapabilityAsyncReporting socialhub.Capability = "moloco_ads_async_reporting"

// Client exposes the read-only report workflow for one Moloco ad account.
type Client struct {
	accountID   socialhub.AccountID
	adAccountID string
	api         *transport.Client
	tokens      *tokenSource
}

func (client *Client) Platform() socialhub.Platform { return platformName }
func (client *Client) Account() socialhub.AccountID { return client.accountID }

func (client *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	return socialhub.Capabilities{
		CapabilityAsyncReporting: {
			Capability: CapabilityAsyncReporting, Supported: true, Approval: socialhub.ApprovalUnknown,
			Reason: "read existing asynchronous report metadata and generation status for the configured ad account",
			DocURL: documentationURL,
		},
		socialhub.CapPublish: {Capability: socialhub.CapPublish, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Moloco paid advertising is not an organic social publishing surface"},
		socialhub.CapFetch:   {Capability: socialhub.CapFetch, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "paid-media reports use the typed Reports workflow"},
		socialhub.CapMedia:   {Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "creative asset management is outside this read-only adapter"},
		socialhub.CapReact:   {Capability: socialhub.CapReact, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Moloco Ads has no organic reaction surface"},
		socialhub.CapMessage: {Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Moloco Ads has no messaging surface"},
		socialhub.CapWebhook: {Capability: socialhub.CapWebhook, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "the Report API exposes polling rather than webhooks"},
	}, nil
}

func (client *Client) Publisher() (socialhub.Publisher, bool)           { return nil, false }
func (client *Client) Fetcher() (socialhub.Fetcher, bool)               { return nil, false }
func (client *Client) MediaUploader() (socialhub.MediaUploader, bool)   { return nil, false }
func (client *Client) Reactor() (socialhub.Reactor, bool)               { return nil, false }
func (client *Client) Messenger() (socialhub.Messenger, bool)           { return nil, false }
func (client *Client) WebhookHandler() (socialhub.WebhookHandler, bool) { return nil, false }

func (client *Client) Reports() ReportWorkflow { return client }

func (client *Client) Close() error {
	client.tokens.close()
	return nil
}

var _ socialhub.Client = (*Client)(nil)
