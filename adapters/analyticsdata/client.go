package analyticsdata

import (
	"context"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const CapabilityAnalyticsData socialhub.Capability = "google_analytics_property_reporting"

type Client struct {
	accountID  socialhub.AccountID
	propertyID string
	api        *transport.Client
	scopes     []string
}

func (client *Client) Platform() socialhub.Platform { return platformName }
func (client *Client) Account() socialhub.AccountID { return client.accountID }

func (client *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	return socialhub.Capabilities{
		CapabilityAnalyticsData: {
			Capability: CapabilityAnalyticsData, Supported: true, Approval: socialhub.ApprovalUnknown,
			Reason: "GA4 property-bound metadata, compatibility, aggregate core, realtime, batch, and pivot reports; user-level Audience Exports are intentionally omitted",
			DocURL: documentationURL,
		},
		socialhub.CapPublish: {Capability: socialhub.CapPublish, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Analytics Data API measures activity and does not publish content"},
		socialhub.CapFetch:   {Capability: socialhub.CapFetch, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "measurement data uses typed Analytics workflows"},
		socialhub.CapMedia:   {Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Analytics Data API does not upload media"},
		socialhub.CapReact:   {Capability: socialhub.CapReact, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Analytics Data API has no organic engagement workflow"},
		socialhub.CapMessage: {Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Analytics Data API has no messaging"},
		socialhub.CapWebhook: {Capability: socialhub.CapWebhook, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Analytics Data API v1beta does not expose report delivery webhooks"},
	}, nil
}

func (client *Client) Publisher() (socialhub.Publisher, bool)           { return nil, false }
func (client *Client) Fetcher() (socialhub.Fetcher, bool)               { return nil, false }
func (client *Client) MediaUploader() (socialhub.MediaUploader, bool)   { return nil, false }
func (client *Client) Reactor() (socialhub.Reactor, bool)               { return nil, false }
func (client *Client) Messenger() (socialhub.Messenger, bool)           { return nil, false }
func (client *Client) WebhookHandler() (socialhub.WebhookHandler, bool) { return nil, false }
func (client *Client) Close() error                                     { return nil }

func (client *Client) Metadata() MetadataWorkflow    { return client }
func (client *Client) Reporting() ReportingWorkflow  { return client }
func (client *Client) Realtime() RealtimeWorkflow    { return client }
func (client *Client) PivotReporting() PivotWorkflow { return client }

func (client *Client) requireReadScope(operation string) error {
	if len(client.scopes) == 0 {
		return nil
	}
	for _, configured := range client.scopes {
		if configured == fullScope || configured == readOnlyScope {
			return nil
		}
	}
	return &socialhub.Error{
		Code: socialhub.CodeApprovalRequired, Class: socialhub.ClassUserAction,
		Platform: platformName, Product: productName, Op: operation,
		RequiredScopes: []string{readOnlyScope, fullScope}, ApprovalURL: defaultAuthURL,
		PlatformMessage: "configured approval scopes do not authorize this Google Analytics workflow",
	}
}

func (client *Client) propertyName() string { return "properties/" + client.propertyID }

var _ socialhub.Client = (*Client)(nil)
