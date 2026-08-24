package ximalaya

import (
	"context"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	CapabilityCategories socialhub.Capability = "ximalaya_categories"
	CapabilityAlbums     socialhub.Capability = "ximalaya_albums"
	CapabilityTracks     socialhub.Capability = "ximalaya_tracks"
	CapabilitySearch     socialhub.Capability = "ximalaya_content_search"
)

// Client exposes approved, server-signed Ximalaya content reads.
type Client struct {
	accountID  socialhub.AccountID
	api        *transport.Client
	clock      socialhub.Clock
	secrets    []string
	redactions []string
}

func (client *Client) Platform() socialhub.Platform { return platformName }
func (client *Client) Account() socialhub.AccountID { return client.accountID }

func (client *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	state := func(capability socialhub.Capability, reason, docURL string) socialhub.CapabilityState {
		return socialhub.CapabilityState{
			Capability: capability, Supported: true, Approval: socialhub.ApprovalRequired,
			Reason: reason, DocURL: docURL,
		}
	}
	return socialhub.Capabilities{
		CapabilityCategories: state(CapabilityCategories, "approved on-demand category discovery", "https://open.ximalaya.com/api-docs/document?id=6"),
		CapabilityAlbums:     state(CapabilityAlbums, "approved category album lists and album browsing", "https://open.ximalaya.com/api-docs/document?id=6"),
		CapabilityTracks:     state(CapabilityTracks, "approved album track metadata reads", "https://open.ximalaya.com/api-docs/document?id=6"),
		CapabilitySearch:     state(CapabilitySearch, "approved multi-condition album and track search", "https://open.ximalaya.com/api-docs/document?id=26"),
		socialhub.CapPublish: {Capability: socialhub.CapPublish, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "publishing is outside this read-only adapter"},
		socialhub.CapFetch:   {Capability: socialhub.CapFetch, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Ximalaya albums and tracks retain provider semantics through the typed workflow"},
		socialhub.CapMedia:   {Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "audio upload and server-side media caching are outside this adapter"},
		socialhub.CapReact:   {Capability: socialhub.CapReact, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "interaction writes are outside this adapter"},
		socialhub.CapMessage: {Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "messaging is outside this Open Platform content surface"},
		socialhub.CapWebhook: {Capability: socialhub.CapWebhook, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "push callbacks and data reporting are outside this read-only adapter"},
	}, nil
}

func (client *Client) Publisher() (socialhub.Publisher, bool)           { return nil, false }
func (client *Client) Fetcher() (socialhub.Fetcher, bool)               { return nil, false }
func (client *Client) MediaUploader() (socialhub.MediaUploader, bool)   { return nil, false }
func (client *Client) Reactor() (socialhub.Reactor, bool)               { return nil, false }
func (client *Client) Messenger() (socialhub.Messenger, bool)           { return nil, false }
func (client *Client) WebhookHandler() (socialhub.WebhookHandler, bool) { return nil, false }
func (client *Client) Close() error                                     { return nil }

// Ximalaya returns the bounded typed content workflow.
func (client *Client) Ximalaya() ReadWorkflow { return client }

var _ socialhub.Client = (*Client)(nil)
