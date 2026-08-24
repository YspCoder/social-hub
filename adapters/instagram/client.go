package instagram

import (
	"context"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

// Client implements the common capabilities supported by Instagram Login.
type Client struct {
	accountID     socialhub.AccountID
	userID        string
	transport     *transport.Client
	webhookSecret string
	webhookToken  string
	scopes        []string
	clock         socialhub.Clock
	containers    *ContainerService
}

func (c *Client) Platform() socialhub.Platform { return "instagram" }
func (c *Client) Account() socialhub.AccountID { return c.accountID }

func (c *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	webhookSupported := c.webhookSecret != ""
	webhookReason := "configure webhook.secret_ref to verify X-Hub-Signature-256"
	if webhookSupported {
		webhookReason = "Instagram webhook payloads are HMAC verified"
	}
	return socialhub.Capabilities{
		CapabilityContainerPublish: capabilityState(CapabilityContainerPublish, true, c.scopes, []string{"instagram_business_basic", "instagram_business_content_publish"}, "publication uses remote media containers and asynchronous status polling", docURL+"content-publishing/"),
		socialhub.CapPublish:       {Capability: socialhub.CapPublish, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "use ContainerWorkflow; Instagram does not support text-only posts or independent media IDs"},
		socialhub.CapFetch:         capabilityState(socialhub.CapFetch, true, c.scopes, []string{"instagram_business_basic"}, "reads are limited to the authorized professional account", docURL),
		socialhub.CapMedia:         {Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Instagram fetches application-hosted media URLs into publication containers"},
		socialhub.CapReact:         capabilityState(socialhub.CapReact, true, c.scopes, []string{"instagram_business_manage_comments"}, "comment listing, replies, and deletion are supported; like mutation is unavailable", docURL+"comment-moderation/"),
		socialhub.CapMessage:       capabilityState(socialhub.CapMessage, true, c.scopes, []string{messagingScope}, "sends one-to-one messages in user-initiated conversations and reads recent message details", docURL+"messaging-api/"),
		socialhub.CapWebhook:       {Capability: socialhub.CapWebhook, Supported: webhookSupported, Approval: socialhub.ApprovalUnknown, Reason: webhookReason, DocURL: docURL + "webhooks/"},
	}, nil
}

func capabilityState(capability socialhub.Capability, supported bool, granted, required []string, reason, docURL string) socialhub.CapabilityState {
	approval := socialhub.ApprovalUnknown
	if len(granted) > 0 {
		approval = socialhub.ApprovalGranted
		for _, scope := range required {
			if !contains(granted, scope) {
				approval = socialhub.ApprovalRequired
				break
			}
		}
	}
	return socialhub.CapabilityState{Capability: capability, Supported: supported, Approval: approval, Scopes: required, Reason: reason, DocURL: docURL}
}

func (c *Client) Publisher() (socialhub.Publisher, bool)         { return nil, false }
func (c *Client) Fetcher() (socialhub.Fetcher, bool)             { return c, true }
func (c *Client) MediaUploader() (socialhub.MediaUploader, bool) { return nil, false }
func (c *Client) Reactor() (socialhub.Reactor, bool)             { return c, true }
func (c *Client) Messenger() (socialhub.Messenger, bool)         { return c, true }
func (c *Client) WebhookHandler() (socialhub.WebhookHandler, bool) {
	if c.webhookSecret == "" {
		return nil, false
	}
	return c, true
}
func (c *Client) Close() error { return nil }

// ContainerWorkflow returns Instagram's typed media-container workflow.
func (c *Client) ContainerWorkflow() ContainerWorkflow { return c.containers }

// MessagingWorkflow returns Instagram's typed text, media, published-post,
// and message-reaction workflow.
func (c *Client) MessagingWorkflow() MessagingWorkflow { return c }

// MessagingProfileWorkflow returns the consented IGSID profile reader.
func (c *Client) MessagingProfileWorkflow() MessagingProfileWorkflow { return c }

func (c *Client) requireScope(operation, scope string) error {
	if len(c.scopes) == 0 || contains(c.scopes, scope) {
		return nil
	}
	return &socialhub.Error{
		Code: socialhub.CodeApprovalRequired, Class: socialhub.ClassUserAction, Platform: "instagram", Product: "instagram-login", Op: operation,
		RequiredScopes: []string{scope}, ApprovalURL: "https://developers.facebook.com/apps/", PlatformMessage: "configured approval scopes do not include " + scope,
	}
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

var _ socialhub.Client = (*Client)(nil)
var _ socialhub.Fetcher = (*Client)(nil)
var _ socialhub.Reactor = (*Client)(nil)
var _ socialhub.Messenger = (*Client)(nil)
