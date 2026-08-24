package itunessearch

import (
	"context"
	"net/http"
	"sync"

	"social-hub/pkg/socialhub"
)

const (
	CapabilitySearch socialhub.Capability = "apple_itunes_catalog_search"
	CapabilityLookup socialhub.Capability = "apple_itunes_catalog_lookup"
)

// Client exposes public Store catalog searches and identifier lookups.
type Client struct {
	accountID  socialhub.AccountID
	httpClient *http.Client
	clock      socialhub.Clock

	mu     sync.RWMutex
	closed bool
}

func (client *Client) Platform() socialhub.Platform { return platformName }
func (client *Client) Account() socialhub.AccountID { return client.accountID }

func (client *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	readState := func(capability socialhub.Capability, reason string) socialhub.CapabilityState {
		return socialhub.CapabilityState{
			Capability: capability, Supported: true, Approval: socialhub.ApprovalGranted,
			Reason: reason, DocURL: documentationURL,
		}
	}
	return socialhub.Capabilities{
		CapabilitySearch:     readState(CapabilitySearch, "public Apple Store catalog search; returned promotional assets remain subject to Apple's usage terms"),
		CapabilityLookup:     readState(CapabilityLookup, "public Apple Store catalog lookup by Apple, AMG, UPC/EAN, or ISBN identifier"),
		socialhub.CapPublish: {Capability: socialhub.CapPublish, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "the iTunes Search API is read-only"},
		socialhub.CapFetch:   {Capability: socialhub.CapFetch, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "catalog resources use the typed Catalog workflow rather than social posts or feeds"},
		socialhub.CapMedia:   {Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "preview and artwork URLs are metadata; the adapter neither uploads nor downloads media"},
		socialhub.CapReact:   {Capability: socialhub.CapReact, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "the iTunes Search API does not expose reactions"},
		socialhub.CapMessage: {Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "the iTunes Search API does not expose messaging"},
		socialhub.CapWebhook: {Capability: socialhub.CapWebhook, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "the iTunes Search API does not expose webhooks"},
	}, nil
}

func (client *Client) Publisher() (socialhub.Publisher, bool)           { return nil, false }
func (client *Client) Fetcher() (socialhub.Fetcher, bool)               { return nil, false }
func (client *Client) MediaUploader() (socialhub.MediaUploader, bool)   { return nil, false }
func (client *Client) Reactor() (socialhub.Reactor, bool)               { return nil, false }
func (client *Client) Messenger() (socialhub.Messenger, bool)           { return nil, false }
func (client *Client) WebhookHandler() (socialhub.WebhookHandler, bool) { return nil, false }

func (client *Client) Catalog() CatalogWorkflow { return client }

func (client *Client) Close() error {
	client.mu.Lock()
	defer client.mu.Unlock()
	client.closed = true
	return nil
}

func (client *Client) dependencies(operation string) (*http.Client, socialhub.Clock, error) {
	client.mu.RLock()
	defer client.mu.RUnlock()
	if client.closed || client.httpClient == nil || client.clock == nil {
		return nil, nil, platformError(operation, socialhub.CodeConflict, socialhub.ClassPermanent, nil)
	}
	return client.httpClient, client.clock, nil
}

var _ socialhub.Client = (*Client)(nil)
