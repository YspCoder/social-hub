package whatsapp

import (
	"context"
	"strings"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	CapabilityTypedMessages   socialhub.Capability = "whatsapp_messages"
	CapabilityMedia           socialhub.Capability = "whatsapp_media"
	CapabilityBusinessProfile socialhub.Capability = "whatsapp_business_profile"
)

// Client implements WhatsApp messaging, media, profile, and webhook contracts.
type Client struct {
	accountID     socialhub.AccountID
	phoneNumberID string
	businessID    string
	transport     *transport.Client
	scopes        []string
	appSecret     string
	verifyToken   string
	clock         socialhub.Clock
}

func (c *Client) Platform() socialhub.Platform { return "whatsapp" }
func (c *Client) Account() socialhub.AccountID { return c.accountID }

func (c *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	webhookReason := "configure account.settings.app_secret_ref for X-Hub-Signature-256 verification"
	if c.appSecret != "" {
		webhookReason = "signed WhatsApp Business Account message and status webhooks"
	}
	return socialhub.Capabilities{
		socialhub.CapPublish:      {Capability: socialhub.CapPublish, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "WhatsApp exposes business conversations, not generic social posts"},
		socialhub.CapFetch:        {Capability: socialhub.CapFetch, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Cloud API does not expose contact profiles or arbitrary message history"},
		socialhub.CapMedia:        {Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "use typed MediaWorkflow for direct multipart upload and media metadata"},
		socialhub.CapReact:        {Capability: socialhub.CapReact, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "message reactions require a recipient and WhatsApp message ID"},
		socialhub.CapMessage:      capabilityState(socialhub.CapMessage, c.scopes, []string{"whatsapp_business_messaging"}, "text and reply messages; arbitrary lookup is unavailable", docURL+"reference/messages/"),
		socialhub.CapWebhook:      {Capability: socialhub.CapWebhook, Supported: c.appSecret != "", Approval: socialhub.ApprovalUnknown, Reason: webhookReason, DocURL: docURL + "webhooks/"},
		CapabilityTypedMessages:   capabilityState(CapabilityTypedMessages, c.scopes, []string{"whatsapp_business_messaging"}, "media, templates, reactions, and read receipts", docURL+"reference/messages/"),
		CapabilityMedia:           capabilityState(CapabilityMedia, c.scopes, []string{"whatsapp_business_messaging"}, "streaming media upload, metadata, and deletion", docURL+"reference/media/"),
		CapabilityBusinessProfile: capabilityState(CapabilityBusinessProfile, c.scopes, []string{"whatsapp_business_management"}, "business profile read and update", docURL+"business-profiles/"),
	}, nil
}

func capabilityState(capability socialhub.Capability, granted, required []string, reason, documentation string) socialhub.CapabilityState {
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
	return socialhub.CapabilityState{
		Capability: capability, Supported: true, Approval: approval,
		Scopes: append([]string(nil), required...), Reason: reason, DocURL: documentation,
	}
}

func (c *Client) Publisher() (socialhub.Publisher, bool)         { return nil, false }
func (c *Client) Fetcher() (socialhub.Fetcher, bool)             { return nil, false }
func (c *Client) MediaUploader() (socialhub.MediaUploader, bool) { return nil, false }
func (c *Client) Reactor() (socialhub.Reactor, bool)             { return nil, false }
func (c *Client) Messenger() (socialhub.Messenger, bool)         { return c, true }
func (c *Client) WebhookHandler() (socialhub.WebhookHandler, bool) {
	if c.appSecret == "" {
		return nil, false
	}
	return c, true
}
func (c *Client) Close() error { return nil }

func (c *Client) MessageWorkflow() MessageWorkflow                 { return c }
func (c *Client) MediaWorkflow() MediaWorkflow                     { return c }
func (c *Client) BusinessProfileWorkflow() BusinessProfileWorkflow { return c }

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
		Code: socialhub.CodeApprovalRequired, Class: socialhub.ClassUserAction, Platform: "whatsapp", Product: productName,
		Op: operation, RequiredScopes: missing, ApprovalURL: "https://developers.facebook.com/apps/",
		PlatformMessage: "configured approval scopes do not include required WhatsApp permissions",
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
var _ socialhub.Messenger = (*Client)(nil)
