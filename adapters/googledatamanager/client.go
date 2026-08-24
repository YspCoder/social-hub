package googledatamanager

import (
	"context"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const CapabilityEventIngestion socialhub.Capability = "event_ingestion"

type Client struct {
	accountID socialhub.AccountID
	api       *transport.Client
	scopes    []string
}

func (client *Client) Platform() socialhub.Platform { return platformName }
func (client *Client) Account() socialhub.AccountID { return client.accountID }

func (client *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	approval := socialhub.ApprovalUnknown
	if validOAuthScopes(client.scopes) {
		approval = socialhub.ApprovalGranted
	}
	return socialhub.Capabilities{
		CapabilityEventIngestion: {
			Capability: CapabilityEventIngestion, Supported: true, Approval: approval,
			Reason: "typed Data Manager API v1 event ingestion across Google Ads, Analytics, and Floodlight destinations",
			DocURL: documentationURL,
		},
		socialhub.CapPublish: {Capability: socialhub.CapPublish, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "conversion telemetry is not an ad or social post"},
		socialhub.CapFetch:   {Capability: socialhub.CapFetch, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "event ingestion does not fetch organic content"},
		socialhub.CapMedia:   {Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Data Manager event ingestion does not upload media"},
		socialhub.CapReact:   {Capability: socialhub.CapReact, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Data Manager has no engagement surface"},
		socialhub.CapMessage: {Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Data Manager has no messaging surface"},
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

func (client *Client) Events() EventIngestor { return client }

func (client *Client) requireScope(operation string) error {
	if len(client.scopes) == 0 || validOAuthScopes(client.scopes) {
		return nil
	}
	return &socialhub.Error{
		Code: socialhub.CodeApprovalRequired, Class: socialhub.ClassUserAction,
		Platform: platformName, Product: productName, Op: operation,
		RequiredScopes: []string{dataManagerScope}, ApprovalURL: approvalURL,
		PlatformMessage: "configured approval scopes do not authorize Data Manager ingestion",
	}
}

var _ socialhub.Client = (*Client)(nil)
