package kick

import (
	"context"
	"crypto/rsa"
	"strings"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	CapabilityUsers         socialhub.Capability = "kick_users"
	CapabilityChannels      socialhub.Capability = "kick_channels"
	CapabilityLivestreams   socialhub.Capability = "kick_livestreams"
	CapabilityCategories    socialhub.Capability = "kick_categories"
	CapabilityChat          socialhub.Capability = "kick_chat"
	CapabilitySubscriptions socialhub.Capability = "kick_event_subscriptions"
)

// Client exposes one Kick app or user access token.
type Client struct {
	accountID         socialhub.AccountID
	broadcasterUserID string
	channelSlug       string
	tokenType         string
	scopes            []string
	api               *transport.Client
	webhookPublicKey  *rsa.PublicKey
}

func (client *Client) Platform() socialhub.Platform { return "kick" }
func (client *Client) Account() socialhub.AccountID { return client.accountID }

func (client *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	chatSupported := client.tokenType == "user"
	chatReason := "chat send/delete requires a user access token"
	if chatSupported {
		chatReason = "typed user/bot chat send and moderator message deletion"
	}
	return socialhub.Capabilities{
		socialhub.CapPublish:    {Capability: socialhub.CapPublish, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Kick does not expose generic social-post publication"},
		socialhub.CapFetch:      {Capability: socialhub.CapFetch, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "users and active livestreams are typed workflows; Kick has no common post/comment read contract"},
		socialhub.CapMedia:      {Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Kick Public API has no general media upload endpoint"},
		socialhub.CapReact:      {Capability: socialhub.CapReact, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Kick interactions do not map to common reactions"},
		socialhub.CapMessage:    {Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "chat has no arbitrary message lookup; use ChatWorkflow"},
		socialhub.CapWebhook:    {Capability: socialhub.CapWebhook, Supported: true, Approval: socialhub.ApprovalGranted, Reason: "RSA-SHA256 verification and typed decoding for all documented v1 events", DocURL: documentationURL + "/events/webhook-security"},
		CapabilityUsers:         client.capability(CapabilityUsers, true, []string{"user:read"}, "typed user lookup", documentationURL+"/apis/users"),
		CapabilityChannels:      client.capability(CapabilityChannels, true, []string{"channel:read"}, "typed channel lookup and user-token metadata update", documentationURL+"/apis/channels"),
		CapabilityLivestreams:   {Capability: CapabilityLivestreams, Supported: true, Approval: socialhub.ApprovalGranted, Reason: "V2 active livestream cursor pagination and user livestream lookup", DocURL: documentationURL + "/apis/livestreams"},
		CapabilityCategories:    {Capability: CapabilityCategories, Supported: true, Approval: socialhub.ApprovalGranted, Reason: "V2 category cursor pagination", DocURL: documentationURL + "/apis/categories"},
		CapabilityChat:          client.capability(CapabilityChat, chatSupported, []string{"chat:write"}, chatReason, documentationURL+"/apis/chat"),
		CapabilitySubscriptions: client.subscriptionCapability(),
	}, nil
}

func (client *Client) capability(capability socialhub.Capability, supported bool, required []string, reason, docURL string) socialhub.CapabilityState {
	approval := socialhub.ApprovalUnknown
	if supported && client.tokenType == "app" {
		approval = socialhub.ApprovalGranted
	} else if supported && len(client.scopes) != 0 {
		approval = socialhub.ApprovalGranted
		for _, scope := range required {
			if !containsScope(client.scopes, scope) {
				approval = socialhub.ApprovalRequired
				break
			}
		}
	}
	return socialhub.CapabilityState{
		Capability: capability, Supported: supported, Approval: approval,
		Scopes: append([]string(nil), required...), Reason: reason, DocURL: docURL,
	}
}

func (client *Client) subscriptionCapability() socialhub.CapabilityState {
	if client.tokenType == "app" {
		return socialhub.CapabilityState{
			Capability: CapabilitySubscriptions, Supported: true, Approval: socialhub.ApprovalGranted,
			Reason: "app-token webhook subscription CRUD", DocURL: documentationURL + "/events/subscribe-to-events",
		}
	}
	return client.capability(CapabilitySubscriptions, true, []string{"events:subscribe"}, "user-token webhook subscription CRUD", documentationURL+"/events/subscribe-to-events")
}

func (client *Client) Publisher() (socialhub.Publisher, bool)         { return nil, false }
func (client *Client) Fetcher() (socialhub.Fetcher, bool)             { return nil, false }
func (client *Client) MediaUploader() (socialhub.MediaUploader, bool) { return nil, false }
func (client *Client) Reactor() (socialhub.Reactor, bool)             { return nil, false }
func (client *Client) Messenger() (socialhub.Messenger, bool)         { return nil, false }
func (client *Client) WebhookHandler() (socialhub.WebhookHandler, bool) {
	return client, client.webhookPublicKey != nil
}
func (client *Client) Close() error { return nil }

func (client *Client) UserWorkflow() UserWorkflow                 { return client }
func (client *Client) ChannelWorkflow() ChannelWorkflow           { return client }
func (client *Client) LivestreamWorkflow() LivestreamWorkflow     { return client }
func (client *Client) CategoryWorkflow() CategoryWorkflow         { return client }
func (client *Client) ChatWorkflow() ChatWorkflow                 { return client }
func (client *Client) SubscriptionWorkflow() SubscriptionWorkflow { return client }

func (client *Client) requireUserToken(operation string) error {
	if client.tokenType == "user" {
		return nil
	}
	return approvalRequired(operation, nil, "operation requires a Kick user access token")
}

func (client *Client) requireScope(operation string, scopes ...string) error {
	if client.tokenType == "app" || len(client.scopes) == 0 {
		return nil
	}
	missing := make([]string, 0, len(scopes))
	for _, scope := range scopes {
		if !containsScope(client.scopes, scope) {
			missing = append(missing, scope)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	return approvalRequired(operation, missing, "configured scopes do not include required Kick permissions")
}

func containsScope(scopes []string, target string) bool {
	for _, scope := range scopes {
		if strings.TrimSpace(scope) == target {
			return true
		}
	}
	return false
}

var _ socialhub.Client = (*Client)(nil)
var _ socialhub.WebhookHandler = (*Client)(nil)
