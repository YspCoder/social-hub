package ads

import (
	"context"
	"net/url"
	"strings"
	"sync"
	"time"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	CapabilityAdsManagement socialhub.Capability = "reddit_ads_management"
	CapabilityAdsReporting  socialhub.Capability = "reddit_ads_reporting"
)

// RateLimit is the most restrictive Reddit Ads quota snapshot observed by the
// client. Quotas are scoped by authorized user, app instance, and endpoint group.
type RateLimit struct {
	Policy     string
	Quota      int
	Window     time.Duration
	Remaining  int
	Reset      time.Duration
	ObservedAt time.Time
}

// Client exposes one Reddit Ad Account's paid-media workflows.
type Client struct {
	accountID   socialhub.AccountID
	adAccountID string
	api         *transport.Client
	baseURL     *url.URL
	scopes      []string
	clock       socialhub.Clock
	rateMu      sync.RWMutex
	rateLimit   RateLimit
}

func (client *Client) Platform() socialhub.Platform { return platformName }
func (client *Client) Account() socialhub.AccountID { return client.accountID }

func (client *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	return socialhub.Capabilities{
		CapabilityAdsManagement: capabilityState(
			CapabilityAdsManagement, client.scopes, []string{readScope, editScope},
			"Ad Account, Funding Instrument, Campaign, Ad Group, and existing-Post Ad workflows",
			documentationURL+"guides/programs/campaign",
		),
		CapabilityAdsReporting: capabilityState(
			CapabilityAdsReporting, client.scopes, []string{readScope},
			"synchronous paginated reports for the configured Ad Account",
			documentationURL+"api/get-a-report",
		),
		socialhub.CapPublish: {Capability: socialhub.CapPublish, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "paid ads are not social posts; use Campaigns(), AdGroups(), and Ads()"},
		socialhub.CapFetch:   {Capability: socialhub.CapFetch, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "advertising reads use typed Reddit Ads resources"},
		socialhub.CapMedia:   {Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "the initial adapter references existing Reddit Post IDs"},
		socialhub.CapReact:   {Capability: socialhub.CapReact, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Reddit Ads API is not an organic engagement product"},
		socialhub.CapMessage: {Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Reddit Ads API does not expose general messaging"},
		socialhub.CapWebhook: {Capability: socialhub.CapWebhook, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "webhooks are outside the initial Reddit Ads adapter contract"},
	}, nil
}

func capabilityState(capability socialhub.Capability, granted, required []string, reason, docURL string) socialhub.CapabilityState {
	approval := socialhub.ApprovalUnknown
	if len(granted) > 0 {
		approval = socialhub.ApprovalGranted
		for _, scope := range required {
			if !scopeGranted(granted, scope) {
				approval = socialhub.ApprovalRequired
				break
			}
		}
	}
	return socialhub.CapabilityState{
		Capability: capability, Supported: true, Approval: approval,
		Scopes: append([]string(nil), required...), Reason: reason, DocURL: docURL,
	}
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

func (client *Client) AdAccounts() AccountWorkflow         { return client }
func (client *Client) FundingInstruments() FundingWorkflow { return client }
func (client *Client) Campaigns() CampaignWorkflow         { return client }
func (client *Client) AdGroups() AdGroupWorkflow           { return client }
func (client *Client) Ads() AdWorkflow                     { return client }
func (client *Client) Reports() ReportWorkflow             { return client }

// RateLimit returns the latest endpoint-group quota state observed by this client.
func (client *Client) RateLimit() RateLimit {
	client.rateMu.RLock()
	defer client.rateMu.RUnlock()
	return client.rateLimit
}

func (client *Client) recordRateLimit(headerValues map[string][]string) {
	limit, ok := parseRateLimit(headerValues)
	if !ok {
		return
	}
	limit.ObservedAt = client.clock.Now()
	client.rateMu.Lock()
	client.rateLimit = limit
	client.rateMu.Unlock()
}

func (client *Client) requireScopes(operation string, required ...string) error {
	if len(client.scopes) == 0 {
		return nil
	}
	missing := make([]string, 0, len(required))
	for _, scope := range required {
		if !scopeGranted(client.scopes, scope) {
			missing = append(missing, scope)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	return &socialhub.Error{
		Code: socialhub.CodeApprovalRequired, Class: socialhub.ClassUserAction,
		Platform: platformName, Product: productName, Op: operation,
		RequiredScopes: missing, ApprovalURL: "https://www.reddit.com/prefs/apps",
		PlatformMessage: "configured scopes do not authorize this Reddit Ads operation",
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
