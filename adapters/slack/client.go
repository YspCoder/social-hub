package slack

import (
	"context"
	"net/http"
	"slices"
	"strings"
	"sync"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	CapabilityChat   socialhub.Capability = "slack_chat"
	CapabilityFiles  socialhub.Capability = "slack_files"
	CapabilityEvents socialhub.Capability = "slack_events"
)

// Client implements Slack capabilities for one workspace installation.
type Client struct {
	accountID        socialhub.AccountID
	workspaceID      string
	tokenKind        TokenKind
	actorID          string
	defaultChannelID string
	api              *transport.Client
	httpClient       *http.Client
	clock            socialhub.Clock
	signingSecret    string
	scopes           []string
	allowHTTPUploads bool

	uploadMu sync.Mutex
	uploads  map[string]*uploadState
	media    map[string]socialhub.Media
}

func (c *Client) Platform() socialhub.Platform { return "slack" }
func (c *Client) Account() socialhub.AccountID { return c.accountID }

func (c *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	publish := c.defaultChannelID != ""
	publishReason := "configure account.settings.default_channel_id for common channel publishing"
	if publish {
		publishReason = "post, reply, inspect, and delete messages in the configured default channel"
	}
	webhookReason := "configure webhook.secret_ref with the Slack app signing secret"
	if c.signingSecret != "" {
		webhookReason = "Events API v0 HMAC verification, replay-window validation, and typed decoding"
	}
	return socialhub.Capabilities{
		socialhub.CapPublish: capabilityState(socialhub.CapPublish, publish, c.scopes, []string{"chat:write"}, publishReason, methodDoc("chat.postMessage")),
		socialhub.CapFetch:   fetchCapabilityState(c.scopes),
		socialhub.CapMedia:   capabilityState(socialhub.CapMedia, true, c.scopes, []string{"files:write"}, "private common uploads and channel/thread-aware typed external uploads", methodDoc("files.getUploadURLExternal")),
		socialhub.CapReact:   capabilityState(socialhub.CapReact, true, c.scopes, []string{"reactions:write", "chat:write"}, "common like maps to thumbsup; comments map to thread replies", methodDoc("reactions.add")),
		socialhub.CapMessage: capabilityState(socialhub.CapMessage, true, c.scopes, []string{"chat:write"}, "send channel, group, and direct messages; lookup is limited to top-level conversation history", methodDoc("chat.postMessage")),
		socialhub.CapWebhook: {Capability: socialhub.CapWebhook, Supported: c.signingSecret != "", Approval: socialhub.ApprovalUnknown, Reason: webhookReason, DocURL: "https://docs.slack.dev/apis/events-api/"},
		CapabilityChat:       capabilityState(CapabilityChat, true, c.scopes, []string{"chat:write"}, "explicit-channel posting and message updates", methodDoc("chat.update")),
		CapabilityFiles:      capabilityState(CapabilityFiles, true, c.scopes, []string{"files:write"}, "three-stage external upload with channel, thread, title, alt text, and initial comment", methodDoc("files.completeUploadExternal")),
		CapabilityEvents:     {Capability: CapabilityEvents, Supported: c.signingSecret != "", Approval: socialhub.ApprovalUnknown, Reason: webhookReason, DocURL: "https://docs.slack.dev/apis/events-api/"},
	}, nil
}

func fetchCapabilityState(scopes []string) socialhub.CapabilityState {
	state := socialhub.CapabilityState{
		Capability: socialhub.CapFetch, Supported: true, Approval: socialhub.ApprovalUnknown,
		Scopes: []string{"users:read", "channels:history", "groups:history", "im:history", "mpim:history"},
		Reason: "workspace users, conversation history, messages, and thread replies; endpoint-specific history scopes apply",
		DocURL: methodDoc("conversations.history"),
	}
	if len(scopes) > 0 {
		state.Approval = socialhub.ApprovalRequired
		if slices.Contains(scopes, "users:read") && containsAnyScope(scopes, "channels:history", "groups:history", "im:history", "mpim:history") {
			state.Approval = socialhub.ApprovalGranted
		}
	}
	return state
}

func capabilityState(capability socialhub.Capability, supported bool, granted, required []string, reason, documentation string) socialhub.CapabilityState {
	approval := socialhub.ApprovalUnknown
	if supported && len(granted) > 0 {
		approval = socialhub.ApprovalGranted
		for _, scope := range required {
			if !slices.Contains(granted, scope) {
				approval = socialhub.ApprovalRequired
				break
			}
		}
	}
	return socialhub.CapabilityState{
		Capability: capability, Supported: supported, Approval: approval,
		Scopes: append([]string(nil), required...), Reason: reason, DocURL: documentation,
	}
}

func methodDoc(method string) string {
	return "https://docs.slack.dev/reference/methods/" + method + "/"
}

func (c *Client) Publisher() (socialhub.Publisher, bool) {
	if c.defaultChannelID == "" {
		return nil, false
	}
	return c, true
}
func (c *Client) Fetcher() (socialhub.Fetcher, bool)             { return c, true }
func (c *Client) MediaUploader() (socialhub.MediaUploader, bool) { return c, true }
func (c *Client) Reactor() (socialhub.Reactor, bool)             { return c, true }
func (c *Client) Messenger() (socialhub.Messenger, bool)         { return c, true }
func (c *Client) WebhookHandler() (socialhub.WebhookHandler, bool) {
	if c.signingSecret == "" {
		return nil, false
	}
	return c, true
}
func (c *Client) Close() error { return nil }

func (c *Client) ChatWorkflow() ChatWorkflow { return c }
func (c *Client) FileWorkflow() FileWorkflow { return c }

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
	return &socialhub.Error{
		Code: socialhub.CodeApprovalRequired, Class: socialhub.ClassUserAction, Platform: "slack", Product: productName,
		Op: operation, RequiredScopes: missing, ApprovalURL: "https://api.slack.com/apps",
		PlatformMessage: "configured approval scopes do not include required Slack permissions",
	}
}

func (c *Client) requireAnyScope(operation string, required ...string) error {
	if containsAnyScope(c.scopes, required...) {
		return nil
	}
	return &socialhub.Error{
		Code: socialhub.CodeApprovalRequired, Class: socialhub.ClassUserAction, Platform: "slack", Product: productName,
		Op: operation, RequiredScopes: append([]string(nil), required...), ApprovalURL: "https://api.slack.com/apps",
		PlatformMessage: "configured approval scopes do not include any accepted Slack permission",
	}
}

func (c *Client) requireHistoryScope(operation, channelID string) error {
	switch channelID[0] {
	case 'C':
		return c.requireScopes(operation, "channels:history")
	case 'D':
		return c.requireScopes(operation, "im:history")
	default:
		return c.requireAnyScope(operation, "groups:history", "mpim:history")
	}
}

func containsAnyScope(scopes []string, required ...string) bool {
	if len(scopes) == 0 || len(required) == 0 {
		return true
	}
	for _, scope := range required {
		if slices.Contains(scopes, strings.TrimSpace(scope)) {
			return true
		}
	}
	return false
}

var _ socialhub.Client = (*Client)(nil)
var _ socialhub.Publisher = (*Client)(nil)
var _ socialhub.Fetcher = (*Client)(nil)
var _ socialhub.MediaUploader = (*Client)(nil)
var _ socialhub.Reactor = (*Client)(nil)
var _ socialhub.Messenger = (*Client)(nil)
var _ socialhub.WebhookHandler = (*Client)(nil)
