package youtubereporting

import (
	"context"
	"net/http"
	"net/url"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const CapabilityYouTubeBulkReporting socialhub.Capability = "youtube_bulk_reporting"

type Client struct {
	accountID      socialhub.AccountID
	contentOwnerID string
	api            *transport.Client
	httpClient     *http.Client
	baseURL        *url.URL
	scopes         []string
}

func (client *Client) Platform() socialhub.Platform { return platformName }
func (client *Client) Account() socialhub.AccountID { return client.accountID }

func (client *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	return socialhub.Capabilities{
		CapabilityYouTubeBulkReporting: {
			Capability: CapabilityYouTubeBulkReporting, Supported: true, Approval: socialhub.ApprovalUnknown,
			Reason: "account-bound asynchronous YouTube bulk-report jobs, report metadata, and bounded CSV downloads",
			DocURL: documentationURL,
		},
		socialhub.CapPublish: {Capability: socialhub.CapPublish, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "content publishing belongs to YouTube Data API v3"},
		socialhub.CapFetch:   {Capability: socialhub.CapFetch, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "bulk analytics uses the typed Reporting workflow"},
		socialhub.CapMedia:   {Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "report download is exposed by the typed Reporting workflow"},
		socialhub.CapReact:   {Capability: socialhub.CapReact, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "engagement mutations belong to YouTube Data API v3"},
		socialhub.CapMessage: {Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "YouTube Reporting API has no messaging"},
		socialhub.CapWebhook: {Capability: socialhub.CapWebhook, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "YouTube Reporting API v1 exposes no webhooks"},
	}, nil
}

func (client *Client) Publisher() (socialhub.Publisher, bool)           { return nil, false }
func (client *Client) Fetcher() (socialhub.Fetcher, bool)               { return nil, false }
func (client *Client) MediaUploader() (socialhub.MediaUploader, bool)   { return nil, false }
func (client *Client) Reactor() (socialhub.Reactor, bool)               { return nil, false }
func (client *Client) Messenger() (socialhub.Messenger, bool)           { return nil, false }
func (client *Client) WebhookHandler() (socialhub.WebhookHandler, bool) { return nil, false }
func (client *Client) Close() error                                     { return nil }

func (client *Client) Reporting() ReportingWorkflow { return client }

func (client *Client) ownerQuery(query url.Values) {
	if client.contentOwnerID != "" {
		query.Set("onBehalfOfContentOwner", client.contentOwnerID)
	}
}

func (client *Client) requireReportingScope(operation string) error {
	if len(client.scopes) == 0 {
		return nil
	}
	for _, scope := range client.scopes {
		if scope == analyticsReadScope || scope == analyticsRevenueScope {
			return nil
		}
	}
	return &socialhub.Error{
		Code: socialhub.CodeApprovalRequired, Class: socialhub.ClassUserAction,
		Platform: platformName, Product: productName, Op: operation,
		RequiredScopes: []string{analyticsReadScope}, ApprovalURL: defaultAuthURL,
		PlatformMessage: "configured approval scopes do not authorize YouTube Reporting API",
	}
}

var _ socialhub.Client = (*Client)(nil)
