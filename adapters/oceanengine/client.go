package oceanengine

import (
	"context"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	CapabilityMarketingManagement socialhub.Capability = "oceanengine_marketing_management"
	CapabilityMarketingReports    socialhub.Capability = "oceanengine_marketing_reports"
)

// Client exposes typed advertising workflows for one configured advertiser.
// Common organic-content capabilities are intentionally unavailable.
type Client struct {
	accountID    socialhub.AccountID
	advertiserID int64
	api          *transport.Client
}

func (client *Client) Platform() socialhub.Platform { return platformName }
func (client *Client) Account() socialhub.AccountID { return client.accountID }

func (client *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	return socialhub.Capabilities{
		CapabilityMarketingManagement: {
			Capability: CapabilityMarketingManagement, Supported: true, Approval: socialhub.ApprovalUnknown,
			Reason: "v3 Project and Promotion reads and mutations; app capability groups and advertiser authorization apply",
			DocURL: documentationURL,
		},
		CapabilityMarketingReports: {
			Capability: CapabilityMarketingReports, Supported: true, Approval: socialhub.ApprovalUnknown,
			Reason: "v3 synchronous custom reports; report permission and account authorization apply",
			DocURL: documentationURL,
		},
		socialhub.CapPublish: {Capability: socialhub.CapPublish, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "advertising resources are not social posts; use Projects() and Promotions()"},
		socialhub.CapFetch:   {Capability: socialhub.CapFetch, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "advertising reads use typed Marketing API resources"},
		socialhub.CapMedia:   {Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "initial adapter references material IDs through promotion extension fields"},
		socialhub.CapReact:   {Capability: socialhub.CapReact, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Marketing API is not an organic engagement product"},
		socialhub.CapMessage: {Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Marketing API is not a messaging product"},
		socialhub.CapWebhook: {Capability: socialhub.CapWebhook, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Ocean Engine SPI subscriptions are outside the initial adapter contract"},
	}, nil
}

func (client *Client) Publisher() (socialhub.Publisher, bool)         { return nil, false }
func (client *Client) Fetcher() (socialhub.Fetcher, bool)             { return nil, false }
func (client *Client) MediaUploader() (socialhub.MediaUploader, bool) { return nil, false }
func (client *Client) Reactor() (socialhub.Reactor, bool)             { return nil, false }
func (client *Client) Messenger() (socialhub.Messenger, bool)         { return nil, false }
func (client *Client) WebhookHandler() (socialhub.WebhookHandler, bool) {
	return nil, false
}
func (client *Client) Close() error { return nil }

func (client *Client) Projects() ProjectWorkflow     { return client }
func (client *Client) Promotions() PromotionWorkflow { return client }
func (client *Client) Reports() ReportWorkflow       { return client }

var _ socialhub.Client = (*Client)(nil)
