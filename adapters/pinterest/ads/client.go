package ads

import (
	"context"
	"strings"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	CapabilityAdsManagement socialhub.Capability = "pinterest_ads_management"
	CapabilityAdsAnalytics  socialhub.Capability = "pinterest_ads_analytics"
)

// Client exposes one Pinterest ad account's paid-media workflows.
type Client struct {
	accountID   socialhub.AccountID
	adAccountID string
	api         *transport.Client
	scopes      []string
	clock       socialhub.Clock
}

func (client *Client) Platform() socialhub.Platform { return platformName }
func (client *Client) Account() socialhub.AccountID { return client.accountID }

func (client *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	return socialhub.Capabilities{
		CapabilityAdsManagement: capabilityState(
			CapabilityAdsManagement, client.scopes, adsWriteScope,
			"Ad Account, Campaign, Ad Group, and Ad reads and mutations; Business Access, advertiser roles, billing, and ads approval apply",
			documentationURL,
		),
		CapabilityAdsAnalytics: capabilityState(
			CapabilityAdsAnalytics, client.scopes, adsReadScope,
			"synchronous account Analytics with bounded date ranges and typed raw metrics",
			documentationURL+"ad_account-analytics/",
		),
		socialhub.CapPublish: {Capability: socialhub.CapPublish, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "paid ads are not social posts; use Campaigns(), AdGroups(), and Ads()"},
		socialhub.CapFetch:   {Capability: socialhub.CapFetch, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "advertising reads use typed Pinterest Ads resources"},
		socialhub.CapMedia:   {Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "ads reference existing Pinterest Pin IDs; use the organic pinterest/v5 adapter to create Pins"},
		socialhub.CapReact:   {Capability: socialhub.CapReact, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Pinterest Ads API does not expose organic reactions"},
		socialhub.CapMessage: {Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Pinterest Ads API is not a messaging product"},
		socialhub.CapWebhook: {Capability: socialhub.CapWebhook, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "webhooks are outside the initial Pinterest Ads adapter contract"},
	}, nil
}

func capabilityState(capability socialhub.Capability, granted []string, required, reason, docURL string) socialhub.CapabilityState {
	approval := socialhub.ApprovalUnknown
	if len(granted) > 0 {
		approval = socialhub.ApprovalRequired
		if scopeGranted(granted, required) {
			approval = socialhub.ApprovalGranted
		}
	}
	return socialhub.CapabilityState{
		Capability: capability, Supported: true, Approval: approval,
		Scopes: []string{required}, Reason: reason, DocURL: docURL,
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
func (client *Client) AdGroups() AdGroupWorkflow     { return client }
func (client *Client) Ads() AdWorkflow               { return client }
func (client *Client) Analytics() AnalyticsWorkflow  { return client }

func (client *Client) requireRead(operation string) error {
	return client.requireScope(operation, adsReadScope)
}

func (client *Client) requireWrite(operation string) error {
	return client.requireScope(operation, adsWriteScope)
}

func (client *Client) requireScope(operation, required string) error {
	if len(client.scopes) == 0 || scopeGranted(client.scopes, required) {
		return nil
	}
	return &socialhub.Error{
		Code: socialhub.CodeApprovalRequired, Class: socialhub.ClassUserAction,
		Platform: platformName, Product: productName, Op: operation,
		RequiredScopes: []string{required}, ApprovalURL: "https://developers.pinterest.com/apps/",
		PlatformMessage: "configured approval scopes do not authorize this Pinterest Ads operation",
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
