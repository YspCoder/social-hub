package unitystatistics

import (
	"context"
	"net/http"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const CapabilityAdvertisingStatistics socialhub.Capability = "unity_advertising_statistics"

// Client exposes one Unity organization's acquisition reporting workflows.
type Client struct {
	accountID      socialhub.AccountID
	organizationID string
	api            *transport.Client
	httpClient     *http.Client
}

func (client *Client) Platform() socialhub.Platform { return platformName }
func (client *Client) Account() socialhub.AccountID { return client.accountID }

func (client *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	return socialhub.Capabilities{
		CapabilityAdvertisingStatistics: {
			Capability: CapabilityAdvertisingStatistics, Supported: true, Approval: socialhub.ApprovalRequired,
			Reason: "organization-bound Acquisition and SKAN reporting; Advertise Stats API Viewer role required",
			DocURL: documentationURL,
		},
		socialhub.CapPublish: {Capability: socialhub.CapPublish, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "statistics reporting is not organic publishing"},
		socialhub.CapFetch:   {Capability: socialhub.CapFetch, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "advertising reports use the typed Reports workflow"},
		socialhub.CapMedia:   {Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "statistics reporting has no media upload surface"},
		socialhub.CapReact:   {Capability: socialhub.CapReact, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "statistics reporting has no engagement workflow"},
		socialhub.CapMessage: {Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "statistics reporting has no messaging surface"},
		socialhub.CapWebhook: {Capability: socialhub.CapWebhook, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Advertising Statistics API v2 does not expose webhooks"},
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

func (client *Client) reportPath(name string) string {
	return "/advertise/stats/v2/organizations/" + client.organizationID + "/reports/" + name
}

var _ socialhub.Client = (*Client)(nil)
