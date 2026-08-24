package micropub

import (
	"context"
	"net/http"
	"net/url"
	"strings"

	"social-hub/pkg/socialhub"
)

const (
	CapabilityEntries       socialhub.Capability = "micropub_entries"
	CapabilityQueries       socialhub.Capability = "micropub_queries"
	CapabilityMediaEndpoint socialhub.Capability = "micropub_media_endpoint"
)

// Client operates on one Micropub endpoint and bearer token.
type Client struct {
	accountID        socialhub.AccountID
	endpoint         *url.URL
	siteURL          string
	token            string
	httpClient       *http.Client
	scopes           []string
	supportsUpdate   bool
	supportsDelete   bool
	supportsUndelete bool
	clock            socialhub.Clock
}

func (client *Client) Platform() socialhub.Platform { return platformName }
func (client *Client) Account() socialhub.AccountID { return client.accountID }

func (client *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	return socialhub.Capabilities{
		socialhub.CapPublish: capabilityState(socialhub.CapPublish, true, client.scopes, []string{"create"}, "form-encoded h-entry creation; typed workflow adds JSON create and optional editing"),
		socialhub.CapFetch:   capabilityState(socialhub.CapFetch, client.supportsUpdate, client.scopes, []string{"update"}, "site identity and q=source reads when the endpoint supports editing"),
		socialhub.CapMedia: {
			Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown,
			Reason: "Micropub Media Endpoint is a single streaming upload, not the common resumable media contract", DocURL: documentationURL,
		},
		socialhub.CapReact:   {Capability: socialhub.CapReact, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "likes and reposts are new h-entry objects, not mutable reactions", DocURL: documentationURL},
		socialhub.CapMessage: {Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Micropub does not define direct messaging", DocURL: documentationURL},
		socialhub.CapWebhook: {Capability: socialhub.CapWebhook, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Micropub does not define inbound webhooks", DocURL: documentationURL},
		CapabilityEntries:    capabilityState(CapabilityEntries, true, client.scopes, []string{"create"}, "typed h-entry create plus explicitly configured update/delete/undelete"),
		CapabilityQueries: {
			Capability: CapabilityQueries, Supported: true, Approval: socialhub.ApprovalUnknown,
			Reason: "q=config and q=syndicate-to; q=source follows supports_update", DocURL: documentationURL,
		},
		CapabilityMediaEndpoint: capabilityState(CapabilityMediaEndpoint, true, client.scopes, []string{"create"}, "streaming upload to a q=config-discovered Media Endpoint"),
	}, nil
}

func capabilityState(capability socialhub.Capability, supported bool, granted, required []string, reason string) socialhub.CapabilityState {
	approval := socialhub.ApprovalUnknown
	if supported && len(granted) != 0 {
		approval = socialhub.ApprovalGranted
		for _, scope := range required {
			if !scopeGranted(granted, scope) {
				approval = socialhub.ApprovalRequired
				break
			}
		}
	}
	return socialhub.CapabilityState{
		Capability: capability, Supported: supported, Approval: approval,
		Scopes: append([]string(nil), required...), Reason: reason, DocURL: documentationURL,
	}
}

func scopeGranted(scopes []string, target string) bool {
	for _, scope := range scopes {
		if strings.EqualFold(strings.TrimSpace(scope), target) {
			return true
		}
	}
	return false
}

func (client *Client) requireScope(operation, scope string) error {
	if len(client.scopes) == 0 || scopeGranted(client.scopes, scope) {
		return nil
	}
	return &socialhub.Error{
		Code: socialhub.CodeApprovalRequired, Class: socialhub.ClassUserAction,
		Platform: platformName, Product: productName, Op: operation,
		RequiredScopes: []string{scope}, PlatformMessage: "configured token scopes do not include the required Micropub permission",
	}
}

func (client *Client) Publisher() (socialhub.Publisher, bool) { return client, true }

func (client *Client) Fetcher() (socialhub.Fetcher, bool) {
	if !client.supportsUpdate {
		return nil, false
	}
	return client, true
}

func (client *Client) MediaUploader() (socialhub.MediaUploader, bool) { return nil, false }
func (client *Client) Reactor() (socialhub.Reactor, bool)             { return nil, false }
func (client *Client) Messenger() (socialhub.Messenger, bool)         { return nil, false }
func (client *Client) WebhookHandler() (socialhub.WebhookHandler, bool) {
	return nil, false
}
func (client *Client) Close() error { return nil }

func (client *Client) EntryWorkflow() EntryWorkflow { return client }
func (client *Client) QueryWorkflow() QueryWorkflow { return client }
func (client *Client) MediaWorkflow() MediaWorkflow { return client }

var _ socialhub.Client = (*Client)(nil)
var _ socialhub.Publisher = (*Client)(nil)
var _ socialhub.Fetcher = (*Client)(nil)
var _ EntryWorkflow = (*Client)(nil)
var _ QueryWorkflow = (*Client)(nil)
var _ MediaWorkflow = (*Client)(nil)
