package cm360

import (
	"context"
	"net/http"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const CapabilityCampaignManager360 socialhub.Capability = "cm360_trafficking_reporting"

// Client exposes advertiser-bound CM360 trafficking reads, archived-first
// Campaign management, and profile-scoped reporting workflows.
type Client struct {
	accountID    socialhub.AccountID
	profileID    string
	advertiserID string
	api          *transport.Client
	httpClient   *http.Client
	scopes       []string
}

func (client *Client) Platform() socialhub.Platform { return platformName }
func (client *Client) Account() socialhub.AccountID { return client.accountID }

func (client *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	return socialhub.Capabilities{
		CapabilityCampaignManager360: {
			Capability: CapabilityCampaignManager360, Supported: true, Approval: socialhub.ApprovalUnknown,
			Reason: "Profile and advertiser reads, archived-first Campaign management, Placement/Ad reads, direct report queries, and bounded report-file downloads",
			DocURL: documentationURL,
		},
		socialhub.CapPublish: {Capability: socialhub.CapPublish, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "CM360 trafficking is not organic social publishing"},
		socialhub.CapFetch:   {Capability: socialhub.CapFetch, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "paid-media reads use typed CM360 workflows"},
		socialhub.CapMedia:   {Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "creative asset upload is outside the initial adapter contract"},
		socialhub.CapReact:   {Capability: socialhub.CapReact, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "CM360 is not an organic engagement product"},
		socialhub.CapMessage: {Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "CM360 does not provide social messaging"},
		socialhub.CapWebhook: {Capability: socialhub.CapWebhook, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "CM360 v5 does not expose these workflows as social webhooks"},
	}, nil
}

func (client *Client) Publisher() (socialhub.Publisher, bool)           { return nil, false }
func (client *Client) Fetcher() (socialhub.Fetcher, bool)               { return nil, false }
func (client *Client) MediaUploader() (socialhub.MediaUploader, bool)   { return nil, false }
func (client *Client) Reactor() (socialhub.Reactor, bool)               { return nil, false }
func (client *Client) Messenger() (socialhub.Messenger, bool)           { return nil, false }
func (client *Client) WebhookHandler() (socialhub.WebhookHandler, bool) { return nil, false }
func (client *Client) Close() error                                     { return nil }

func (client *Client) Profiles() ProfileWorkflow       { return client }
func (client *Client) Advertisers() AdvertiserWorkflow { return client }
func (client *Client) Campaigns() CampaignWorkflow     { return client }
func (client *Client) Placements() PlacementWorkflow   { return client }
func (client *Client) Ads() AdWorkflow                 { return client }
func (client *Client) Reporting() ReportingWorkflow    { return client }

func (client *Client) requireScope(operation, required string) error {
	if len(client.scopes) == 0 {
		return nil
	}
	for _, scope := range client.scopes {
		if scope == required {
			return nil
		}
	}
	return &socialhub.Error{
		Code: socialhub.CodeApprovalRequired, Class: socialhub.ClassUserAction,
		Platform: platformName, Product: productName, Op: operation,
		RequiredScopes: []string{required}, ApprovalURL: defaultAuthURL,
		PlatformMessage: "configured approval scopes do not authorize this Campaign Manager 360 workflow",
	}
}

func (client *Client) requireAnyScope(operation string, allowed ...string) error {
	if len(client.scopes) == 0 {
		return nil
	}
	for _, configured := range client.scopes {
		for _, candidate := range allowed {
			if configured == candidate {
				return nil
			}
		}
	}
	return &socialhub.Error{
		Code: socialhub.CodeApprovalRequired, Class: socialhub.ClassUserAction,
		Platform: platformName, Product: productName, Op: operation,
		RequiredScopes: append([]string(nil), allowed...), ApprovalURL: defaultAuthURL,
		PlatformMessage: "configured approval scopes do not authorize Campaign Manager 360 profile access",
	}
}

var _ socialhub.Client = (*Client)(nil)
