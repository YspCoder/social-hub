package flickr

import (
	"context"
	"net/http"
	"time"

	"social-hub/pkg/socialhub"
)

const (
	PermissionRead   = "read"
	PermissionWrite  = "write"
	PermissionDelete = "delete"

	CapabilityPhoto       socialhub.Capability = "photo"
	CapabilityPhotoUpload socialhub.Capability = "photo_upload"
	CapabilityAlbum       socialhub.Capability = "album"
)

// Client implements Flickr's common fetcher/reactor and typed workflows.
type Client struct {
	accountID      socialhub.AccountID
	apiKey         string
	consumerSecret string
	accessToken    string
	tokenSecret    string
	userID         string
	permission     string
	baseURL        string
	uploadURL      string
	public         *http.Client
	signed         *http.Client
	clock          socialhub.Clock
	upload         *PhotoUploadService
}

func (c *Client) Platform() socialhub.Platform { return "flickr" }
func (c *Client) Account() socialhub.AccountID { return c.accountID }

func (c *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	return socialhub.Capabilities{
		socialhub.CapFetch:    c.capability(socialhub.CapFetch, PermissionRead, true, "public users, photos, photostreams, and comments; authenticated tokens include visible private photos", documentationURL),
		CapabilityPhoto:       c.capability(CapabilityPhoto, PermissionRead, true, "typed photo reads, metadata updates, and delete-permission deletion", "https://www.flickr.com/services/api/flickr.photos.getInfo.html"),
		CapabilityPhotoUpload: c.capability(CapabilityPhotoUpload, PermissionWrite, true, "streaming Flickr Upload API for photos and videos", "https://www.flickr.com/services/api/upload.api.html"),
		CapabilityAlbum:       c.capability(CapabilityAlbum, PermissionRead, true, "photoset reads, creation, and membership", "https://www.flickr.com/services/api/flickr.photosets.getList.html"),
		socialhub.CapReact:    c.capability(socialhub.CapReact, PermissionWrite, true, "favorites and flat photo comments", "https://www.flickr.com/services/api/flickr.favorites.add.html"),
		socialhub.CapPublish:  {Capability: socialhub.CapPublish, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Flickr publication is a binary upload that creates the photo resource; use PhotoUploadWorkflow"},
		socialhub.CapMedia:    {Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Flickr does not create detached media IDs; use PhotoUploadWorkflow"},
		socialhub.CapMessage:  {Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Flickr Services API does not expose direct messaging"},
		socialhub.CapWebhook:  {Capability: socialhub.CapWebhook, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Flickr Push endpoints do not provide a current documented callback-signature contract for this adapter"},
	}, nil
}

func (c *Client) capability(capability socialhub.Capability, permission string, supported bool, reason, docURL string) socialhub.CapabilityState {
	approval := socialhub.ApprovalGranted
	if permission != PermissionRead || c.signed != nil {
		if c.signed == nil || !permissionAtLeast(c.permission, permission) {
			approval = socialhub.ApprovalRequired
		}
	}
	return socialhub.CapabilityState{Capability: capability, Supported: supported, Approval: approval, Scopes: []string{permission}, Reason: reason, DocURL: docURL}
}

func (c *Client) Publisher() (socialhub.Publisher, bool)         { return nil, false }
func (c *Client) Fetcher() (socialhub.Fetcher, bool)             { return c, true }
func (c *Client) MediaUploader() (socialhub.MediaUploader, bool) { return nil, false }
func (c *Client) Reactor() (socialhub.Reactor, bool) {
	return c, c.signed != nil && permissionAtLeast(c.permission, PermissionWrite)
}
func (c *Client) Messenger() (socialhub.Messenger, bool)           { return nil, false }
func (c *Client) WebhookHandler() (socialhub.WebhookHandler, bool) { return nil, false }
func (c *Client) Close() error                                     { return nil }

// PhotoWorkflow returns typed Flickr photo operations.
func (c *Client) PhotoWorkflow() PhotoWorkflow { return c }

// PhotoUploadWorkflow returns Flickr's direct binary upload operation.
func (c *Client) PhotoUploadWorkflow() PhotoUploadWorkflow { return c.upload }

// AlbumWorkflow returns Flickr photoset operations.
func (c *Client) AlbumWorkflow() AlbumWorkflow { return c }

func (c *Client) requirePermission(operation, required string) error {
	if c.signed != nil && permissionAtLeast(c.permission, required) {
		return nil
	}
	return &socialhub.Error{
		Code: socialhub.CodeApprovalRequired, Class: socialhub.ClassUserAction,
		Platform: "flickr", Product: productName, Op: operation,
		RequiredScopes: []string{required}, ApprovalURL: "https://www.flickr.com/services/apps/create/",
		PlatformMessage: "Flickr OAuth token and permission are not configured",
	}
}

func permissionAtLeast(granted, required string) bool {
	return permissionRank(granted) >= permissionRank(required) && permissionRank(required) > 0
}

func permissionRank(permission string) int {
	switch permission {
	case PermissionRead:
		return 1
	case PermissionWrite:
		return 2
	case PermissionDelete:
		return 3
	default:
		return 0
	}
}

func configuredPermission(scopes []string) string {
	if len(scopes) == 1 {
		return scopes[0]
	}
	return ""
}

func validPermissionScopes(scopes []string) bool {
	return len(scopes) == 1 && permissionRank(scopes[0]) > 0
}

func unixTime(value scalar) *time.Time {
	seconds, ok := value.Int64()
	if !ok || seconds <= 0 {
		return nil
	}
	parsed := time.Unix(seconds, 0).UTC()
	return &parsed
}

var _ socialhub.Client = (*Client)(nil)
var _ socialhub.Fetcher = (*Client)(nil)
var _ socialhub.Reactor = (*Client)(nil)
