package pixabay

import (
	"context"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	CapabilityImageSearch socialhub.Capability = "pixabay_image_search"
	CapabilityVideoSearch socialhub.Capability = "pixabay_video_search"
)

// Client exposes typed public Pixabay catalog search for one API key.
type Client struct {
	accountID socialhub.AccountID
	api       *transport.Client
	apiKey    string
}

func (client *Client) Platform() socialhub.Platform { return platformName }
func (client *Client) Account() socialhub.AccountID { return client.accountID }

func (client *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	return socialhub.Capabilities{
		CapabilityImageSearch: {
			Capability: CapabilityImageSearch, Supported: true, Approval: socialhub.ApprovalGranted,
			Reason: "public Pixabay image search with documented filters and contributor metadata", DocURL: documentationURL,
		},
		CapabilityVideoSearch: {
			Capability: CapabilityVideoSearch, Supported: true, Approval: socialhub.ApprovalGranted,
			Reason: "public Pixabay video search with documented renditions and contributor metadata", DocURL: documentationURL,
		},
		socialhub.CapPublish: {Capability: socialhub.CapPublish, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "uploads and writes are outside this public catalog adapter"},
		socialhub.CapFetch:   {Capability: socialhub.CapFetch, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Pixabay media uses a typed catalog workflow rather than social posts"},
		socialhub.CapMedia:   {Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "the adapter returns provider URLs but never uploads, downloads, or proxies media bytes"},
		socialhub.CapReact:   {Capability: socialhub.CapReact, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "likes and comments are outside the implemented read-only API surface"},
		socialhub.CapMessage: {Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Pixabay catalog search does not expose messaging"},
		socialhub.CapWebhook: {Capability: socialhub.CapWebhook, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "the implemented Pixabay endpoints are request/response searches"},
	}, nil
}

func (client *Client) Publisher() (socialhub.Publisher, bool)           { return nil, false }
func (client *Client) Fetcher() (socialhub.Fetcher, bool)               { return nil, false }
func (client *Client) MediaUploader() (socialhub.MediaUploader, bool)   { return nil, false }
func (client *Client) Reactor() (socialhub.Reactor, bool)               { return nil, false }
func (client *Client) Messenger() (socialhub.Messenger, bool)           { return nil, false }
func (client *Client) WebhookHandler() (socialhub.WebhookHandler, bool) { return nil, false }
func (client *Client) Close() error                                     { return nil }

// Catalog returns the bounded public Pixabay search workflow.
func (client *Client) Catalog() CatalogWorkflow { return client }

var _ socialhub.Client = (*Client)(nil)
