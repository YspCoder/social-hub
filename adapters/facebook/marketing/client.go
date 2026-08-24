package marketing

import (
	"context"
	"strings"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	CapabilityMarketingManagement socialhub.Capability = "facebook_marketing_management"
	CapabilityMarketingInsights   socialhub.Capability = "facebook_marketing_insights"
)

// Client exposes typed advertising workflows for one configured Meta ad
// account. Common social-content capabilities are intentionally unavailable.
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
		CapabilityMarketingManagement: marketingCapability(
			CapabilityMarketingManagement, client.scopes, []string{managementScope},
			"Ad Account, Campaign, Ad Set, Ad Creative, and Ad reads and mutations; Meta App Review and ad-account roles apply",
			documentationURL,
		),
		CapabilityMarketingInsights: marketingCapability(
			CapabilityMarketingInsights, client.scopes, []string{readScope, managementScope},
			"synchronous account or object Insights reads; either ads_read or ads_management is accepted",
			documentationURL+"insights/",
		),
		socialhub.CapPublish: {Capability: socialhub.CapPublish, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "advertising resources are not social posts; use Management()"},
		socialhub.CapFetch:   {Capability: socialhub.CapFetch, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "advertising reads use typed Marketing API resources"},
		socialhub.CapMedia:   {Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "initial adapter references existing image hashes, posts, and creative IDs"},
		socialhub.CapReact:   {Capability: socialhub.CapReact, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Marketing API does not expose organic reactions through this adapter"},
		socialhub.CapMessage: {Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Marketing API is not a messaging product"},
		socialhub.CapWebhook: {Capability: socialhub.CapWebhook, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "lead and Conversions API callbacks are separate products"},
	}, nil
}

func marketingCapability(capability socialhub.Capability, granted, acceptable []string, reason, docURL string) socialhub.CapabilityState {
	approval := socialhub.ApprovalUnknown
	if len(granted) > 0 {
		approval = socialhub.ApprovalRequired
		for _, scope := range acceptable {
			if scopeGranted(granted, scope) {
				approval = socialhub.ApprovalGranted
				break
			}
		}
	}
	return socialhub.CapabilityState{
		Capability: capability, Supported: true, Approval: approval,
		Scopes: append([]string(nil), acceptable...), Reason: reason, DocURL: docURL,
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

func (client *Client) Management() ManagementWorkflow { return client }
func (client *Client) Insights() InsightsWorkflow     { return client }

func (client *Client) adAccountResource() string { return "act_" + client.adAccountID }

func (client *Client) requireManagement(operation string) error {
	return client.requireAnyScope(operation, []string{managementScope})
}

func (client *Client) requireRead(operation string) error {
	return client.requireAnyScope(operation, []string{readScope, managementScope})
}

func (client *Client) requireAnyScope(operation string, acceptable []string) error {
	if len(client.scopes) == 0 {
		return nil
	}
	for _, scope := range acceptable {
		if scopeGranted(client.scopes, scope) {
			return nil
		}
	}
	return &socialhub.Error{
		Code: socialhub.CodeApprovalRequired, Class: socialhub.ClassUserAction,
		Platform: platformName, Product: productName, Op: operation,
		RequiredScopes: append([]string(nil), acceptable...), ApprovalURL: defaultAuthURL,
		PlatformMessage: "configured approval scopes do not authorize this Marketing API operation",
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
