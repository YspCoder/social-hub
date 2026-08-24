package kochava

import (
	"context"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	CapabilityInstalls   socialhub.Capability = "kochava_s2s_installs"
	CapabilityEvents     socialhub.Capability = "kochava_s2s_events"
	CapabilityIDFAUpdate socialhub.Capability = "kochava_s2s_idfa_update"
)

// Client exposes Kochava S2S measurement for one configured App GUID.
type Client struct {
	accountID socialhub.AccountID
	appGUID   string
	apiKey    string
	appSecret string
	paid      bool
	api       *transport.Client
}

func (client *Client) Platform() socialhub.Platform { return platformName }
func (client *Client) Account() socialhub.AccountID { return client.accountID }

func (client *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	approval := socialhub.ApprovalRequired
	reason := "Kochava S2S measurement requires a paid account; set approval.account_type to paid after provisioning"
	if client.paid {
		approval = socialhub.ApprovalGranted
		reason = "paid Kochava S2S measurement account is recorded as provisioned"
	}
	return socialhub.Capabilities{
		CapabilityInstalls: {
			Capability: CapabilityInstalls, Supported: true, Approval: approval, Reason: reason,
			DocURL: "https://support.kochava.com/articles/server-to-server-integration/179-install-notification-setup",
		},
		CapabilityEvents: {
			Capability: CapabilityEvents, Supported: true, Approval: approval, Reason: reason,
			DocURL: "https://support.kochava.com/articles/server-to-server-integration/185-post-install-event-setup",
		},
		CapabilityIDFAUpdate: {
			Capability: CapabilityIDFAUpdate, Supported: true, Approval: approval, Reason: reason,
			DocURL: "https://support.kochava.com/articles/server-to-server-integration/185-post-install-event-setup",
		},
		socialhub.CapPublish: {Capability: socialhub.CapPublish, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "measurement events are not organic posts"},
		socialhub.CapFetch:   {Capability: socialhub.CapFetch, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "this adapter exposes ingestion, not Kochava reporting"},
		socialhub.CapMedia:   {Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Kochava S2S measurement does not upload media"},
		socialhub.CapReact:   {Capability: socialhub.CapReact, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Kochava S2S measurement has no engagement surface"},
		socialhub.CapMessage: {Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Kochava S2S measurement has no messaging surface"},
		socialhub.CapWebhook: {Capability: socialhub.CapWebhook, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Kochava postbacks are a separate product surface"},
	}, nil
}

func (client *Client) Publisher() (socialhub.Publisher, bool)           { return nil, false }
func (client *Client) Fetcher() (socialhub.Fetcher, bool)               { return nil, false }
func (client *Client) MediaUploader() (socialhub.MediaUploader, bool)   { return nil, false }
func (client *Client) Reactor() (socialhub.Reactor, bool)               { return nil, false }
func (client *Client) Messenger() (socialhub.Messenger, bool)           { return nil, false }
func (client *Client) WebhookHandler() (socialhub.WebhookHandler, bool) { return nil, false }
func (client *Client) Close() error                                     { return nil }

func (client *Client) S2S() S2SWorkflow { return client }

func (client *Client) requirePaid(operation string) error {
	if client.paid {
		return nil
	}
	return &socialhub.Error{
		Code: socialhub.CodeApprovalRequired, Class: socialhub.ClassUserAction,
		Platform: platformName, Product: productName, Op: operation,
		ApprovalURL:     documentationURL,
		PlatformMessage: "Kochava S2S measurement requires a paid account; set approval.account_type to paid only after provisioning",
	}
}

var _ socialhub.Client = (*Client)(nil)
