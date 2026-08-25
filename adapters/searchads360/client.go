package searchads360

import (
	"context"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	CapabilityReports       socialhub.Capability = "search_ads_360_reports"
	CapabilityCustomColumns socialhub.Capability = "search_ads_360_custom_columns"
	CapabilityFieldMetadata socialhub.Capability = "search_ads_360_field_metadata"
)

// Client exposes read-only reporting workflows for one Search Ads 360
// customer. Organic social capabilities are intentionally unavailable.
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
		CapabilityReports: {
			Capability: CapabilityReports, Supported: true, Approval: socialhub.ApprovalUnknown,
			Reason: "bounded, paginated Search Ads 360 Query Language reporting with exact raw row values",
			DocURL: documentationURL,
		},
		CapabilityCustomColumns: {
			Capability: CapabilityCustomColumns, Supported: true, Approval: socialhub.ApprovalUnknown,
			Reason: "customer-bound Custom Column discovery and metadata reads",
			DocURL: documentationURL,
		},
		CapabilityFieldMetadata: {
			Capability: CapabilityFieldMetadata, Supported: true, Approval: socialhub.ApprovalUnknown,
			Reason: "Search Ads 360 field lookup and paginated field compatibility search",
			DocURL: documentationURL,
		},
		socialhub.CapPublish: {Capability: socialhub.CapPublish, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "the Reporting API is read-only and paid-media data is not a social post"},
		socialhub.CapFetch:   {Capability: socialhub.CapFetch, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "paid-media reads use typed reporting workflows rather than common social entities"},
		socialhub.CapMedia:   {Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "the Reporting API does not upload creative media"},
		socialhub.CapReact:   {Capability: socialhub.CapReact, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Search Ads 360 does not expose organic reactions"},
		socialhub.CapMessage: {Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Search Ads 360 does not expose social messaging"},
		socialhub.CapWebhook: {Capability: socialhub.CapWebhook, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Reporting API v0 does not expose webhooks"},
	}, nil
}

func (client *Client) Publisher() (socialhub.Publisher, bool)           { return nil, false }
func (client *Client) Fetcher() (socialhub.Fetcher, bool)               { return nil, false }
func (client *Client) MediaUploader() (socialhub.MediaUploader, bool)   { return nil, false }
func (client *Client) Reactor() (socialhub.Reactor, bool)               { return nil, false }
func (client *Client) Messenger() (socialhub.Messenger, bool)           { return nil, false }
func (client *Client) WebhookHandler() (socialhub.WebhookHandler, bool) { return nil, false }
func (client *Client) Close() error                                     { return nil }

func (client *Client) Customers() CustomerWorkflow         { return client }
func (client *Client) Reports() ReportWorkflow             { return client }
func (client *Client) CustomColumns() CustomColumnWorkflow { return client }
func (client *Client) Fields() FieldWorkflow               { return client }

func (client *Client) requireAccess(operation string) error {
	if len(client.scopes) == 0 {
		return nil
	}
	for _, scope := range client.scopes {
		if scope == reportingScope {
			return nil
		}
	}
	return &socialhub.Error{
		Code: socialhub.CodeApprovalRequired, Class: socialhub.ClassUserAction,
		Platform: platformName, Product: productName, Op: operation,
		RequiredScopes: []string{reportingScope}, ApprovalURL: defaultAuthURL,
		PlatformMessage: "configured approval scopes do not authorize Search Ads 360 Reporting API access",
	}
}

var _ socialhub.Client = (*Client)(nil)
