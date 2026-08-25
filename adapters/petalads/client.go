package petalads

import (
	"context"
	"strings"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	CapabilityAccountDiscovery socialhub.Capability = "petal_ads_account_discovery"
	CapabilityCampaignRead     socialhub.Capability = "petal_ads_campaign_read"
	CapabilityReporting        socialhub.Capability = "petal_ads_reporting"
)

// Client exposes one overseas Petal Ads authorization's read workflows.
type Client struct {
	accountID       socialhub.AccountID
	advertiserID    string
	region          Region
	api             *transport.Client
	scopes          []string
	clock           socialhub.Clock
	requestIDValues []string
}

func (client *Client) Platform() socialhub.Platform { return platformName }
func (client *Client) Account() socialhub.AccountID { return client.accountID }

func (client *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	return socialhub.Capabilities{
		CapabilityAccountDiscovery: client.capability(
			CapabilityAccountDiscovery, ScopeAccount,
			"token-linked overseas advertiser-account discovery; Petal Ads app access and advertiser authorization are required",
		),
		CapabilityCampaignRead: client.capability(
			CapabilityCampaignRead, ScopePromotion,
			"read-only Campaign query using the overseas Marketing API v1 contract",
		),
		CapabilityReporting: client.capability(
			CapabilityReporting, ScopeReport,
			"synchronous advertiser, Campaign, Ad Group, Creative, and Country reports using the v2 reporting contract",
		),
		socialhub.CapPublish: {Capability: socialhub.CapPublish, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "paid-media Campaigns are not social posts"},
		socialhub.CapFetch:   {Capability: socialhub.CapFetch, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "paid-media resources use typed Petal Ads workflows"},
		socialhub.CapMedia:   {Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "creative asset management is outside the initial adapter contract"},
		socialhub.CapReact:   {Capability: socialhub.CapReact, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "advertising APIs do not expose organic reactions"},
		socialhub.CapMessage: {Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Petal Ads is not a messaging product"},
		socialhub.CapWebhook: {Capability: socialhub.CapWebhook, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "webhooks are outside the initial adapter contract"},
	}, nil
}

func (client *Client) capability(capability socialhub.Capability, scope, reason string) socialhub.CapabilityState {
	approval := socialhub.ApprovalUnknown
	if len(client.scopes) > 0 {
		approval = socialhub.ApprovalRequired
		if scopeGranted(client.scopes, scope) {
			approval = socialhub.ApprovalGranted
		}
	}
	return socialhub.CapabilityState{
		Capability: capability, Supported: true, Approval: approval,
		Scopes: []string{scope}, Reason: reason, DocURL: documentationURL,
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

func (client *Client) requireScope(operation, scope string) error {
	if len(client.scopes) == 0 || scopeGranted(client.scopes, scope) {
		return nil
	}
	return &socialhub.Error{
		Code: socialhub.CodeApprovalRequired, Class: socialhub.ClassUserAction,
		Platform: platformName, Product: productName, Op: operation,
		RequiredScopes: []string{scope}, ApprovalURL: defaultAuthURL,
		PlatformMessage: "configured approval scopes do not authorize this Petal Ads workflow",
	}
}

func (client *Client) Publisher() (socialhub.Publisher, bool)           { return nil, false }
func (client *Client) Fetcher() (socialhub.Fetcher, bool)               { return nil, false }
func (client *Client) MediaUploader() (socialhub.MediaUploader, bool)   { return nil, false }
func (client *Client) Reactor() (socialhub.Reactor, bool)               { return nil, false }
func (client *Client) Messenger() (socialhub.Messenger, bool)           { return nil, false }
func (client *Client) WebhookHandler() (socialhub.WebhookHandler, bool) { return nil, false }
func (client *Client) Close() error                                     { return nil }

func (client *Client) Accounts() AccountWorkflow   { return client }
func (client *Client) Campaigns() CampaignWorkflow { return client }
func (client *Client) Reports() ReportWorkflow     { return client }

var _ socialhub.Client = (*Client)(nil)
