package conversions

import (
	"context"
	"strings"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const CapabilityConversionEvents socialhub.Capability = "facebook_conversion_events"

// Client exposes typed server-side event submission for one configured Pixel
// or dataset. Common organic social capabilities are intentionally unavailable.
type Client struct {
	accountID socialhub.AccountID
	pixelID   string
	api       *transport.Client
	scopes    []string
	clock     socialhub.Clock
}

func (client *Client) Platform() socialhub.Platform { return platformName }
func (client *Client) Account() socialhub.AccountID { return client.accountID }

func (client *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	approval := socialhub.ApprovalUnknown
	if len(client.scopes) > 0 {
		approval = socialhub.ApprovalRequired
		if scopeGranted(client.scopes, managementScope) {
			approval = socialhub.ApprovalGranted
		}
	}
	return socialhub.Capabilities{
		CapabilityConversionEvents: {
			Capability: CapabilityConversionEvents, Supported: true, Approval: approval,
			Scopes: []string{managementScope},
			Reason: "server-side Web, App, offline, and business-messaging conversion events with local PII normalization and hashing",
			DocURL: documentationURL,
		},
		socialhub.CapPublish: {Capability: socialhub.CapPublish, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "conversion events are paid-media telemetry, not organic posts"},
		socialhub.CapFetch:   {Capability: socialhub.CapFetch, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Conversions API is an event-ingestion product"},
		socialhub.CapMedia:   {Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Conversions API does not upload media"},
		socialhub.CapReact:   {Capability: socialhub.CapReact, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Conversions API has no organic engagement surface"},
		socialhub.CapMessage: {Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "business messaging is an action source, not a send-message API"},
		socialhub.CapWebhook: {Capability: socialhub.CapWebhook, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Conversions API event submission does not expose webhooks"},
	}, nil
}

func (client *Client) Publisher() (socialhub.Publisher, bool)           { return nil, false }
func (client *Client) Fetcher() (socialhub.Fetcher, bool)               { return nil, false }
func (client *Client) MediaUploader() (socialhub.MediaUploader, bool)   { return nil, false }
func (client *Client) Reactor() (socialhub.Reactor, bool)               { return nil, false }
func (client *Client) Messenger() (socialhub.Messenger, bool)           { return nil, false }
func (client *Client) WebhookHandler() (socialhub.WebhookHandler, bool) { return nil, false }
func (client *Client) Close() error                                     { return nil }

func (client *Client) Conversions() ConversionWorkflow { return client }

func (client *Client) requireScope(operation string) error {
	if len(client.scopes) == 0 || scopeGranted(client.scopes, managementScope) {
		return nil
	}
	return &socialhub.Error{
		Code: socialhub.CodeApprovalRequired, Class: socialhub.ClassUserAction,
		Platform: platformName, Product: productName, Op: operation,
		RequiredScopes: []string{managementScope}, ApprovalURL: documentationURL,
		PlatformMessage: "configured approval scopes do not authorize Conversions API event submission",
	}
}

func scopeGranted(scopes []string, target string) bool {
	for _, scope := range scopes {
		if strings.TrimSpace(scope) == target {
			return true
		}
	}
	return false
}

var _ socialhub.Client = (*Client)(nil)
