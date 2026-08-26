package tidal

import (
	"context"
	"net/http"

	"social-hub/pkg/socialhub"
)

const (
	// CapabilityCatalogRead covers typed artist, album, and track metadata reads.
	CapabilityCatalogRead socialhub.Capability = "tidal_catalog_read"
	// CapabilityCatalogSearch covers typed catalog searches.
	CapabilityCatalogSearch socialhub.Capability = "tidal_catalog_search"
)

// Client exposes the supported TIDAL catalog reads for one externally managed token.
type Client struct {
	accountID   socialhub.AccountID
	httpClient  *http.Client
	accessToken string
	clock       socialhub.Clock
}

func (client *Client) Platform() socialhub.Platform { return platformName }
func (client *Client) Account() socialhub.AccountID { return client.accountID }

func (*Client) String() string   { return "tidal.Client(<redacted credentials>)" }
func (*Client) GoString() string { return "tidal.Client(<redacted credentials>)" }

func (client *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	return socialhub.Capabilities{
		CapabilityCatalogRead: {
			Capability: CapabilityCatalogRead, Supported: true, Approval: socialhub.ApprovalUnknown,
			Reason: "read-only artist, album, and track metadata through TIDAL API v2",
			DocURL: documentationURL,
		},
		CapabilityCatalogSearch: {
			Capability: CapabilityCatalogSearch, Supported: true, Approval: socialhub.ApprovalUnknown,
			Reason: "read-only catalog search through TIDAL API v2",
			DocURL: documentationURL,
		},
		socialhub.CapPublish: {Capability: socialhub.CapPublish, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "catalog writes are outside this adapter"},
		socialhub.CapFetch:   {Capability: socialhub.CapFetch, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "TIDAL catalog resources use the typed Catalog workflow rather than social posts"},
		socialhub.CapMedia:   {Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "playback, downloads, source files, DRM, and media uploads are excluded"},
		socialhub.CapReact:   {Capability: socialhub.CapReact, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "reactions and collection mutations are outside this adapter"},
		socialhub.CapMessage: {Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "TIDAL API v2 does not expose messaging through this adapter"},
		socialhub.CapWebhook: {Capability: socialhub.CapWebhook, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "the implemented catalog workflows are request/response based"},
	}, nil
}

func (client *Client) Publisher() (socialhub.Publisher, bool)           { return nil, false }
func (client *Client) Fetcher() (socialhub.Fetcher, bool)               { return nil, false }
func (client *Client) MediaUploader() (socialhub.MediaUploader, bool)   { return nil, false }
func (client *Client) Reactor() (socialhub.Reactor, bool)               { return nil, false }
func (client *Client) Messenger() (socialhub.Messenger, bool)           { return nil, false }
func (client *Client) WebhookHandler() (socialhub.WebhookHandler, bool) { return nil, false }
func (client *Client) Close() error                                     { return nil }

// Catalog returns the typed, read-only catalog workflow.
func (client *Client) Catalog() CatalogWorkflow { return client }

var _ socialhub.Client = (*Client)(nil)
