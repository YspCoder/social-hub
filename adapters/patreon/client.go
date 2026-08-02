package patreon

import (
	"context"
	"strings"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	CapabilityCampaigns socialhub.Capability = "patreon_campaigns"
	CapabilityMembers   socialhub.Capability = "patreon_members"
	CapabilityPosts     socialhub.Capability = "patreon_posts"
)

// Client reads one creator campaign and verifies its webhooks.
type Client struct {
	accountID     socialhub.AccountID
	campaignID    string
	userID        string
	api           *transport.Client
	scopes        []string
	webhookSecret string
	clock         socialhub.Clock
}

func (client *Client) Platform() socialhub.Platform { return "patreon" }
func (client *Client) Account() socialhub.AccountID { return client.accountID }

func (client *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	return socialhub.Capabilities{
		socialhub.CapPublish: {Capability: socialhub.CapPublish, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Patreon API v2 does not expose creator Post publishing"},
		socialhub.CapFetch:   capabilityState(socialhub.CapFetch, client.api != nil, client.scopes, []string{"identity", "campaigns.posts"}, "authorized identity and campaign Posts; Patreon comments are not exposed"),
		socialhub.CapMedia:   {Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Patreon API v2 does not expose a general media upload contract"},
		socialhub.CapReact:   {Capability: socialhub.CapReact, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Patreon API v2 does not expose Post reactions or comments"},
		socialhub.CapMessage: {Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Patreon API v2 does not expose direct messaging"},
		socialhub.CapWebhook: {
			Capability: socialhub.CapWebhook, Supported: client.webhookSecret != "", Approval: webhookApproval(client.webhookSecret),
			Reason: "HMAC-MD5 verification for Patreon campaign member and Post webhook triggers", DocURL: documentationURL,
		},
		CapabilityCampaigns: capabilityState(CapabilityCampaigns, client.api != nil, client.scopes, []string{"campaigns"}, "owned Campaign metadata"),
		CapabilityMembers:   capabilityState(CapabilityMembers, client.api != nil, client.scopes, []string{"campaigns.members"}, "campaign Member and entitlement reads"),
		CapabilityPosts:     capabilityState(CapabilityPosts, client.api != nil, client.scopes, []string{"campaigns.posts"}, "campaign Post reads with JSON:API cursor pagination"),
	}, nil
}

func webhookApproval(secret string) socialhub.ApprovalState {
	if secret == "" {
		return socialhub.ApprovalUnknown
	}
	return socialhub.ApprovalGranted
}

func capabilityState(capability socialhub.Capability, supported bool, granted, required []string, reason string) socialhub.CapabilityState {
	approval := socialhub.ApprovalUnknown
	if supported && len(granted) > 0 {
		approval = socialhub.ApprovalGranted
		for _, scope := range required {
			if !scopeGranted(granted, scope) {
				approval = socialhub.ApprovalRequired
				break
			}
		}
	}
	return socialhub.CapabilityState{
		Capability: capability, Supported: supported, Approval: approval, Scopes: append([]string(nil), required...),
		Reason: reason, DocURL: documentationURL,
	}
}

func (client *Client) Publisher() (socialhub.Publisher, bool) { return nil, false }
func (client *Client) Fetcher() (socialhub.Fetcher, bool) {
	if client.api == nil {
		return nil, false
	}
	return client, true
}
func (client *Client) MediaUploader() (socialhub.MediaUploader, bool) { return nil, false }
func (client *Client) Reactor() (socialhub.Reactor, bool)             { return nil, false }
func (client *Client) Messenger() (socialhub.Messenger, bool)         { return nil, false }
func (client *Client) WebhookHandler() (socialhub.WebhookHandler, bool) {
	if client.webhookSecret == "" {
		return nil, false
	}
	return client, true
}
func (client *Client) Close() error { return nil }

func (client *Client) CampaignWorkflow() CampaignWorkflow { return client }
func (client *Client) MemberWorkflow() MemberWorkflow     { return client }

func (client *Client) requireAPI(operation string) (*transport.Client, error) {
	if client.api == nil {
		return nil, &socialhub.Error{
			Code: socialhub.CodeUnauthenticated, Class: socialhub.ClassUserAction, Platform: "patreon", Product: productName,
			Op: operation, PlatformMessage: "configure access_token_ref with a Patreon API v2 OAuth token",
		}
	}
	return client.api, nil
}

func (client *Client) requireScopes(operation string, required ...string) error {
	if len(client.scopes) == 0 {
		return nil
	}
	missing := make([]string, 0, len(required))
	for _, scope := range required {
		if !scopeGranted(client.scopes, scope) {
			missing = append(missing, scope)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	return &socialhub.Error{
		Code: socialhub.CodeApprovalRequired, Class: socialhub.ClassUserAction, Platform: "patreon", Product: productName,
		Op: operation, RequiredScopes: missing, ApprovalURL: defaultAuthURL,
		PlatformMessage: "configured approval scopes do not include required Patreon API v2 permissions",
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
var _ socialhub.Fetcher = (*Client)(nil)
var _ socialhub.WebhookHandler = (*Client)(nil)
