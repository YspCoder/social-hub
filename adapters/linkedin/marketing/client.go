package marketing

import (
	"context"
	"strings"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	CapabilityAdsManagement socialhub.Capability = "linkedin_ads_management"
	CapabilityAdsReporting  socialhub.Capability = "linkedin_ads_reporting"
)

// Client exposes one LinkedIn Ad Account's paid-media workflows.
type Client struct {
	accountID   socialhub.AccountID
	adAccountID string
	api         *transport.Client
	scopes      []string
}

func (client *Client) Platform() socialhub.Platform { return platformName }
func (client *Client) Account() socialhub.AccountID { return client.accountID }

func (client *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	return socialhub.Capabilities{
		CapabilityAdsManagement: capabilityState(
			CapabilityAdsManagement, client.scopes, []string{writeAdsScope},
			"Ad Account, Campaign Group, Campaign, and Creative reads and mutations; Advertising API tier and Ad Account roles apply",
			documentationURL+"integrations/ads/ads-overview",
		),
		CapabilityAdsReporting: capabilityState(
			CapabilityAdsReporting, client.scopes, []string{reportingAdsScope},
			"bounded synchronous Ad Analytics for the configured Ad Account",
			documentationURL+"integrations/ads-reporting/ads-reporting",
		),
		socialhub.CapPublish: {Capability: socialhub.CapPublish, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "paid creatives are not organic social posts; use Campaigns() and Creatives()"},
		socialhub.CapFetch:   {Capability: socialhub.CapFetch, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "advertising reads use typed LinkedIn Marketing resources"},
		socialhub.CapMedia:   {Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "the initial adapter references existing share or ugcPost URNs"},
		socialhub.CapReact:   {Capability: socialhub.CapReact, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "LinkedIn Advertising API does not expose organic reactions through this product"},
		socialhub.CapMessage: {Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "LinkedIn Advertising API is not general messaging"},
		socialhub.CapWebhook: {Capability: socialhub.CapWebhook, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "lead sync and event subscriptions require separate reviewed products"},
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

func (client *Client) AdAccounts() AdAccountWorkflow         { return client }
func (client *Client) CampaignGroups() CampaignGroupWorkflow { return client }
func (client *Client) Campaigns() CampaignWorkflow           { return client }
func (client *Client) Creatives() CreativeWorkflow           { return client }
func (client *Client) Analytics() AnalyticsWorkflow          { return client }

func (client *Client) requireRead(operation string) error {
	if len(client.scopes) == 0 || scopeGranted(client.scopes, readAdsScope) || scopeGranted(client.scopes, writeAdsScope) {
		return nil
	}
	return approvalRequired(operation, []string{readAdsScope, writeAdsScope}, "configured scopes do not authorize LinkedIn Advertising API reads")
}

func (client *Client) requireWrite(operation string) error {
	if len(client.scopes) == 0 || scopeGranted(client.scopes, writeAdsScope) {
		return nil
	}
	return approvalRequired(operation, []string{writeAdsScope}, "configured scopes do not authorize LinkedIn Advertising API mutations")
}

func (client *Client) requireReporting(operation string) error {
	if len(client.scopes) == 0 || scopeGranted(client.scopes, reportingAdsScope) {
		return nil
	}
	return approvalRequired(operation, []string{reportingAdsScope}, "configured scopes do not authorize LinkedIn Ad Analytics")
}

func approvalRequired(operation string, scopes []string, message string) error {
	return &socialhub.Error{
		Code: socialhub.CodeApprovalRequired, Class: socialhub.ClassUserAction,
		Platform: platformName, Product: productName, Op: operation,
		RequiredScopes: scopes, ApprovalURL: "https://www.linkedin.com/developers/apps",
		PlatformMessage: message,
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
