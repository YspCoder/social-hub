package branch

import (
	"context"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	CapabilityEvents     socialhub.Capability = "branch_s2s_events"
	CapabilityIPOverride socialhub.Capability = "branch_ip_override"
)

// Client exposes Branch event ingestion for one configured app key. Organic
// social capabilities are intentionally unavailable.
type Client struct {
	accountID         socialhub.AccountID
	branchKey         string
	api               *transport.Client
	ipOverrideEnabled bool
}

func (client *Client) Platform() socialhub.Platform { return platformName }
func (client *Client) Account() socialhub.AccountID { return client.accountID }

func (client *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	ipApproval := socialhub.ApprovalRequired
	ipReason := "X-IP-Override requires Branch to allowlist the app ID"
	if client.ipOverrideEnabled {
		ipApproval = socialhub.ApprovalGranted
		ipReason = "the account records Branch X-IP-Override allowlisting as enabled"
	}
	return socialhub.Capabilities{
		CapabilityEvents: {
			Capability: CapabilityEvents, Supported: true, Approval: socialhub.ApprovalUnknown,
			Reason: "real-time standard and custom mobile attribution events",
			DocURL: documentationURL,
		},
		CapabilityIPOverride: {
			Capability: CapabilityIPOverride, Supported: true, Approval: ipApproval,
			Reason: ipReason, DocURL: documentationURL,
		},
		socialhub.CapPublish: {Capability: socialhub.CapPublish, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "attribution events are not organic posts"},
		socialhub.CapFetch:   {Capability: socialhub.CapFetch, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Events API v2 is an ingestion product"},
		socialhub.CapMedia:   {Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Events API v2 does not upload media"},
		socialhub.CapReact:   {Capability: socialhub.CapReact, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Events API v2 has no engagement surface"},
		socialhub.CapMessage: {Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Events API v2 has no messaging surface"},
		socialhub.CapWebhook: {Capability: socialhub.CapWebhook, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Branch outbound webhooks are a separate dashboard product"},
	}, nil
}

func (client *Client) Publisher() (socialhub.Publisher, bool)           { return nil, false }
func (client *Client) Fetcher() (socialhub.Fetcher, bool)               { return nil, false }
func (client *Client) MediaUploader() (socialhub.MediaUploader, bool)   { return nil, false }
func (client *Client) Reactor() (socialhub.Reactor, bool)               { return nil, false }
func (client *Client) Messenger() (socialhub.Messenger, bool)           { return nil, false }
func (client *Client) WebhookHandler() (socialhub.WebhookHandler, bool) { return nil, false }
func (client *Client) Close() error                                     { return nil }

func (client *Client) Events() EventWorkflow { return client }

func (client *Client) requireIPOverride(operation, value string) error {
	if value == "" || client.ipOverrideEnabled {
		return nil
	}
	return &socialhub.Error{
		Code: socialhub.CodeApprovalRequired, Class: socialhub.ClassUserAction,
		Platform: platformName, Product: productName, Op: operation,
		RequiredScopes: []string{string(CapabilityIPOverride)}, ApprovalURL: supportURL,
		PlatformMessage: "X-IP-Override is not recorded as allowlisted for this Branch app",
	}
}

var _ socialhub.Client = (*Client)(nil)
