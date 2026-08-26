package tenjin

import (
	"context"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	CapabilityEvents    socialhub.Capability = "tenjin_s2s_events"
	CapabilityPurchases socialhub.Capability = "tenjin_s2s_purchases"
	CapabilityAdRevenue socialhub.Capability = "tenjin_s2s_ad_revenue"
)

type Client struct {
	accountID socialhub.AccountID
	bundleID  string
	platform  Platform
	googleAds bool
	metaAEM   bool
	api       *transport.Client
}

func (client *Client) Platform() socialhub.Platform { return platformName }
func (client *Client) Account() socialhub.AccountID { return client.accountID }

func (*Client) String() string   { return "tenjin.Client(<redacted credentials>)" }
func (*Client) GoString() string { return "tenjin.Client(<redacted credentials>)" }

func (client *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	return socialhub.Capabilities{
		CapabilityEvents: {
			Capability: CapabilityEvents, Supported: true, Approval: socialhub.ApprovalUnknown,
			Reason: "app opens and custom measurement events through Tenjin S2S v0",
			DocURL: documentationURL,
		},
		CapabilityPurchases: {
			Capability: CapabilityPurchases, Supported: true, Approval: socialhub.ApprovalUnknown,
			Reason: "purchase measurement events through Tenjin S2S v0",
			DocURL: documentationURL,
		},
		CapabilityAdRevenue: {
			Capability: CapabilityAdRevenue, Supported: true, Approval: socialhub.ApprovalUnknown,
			Reason: "direct generic impression-level ad revenue ingestion",
			DocURL: "https://tenjin.com/docs/impression-level-revenue-data-api-s2s/",
		},
		socialhub.CapPublish: {Capability: socialhub.CapPublish, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "measurement events are not organic posts"},
		socialhub.CapFetch:   {Capability: socialhub.CapFetch, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "this adapter exposes ingestion; Tenjin reporting APIs are separate"},
		socialhub.CapMedia:   {Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Tenjin S2S measurement does not upload media"},
		socialhub.CapReact:   {Capability: socialhub.CapReact, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Tenjin S2S measurement has no engagement surface"},
		socialhub.CapMessage: {Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Tenjin S2S measurement has no messaging surface"},
		socialhub.CapWebhook: {Capability: socialhub.CapWebhook, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "S2S callbacks are configured through a separate Tenjin API"},
	}, nil
}

func (client *Client) Publisher() (socialhub.Publisher, bool)           { return nil, false }
func (client *Client) Fetcher() (socialhub.Fetcher, bool)               { return nil, false }
func (client *Client) MediaUploader() (socialhub.MediaUploader, bool)   { return nil, false }
func (client *Client) Reactor() (socialhub.Reactor, bool)               { return nil, false }
func (client *Client) Messenger() (socialhub.Messenger, bool)           { return nil, false }
func (client *Client) WebhookHandler() (socialhub.WebhookHandler, bool) { return nil, false }
func (client *Client) Close() error                                     { return nil }

func (client *Client) S2S() S2SWorkflow { return client }

var _ socialhub.Client = (*Client)(nil)
