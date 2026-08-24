package airbridge

import (
	"context"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	CapabilityMobileEvents socialhub.Capability = "airbridge_s2s_mobile_events"
	CapabilityWebEvents    socialhub.Capability = "airbridge_s2s_web_events"
)

// Client exposes Airbridge event ingestion for one configured app.
type Client struct {
	accountID socialhub.AccountID
	appName   string
	api       *transport.Client
	clock     socialhub.Clock
}

func (client *Client) Platform() socialhub.Platform { return platformName }
func (client *Client) Account() socialhub.AccountID { return client.accountID }

func (client *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	return socialhub.Capabilities{
		CapabilityMobileEvents: {
			Capability: CapabilityMobileEvents, Supported: true, Approval: socialhub.ApprovalUnknown,
			Reason: "mobile-app S2S events with a documented 1,000 requests/minute app limit", DocURL: documentationURL,
		},
		CapabilityWebEvents: {
			Capability: CapabilityWebEvents, Supported: true, Approval: socialhub.ApprovalUnknown,
			Reason: "web S2S events with browser attribution data and a documented 1,000 requests/minute app limit", DocURL: documentationURL,
		},
		socialhub.CapPublish: {Capability: socialhub.CapPublish, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "attribution events are not organic posts"},
		socialhub.CapFetch:   {Capability: socialhub.CapFetch, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "S2S Events API v2 is an ingestion product"},
		socialhub.CapMedia:   {Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "S2S Events API v2 does not upload media"},
		socialhub.CapReact:   {Capability: socialhub.CapReact, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "S2S Events API v2 has no engagement surface"},
		socialhub.CapMessage: {Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "S2S Events API v2 has no messaging surface"},
		socialhub.CapWebhook: {Capability: socialhub.CapWebhook, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Airbridge outbound integrations are separate products"},
	}, nil
}

func (client *Client) Publisher() (socialhub.Publisher, bool)           { return nil, false }
func (client *Client) Fetcher() (socialhub.Fetcher, bool)               { return nil, false }
func (client *Client) MediaUploader() (socialhub.MediaUploader, bool)   { return nil, false }
func (client *Client) Reactor() (socialhub.Reactor, bool)               { return nil, false }
func (client *Client) Messenger() (socialhub.Messenger, bool)           { return nil, false }
func (client *Client) WebhookHandler() (socialhub.WebhookHandler, bool) { return nil, false }
func (client *Client) Close() error                                     { return nil }

func (client *Client) Events() EventWorkflow { return client }

var _ socialhub.Client = (*Client)(nil)
