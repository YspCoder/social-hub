package lark

import (
	"context"
	"slices"
	"strings"
	"sync"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	CapabilityMessages  socialhub.Capability = "lark_messages"
	CapabilityResources socialhub.Capability = "lark_resources"
	CapabilityReactions socialhub.Capability = "lark_reactions"
	CapabilityEvents    socialhub.Capability = "lark_events"
)

// Client implements one Feishu or Lark installation.
type Client struct {
	accountID         socialhub.AccountID
	appID             string
	tenantKey         string
	region            Region
	tokenKind         TokenKind
	userIDType        UserIDType
	actorID           string
	defaultChatID     string
	api               *transport.Client
	clock             socialhub.Clock
	verificationToken string
	encryptKey        string
	scopes            []string

	uploadMu sync.Mutex
	uploads  map[string]*uploadState
	media    map[string]socialhub.Media
}

func (c *Client) Platform() socialhub.Platform { return "lark" }
func (c *Client) Account() socialhub.AccountID { return c.accountID }

func (c *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	publish := c.defaultChatID != ""
	publishReason := "configure account.settings.default_chat_id for common publishing"
	if publish {
		publishReason = "text or single-resource messages in the configured default chat"
	}
	mediaSupported := c.tokenKind == TokenTenant
	mediaReason := "IM image/file upload requires tenant_access_token"
	if mediaSupported {
		mediaReason = "single-part IM image and file upload (10 MB images, 30 MB files)"
	}
	reactorSupported := c.actorID != ""
	reactorReason := "configure account.settings.actor_id so common reaction removal can identify the caller's reaction"
	if reactorSupported {
		reactorReason = "THUMBSUP reactions and message replies; typed workflow preserves exact emoji/reaction IDs"
	}
	webhookSupported := c.verificationToken != ""
	webhookReason := "configure webhook.token_ref; webhook.aes_key_ref enables encrypted signed callbacks"
	if webhookSupported {
		webhookReason = "verification-token binding with optional Encrypt Key signature and AES-256-CBC event decryption"
	}
	return socialhub.Capabilities{
		socialhub.CapPublish: messageCapabilityState(c.tokenKind, socialhub.CapPublish, publish, c.scopes, publishReason, messageDoc("create")),
		socialhub.CapFetch:   capabilityState(socialhub.CapFetch, true, c.scopes, []string{"im:message", "im:message:readonly", "contact:contact.base:readonly"}, "users, individual messages, chat history, and thread replies; endpoint-specific scopes apply", messageDoc("list")),
		socialhub.CapMedia:   capabilityState(socialhub.CapMedia, mediaSupported, c.scopes, []string{"im:resource"}, mediaReason, apiDoc("im-v1/image/create")),
		socialhub.CapReact:   capabilityState(socialhub.CapReact, reactorSupported, c.scopes, []string{"im:message.reactions:write_only", "im:message.reactions:read", "im:message"}, reactorReason, apiDoc("im-v1/message-reaction/create")),
		socialhub.CapMessage: messageCapabilityState(c.tokenKind, socialhub.CapMessage, true, c.scopes, "text and uploaded-resource sends/replies plus individual message lookup", messageDoc("create")),
		socialhub.CapWebhook: {Capability: socialhub.CapWebhook, Supported: webhookSupported, Approval: socialhub.ApprovalUnknown, Reason: webhookReason, DocURL: apiDoc("event-subscription-guide/event-subscription-configure-/encrypt-key-encryption-configuration-case")},
		CapabilityMessages:   messageCapabilityState(c.tokenKind, CapabilityMessages, true, c.scopes, "typed text, post, card, image, file, audio, video, update, reply, and delete operations", messageDoc("introduction")),
		CapabilityResources:  capabilityState(CapabilityResources, mediaSupported, c.scopes, []string{"im:resource"}, mediaReason, apiDoc("im-v1/file/create")),
		CapabilityReactions:  capabilityState(CapabilityReactions, true, c.scopes, []string{"im:message.reactions:write_only"}, "typed emoji add and exact reaction-ID removal", apiDoc("im-v1/message-reaction/create")),
		CapabilityEvents:     {Capability: CapabilityEvents, Supported: webhookSupported, Approval: socialhub.ApprovalUnknown, Reason: webhookReason, DocURL: apiDoc("event-subscription-guide/event-subscription-configure-/encrypt-key-encryption-configuration-case")},
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
	return socialhub.CapabilityState{
		Capability: capability, Supported: supported, Approval: approval,
		Scopes: append([]string(nil), required...), Reason: reason, DocURL: documentation,
	}
}

func messageCapabilityState(kind TokenKind, capability socialhub.Capability, supported bool, granted []string, reason, documentation string) socialhub.CapabilityState {
	required := messageWriteScopes(kind)
	state := capabilityState(capability, supported, granted, required, reason, documentation)
	if !supported || kind != TokenUser || len(granted) == 0 {
		return state
	}
	state.Approval = socialhub.ApprovalGranted
	for _, scope := range required {
		if !slices.Contains(granted, scope) {
			state.Approval = socialhub.ApprovalRequired
			break
		}
	}
	return state
}

func apiDoc(path string) string { return docURL + strings.TrimLeft(path, "/") }
func messageDoc(operation string) string {
	return apiDoc("im-v1/message/" + operation)
}

func (c *Client) Publisher() (socialhub.Publisher, bool) {
	if c.defaultChatID == "" {
		return nil, false
	}
	return c, true
}
func (c *Client) Fetcher() (socialhub.Fetcher, bool) { return c, true }
func (c *Client) MediaUploader() (socialhub.MediaUploader, bool) {
	if c.tokenKind != TokenTenant {
		return nil, false
	}
	return c, true
}
func (c *Client) Reactor() (socialhub.Reactor, bool) {
	if c.actorID == "" {
		return nil, false
	}
	return c, true
}
func (c *Client) Messenger() (socialhub.Messenger, bool) { return c, true }
func (c *Client) WebhookHandler() (socialhub.WebhookHandler, bool) {
	if c.verificationToken == "" {
		return nil, false
	}
	return c, true
}
func (c *Client) Close() error { return nil }

func (c *Client) MessageWorkflow() MessageWorkflow   { return c }
func (c *Client) ReactionWorkflow() ReactionWorkflow { return c }

func (c *Client) requireScopes(operation string, required ...string) error {
	if len(c.scopes) == 0 {
		return nil
	}
	missing := make([]string, 0, len(required))
	for _, scope := range required {
		if !slices.Contains(c.scopes, scope) {
			missing = append(missing, scope)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	return approvalError(operation, missing)
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

func (c *Client) requireMessageWrite(operation string) error {
	if c.tokenKind == TokenUser {
		return c.requireScopes(operation, "im:message", "im:message.send_as_user")
	}
	return c.requireAnyScope(operation, "im:message", "im:message:send_as_bot", "im:message:send")
}

func (c *Client) requireMessageRead(operation string) error {
	return c.requireAnyScope(operation, "im:message", "im:message:readonly", "im:message.history:readonly")
}

func messageWriteScopes(kind TokenKind) []string {
	if kind == TokenUser {
		return []string{"im:message", "im:message.send_as_user"}
	}
	return []string{"im:message", "im:message:send_as_bot", "im:message:send"}
}

func approvalError(operation string, scopes []string) error {
	return &socialhub.Error{
		Code: socialhub.CodeApprovalRequired, Class: socialhub.ClassUserAction, Platform: "lark", Product: productName,
		Op: operation, RequiredScopes: append([]string(nil), scopes...),
		ApprovalURL:     docURL + "application-scope/scope-list",
		PlatformMessage: "configured approval scopes do not satisfy this Feishu/Lark operation",
	}
}

var _ socialhub.Client = (*Client)(nil)
var _ socialhub.Publisher = (*Client)(nil)
var _ socialhub.Fetcher = (*Client)(nil)
var _ socialhub.MediaUploader = (*Client)(nil)
var _ socialhub.Reactor = (*Client)(nil)
var _ socialhub.Messenger = (*Client)(nil)
var _ socialhub.WebhookHandler = (*Client)(nil)
