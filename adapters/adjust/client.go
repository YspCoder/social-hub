package adjust

import (
	"context"
	"strings"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	CapabilityEvents    socialhub.Capability = "adjust_s2s_events"
	CapabilitySessions  socialhub.Capability = "adjust_s2s_sessions"
	CapabilityAdRevenue socialhub.Capability = "adjust_s2s_ad_revenue"

	ScopeEvents    = "events"
	ScopeSessions  = "sessions"
	ScopeAdRevenue = "ad_revenue"
)

type Client struct {
	accountID        socialhub.AccountID
	appToken         string
	api              *transport.Client
	clock            socialhub.Clock
	scopes           []string
	sessionEnabled   bool
	adRevenueEnabled bool
}

func (client *Client) Platform() socialhub.Platform { return platformName }
func (client *Client) Account() socialhub.AccountID { return client.accountID }

func (client *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	return socialhub.Capabilities{
		CapabilityEvents: capabilityState(CapabilityEvents, ScopeEvents, true, client.scopes,
			"server-to-server event measurement", documentationURL+"events/"),
		CapabilitySessions: capabilityState(CapabilitySessions, ScopeSessions, client.sessionEnabled, client.scopes,
			"approval-gated server-to-server session measurement", documentationURL+"sessions/"),
		CapabilityAdRevenue: capabilityState(CapabilityAdRevenue, ScopeAdRevenue, client.adRevenueEnabled, client.scopes,
			"package-gated publisher ad-revenue measurement", documentationURL+"ad-revenue/"),
		socialhub.CapPublish: {Capability: socialhub.CapPublish, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "measurement telemetry is not an organic post"},
		socialhub.CapFetch:   {Capability: socialhub.CapFetch, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "the S2S API exposes ingestion endpoints"},
		socialhub.CapMedia:   {Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "the S2S API does not upload media"},
		socialhub.CapReact:   {Capability: socialhub.CapReact, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "the S2S API has no engagement surface"},
		socialhub.CapMessage: {Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "the S2S API has no messaging surface"},
		socialhub.CapWebhook: {Capability: socialhub.CapWebhook, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "the ingestion API does not expose webhooks"},
	}, nil
}

func capabilityState(capability socialhub.Capability, scope string, enabled bool, scopes []string, reason, docURL string) socialhub.CapabilityState {
	approval := socialhub.ApprovalUnknown
	if !enabled || len(scopes) > 0 && !scopeGranted(scopes, scope) {
		approval = socialhub.ApprovalRequired
	} else if scope != ScopeEvents || len(scopes) > 0 {
		approval = socialhub.ApprovalGranted
	}
	return socialhub.CapabilityState{
		Capability: capability, Supported: true, Approval: approval,
		Scopes: []string{scope}, Reason: reason, DocURL: docURL,
	}
}

func (client *Client) Publisher() (socialhub.Publisher, bool)           { return nil, false }
func (client *Client) Fetcher() (socialhub.Fetcher, bool)               { return nil, false }
func (client *Client) MediaUploader() (socialhub.MediaUploader, bool)   { return nil, false }
func (client *Client) Reactor() (socialhub.Reactor, bool)               { return nil, false }
func (client *Client) Messenger() (socialhub.Messenger, bool)           { return nil, false }
func (client *Client) WebhookHandler() (socialhub.WebhookHandler, bool) { return nil, false }
func (client *Client) Close() error                                     { return nil }

func (client *Client) S2S() Workflow { return client }

func (client *Client) requireCapability(operation, scope string, enabled bool, message string) error {
	if enabled && (len(client.scopes) == 0 || scopeGranted(client.scopes, scope)) {
		return nil
	}
	return &socialhub.Error{
		Code: socialhub.CodeApprovalRequired, Class: socialhub.ClassUserAction,
		Platform: platformName, Product: productName, Op: operation,
		RequiredScopes: []string{scope}, ApprovalURL: documentationURL + "security/",
		PlatformMessage: message,
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

func validScopes(scopes []string) bool {
	seen := make(map[string]struct{}, len(scopes))
	for _, scope := range scopes {
		if scope != ScopeEvents && scope != ScopeSessions && scope != ScopeAdRevenue {
			return false
		}
		if _, exists := seen[scope]; exists {
			return false
		}
		seen[scope] = struct{}{}
	}
	return true
}

var _ socialhub.Client = (*Client)(nil)
