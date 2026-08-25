package naversearchads

import (
	"context"
	"net/http"
	"net/url"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	CapabilityCampaignManagement socialhub.Capability = "naver_search_ads_campaign_management"
	CapabilityAdGroupManagement  socialhub.Capability = "naver_search_ads_adgroup_management"
	CapabilityKeywordManagement  socialhub.Capability = "naver_search_ads_keyword_management"
	CapabilityStats              socialhub.Capability = "naver_search_ads_stats"
	CapabilityStatReports        socialhub.Capability = "naver_search_ads_stat_reports"
)

// Client exposes one advertiser's paid-media workflows. Organic social
// capabilities are intentionally unavailable.
type Client struct {
	accountID   socialhub.AccountID
	customerID  int64
	api         *transport.Client
	httpClient  *http.Client
	baseURL     *url.URL
	decodeError func(int, http.Header, []byte) error
}

func (client *Client) Platform() socialhub.Platform { return platformName }
func (client *Client) Account() socialhub.AccountID { return client.accountID }

func (client *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	approved := socialhub.ApprovalRequired
	management := func(capability socialhub.Capability, reason string) socialhub.CapabilityState {
		return socialhub.CapabilityState{
			Capability: capability, Supported: true, Approval: approved,
			Reason: reason, DocURL: documentationURL,
		}
	}
	return socialhub.Capabilities{
		CapabilityCampaignManagement: management(CapabilityCampaignManagement, "API license, secret key, customer access, and explicit advertiser-side authorization are required; creates are forced paused"),
		CapabilityAdGroupManagement:  management(CapabilityAdGroupManagement, "customer-scoped Ad Group reads and paused-first management"),
		CapabilityKeywordManagement:  management(CapabilityKeywordManagement, "customer-scoped Keyword reads, paused-first batch creation, and bounded batch updates"),
		CapabilityStats:              management(CapabilityStats, "KST synchronous entity Stats with typed fields and exact JSON values"),
		CapabilityStatReports:        management(CapabilityStatReports, "asynchronous Stat Report jobs and bounded authenticated downloads"),
		socialhub.CapPublish:         {Capability: socialhub.CapPublish, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "paid search management is not organic publishing"},
		socialhub.CapFetch:           {Capability: socialhub.CapFetch, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "advertising reads use typed NAVER workflows"},
		socialhub.CapMedia:           {Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "creative asset workflows are outside this adapter version"},
		socialhub.CapReact:           {Capability: socialhub.CapReact, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Search AD API has no organic reaction surface"},
		socialhub.CapMessage:         {Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Search AD API has no messaging surface"},
		socialhub.CapWebhook:         {Capability: socialhub.CapWebhook, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Search AD API does not expose webhooks"},
	}, nil
}

func (client *Client) Publisher() (socialhub.Publisher, bool)           { return nil, false }
func (client *Client) Fetcher() (socialhub.Fetcher, bool)               { return nil, false }
func (client *Client) MediaUploader() (socialhub.MediaUploader, bool)   { return nil, false }
func (client *Client) Reactor() (socialhub.Reactor, bool)               { return nil, false }
func (client *Client) Messenger() (socialhub.Messenger, bool)           { return nil, false }
func (client *Client) WebhookHandler() (socialhub.WebhookHandler, bool) { return nil, false }
func (client *Client) Close() error                                     { return nil }

func (client *Client) Campaigns() CampaignWorkflow     { return client }
func (client *Client) AdGroups() AdGroupWorkflow       { return client }
func (client *Client) Keywords() KeywordWorkflow       { return client }
func (client *Client) Statistics() StatisticsWorkflow  { return client }
func (client *Client) StatReports() StatReportWorkflow { return client }

var _ socialhub.Client = (*Client)(nil)
