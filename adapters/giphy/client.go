package giphy

import (
	"context"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	CapabilityDiscovery socialhub.Capability = "giphy_discovery"
	CapabilityUpload    socialhub.Capability = "giphy_upload"
	CapabilityAnalytics socialhub.Capability = "giphy_analytics"
)

// Client implements GIPHY's typed media discovery, upload, and analytics workflows.
type Client struct {
	accountID       socialhub.AccountID
	api             *transport.Client
	upload          *transport.Client
	analytics       *transport.Client
	analyticsOrigin string
	clock           socialhub.Clock
}

func (client *Client) Platform() socialhub.Platform { return "giphy" }
func (client *Client) Account() socialhub.AccountID { return client.accountID }

func (client *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	return socialhub.Capabilities{
		CapabilityDiscovery:  {Capability: CapabilityDiscovery, Supported: true, Approval: socialhub.ApprovalGranted, Reason: "typed GIF/Sticker search, trending, translation, random, lookup, categories, terms, and Random ID; Search and Trending must be called client-side", DocURL: documentationURL},
		CapabilityUpload:     {Capability: CapabilityUpload, Supported: true, Approval: socialhub.ApprovalGranted, Reason: "typed streaming GIF/video upload; beta keys are quota-limited and channel usernames require production approval", DocURL: documentationURL},
		CapabilityAnalytics:  {Capability: CapabilityAnalytics, Supported: true, Approval: socialhub.ApprovalGranted, Reason: "validated analytics pingbacks from response-provided tracking URLs", DocURL: documentationURL},
		socialhub.CapPublish: {Capability: socialhub.CapPublish, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "GIPHY upload creates media but does not expose a platform-neutral social post contract"},
		socialhub.CapFetch:   {Capability: socialhub.CapFetch, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "use DiscoveryWorkflow; Search and Trending are media discovery, not user post timelines"},
		socialhub.CapMedia:   {Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "GIPHY uses one multipart upload request; use UploadWorkflow instead of the common resumable lifecycle"},
		socialhub.CapReact:   {Capability: socialhub.CapReact, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "GIPHY API does not expose likes or comments; use AnalyticsWorkflow for display/click/send events"},
		socialhub.CapMessage: {Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "GIPHY API does not expose direct messaging"},
		socialhub.CapWebhook: {Capability: socialhub.CapWebhook, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "GIPHY API does not publish a signed webhook contract"},
	}, nil
}

func (client *Client) Publisher() (socialhub.Publisher, bool)           { return nil, false }
func (client *Client) Fetcher() (socialhub.Fetcher, bool)               { return nil, false }
func (client *Client) MediaUploader() (socialhub.MediaUploader, bool)   { return nil, false }
func (client *Client) Reactor() (socialhub.Reactor, bool)               { return nil, false }
func (client *Client) Messenger() (socialhub.Messenger, bool)           { return nil, false }
func (client *Client) WebhookHandler() (socialhub.WebhookHandler, bool) { return nil, false }
func (client *Client) Close() error                                     { return nil }

// DiscoveryWorkflow returns GIPHY search and catalog operations.
func (client *Client) DiscoveryWorkflow() DiscoveryWorkflow { return client }

// UploadWorkflow returns GIPHY's single-request streaming upload operation.
func (client *Client) UploadWorkflow() UploadWorkflow { return client }

// AnalyticsWorkflow returns GIPHY action registration.
func (client *Client) AnalyticsWorkflow() AnalyticsWorkflow { return client }

var _ socialhub.Client = (*Client)(nil)
