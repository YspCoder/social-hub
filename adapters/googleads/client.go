package googleads

import (
	"context"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	CapabilityAdsManagement socialhub.Capability = "google_ads_management"
	CapabilityGAQLReports   socialhub.Capability = "google_ads_gaql_reports"
)

// Client exposes customer-scoped paid-media workflows. Organic social
// capabilities are intentionally unavailable.
type Client struct {
	accountID  socialhub.AccountID
	customerID string
	api        *transport.Client
	scopes     []string
}

func (client *Client) Platform() socialhub.Platform { return platformName }
func (client *Client) Account() socialhub.AccountID { return client.accountID }

func (client *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	return socialhub.Capabilities{
		CapabilityAdsManagement: {
			Capability: CapabilityAdsManagement, Supported: true, Approval: socialhub.ApprovalUnknown,
			Reason: "Customer, Campaign Budget, Search Campaign, Ad Group, and responsive search ad workflows; developer-token access and account authorization apply",
			DocURL: documentationURL,
		},
		CapabilityGAQLReports: {
			Capability: CapabilityGAQLReports, Supported: true, Approval: socialhub.ApprovalUnknown,
			Reason: "bounded paginated Google Ads Query Language Search; customer access and query field compatibility apply",
			DocURL: documentationURL,
		},
		socialhub.CapPublish: {Capability: socialhub.CapPublish, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "paid ads are not social posts; use Campaigns(), AdGroups(), and Ads()"},
		socialhub.CapFetch:   {Capability: socialhub.CapFetch, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "advertising reads use typed Google Ads resources and GAQL"},
		socialhub.CapMedia:   {Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "asset upload is outside the initial Search Ads contract"},
		socialhub.CapReact:   {Capability: socialhub.CapReact, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Google Ads API is not an organic engagement product"},
		socialhub.CapMessage: {Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Google Ads API does not provide social messaging"},
		socialhub.CapWebhook: {Capability: socialhub.CapWebhook, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "webhooks are outside the initial adapter contract"},
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

func (client *Client) Customers() CustomerWorkflow     { return client }
func (client *Client) CampaignBudgets() BudgetWorkflow { return client }
func (client *Client) Campaigns() CampaignWorkflow     { return client }
func (client *Client) AdGroups() AdGroupWorkflow       { return client }
func (client *Client) Ads() AdWorkflow                 { return client }
func (client *Client) Reports() ReportWorkflow         { return client }

func (client *Client) requireAccess(operation string) error {
	if len(client.scopes) == 0 {
		return nil
	}
	for _, scope := range client.scopes {
		if scope == adwordsScope {
			return nil
		}
	}
	return &socialhub.Error{
		Code: socialhub.CodeApprovalRequired, Class: socialhub.ClassUserAction,
		Platform: platformName, Product: productName, Op: operation,
		RequiredScopes: []string{adwordsScope}, ApprovalURL: defaultAuthURL,
		PlatformMessage: "configured approval scopes do not authorize Google Ads API access",
	}
}

var _ socialhub.Client = (*Client)(nil)
