package applovinreporting

import (
	"context"
	"net/http"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	CapabilityGrowthReporting   socialhub.Capability = "applovin_growth_reporting"
	CapabilityAssetReporting    socialhub.Capability = "applovin_growth_asset_reporting"
	CapabilityPlayableReporting socialhub.Capability = "applovin_growth_playable_reporting"
)

// Client exposes one AppLovin Ads Manager account's reporting workflows.
type Client struct {
	accountID     socialhub.AccountID
	axonAccountID string
	accountType   AccountType
	api           *transport.Client
	httpClient    *http.Client
	clock         socialhub.Clock
}

func (client *Client) Platform() socialhub.Platform { return platformName }
func (client *Client) Account() socialhub.AccountID { return client.accountID }

func (client *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	playable := socialhub.CapabilityState{
		Capability: CapabilityPlayableReporting, Supported: client.accountType == AccountTypeApp,
		Approval: socialhub.ApprovalUnknown, DocURL: playableDocumentationURL,
		Reason: "HTML/playable interaction, completion, redirect, and rendering-health metrics; APP accounts only",
	}
	if client.accountType == AccountTypeWeb {
		playable.Reason = "HTML Metrics API is not available to WEB advertiser accounts"
	}
	return socialhub.Capabilities{
		CapabilityGrowthReporting: {
			Capability: CapabilityGrowthReporting, Supported: true, Approval: socialhub.ApprovalUnknown,
			Reason: "account-bound advertiser reports; APP accounts can also request the distinct publisher report type", DocURL: documentationURL,
		},
		CapabilityAssetReporting: {
			Capability: CapabilityAssetReporting, Supported: true, Approval: socialhub.ApprovalUnknown,
			Reason: "asset-level spend, impression, click, and CTR reports", DocURL: assetDocumentationURL,
		},
		CapabilityPlayableReporting: playable,
		socialhub.CapPublish:        {Capability: socialhub.CapPublish, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Growth Reporting APIs are read-only"},
		socialhub.CapFetch:          {Capability: socialhub.CapFetch, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "paid-media reports use the typed Reports workflow"},
		socialhub.CapMedia:          {Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "use the separate Growth Campaign Management adapter for creative Assets"},
		socialhub.CapReact:          {Capability: socialhub.CapReact, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Growth Reporting APIs do not expose organic engagement"},
		socialhub.CapMessage:        {Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Growth Reporting APIs do not expose messaging"},
		socialhub.CapWebhook:        {Capability: socialhub.CapWebhook, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Growth Reporting APIs do not expose webhooks"},
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
