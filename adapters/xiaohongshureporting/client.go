package xiaohongshureporting

import (
	"context"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	CapabilitySpotlightBalance           socialhub.Capability = "xiaohongshu_spotlight_balance"
	CapabilitySpotlightOfflineReporting  socialhub.Capability = "xiaohongshu_spotlight_offline_reporting"
	CapabilitySpotlightRealtimeReporting socialhub.Capability = "xiaohongshu_spotlight_realtime_reporting"
)

// Client exposes one Spotlight advertiser's read-only financial and reporting workflows.
type Client struct {
	accountID    socialhub.AccountID
	advertiserID uint64
	api          *transport.Client
	clock        socialhub.Clock
}

func (client *Client) Platform() socialhub.Platform { return platformName }
func (client *Client) Account() socialhub.AccountID { return client.accountID }

func (client *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	return socialhub.Capabilities{
		CapabilitySpotlightBalance: {
			Capability: CapabilitySpotlightBalance, Supported: true, Approval: socialhub.ApprovalRequired,
			Reason: "advertiser balance visibility requires an approved Spotlight application, authorized advertiser, and account scope",
			DocURL: documentationURL,
		},
		CapabilitySpotlightOfflineReporting: {
			Capability: CapabilitySpotlightOfflineReporting, Supported: true, Approval: socialhub.ApprovalRequired,
			Reason: "account through search-word offline reports require Spotlight reporting approval",
			DocURL: documentationURL,
		},
		CapabilitySpotlightRealtimeReporting: {
			Capability: CapabilitySpotlightRealtimeReporting, Supported: true, Approval: socialhub.ApprovalRequired,
			Reason: "account through targeting realtime reports require Spotlight reporting approval",
			DocURL: documentationURL,
		},
		socialhub.CapPublish: {Capability: socialhub.CapPublish, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "this adapter is intentionally read-only"},
		socialhub.CapFetch:   {Capability: socialhub.CapFetch, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "paid-media reports use the typed Reports workflow"},
		socialhub.CapMedia:   {Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "this adapter does not upload Spotlight creatives"},
		socialhub.CapReact:   {Capability: socialhub.CapReact, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "advertising reports have no organic reaction surface"},
		socialhub.CapMessage: {Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "advertising reports have no messaging surface"},
		socialhub.CapWebhook: {Capability: socialhub.CapWebhook, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Spotlight reporting does not expose a webhook workflow"},
	}, nil
}

func (client *Client) Publisher() (socialhub.Publisher, bool)           { return nil, false }
func (client *Client) Fetcher() (socialhub.Fetcher, bool)               { return nil, false }
func (client *Client) MediaUploader() (socialhub.MediaUploader, bool)   { return nil, false }
func (client *Client) Reactor() (socialhub.Reactor, bool)               { return nil, false }
func (client *Client) Messenger() (socialhub.Messenger, bool)           { return nil, false }
func (client *Client) WebhookHandler() (socialhub.WebhookHandler, bool) { return nil, false }
func (client *Client) Close() error                                     { return nil }

func (client *Client) Reports() ReportWorkflow   { return client }
func (client *Client) Accounts() AccountWorkflow { return client }

var _ socialhub.Client = (*Client)(nil)
