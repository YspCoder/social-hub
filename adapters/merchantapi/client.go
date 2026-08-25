package merchantapi

import (
	"context"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	CapabilityMerchantAccount socialhub.Capability = "google_merchant_account"
	CapabilityProductFeeds    socialhub.Capability = "google_merchant_product_feeds"
	CapabilityMerchantReports socialhub.Capability = "google_merchant_reports"
	CapabilityMerchantQuotas  socialhub.Capability = "google_merchant_quotas"
)

// Client exposes Merchant Center shopping-data workflows for one account.
type Client struct {
	accountID         socialhub.AccountID
	merchantAccountID string
	api               *transport.Client
	scopes            []string
	tokens            *staticTokenSource
	requestIDs        *requestIDFilter
}

func (client *Client) Platform() socialhub.Platform { return platformName }
func (client *Client) Account() socialhub.AccountID { return client.accountID }

func (client *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	return socialhub.Capabilities{
		CapabilityMerchantAccount: {
			Capability: CapabilityMerchantAccount, Supported: true, Approval: socialhub.ApprovalUnknown,
			Reason: "Merchant Center account identity and account-level issue discovery",
			DocURL: documentationURL,
		},
		CapabilityProductFeeds: {
			Capability: CapabilityProductFeeds, Supported: true, Approval: socialhub.ApprovalUnknown,
			Reason: "Data Source reads, processed Product reads, and explicit ProductInput insert, patch, and delete workflows",
			DocURL: documentationURL,
		},
		CapabilityMerchantReports: {
			Capability: CapabilityMerchantReports, Supported: true, Approval: socialhub.ApprovalUnknown,
			Reason: "exact-value Merchant Query Language reports for product visibility, performance, pricing, and competitive insights",
			DocURL: documentationURL,
		},
		CapabilityMerchantQuotas: {
			Capability: CapabilityMerchantQuotas, Supported: true, Approval: socialhub.ApprovalUnknown,
			Reason: "dynamic method-group quota usage and Shopping product account limits",
			DocURL: documentationURL,
		},
		socialhub.CapPublish: {Capability: socialhub.CapPublish, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Merchant product data feeds Shopping surfaces but is not a social post"},
		socialhub.CapFetch:   {Capability: socialhub.CapFetch, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "commerce reads use Merchant-specific typed workflows"},
		socialhub.CapMedia:   {Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Merchant API accepts image URLs in product attributes and does not upload media"},
		socialhub.CapReact:   {Capability: socialhub.CapReact, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Merchant API has no organic engagement surface"},
		socialhub.CapMessage: {Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Merchant API has no messaging surface"},
		socialhub.CapWebhook: {Capability: socialhub.CapWebhook, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "notifications are a separate Merchant sub-API and are outside this adapter version"},
	}, nil
}

func (client *Client) Publisher() (socialhub.Publisher, bool)           { return nil, false }
func (client *Client) Fetcher() (socialhub.Fetcher, bool)               { return nil, false }
func (client *Client) MediaUploader() (socialhub.MediaUploader, bool)   { return nil, false }
func (client *Client) Reactor() (socialhub.Reactor, bool)               { return nil, false }
func (client *Client) Messenger() (socialhub.Messenger, bool)           { return nil, false }
func (client *Client) WebhookHandler() (socialhub.WebhookHandler, bool) { return nil, false }
func (client *Client) Close() error {
	if client.tokens != nil {
		client.tokens.Close()
	}
	if client.requestIDs != nil {
		client.requestIDs.clear()
	}
	return nil
}

func (client *Client) MerchantAccount() AccountWorkflow { return client }
func (client *Client) DataSources() DataSourceWorkflow  { return client }
func (client *Client) Products() ProductWorkflow        { return client }
func (client *Client) Reports() ReportWorkflow          { return client }
func (client *Client) Quotas() QuotaWorkflow            { return client }

func (client *Client) requireAccess(operation string) error {
	if len(client.scopes) == 0 {
		return nil
	}
	for _, scope := range client.scopes {
		if scope == contentScope {
			return nil
		}
	}
	return &socialhub.Error{
		Code: socialhub.CodeApprovalRequired, Class: socialhub.ClassUserAction,
		Platform: platformName, Product: productName, Op: operation,
		RequiredScopes: []string{contentScope}, ApprovalURL: defaultAuthURL,
		PlatformMessage: "configured approval scopes do not authorize Google Merchant API access",
	}
}

func (client *Client) accountName() string { return "accounts/" + client.merchantAccountID }

var _ socialhub.Client = (*Client)(nil)
