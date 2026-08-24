package lineads

import (
	"context"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	CapabilityAdAccountRead             socialhub.Capability = "line_ads_adaccount_read"
	CapabilityCampaignRead              socialhub.Capability = "line_ads_campaign_read"
	CapabilityPerformanceReportMetadata socialhub.Capability = "line_ads_performance_report_metadata"
	CapabilityOnlineReport              socialhub.Capability = "line_ads_online_report"
)

// Client exposes LINE Ads read workflows for one API-enabled group.
type Client struct {
	accountID   socialhub.AccountID
	groupID     string
	partnerType PartnerType
	api         *transport.Client
}

func (client *Client) Platform() socialhub.Platform { return platformName }
func (client *Client) Account() socialhub.AccountID { return client.accountID }

func (client *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	reportApproval := socialhub.ApprovalGranted
	reportReason := "Reporting (General) or Ad Tech (General) partner entitlement"
	if client.partnerType == PartnerDataGeneral {
		reportApproval = socialhub.ApprovalRequired
		reportReason = "Data Provider (General) documentation does not grant Campaign or reporting resources"
	}
	return socialhub.Capabilities{
		CapabilityAdAccountRead: {
			Capability: CapabilityAdAccountRead, Supported: true, Approval: socialhub.ApprovalGranted,
			Reason: "ad accounts authorized by the configured API-enabled group", DocURL: documentationURL,
		},
		CapabilityCampaignRead: {
			Capability: CapabilityCampaignRead, Supported: true, Approval: reportApproval,
			Reason: reportReason, DocURL: documentationURL,
		},
		CapabilityPerformanceReportMetadata: {
			Capability: CapabilityPerformanceReportMetadata, Supported: true, Approval: reportApproval,
			Reason: reportReason, DocURL: documentationURL,
		},
		CapabilityOnlineReport: {
			Capability: CapabilityOnlineReport, Supported: true, Approval: reportApproval,
			Reason: reportReason, DocURL: documentationURL,
		},
		socialhub.CapPublish: {Capability: socialhub.CapPublish, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "LINE Ads API is not an organic publishing product"},
		socialhub.CapFetch:   {Capability: socialhub.CapFetch, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "advertising reads use typed LINE Ads workflows"},
		socialhub.CapMedia:   {Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "media mutations are outside this restricted read surface"},
		socialhub.CapReact:   {Capability: socialhub.CapReact, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "LINE Ads API exposes no organic reactions"},
		socialhub.CapMessage: {Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "LINE Ads API is not LINE Messaging API"},
		socialhub.CapWebhook: {Capability: socialhub.CapWebhook, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "these workflows are request/response reads"},
	}, nil
}

func (client *Client) Publisher() (socialhub.Publisher, bool)           { return nil, false }
func (client *Client) Fetcher() (socialhub.Fetcher, bool)               { return nil, false }
func (client *Client) MediaUploader() (socialhub.MediaUploader, bool)   { return nil, false }
func (client *Client) Reactor() (socialhub.Reactor, bool)               { return nil, false }
func (client *Client) Messenger() (socialhub.Messenger, bool)           { return nil, false }
func (client *Client) WebhookHandler() (socialhub.WebhookHandler, bool) { return nil, false }
func (client *Client) Close() error                                     { return nil }

func (client *Client) Management() ManagementWorkflow { return client }

var _ socialhub.Client = (*Client)(nil)
