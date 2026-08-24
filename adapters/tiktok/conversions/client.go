package conversions

import (
	"context"
	"strings"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const CapabilityConversionEvents socialhub.Capability = "tiktok_conversion_events"

type Client struct {
	accountID     socialhub.AccountID
	eventSource   EventSource
	eventSourceID string
	api           *transport.Client
	permissions   []string
	clock         socialhub.Clock
}

func (client *Client) Platform() socialhub.Platform { return platformName }
func (client *Client) Account() socialhub.AccountID { return client.accountID }

func (client *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	approval := socialhub.ApprovalUnknown
	if len(client.permissions) > 0 {
		approval = socialhub.ApprovalGranted
		if !permissionGranted(client.permissions, conversionPermission) {
			approval = socialhub.ApprovalRequired
		}
	}
	reason := "Web, App, Offline, or CRM conversion batches for one configured TikTok event source"
	if client.eventSource == EventSourceApp {
		reason += "; App Events reporting is allowlist-only"
	}
	return socialhub.Capabilities{
		CapabilityConversionEvents: {
			Capability: CapabilityConversionEvents, Supported: true, Approval: approval,
			Scopes: []string{conversionPermission}, Reason: reason, DocURL: documentationURL,
		},
		socialhub.CapPublish: {Capability: socialhub.CapPublish, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "conversion telemetry is not organic TikTok content"},
		socialhub.CapFetch:   {Capability: socialhub.CapFetch, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Events API is an ingestion endpoint"},
		socialhub.CapMedia:   {Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Events API does not upload media"},
		socialhub.CapReact:   {Capability: socialhub.CapReact, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Events API has no engagement surface"},
		socialhub.CapMessage: {Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Events API has no messaging surface"},
		socialhub.CapWebhook: {Capability: socialhub.CapWebhook, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "event submission does not expose webhooks"},
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

func (client *Client) requirePermission(operation string) error {
	if len(client.permissions) == 0 || permissionGranted(client.permissions, conversionPermission) {
		return nil
	}
	return &socialhub.Error{
		Code: socialhub.CodeApprovalRequired, Class: socialhub.ClassUserAction,
		Platform: platformName, Product: productName, Op: operation,
		RequiredScopes: []string{conversionPermission}, ApprovalURL: approvalURL,
		PlatformMessage: "configured permissions do not authorize TikTok conversion events",
	}
}

func permissionGranted(permissions []string, target string) bool {
	for _, permission := range permissions {
		if strings.EqualFold(strings.TrimSpace(permission), target) {
			return true
		}
	}
	return false
}

var _ socialhub.Client = (*Client)(nil)
