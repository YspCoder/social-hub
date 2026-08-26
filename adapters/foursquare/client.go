package foursquare

import (
	"context"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const CapabilityPlaces socialhub.Capability = "foursquare_places"

// Client exposes typed, read-only place discovery for one service-key account.
type Client struct {
	accountID socialhub.AccountID
	api       *transport.Client
}

func (client *Client) Platform() socialhub.Platform { return platformName }
func (client *Client) Account() socialhub.AccountID { return client.accountID }

func (client *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	return socialhub.Capabilities{
		CapabilityPlaces: {
			Capability: CapabilityPlaces, Supported: true, Approval: socialhub.ApprovalGranted,
			Reason: "Pro-field place search and place details using a Foursquare service key", DocURL: documentationURL,
		},
		socialhub.CapPublish: {Capability: socialhub.CapPublish, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Foursquare Places API is a read-only place data product"},
		socialhub.CapFetch:   {Capability: socialhub.CapFetch, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "place entities are exposed through PlacesWorkflow, not social posts or timelines"},
		socialhub.CapMedia:   {Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "place photos are a Premium read endpoint and are outside this adapter"},
		socialhub.CapReact:   {Capability: socialhub.CapReact, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Places API does not expose portable social reactions"},
		socialhub.CapMessage: {Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Places API does not expose messaging"},
		socialhub.CapWebhook: {Capability: socialhub.CapWebhook, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "these Places API workflows are request/response based"},
	}, nil
}

func (client *Client) Publisher() (socialhub.Publisher, bool)           { return nil, false }
func (client *Client) Fetcher() (socialhub.Fetcher, bool)               { return nil, false }
func (client *Client) MediaUploader() (socialhub.MediaUploader, bool)   { return nil, false }
func (client *Client) Reactor() (socialhub.Reactor, bool)               { return nil, false }
func (client *Client) Messenger() (socialhub.Messenger, bool)           { return nil, false }
func (client *Client) WebhookHandler() (socialhub.WebhookHandler, bool) { return nil, false }
func (client *Client) Close() error                                     { return nil }

// Places returns Foursquare-specific place discovery operations.
func (client *Client) Places() PlacesWorkflow { return client }

var _ socialhub.Client = (*Client)(nil)
