package yandexdirect

import (
	"context"
	"net/http"
	"strings"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	CapabilityCampaignManagement socialhub.Capability = "yandex_direct_campaign_management"
	CapabilityAdGroupManagement  socialhub.Capability = "yandex_direct_adgroup_management"
	CapabilityKeywordManagement  socialhub.Capability = "yandex_direct_keyword_management"
	CapabilityReports            socialhub.Capability = "yandex_direct_reports"
)

// Client exposes one advertiser's paid-media workflows. For agency tokens,
// Client-Login fixes every request to the configured client advertiser.
type Client struct {
	accountID        socialhub.AccountID
	api              *transport.Client
	reportsAPI       *transport.Client
	httpClient       *http.Client
	clientLogin      string
	useOperatorUnits bool
	acceptLanguage   string
	scopes           []string
	clock            socialhub.Clock
	requestIDValues  []string
}

func (client *Client) Platform() socialhub.Platform { return platformName }
func (client *Client) Account() socialhub.AccountID { return client.accountID }

func (client *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	approval := socialhub.ApprovalUnknown
	if len(client.scopes) > 0 {
		approval = socialhub.ApprovalRequired
		if scopeGranted(client.scopes, directScope) {
			approval = socialhub.ApprovalGranted
		}
	}
	state := func(capability socialhub.Capability, reason string) socialhub.CapabilityState {
		return socialhub.CapabilityState{
			Capability: capability, Supported: true, Approval: approval,
			Scopes: []string{directScope}, Reason: reason, DocURL: documentationURL,
		}
	}
	return socialhub.Capabilities{
		CapabilityCampaignManagement: state(CapabilityCampaignManagement, "advertiser-bound classic Text Campaign reads, metadata updates, suspend/resume, and guarded delete"),
		CapabilityAdGroupManagement:  state(CapabilityAdGroupManagement, "classic Text Ad Group reads and management behind a non-serving parent Campaign creation gate"),
		CapabilityKeywordManagement:  state(CapabilityKeywordManagement, "typed Keyword reads, batch mutations, and explicit suspend/resume with per-item outcomes"),
		CapabilityReports:            state(CapabilityReports, "v501 online/offline TSV Reports with bounded streaming and retry metadata"),
		socialhub.CapPublish:         {Capability: socialhub.CapPublish, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "paid advertising management is not organic publishing"},
		socialhub.CapFetch:           {Capability: socialhub.CapFetch, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "advertising reads use typed Yandex Direct workflows"},
		socialhub.CapMedia:           {Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "ad assets and creatives are outside this initial adapter surface"},
		socialhub.CapReact:           {Capability: socialhub.CapReact, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Yandex Direct has no organic reaction surface"},
		socialhub.CapMessage:         {Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Yandex Direct has no messaging surface"},
		socialhub.CapWebhook:         {Capability: socialhub.CapWebhook, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Yandex Direct does not expose management webhooks"},
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
	if len(client.scopes) == 0 || scopeGranted(client.scopes, directScope) {
		return nil
	}
	return &socialhub.Error{
		Code: socialhub.CodeApprovalRequired, Class: socialhub.ClassUserAction,
		Platform: platformName, Product: productName, Op: operation,
		RequiredScopes: []string{directScope}, ApprovalURL: "https://oauth.yandex.com/",
		PlatformMessage: "configured approval scopes do not authorize Yandex Direct API access",
	}
}

func (client *Client) applyHeaders(request *http.Request) {
	request.Header.Set("Accept-Language", client.acceptLanguage)
	if client.clientLogin != "" {
		request.Header.Set("Client-Login", client.clientLogin)
	}
	if client.useOperatorUnits {
		request.Header.Set("Use-Operator-Units", "true")
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
