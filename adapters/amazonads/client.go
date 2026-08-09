package amazonads

import (
	"context"
	"strings"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	CapabilitySponsoredProducts socialhub.Capability = "amazon_sponsored_products_management"
	CapabilityReportingV3       socialhub.Capability = "amazon_ads_reporting_v3"
)

// Client exposes one profile's paid-media workflows. Organic social
// capabilities are intentionally unavailable.
type Client struct {
	accountID socialhub.AccountID
	profileID string
	region    Region
	api       *transport.Client
	scopes    []string
}

func (client *Client) Platform() socialhub.Platform { return platformName }
func (client *Client) Account() socialhub.AccountID { return client.accountID }

func (client *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	approval := socialhub.ApprovalUnknown
	if len(client.scopes) > 0 {
		approval = socialhub.ApprovalRequired
		if scopeGranted(client.scopes, managementScope) {
			approval = socialhub.ApprovalGranted
		}
	}
	management := socialhub.CapabilityState{
		Capability: CapabilitySponsoredProducts, Supported: true, Approval: approval,
		Scopes: []string{managementScope},
		Reason: "profile-scoped Sponsored Products v3 Campaign, Ad Group, Product Ad, and Keyword workflows; Amazon Ads onboarding and profile authorization apply",
		DocURL: documentationURL,
	}
	reports := management
	reports.Capability = CapabilityReportingV3
	reports.Reason = "Reporting v3 asynchronous create and status workflows; Unified Reporting migration deadlines apply"
	return socialhub.Capabilities{
		CapabilitySponsoredProducts: management,
		CapabilityReportingV3:       reports,
		socialhub.CapPublish:        {Capability: socialhub.CapPublish, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "paid ads are not social posts; use Campaigns(), AdGroups(), ProductAds(), and Keywords()"},
		socialhub.CapFetch:          {Capability: socialhub.CapFetch, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "advertising reads use typed Amazon Ads resources"},
		socialhub.CapMedia:          {Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Sponsored Products reference catalog ASINs or SKUs rather than social media uploads"},
		socialhub.CapReact:          {Capability: socialhub.CapReact, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Amazon Ads does not expose organic reactions through this adapter"},
		socialhub.CapMessage:        {Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Amazon Ads is not a messaging product"},
		socialhub.CapWebhook:        {Capability: socialhub.CapWebhook, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "webhooks are outside the initial adapter contract"},
	}, nil
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

func (client *Client) Profiles() ProfileWorkflow     { return client }
func (client *Client) Campaigns() CampaignWorkflow   { return client }
func (client *Client) AdGroups() AdGroupWorkflow     { return client }
func (client *Client) ProductAds() ProductAdWorkflow { return client }
func (client *Client) Keywords() KeywordWorkflow     { return client }
func (client *Client) Reports() ReportWorkflow       { return client }

func (client *Client) requireAccess(operation string) error {
	if len(client.scopes) == 0 || scopeGranted(client.scopes, managementScope) {
		return nil
	}
	return &socialhub.Error{
		Code: socialhub.CodeApprovalRequired, Class: socialhub.ClassUserAction,
		Platform: platformName, Product: productName, Op: operation,
		RequiredScopes: []string{managementScope}, ApprovalURL: defaultAuthURL,
		PlatformMessage: "configured approval scopes do not authorize Amazon Ads campaign management",
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
