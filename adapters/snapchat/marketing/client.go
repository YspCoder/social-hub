package marketing

import (
	"context"
	"net/url"
	"strings"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	CapabilityAdsManagement socialhub.Capability = "snapchat_ads_management"
	CapabilityAdsReporting  socialhub.Capability = "snapchat_ads_reporting"
)

// Client exposes one Snapchat Ad Account's paid-media workflows.
type Client struct {
	accountID   socialhub.AccountID
	adAccountID string
	api         *transport.Client
	baseURL     *url.URL
	scopes      []string
}

func (client *Client) Platform() socialhub.Platform { return platformName }
func (client *Client) Account() socialhub.AccountID { return client.accountID }

func (client *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	return socialhub.Capabilities{
		CapabilityAdsManagement: capabilityState(
			CapabilityAdsManagement, client.scopes,
			"Ad Account, Campaign, Ad Squad, and Ad reads and mutations; Organization and Ad Account roles apply",
			documentationURL+"introduction",
		),
		CapabilityAdsReporting: capabilityState(
			CapabilityAdsReporting, client.scopes,
			"bounded synchronous statistics for the configured Ad Account",
			documentationURL+"measurement",
		),
		socialhub.CapPublish: {Capability: socialhub.CapPublish, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "paid ads are not social posts; use Campaigns(), AdSquads(), and Ads()"},
		socialhub.CapFetch:   {Capability: socialhub.CapFetch, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "advertising reads use typed Snapchat Marketing resources"},
		socialhub.CapMedia:   {Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "the initial adapter references existing Snapchat Creative IDs"},
		socialhub.CapReact:   {Capability: socialhub.CapReact, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Snapchat Marketing API is not an organic engagement product"},
		socialhub.CapMessage: {Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Sponsored Snaps are ads, not general messaging"},
		socialhub.CapWebhook: {Capability: socialhub.CapWebhook, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "webhooks are outside the initial Snapchat Marketing adapter contract"},
	}, nil
}

func capabilityState(capability socialhub.Capability, granted []string, reason, docURL string) socialhub.CapabilityState {
	approval := socialhub.ApprovalUnknown
	if len(granted) > 0 {
		approval = socialhub.ApprovalRequired
		if scopeGranted(granted, marketingScope) {
			approval = socialhub.ApprovalGranted
		}
	}
	return socialhub.CapabilityState{
		Capability: capability, Supported: true, Approval: approval,
		Scopes: []string{marketingScope}, Reason: reason, DocURL: docURL,
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

func (client *Client) AdAccounts() AdAccountWorkflow { return client }
func (client *Client) Campaigns() CampaignWorkflow   { return client }
func (client *Client) AdSquads() AdSquadWorkflow     { return client }
func (client *Client) Ads() AdWorkflow               { return client }
func (client *Client) Stats() StatsWorkflow          { return client }

func (client *Client) requireScope(operation string) error {
	if len(client.scopes) == 0 || scopeGranted(client.scopes, marketingScope) {
		return nil
	}
	return &socialhub.Error{
		Code: socialhub.CodeApprovalRequired, Class: socialhub.ClassUserAction,
		Platform: platformName, Product: productName, Op: operation,
		RequiredScopes: []string{marketingScope}, ApprovalURL: "https://business.snapchat.com/",
		PlatformMessage: "configured scopes do not authorize Snapchat Marketing API operations",
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
