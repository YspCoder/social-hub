package applovinmax

import (
	"context"
	"net/http"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const CapabilityMAXReporting socialhub.Capability = "applovin_max_reporting"

// Client exposes one AppLovin account's MAX reporting workflows.
type Client struct {
	accountID       socialhub.AccountID
	api             *transport.Client
	httpClient      *http.Client
	clock           socialhub.Clock
	downloadOrigins map[string]struct{}
}

func (client *Client) Platform() socialhub.Platform { return platformName }
func (client *Client) Account() socialhub.AccountID { return client.accountID }

func (client *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	return socialhub.Capabilities{
		CapabilityMAXReporting: {
			Capability: CapabilityMAXReporting, Supported: true, Approval: socialhub.ApprovalUnknown,
			Reason: "Report Key access to MAX revenue, user-level revenue, and cohort reports",
			DocURL: documentationURL,
		},
		socialhub.CapPublish: {Capability: socialhub.CapPublish, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "MAX Reporting APIs are read-only monetization reports"},
		socialhub.CapFetch:   {Capability: socialhub.CapFetch, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "MAX reports use the typed Reports workflow"},
		socialhub.CapMedia:   {Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "MAX Reporting APIs do not upload media"},
		socialhub.CapReact:   {Capability: socialhub.CapReact, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "MAX Reporting APIs do not expose engagement actions"},
		socialhub.CapMessage: {Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "MAX Reporting APIs do not expose messaging"},
		socialhub.CapWebhook: {Capability: socialhub.CapWebhook, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "S2S impression postbacks are configured outside the reporting client"},
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
