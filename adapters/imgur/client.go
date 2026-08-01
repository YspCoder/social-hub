package imgur

import (
	"context"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	CapabilityImages  socialhub.Capability = "imgur_images"
	CapabilityAlbums  socialhub.Capability = "imgur_albums"
	CapabilityGallery socialhub.Capability = "imgur_gallery"
	CapabilityCredits socialhub.Capability = "imgur_credits"
)

// Client implements Imgur public reads, account operations, and typed workflows.
type Client struct {
	accountID socialhub.AccountID
	username  string
	public    *transport.Client
	user      *transport.Client
	clock     socialhub.Clock
}

func (client *Client) Platform() socialhub.Platform { return "imgur" }
func (client *Client) Account() socialhub.AccountID { return client.accountID }

func (client *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	hasUser := client.user != nil
	return socialhub.Capabilities{
		socialhub.CapPublish: {Capability: socialhub.CapPublish, Supported: hasUser, Approval: approval(hasUser), Reason: userReason(hasUser, "share one existing uploaded image to the public Gallery"), DocURL: documentationURL},
		socialhub.CapFetch:   {Capability: socialhub.CapFetch, Supported: true, Approval: socialhub.ApprovalGranted, Reason: "public profiles, image metadata, Gallery comments, and bearer account image pages", DocURL: documentationURL},
		socialhub.CapMedia:   {Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Imgur uses one multipart upload request; use ImageWorkflow rather than the resumable common lifecycle"},
		socialhub.CapReact:   {Capability: socialhub.CapReact, Supported: hasUser, Approval: approval(hasUser), Reason: userReason(hasUser, "Gallery up-votes, vote removal, comments, and comment deletion"), DocURL: documentationURL},
		socialhub.CapMessage: {Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Imgur API v3 does not expose a supported direct-message contract"},
		socialhub.CapWebhook: {Capability: socialhub.CapWebhook, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Imgur API v3 does not publish a signed webhook contract"},
		CapabilityImages:     {Capability: CapabilityImages, Supported: true, Approval: socialhub.ApprovalGranted, Reason: "public reads plus anonymous or bearer upload, update, and deletion", DocURL: documentationURL},
		CapabilityAlbums:     {Capability: CapabilityAlbums, Supported: true, Approval: socialhub.ApprovalGranted, Reason: "public reads plus anonymous or bearer album creation and management", DocURL: documentationURL},
		CapabilityGallery:    {Capability: CapabilityGallery, Supported: hasUser, Approval: approval(hasUser), Reason: userReason(hasUser, "Gallery sharing, removal, voting, and comments"), DocURL: documentationURL},
		CapabilityCredits:    {Capability: CapabilityCredits, Supported: true, Approval: socialhub.ApprovalGranted, Reason: "application and user credit counters from /credits", DocURL: documentationURL},
	}, nil
}

func approval(supported bool) socialhub.ApprovalState {
	if supported {
		return socialhub.ApprovalGranted
	}
	return socialhub.ApprovalUnknown
}

func userReason(configured bool, supported string) string {
	if configured {
		return supported
	}
	return "configure access_token_ref with an Imgur OAuth2 user token"
}

func (client *Client) Publisher() (socialhub.Publisher, bool) {
	if client.user == nil {
		return nil, false
	}
	return client, true
}
func (client *Client) Fetcher() (socialhub.Fetcher, bool) { return client, true }
func (client *Client) MediaUploader() (socialhub.MediaUploader, bool) {
	return nil, false
}
func (client *Client) Reactor() (socialhub.Reactor, bool) {
	if client.user == nil {
		return nil, false
	}
	return client, true
}
func (client *Client) Messenger() (socialhub.Messenger, bool) { return nil, false }
func (client *Client) WebhookHandler() (socialhub.WebhookHandler, bool) {
	return nil, false
}
func (client *Client) Close() error { return nil }

// ImageWorkflow returns Imgur image and upload operations.
func (client *Client) ImageWorkflow() ImageWorkflow { return client }

// AlbumWorkflow returns Imgur album operations.
func (client *Client) AlbumWorkflow() AlbumWorkflow { return client }

// GalleryWorkflow returns Imgur Gallery social operations.
func (client *Client) GalleryWorkflow() GalleryWorkflow { return client }

// CreditWorkflow returns Imgur credit inspection.
func (client *Client) CreditWorkflow() CreditWorkflow { return client }

func (client *Client) requireUser(operation string) (*transport.Client, error) {
	if client.user == nil {
		return nil, &socialhub.Error{
			Code: socialhub.CodeUnauthenticated, Class: socialhub.ClassUserAction,
			Platform: "imgur", Product: productName, Op: operation,
			PlatformMessage: "configure access_token_ref with an Imgur OAuth2 user token",
		}
	}
	return client.user, nil
}

func (client *Client) active() *transport.Client {
	if client.user != nil {
		return client.user
	}
	return client.public
}

var _ socialhub.Client = (*Client)(nil)
var _ socialhub.Publisher = (*Client)(nil)
var _ socialhub.Fetcher = (*Client)(nil)
var _ socialhub.Reactor = (*Client)(nil)
