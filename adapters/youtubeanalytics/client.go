package youtubeanalytics

import (
	"context"
	"net/url"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const CapabilityYouTubeAnalytics socialhub.Capability = "youtube_targeted_analytics"

type Client struct {
	accountID socialhub.AccountID
	binding   AccountSettings
	api       *transport.Client
	scopes    []string
}

func (client *Client) Platform() socialhub.Platform { return platformName }
func (client *Client) Account() socialhub.AccountID { return client.accountID }

func (client *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	return socialhub.Capabilities{
		CapabilityYouTubeAnalytics: {
			Capability: CapabilityYouTubeAnalytics, Supported: true, Approval: socialhub.ApprovalUnknown,
			Reason: "account-bound targeted reports and private Analytics group management for channels and YouTube CMS content owners",
			DocURL: documentationURL,
		},
		socialhub.CapPublish: {Capability: socialhub.CapPublish, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "YouTube Analytics measures content and does not publish it"},
		socialhub.CapFetch:   {Capability: socialhub.CapFetch, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "analytics data uses the typed Reporting workflow"},
		socialhub.CapMedia:   {Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "media upload belongs to YouTube Data API v3"},
		socialhub.CapReact:   {Capability: socialhub.CapReact, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "engagement mutations belong to YouTube Data API v3"},
		socialhub.CapMessage: {Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "YouTube Analytics API has no messaging"},
		socialhub.CapWebhook: {Capability: socialhub.CapWebhook, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "YouTube Analytics API v2 exposes no webhooks"},
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
func (client *Client) Groups() GroupsWorkflow       { return client }

func (client *Client) reportIDs() string {
	if client.binding.ChannelID != "" {
		return "channel==" + client.binding.ChannelID
	}
	return "contentOwner==" + client.binding.ContentOwnerID
}

func (client *Client) ownerQuery(query url.Values) {
	if client.binding.ContentOwnerID != "" {
		query.Set("onBehalfOfContentOwner", client.binding.ContentOwnerID)
	}
}

func (client *Client) hasScope(target string) bool {
	if len(client.scopes) == 0 {
		return true
	}
	for _, scope := range client.scopes {
		if scope == target {
			return true
		}
	}
	return false
}

func (client *Client) hasIdentityScope() bool {
	return client.hasScope(youtubeReadOnlyScope) || client.hasScope(youtubeScope) || client.hasScope(youtubePartnerScope)
}

func (client *Client) requireReportScope(operation string, monetary bool) error {
	if len(client.scopes) == 0 {
		return nil
	}
	analyticsScope := analyticsReadScope
	if monetary {
		analyticsScope = analyticsRevenueScope
	}
	if client.hasIdentityScope() && client.hasScope(analyticsScope) {
		return nil
	}
	return approvalError(operation, []string{youtubeReadOnlyScope, analyticsScope}, "configured approval scopes do not authorize this YouTube Analytics report")
}

func (client *Client) requireGroupReadScope(operation string) error {
	if len(client.scopes) == 0 || client.hasScope(youtubeScope) || client.hasScope(youtubePartnerScope) ||
		(client.hasScope(youtubeReadOnlyScope) && (client.hasScope(analyticsReadScope) || client.hasScope(analyticsRevenueScope))) {
		return nil
	}
	return approvalError(operation, []string{youtubeReadOnlyScope, analyticsReadScope}, "configured approval scopes do not authorize YouTube Analytics group reads")
}

func (client *Client) requireGroupWriteScope(operation string) error {
	if len(client.scopes) == 0 {
		return nil
	}
	required := youtubeScope
	if client.binding.ContentOwnerID != "" {
		required = youtubePartnerScope
	}
	if client.hasScope(required) {
		return nil
	}
	return approvalError(operation, []string{required}, "configured approval scopes do not authorize YouTube Analytics group mutations")
}

func approvalError(operation string, scopes []string, message string) error {
	return &socialhub.Error{
		Code: socialhub.CodeApprovalRequired, Class: socialhub.ClassUserAction,
		Platform: platformName, Product: productName, Op: operation,
		RequiredScopes: scopes, ApprovalURL: defaultAuthURL, PlatformMessage: message,
	}
}

var _ socialhub.Client = (*Client)(nil)
