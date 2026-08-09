package microsoftads

import (
	"context"
	"net/http"
	"net/url"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	CapabilityAdsManagement socialhub.Capability = "microsoft_ads_management"
	CapabilityReports       socialhub.Capability = "microsoft_ads_reporting"
)

// Client exposes ad-account-scoped paid-media workflows. Organic social
// capabilities are intentionally unavailable.
type Client struct {
	accountID         socialhub.AccountID
	customerID        string
	customerAccountID string
	campaign          *transport.Client
	customer          *transport.Client
	reporting         *transport.Client
	httpClient        *http.Client
	reportingBaseURL  *url.URL
	maxReportBytes    int64
	scopes            []string
}

func (client *Client) Platform() socialhub.Platform { return platformName }
func (client *Client) Account() socialhub.AccountID { return client.accountID }

func (client *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	return socialhub.Capabilities{
		CapabilityAdsManagement: {
			Capability: CapabilityAdsManagement, Supported: true, Approval: socialhub.ApprovalUnknown,
			Reason: "Account validation and paused-first Search Campaign, Ad Group, responsive search ad, and Keyword workflows; developer token and account authorization apply",
			DocURL: documentationURL,
		},
		CapabilityReports: {
			Capability: CapabilityReports, Supported: true, Approval: socialhub.ApprovalUnknown,
			Reason: "asynchronous Campaign Performance report submit, poll, and bounded secure download",
			DocURL: documentationURL,
		},
		socialhub.CapPublish: {Capability: socialhub.CapPublish, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "paid ads are not social posts; use Campaigns(), AdGroups(), Ads(), and Keywords()"},
		socialhub.CapFetch:   {Capability: socialhub.CapFetch, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "advertising reads use typed Microsoft Advertising resources and Reports()"},
		socialhub.CapMedia:   {Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "media asset upload is outside the initial Search Ads contract"},
		socialhub.CapReact:   {Capability: socialhub.CapReact, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Microsoft Advertising is not an organic engagement product"},
		socialhub.CapMessage: {Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Microsoft Advertising does not provide social messaging"},
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

func (client *Client) Accounts() AccountWorkflow   { return client }
func (client *Client) Campaigns() CampaignWorkflow { return client }
func (client *Client) AdGroups() AdGroupWorkflow   { return client }
func (client *Client) Ads() AdWorkflow             { return client }
func (client *Client) Keywords() KeywordWorkflow   { return client }
func (client *Client) Reports() ReportWorkflow     { return client }

func (client *Client) requireAccess(operation string) error {
	if len(client.scopes) == 0 {
		return nil
	}
	for _, scope := range client.scopes {
		if scope == adsManageScope {
			return nil
		}
	}
	return &socialhub.Error{
		Code: socialhub.CodeApprovalRequired, Class: socialhub.ClassUserAction,
		Platform: platformName, Product: productName, Op: operation,
		RequiredScopes: []string{adsManageScope}, ApprovalURL: defaultAuthURL,
		PlatformMessage: "configured approval scopes do not authorize Microsoft Advertising API access",
	}
}

var _ socialhub.Client = (*Client)(nil)
