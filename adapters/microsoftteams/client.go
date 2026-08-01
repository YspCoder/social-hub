package microsoftteams

import (
	"context"
	"net/url"
	"slices"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	CapabilityMessages      socialhub.Capability = "microsoft_teams_messages"
	CapabilityReactions     socialhub.Capability = "microsoft_teams_reactions"
	CapabilitySubscriptions socialhub.Capability = "microsoft_graph_subscriptions"
)

// Client implements one Microsoft Teams account.
type Client struct {
	accountID     socialhub.AccountID
	cloud         Cloud
	tokenKind     TokenKind
	tenantID      string
	actorID       string
	defaultTarget *Target
	api           *transport.Client
	baseURL       *url.URL
	clock         socialhub.Clock
	clientState   string
	scopes        []string
}

func (c *Client) Platform() socialhub.Platform { return "microsoft-teams" }
func (c *Client) Account() socialhub.AccountID { return c.accountID }

func (c *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	delegated := c.tokenKind == TokenDelegated
	defaultWrite := delegated && c.defaultTarget != nil
	webhook := c.clientState != ""
	return socialhub.Capabilities{
		socialhub.CapPublish:    capabilityState(socialhub.CapPublish, defaultWrite, c.scopes, []string{"ChannelMessage.Send", "ChatMessage.Send", "Chat.ReadWrite"}, "delegated work/school token and a default chat or channel are required", messageDoc()),
		socialhub.CapFetch:      capabilityState(socialhub.CapFetch, true, c.scopes, readScopes(c.tokenKind), "chat/channel messages and threaded replies; Graph does not expose user lookup through this adapter", messageDoc()),
		socialhub.CapMedia:      {Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Teams files live in SharePoint or OneDrive; typed sends support inline hosted content only", DocURL: hostedContentDoc()},
		socialhub.CapReact:      capabilityState(socialhub.CapReact, delegated, c.scopes, []string{"ChannelMessage.Send", "Chat.ReadWrite", "ChatMessage.Send"}, "delegated work/school token required", reactionDoc()),
		socialhub.CapMessage:    capabilityState(socialhub.CapMessage, true, c.scopes, readScopes(c.tokenKind), "application tokens are read-only; ordinary sends require delegated work/school tokens", messageDoc()),
		socialhub.CapWebhook:    {Capability: socialhub.CapWebhook, Supported: webhook, Approval: socialhub.ApprovalUnknown, Reason: "configure webhook.secret_ref as the subscription clientState", DocURL: webhookDoc()},
		CapabilityMessages:      capabilityState(CapabilityMessages, true, c.scopes, readScopes(c.tokenKind), "typed chat/channel root and reply operations", messageDoc()),
		CapabilityReactions:     capabilityState(CapabilityReactions, delegated, c.scopes, []string{"ChannelMessage.Send", "Chat.ReadWrite", "ChatMessage.Send"}, "Unicode and compatibility reactions", reactionDoc()),
		CapabilitySubscriptions: {Capability: CapabilitySubscriptions, Supported: webhook, Approval: socialhub.ApprovalUnknown, Reason: "basic notifications only; encrypted rich resource data is intentionally unsupported", DocURL: webhookDoc()},
	}, nil
}

func capabilityState(capability socialhub.Capability, supported bool, granted, required []string, reason, documentation string) socialhub.CapabilityState {
	approval := socialhub.ApprovalUnknown
	if supported && len(granted) > 0 {
		approval = socialhub.ApprovalRequired
		for _, scope := range required {
			if slices.Contains(granted, scope) {
				approval = socialhub.ApprovalGranted
				break
			}
		}
	}
	return socialhub.CapabilityState{Capability: capability, Supported: supported, Approval: approval, Scopes: append([]string(nil), required...), Reason: reason, DocURL: documentation}
}

func readScopes(kind TokenKind) []string {
	if kind == TokenApplication {
		return []string{"ChannelMessage.Read.Group", "ChannelMessage.Read.All", "ChatMessage.Read.Chat", "Chat.Read.All", "Chat.ReadWrite.All"}
	}
	return []string{"ChannelMessage.Read.All", "Chat.Read", "Chat.ReadWrite"}
}

func (c *Client) Publisher() (socialhub.Publisher, bool) {
	if c.tokenKind != TokenDelegated || c.defaultTarget == nil {
		return nil, false
	}
	return c, true
}
func (c *Client) Fetcher() (socialhub.Fetcher, bool)             { return c, true }
func (c *Client) MediaUploader() (socialhub.MediaUploader, bool) { return nil, false }
func (c *Client) Reactor() (socialhub.Reactor, bool) {
	if c.tokenKind != TokenDelegated {
		return nil, false
	}
	return c, true
}
func (c *Client) Messenger() (socialhub.Messenger, bool) { return c, true }
func (c *Client) WebhookHandler() (socialhub.WebhookHandler, bool) {
	if c.clientState == "" {
		return nil, false
	}
	return c, true
}
func (c *Client) Close() error { return nil }

func (c *Client) MessageWorkflow() MessageWorkflow           { return c }
func (c *Client) ReactionWorkflow() ReactionWorkflow         { return c }
func (c *Client) SubscriptionWorkflow() SubscriptionWorkflow { return c }

func (c *Client) requireDelegated(operation string) error {
	if c.tokenKind == TokenDelegated {
		return nil
	}
	return unsupported(operation, "ordinary Teams message mutations require a delegated work/school token; application and migration tokens are not substitutes")
}

func (c *Client) requireAnyScope(operation string, accepted ...string) error {
	if len(c.scopes) == 0 {
		return nil
	}
	for _, scope := range accepted {
		if slices.Contains(c.scopes, scope) {
			return nil
		}
	}
	return approvalError(operation, accepted)
}

func (c *Client) requireRead(operation string, target Target) error {
	if target.Kind == TargetChannel {
		if c.tokenKind == TokenApplication {
			return c.requireAnyScope(operation, "ChannelMessage.Read.Group", "ChannelMessage.Read.All")
		}
		return c.requireAnyScope(operation, "ChannelMessage.Read.All")
	}
	if c.tokenKind == TokenApplication {
		return c.requireAnyScope(operation, "ChatMessage.Read.Chat", "Chat.Read.All", "Chat.ReadWrite.All")
	}
	return c.requireAnyScope(operation, "Chat.Read", "Chat.ReadWrite")
}

func (c *Client) requireSend(operation string, target Target) error {
	if err := c.requireDelegated(operation); err != nil {
		return err
	}
	if target.Kind == TargetChannel {
		return c.requireAnyScope(operation, "ChannelMessage.Send")
	}
	return c.requireAnyScope(operation, "ChatMessage.Send", "Chat.ReadWrite")
}

func (c *Client) requireEdit(operation string, target Target) error {
	if err := c.requireDelegated(operation); err != nil {
		return err
	}
	if target.Kind == TargetChannel {
		return c.requireAnyScope(operation, "ChannelMessage.ReadWrite")
	}
	return c.requireAnyScope(operation, "Chat.ReadWrite")
}

func (c *Client) requireReaction(operation string, target Target) error {
	if err := c.requireDelegated(operation); err != nil {
		return err
	}
	if target.Kind == TargetChannel {
		return c.requireAnyScope(operation, "ChannelMessage.Send")
	}
	return c.requireAnyScope(operation, "Chat.ReadWrite", "ChatMessage.Send")
}

func approvalError(operation string, scopes []string) error {
	return &socialhub.Error{
		Code: socialhub.CodeApprovalRequired, Class: socialhub.ClassUserAction, Platform: "microsoft-teams", Product: productName,
		Op: operation, RequiredScopes: append([]string(nil), scopes...),
		ApprovalURL:     "https://learn.microsoft.com/en-us/graph/permissions-reference",
		PlatformMessage: "configured approval scopes do not satisfy this Microsoft Graph operation",
	}
}

func messageDoc() string {
	return "https://learn.microsoft.com/en-us/graph/api/chatmessage-post?view=graph-rest-1.0"
}
func reactionDoc() string {
	return "https://learn.microsoft.com/en-us/graph/api/chatmessage-setreaction?view=graph-rest-1.0"
}
func hostedContentDoc() string {
	return "https://learn.microsoft.com/en-us/graph/api/resources/chatmessagehostedcontent?view=graph-rest-1.0"
}
func webhookDoc() string {
	return "https://learn.microsoft.com/en-us/graph/change-notifications-delivery-webhooks"
}

var _ socialhub.Client = (*Client)(nil)
var _ socialhub.Publisher = (*Client)(nil)
var _ socialhub.Fetcher = (*Client)(nil)
var _ socialhub.Reactor = (*Client)(nil)
var _ socialhub.Messenger = (*Client)(nil)
var _ socialhub.WebhookHandler = (*Client)(nil)
