package musicbrainz

import (
	"context"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const CapabilityCatalog socialhub.Capability = "musicbrainz_catalog"

// Client exposes typed MusicBrainz catalog workflows.
type Client struct {
	accountID socialhub.AccountID
	api       *transport.Client
	gate      *requestGate
}

func (c *Client) Platform() socialhub.Platform { return "musicbrainz" }
func (c *Client) Account() socialhub.AccountID { return c.accountID }

func (c *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	return socialhub.Capabilities{
		CapabilityCatalog: {
			Capability: CapabilityCatalog, Supported: true, Approval: socialhub.ApprovalGranted,
			Reason: "public Artist, Release Group, Release, Recording, and Work metadata; core and supplementary data have different licenses",
			DocURL: documentationURL,
		},
		socialhub.CapPublish: {Capability: socialhub.CapPublish, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "catalog edits are not social publishing"},
		socialhub.CapFetch:   {Capability: socialhub.CapFetch, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "music metadata is exposed through typed workflows"},
		socialhub.CapMedia:   {Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "MusicBrainz does not host media or cover art"},
		socialhub.CapReact:   {Capability: socialhub.CapReact, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "public WS/2 reactions are not available"},
		socialhub.CapMessage: {Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "MusicBrainz does not expose direct messaging"},
		socialhub.CapWebhook: {Capability: socialhub.CapWebhook, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "MusicBrainz does not document signed metadata webhooks"},
	}, nil
}

func (c *Client) Publisher() (socialhub.Publisher, bool)           { return nil, false }
func (c *Client) Fetcher() (socialhub.Fetcher, bool)               { return nil, false }
func (c *Client) MediaUploader() (socialhub.MediaUploader, bool)   { return nil, false }
func (c *Client) Reactor() (socialhub.Reactor, bool)               { return nil, false }
func (c *Client) Messenger() (socialhub.Messenger, bool)           { return nil, false }
func (c *Client) WebhookHandler() (socialhub.WebhookHandler, bool) { return nil, false }
func (c *Client) Close() error                                     { return nil }

func (c *Client) CatalogWorkflow() CatalogWorkflow { return c }

var _ socialhub.Client = (*Client)(nil)
