package xandr

import (
	"context"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	CapabilityAdvertiserRead socialhub.Capability = "xandr_advertiser_read"
	CapabilityCampaignRead   socialhub.Capability = "xandr_campaign_read"
)

// Client exposes one API user's typed, read-only workflows.
type Client struct {
	accountID socialhub.AccountID
	api       *transport.Client
	sessions  *sessionTokenSource
	clock     socialhub.Clock
}

func (client *Client) Platform() socialhub.Platform { return platformName }
func (client *Client) Account() socialhub.AccountID { return client.accountID }

func (client *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	return socialhub.Capabilities{
		CapabilityAdvertiserRead: {
			Capability: CapabilityAdvertiserRead, Supported: true, Approval: socialhub.ApprovalUnknown,
			Reason: "advertiser lookup and bounded pagination; visible objects depend on the API user's member permissions",
			DocURL: documentationURL + "advertiser-service",
		},
		CapabilityCampaignRead: {
			Capability: CapabilityCampaignRead, Supported: true, Approval: socialhub.ApprovalUnknown,
			Reason: "advertiser-scoped campaign lookup and bounded pagination",
			DocURL: documentationURL + "campaign-service",
		},
		socialhub.CapPublish: {Capability: socialhub.CapPublish, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "programmatic advertising is separate from organic publishing"},
		socialhub.CapFetch:   {Capability: socialhub.CapFetch, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "paid-media reads use typed Xandr workflows"},
		socialhub.CapMedia:   {Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "creative and media services are outside this initial adapter"},
		socialhub.CapReact:   {Capability: socialhub.CapReact, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Digital Platform API has no organic reaction surface"},
		socialhub.CapMessage: {Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Digital Platform API has no general messaging surface"},
		socialhub.CapWebhook: {Capability: socialhub.CapWebhook, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "webhooks are outside these read-only workflows"},
	}, nil
}

func (client *Client) Publisher() (socialhub.Publisher, bool)           { return nil, false }
func (client *Client) Fetcher() (socialhub.Fetcher, bool)               { return nil, false }
func (client *Client) MediaUploader() (socialhub.MediaUploader, bool)   { return nil, false }
func (client *Client) Reactor() (socialhub.Reactor, bool)               { return nil, false }
func (client *Client) Messenger() (socialhub.Messenger, bool)           { return nil, false }
func (client *Client) WebhookHandler() (socialhub.WebhookHandler, bool) { return nil, false }
func (client *Client) Close() error {
	if client.sessions != nil {
		client.sessions.Close()
	}
	return nil
}

func (client *Client) Advertisers() AdvertiserWorkflow { return client }
func (client *Client) Campaigns() CampaignWorkflow     { return client }

var _ socialhub.Client = (*Client)(nil)
var _ AdvertiserWorkflow = (*Client)(nil)
var _ CampaignWorkflow = (*Client)(nil)
