package unsplash

import (
	"context"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	CapabilityPhotoSearch      socialhub.Capability = "unsplash_photo_search"
	CapabilityPhotoRead        socialhub.Capability = "unsplash_photo_read"
	CapabilityUserRead         socialhub.Capability = "unsplash_user_read"
	CapabilityCollectionRead   socialhub.Capability = "unsplash_collection_read"
	CapabilityDownloadTracking socialhub.Capability = "unsplash_download_tracking"
)

// Client exposes public, typed Unsplash reads for one application access key.
type Client struct {
	accountID socialhub.AccountID
	api       *transport.Client
	accessKey string
}

func (client *Client) Platform() socialhub.Platform { return platformName }
func (client *Client) Account() socialhub.AccountID { return client.accountID }

func (client *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	readState := func(capability socialhub.Capability, reason, docURL string) socialhub.CapabilityState {
		return socialhub.CapabilityState{
			Capability: capability, Supported: true, Approval: socialhub.ApprovalGranted,
			Reason: reason, DocURL: docURL,
		}
	}
	return socialhub.Capabilities{
		CapabilityPhotoSearch:      readState(CapabilityPhotoSearch, "public photo search with documented filters", "https://unsplash.com/documentation#search-photos"),
		CapabilityPhotoRead:        readState(CapabilityPhotoRead, "public photo details and hotlink image URLs", "https://unsplash.com/documentation#get-a-photo"),
		CapabilityUserRead:         readState(CapabilityUserRead, "public user profiles, photos, and collections", "https://unsplash.com/documentation#users"),
		CapabilityCollectionRead:   readState(CapabilityCollectionRead, "public collections and collection photos", "https://unsplash.com/documentation#collections"),
		CapabilityDownloadTracking: readState(CapabilityDownloadTracking, "download-like event tracking; no image bytes are downloaded", "https://unsplash.com/documentation#track-a-photo-download"),
		socialhub.CapPublish:       {Capability: socialhub.CapPublish, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "photo uploads and writes require user OAuth and are outside this adapter"},
		socialhub.CapFetch:         {Capability: socialhub.CapFetch, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Unsplash assets use typed photo, user, and collection workflows rather than social posts"},
		socialhub.CapMedia:         {Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "the adapter returns provider hotlink URLs and never uploads, downloads, or proxies image bytes"},
		socialhub.CapReact:         {Capability: socialhub.CapReact, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "likes require user OAuth and are outside this public-read adapter"},
		socialhub.CapMessage:       {Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Unsplash API v1 does not expose messaging"},
		socialhub.CapWebhook:       {Capability: socialhub.CapWebhook, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "the implemented Unsplash workflows are request/response based"},
	}, nil
}

func (client *Client) Publisher() (socialhub.Publisher, bool)           { return nil, false }
func (client *Client) Fetcher() (socialhub.Fetcher, bool)               { return nil, false }
func (client *Client) MediaUploader() (socialhub.MediaUploader, bool)   { return nil, false }
func (client *Client) Reactor() (socialhub.Reactor, bool)               { return nil, false }
func (client *Client) Messenger() (socialhub.Messenger, bool)           { return nil, false }
func (client *Client) WebhookHandler() (socialhub.WebhookHandler, bool) { return nil, false }
func (client *Client) Close() error                                     { return nil }

func (client *Client) Photos() PhotosWorkflow           { return client }
func (client *Client) Users() UsersWorkflow             { return client }
func (client *Client) Collections() CollectionsWorkflow { return client }

var _ socialhub.Client = (*Client)(nil)
