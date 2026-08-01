package twitch

import (
	"context"
	"strings"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	CapabilityLive     socialhub.Capability = "live_discovery"
	CapabilityEventSub socialhub.Capability = "eventsub"
)

// Client implements common and typed Twitch capabilities for one token.
type Client struct {
	accountID     socialhub.AccountID
	userID        string
	clientID      string
	transport     *transport.Client
	appTransport  *transport.Client
	scopes        []string
	webhookSecret string
	clock         socialhub.Clock
}

func (c *Client) Platform() socialhub.Platform { return "twitch" }
func (c *Client) Account() socialhub.AccountID { return c.accountID }

func (c *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	messageReason := "configure account.settings.user_id to send channel chat messages"
	messageSupported := c.userID != ""
	if messageSupported {
		messageReason = "send channel chat messages; arbitrary chat-message lookup is unavailable"
	}
	webhookReason := "configure account.settings.eventsub_secret_ref to verify EventSub webhook deliveries"
	if c.webhookSecret != "" {
		webhookReason = "EventSub webhook HMAC, timestamp, challenge, and raw event decoding"
	}
	return socialhub.Capabilities{
		socialhub.CapPublish: {Capability: socialhub.CapPublish, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Twitch does not expose generic social-post publication"},
		socialhub.CapFetch:   {Capability: socialhub.CapFetch, Supported: true, Approval: socialhub.ApprovalUnknown, Reason: "users and published VODs map to common models; comments are not exposed", DocURL: docURL + "api/reference/"},
		socialhub.CapMedia:   {Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Helix does not provide general VOD upload through the API"},
		socialhub.CapReact:   {Capability: socialhub.CapReact, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Twitch interactions do not map to generic likes and comments"},
		socialhub.CapMessage: capabilityState(socialhub.CapMessage, messageSupported, c.scopes, []string{"user:write:chat"}, messageReason, docURL+"chat/send-receive-messages/"),
		socialhub.CapWebhook: {Capability: socialhub.CapWebhook, Supported: c.webhookSecret != "", Approval: socialhub.ApprovalUnknown, Reason: webhookReason, DocURL: docURL + "eventsub/handling-webhook-events/"},
		CapabilityLive:       {Capability: CapabilityLive, Supported: true, Approval: socialhub.ApprovalUnknown, Reason: "streams, channels, schedules, clips, and clip creation", DocURL: docURL + "api/reference/"},
		CapabilityEventSub:   {Capability: CapabilityEventSub, Supported: true, Approval: socialhub.ApprovalUnknown, Reason: "webhook subscription create, list, and delete; API calls require an app access token", DocURL: docURL + "eventsub/manage-subscriptions/"},
	}, nil
}

func capabilityState(capability socialhub.Capability, supported bool, granted, required []string, reason, documentation string) socialhub.CapabilityState {
	approval := socialhub.ApprovalUnknown
	if supported && len(granted) > 0 {
		approval = socialhub.ApprovalGranted
		for _, scope := range required {
			if !contains(granted, scope) {
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

func (c *Client) Publisher() (socialhub.Publisher, bool)         { return nil, false }
func (c *Client) Fetcher() (socialhub.Fetcher, bool)             { return c, true }
func (c *Client) MediaUploader() (socialhub.MediaUploader, bool) { return nil, false }
func (c *Client) Reactor() (socialhub.Reactor, bool)             { return nil, false }
func (c *Client) Messenger() (socialhub.Messenger, bool) {
	if c.userID == "" {
		return nil, false
	}
	return c, true
}
func (c *Client) WebhookHandler() (socialhub.WebhookHandler, bool) {
	if c.webhookSecret == "" {
		return nil, false
	}
	return c, true
}
func (c *Client) Close() error { return nil }

func (c *Client) LiveWorkflow() LiveWorkflow         { return c }
func (c *Client) EventSubWorkflow() EventSubWorkflow { return c }

func (c *Client) requireScope(operation string, required ...string) error {
	if len(c.scopes) == 0 {
		return nil
	}
	missing := make([]string, 0, len(required))
	for _, scope := range required {
		if !contains(c.scopes, scope) {
			missing = append(missing, scope)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	return &socialhub.Error{
		Code: socialhub.CodeApprovalRequired, Class: socialhub.ClassUserAction, Platform: "twitch", Product: productName,
		Op: operation, RequiredScopes: missing, ApprovalURL: "https://dev.twitch.tv/console/apps",
		PlatformMessage: "configured approval scopes do not include required Twitch permissions",
	}
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) == target {
			return true
		}
	}
	return false
}

var _ socialhub.Client = (*Client)(nil)
var _ socialhub.Fetcher = (*Client)(nil)
var _ socialhub.Messenger = (*Client)(nil)
