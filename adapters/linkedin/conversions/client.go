package conversions

import (
	"context"
	"strings"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const CapabilityConversionEvents socialhub.Capability = "linkedin_conversion_events"

type Client struct {
	accountID     socialhub.AccountID
	adAccountID   string
	conversionURN string
	api           *transport.Client
	scopes        []string
	clock         socialhub.Clock
}

func (client *Client) Platform() socialhub.Platform { return platformName }
func (client *Client) Account() socialhub.AccountID { return client.accountID }

func (client *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	approval := socialhub.ApprovalUnknown
	if len(client.scopes) > 0 {
		approval = socialhub.ApprovalGranted
		if !scopeGranted(client.scopes, writeScope) || !scopeGranted(client.scopes, readAdsScope) {
			approval = socialhub.ApprovalRequired
		}
	}
	return socialhub.Capabilities{
		CapabilityConversionEvents: {
			Capability: CapabilityConversionEvents, Supported: true, Approval: approval,
			Scopes: []string{writeScope, readAdsScope},
			Reason: "single and atomic batch conversion events for one configured LinkedIn Conversion Rule",
			DocURL: documentationURL,
		},
		socialhub.CapPublish: {Capability: socialhub.CapPublish, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "conversion telemetry is not organic LinkedIn content"},
		socialhub.CapFetch:   {Capability: socialhub.CapFetch, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "this adapter implements conversion event ingestion only"},
		socialhub.CapMedia:   {Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Conversions API does not upload media"},
		socialhub.CapReact:   {Capability: socialhub.CapReact, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Conversions API has no engagement surface"},
		socialhub.CapMessage: {Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Conversions API has no messaging surface"},
		socialhub.CapWebhook: {Capability: socialhub.CapWebhook, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "event ingestion does not expose webhooks"},
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

func (client *Client) requireScopes(operation string) error {
	if len(client.scopes) == 0 || scopeGranted(client.scopes, writeScope) && scopeGranted(client.scopes, readAdsScope) {
		return nil
	}
	return &socialhub.Error{
		Code: socialhub.CodeApprovalRequired, Class: socialhub.ClassUserAction,
		Platform: platformName, Product: productName, Op: operation,
		RequiredScopes: []string{writeScope, readAdsScope}, ApprovalURL: approvalURL,
		PlatformMessage: "configured scopes do not authorize LinkedIn conversion events",
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
