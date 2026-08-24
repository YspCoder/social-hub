package kakaomoment

import (
	"context"
	"strings"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	CapabilityAccountRead        socialhub.Capability = "kakao_moment_account_read"
	CapabilityCampaignManagement socialhub.Capability = "kakao_moment_campaign_management"
	CapabilityAdGroupManagement  socialhub.Capability = "kakao_moment_adgroup_management"
	CapabilityCreativeManagement socialhub.Capability = "kakao_moment_creative_management"
	CapabilityReports            socialhub.Capability = "kakao_moment_reports"
	CapabilityGuardedDelete      socialhub.Capability = "kakao_moment_guarded_delete"
)

// Client exposes one Kakao Moment ad account's management workflows.
type Client struct {
	accountID   socialhub.AccountID
	adAccountID int64
	api         *transport.Client
	scopes      []string
	clock       socialhub.Clock
}

func (client *Client) Platform() socialhub.Platform { return platformName }
func (client *Client) Account() socialhub.AccountID { return client.accountID }

func (client *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	managementApproval := client.approvalFor(ScopeManagement)
	deleteApproval := client.approvalFor(ScopeManagement, ScopeDelete)
	state := func(capability socialhub.Capability, approval socialhub.ApprovalState, scopes []string, reason, doc string) socialhub.CapabilityState {
		return socialhub.CapabilityState{
			Capability: capability, Supported: true, Approval: approval,
			Scopes: scopes, Reason: reason, DocURL: doc,
		}
	}
	return socialhub.Capabilities{
		CapabilityAccountRead: state(CapabilityAccountRead, managementApproval, []string{ScopeManagement},
			"ad-account detail and real-time balance", "https://developers.kakao.com/docs/en/kakaomoment/ad-account"),
		CapabilityCampaignManagement: state(CapabilityCampaignManagement, managementApproval, []string{ScopeManagement},
			"Campaign reads, native ON-first creation followed immediately by OFF, editing, budget, and status", "https://developers.kakao.com/docs/en/kakaomoment/campaign"),
		CapabilityAdGroupManagement: state(CapabilityAdGroupManagement, managementApproval, []string{ScopeManagement},
			"Ad Group reads plus daily-budget, bid, and status changes", "https://developers.kakao.com/docs/en/kakaomoment/ad-group"),
		CapabilityCreativeManagement: state(CapabilityCreativeManagement, managementApproval, []string{ScopeManagement},
			"Creative reads and supported Display Creative status changes", "https://developers.kakao.com/docs/en/kakaomoment/creatives"),
		CapabilityReports: state(CapabilityReports, managementApproval, []string{ScopeManagement},
			"synchronous account, Campaign, Ad Group, and Creative reports", "https://developers.kakao.com/docs/en/kakaomoment/report"),
		CapabilityGuardedDelete: state(CapabilityGuardedDelete, deleteApproval, []string{ScopeManagement, ScopeDelete},
			"OFF-preflight deletion for Campaigns, Ad Groups, and Creatives", "https://developers.kakao.com/docs/en/kakaomoment/common"),
		socialhub.CapPublish: {Capability: socialhub.CapPublish, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "paid advertising management is not organic publishing"},
		socialhub.CapFetch:   {Capability: socialhub.CapFetch, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "advertising reads use typed Moment workflows"},
		socialhub.CapMedia:   {Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "asset upload is outside this initial adapter surface"},
		socialhub.CapReact:   {Capability: socialhub.CapReact, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Moment has no organic reaction surface"},
		socialhub.CapMessage: {Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Moment message-delivery workflows are outside this initial adapter surface"},
		socialhub.CapWebhook: {Capability: socialhub.CapWebhook, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Moment Open API does not expose management webhooks"},
	}, nil
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
func (client *Client) AdGroups() AdGroupWorkflow   { return client }
func (client *Client) Creatives() CreativeWorkflow { return client }
func (client *Client) Reports() ReportWorkflow     { return client }

func (client *Client) approvalFor(scopes ...string) socialhub.ApprovalState {
	if len(client.scopes) == 0 {
		return socialhub.ApprovalUnknown
	}
	for _, scope := range scopes {
		if !scopeGranted(client.scopes, scope) {
			return socialhub.ApprovalRequired
		}
	}
	return socialhub.ApprovalGranted
}

func (client *Client) requireScopes(operation string, scopes ...string) error {
	if len(client.scopes) == 0 {
		return nil
	}
	missing := make([]string, 0, len(scopes))
	for _, scope := range scopes {
		if !scopeGranted(client.scopes, scope) {
			missing = append(missing, scope)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	return &socialhub.Error{
		Code: socialhub.CodeApprovalRequired, Class: socialhub.ClassUserAction,
		Platform: platformName, Product: productName, Op: operation,
		RequiredScopes: missing, ApprovalURL: approvalURL,
		PlatformMessage: "configured Business Authentication scopes do not authorize this operation",
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
