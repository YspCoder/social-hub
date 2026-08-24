package panglereporting

import (
	"context"
	"net/http"
	"net/url"

	"social-hub/pkg/socialhub"
)

const CapabilityPublisherReporting socialhub.Capability = "pangle_publisher_reporting"

// Client exposes one Pangle role's non-mainland publisher income reports.
type Client struct {
	accountID        socialhub.AccountID
	userID           string
	roleID           string
	securityKey      string
	baseURL          *url.URL
	httpClient       *http.Client
	clock            socialhub.Clock
	maxResponseBytes int64
}

func (client *Client) Platform() socialhub.Platform { return platformName }
func (client *Client) Account() socialhub.AccountID { return client.accountID }

func (client *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	return socialhub.Capabilities{
		CapabilityPublisherReporting: {
			Capability: CapabilityPublisherReporting, Supported: true, Approval: socialhub.ApprovalRequired,
			Reason: "non-Chinese-mainland publisher revenue and delivery metrics; Pangle Data API role access and a Security Key are required",
			DocURL: documentationURL,
		},
		socialhub.CapPublish: {Capability: socialhub.CapPublish, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Publisher Reporting API 2.0 is read-only"},
		socialhub.CapFetch:   {Capability: socialhub.CapFetch, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "publisher income uses the typed Reports workflow"},
		socialhub.CapMedia:   {Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Reporting API 2.0 does not upload ad assets"},
		socialhub.CapReact:   {Capability: socialhub.CapReact, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "publisher reports have no organic engagement surface"},
		socialhub.CapMessage: {Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "publisher reports have no messaging surface"},
		socialhub.CapWebhook: {Capability: socialhub.CapWebhook, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Reporting API 2.0 does not expose webhooks"},
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
