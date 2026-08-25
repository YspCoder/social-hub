package yahoosearchads

import (
	"context"
	"net/http"
	"strings"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	CapabilityCampaignManagement socialhub.Capability = "line_yahoo_search_ads_campaign_management"
	CapabilityAdGroupManagement  socialhub.Capability = "line_yahoo_search_ads_adgroup_management"
	CapabilityKeywordManagement  socialhub.Capability = "line_yahoo_search_ads_keyword_management"
	CapabilityReports            socialhub.Capability = "line_yahoo_search_ads_reports"
)

// Client exposes one advertiser's Search Ads workflows. The base account
// header independently constrains the MCC/account subtree available to calls.
type Client struct {
	accountID           socialhub.AccountID
	advertiserAccountID int64
	api                 *transport.Client
	httpClient          *http.Client
	scopes              []string
	clock               socialhub.Clock
	requestIDs          *requestIDFilter
	decodeError         transport.ErrorDecoder
}

func (client *Client) Platform() socialhub.Platform { return platformName }
func (client *Client) Account() socialhub.AccountID { return client.accountID }

func (client *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	approval := socialhub.ApprovalUnknown
	if len(client.scopes) > 0 {
		approval = socialhub.ApprovalRequired
		if scopeGranted(client.scopes, oauthScope) {
			approval = socialhub.ApprovalGranted
		}
	}
	state := func(capability socialhub.Capability, reason string) socialhub.CapabilityState {
		return socialhub.CapabilityState{
			Capability: capability, Supported: true, Approval: approval,
			Scopes: []string{oauthScope}, Reason: reason, DocURL: documentationURL,
		}
	}
	return socialhub.Capabilities{
		CapabilityCampaignManagement: state(CapabilityCampaignManagement, "advertiser-bound standard Search Campaign reads, paused-first CPC creation, updates, explicit enablement, and guarded removal"),
		CapabilityAdGroupManagement:  state(CapabilityAdGroupManagement, "typed Ad Group reads and paused-first CPC management"),
		CapabilityKeywordManagement:  state(CapabilityKeywordManagement, "biddable Keyword reads and paused-first batch management with per-item outcomes"),
		CapabilityReports:            state(CapabilityReports, "asynchronous Search Ads report jobs and bounded CSV/TSV/XML downloads"),
		socialhub.CapPublish:         {Capability: socialhub.CapPublish, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "paid advertising management is not organic publishing"},
		socialhub.CapFetch:           {Capability: socialhub.CapFetch, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "advertising reads use typed Search Ads workflows"},
		socialhub.CapMedia:           {Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "ad assets and creatives are outside this initial adapter surface"},
		socialhub.CapReact:           {Capability: socialhub.CapReact, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Search Ads has no organic reaction surface"},
		socialhub.CapMessage:         {Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Search Ads has no messaging surface"},
		socialhub.CapWebhook:         {Capability: socialhub.CapWebhook, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Search Ads API does not expose management webhooks"},
	}, nil
}

func (client *Client) Publisher() (socialhub.Publisher, bool)           { return nil, false }
func (client *Client) Fetcher() (socialhub.Fetcher, bool)               { return nil, false }
func (client *Client) MediaUploader() (socialhub.MediaUploader, bool)   { return nil, false }
func (client *Client) Reactor() (socialhub.Reactor, bool)               { return nil, false }
func (client *Client) Messenger() (socialhub.Messenger, bool)           { return nil, false }
func (client *Client) WebhookHandler() (socialhub.WebhookHandler, bool) { return nil, false }
func (client *Client) Close() error                                     { return nil }

func (client *Client) Campaigns() CampaignWorkflow { return client }
func (client *Client) AdGroups() AdGroupWorkflow   { return client }
func (client *Client) Keywords() KeywordWorkflow   { return client }
func (client *Client) Reports() ReportWorkflow     { return client }

func (client *Client) requireAccess(operation string) error {
	if len(client.scopes) == 0 || scopeGranted(client.scopes, oauthScope) {
		return nil
	}
	return &socialhub.Error{
		Code: socialhub.CodeApprovalRequired, Class: socialhub.ClassUserAction,
		Platform: platformName, Product: productName, Op: operation,
		RequiredScopes: []string{oauthScope}, ApprovalURL: "https://ads-developers.yahoo.co.jp/en/ads-api/startup-guide/before_you_start.html",
		PlatformMessage: "configured approval scopes do not authorize LINE Yahoo Ads API access",
	}
}

func scopeGranted(scopes []string, target string) bool {
	for _, scope := range scopes {
		if strings.TrimSpace(scope) == target {
			return true
		}
	}
	return false
}

var _ socialhub.Client = (*Client)(nil)
