package zalo

import (
	"context"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	CapabilityConsultationMessages socialhub.Capability = "zalo_consultation_messages"
	CapabilityOAProfile            socialhub.Capability = "zalo_oa_profile"
	CapabilityUserProfiles         socialhub.Capability = "zalo_user_profiles"
)

// Client implements Zalo OA messaging, profile, and webhook workflows for one
// configured Official Account.
type Client struct {
	accountID     socialhub.AccountID
	api           *transport.Client
	appID         string
	oaID          string
	webhookSecret string
}

func (c *Client) Platform() socialhub.Platform { return "zalo" }
func (c *Client) Account() socialhub.AccountID { return c.accountID }

func (c *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	webhookEnabled := c.appID != "" && c.webhookSecret != ""
	webhookReason := "configure app_id and webhook.secret_ref with the OA Secret Key"
	if webhookEnabled {
		webhookReason = "verifies X-ZEvent-Signature and decodes OA events"
	}
	return socialhub.Capabilities{
		socialhub.CapPublish:           {Capability: socialhub.CapPublish, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Zalo articles require a typed asynchronous article workflow with title, cover, and body blocks"},
		socialhub.CapFetch:             capability(socialhub.CapFetch, "reads authorized OA-scoped user profiles; posts, comments, and message history are unavailable through the common Fetcher"),
		socialhub.CapMedia:             {Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Zalo message attachments and Article video uploads use distinct product workflows"},
		socialhub.CapReact:             {Capability: socialhub.CapReact, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "OA OpenAPI does not expose generic post reactions through this adapter"},
		socialhub.CapMessage:           capability(socialhub.CapMessage, "sends v3 consultation text messages to eligible users within Zalo interaction and quota rules"),
		socialhub.CapWebhook:           {Capability: socialhub.CapWebhook, Supported: webhookEnabled, Approval: socialhub.ApprovalUnknown, Reason: webhookReason, DocURL: docURL},
		CapabilityConsultationMessages: capability(CapabilityConsultationMessages, "v3 consultation text messages with returned quota metadata"),
		CapabilityOAProfile:            capability(CapabilityOAProfile, "reads the linked Official Account profile and package metadata"),
		CapabilityUserProfiles:         capability(CapabilityUserProfiles, "reads OA-scoped user profiles after the management permission is granted"),
	}, nil
}

func capability(name socialhub.Capability, reason string) socialhub.CapabilityState {
	return socialhub.CapabilityState{
		Capability: name, Supported: true, Approval: socialhub.ApprovalUnknown,
		Reason: reason, DocURL: docURL,
	}
}

func (c *Client) Publisher() (socialhub.Publisher, bool)         { return nil, false }
func (c *Client) Fetcher() (socialhub.Fetcher, bool)             { return c, true }
func (c *Client) MediaUploader() (socialhub.MediaUploader, bool) { return nil, false }
func (c *Client) Reactor() (socialhub.Reactor, bool)             { return nil, false }
func (c *Client) Messenger() (socialhub.Messenger, bool)         { return c, true }
func (c *Client) WebhookHandler() (socialhub.WebhookHandler, bool) {
	if c.appID == "" || c.webhookSecret == "" {
		return nil, false
	}
	return c, true
}
func (c *Client) Close() error { return nil }

func (c *Client) MessageWorkflow() MessageWorkflow { return c }
func (c *Client) ProfileWorkflow() ProfileWorkflow { return c }

var _ socialhub.Client = (*Client)(nil)
var _ socialhub.Fetcher = (*Client)(nil)
var _ socialhub.Messenger = (*Client)(nil)
var _ socialhub.WebhookHandler = (*Client)(nil)
