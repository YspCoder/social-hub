package googlephotos

import (
	"context"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	CapabilityAlbumRead     socialhub.Capability = "google_photos_app_created_album_read"
	CapabilityMediaItemRead socialhub.Capability = "google_photos_app_created_media_item_read"
	CapabilityMediaSearch   socialhub.Capability = "google_photos_app_created_media_search"
)

// Client exposes provider-native Google Photos reads for app-created data.
type Client struct {
	accountID socialhub.AccountID
	scopes    []string
	api       *transport.Client
	clock     socialhub.Clock
}

func (client *Client) Platform() socialhub.Platform { return platformName }
func (client *Client) Account() socialhub.AccountID { return client.accountID }

func (client *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	approval := socialhub.ApprovalUnknown
	if len(client.scopes) > 0 {
		approval = socialhub.ApprovalRequired
		if containsScope(client.scopes, ScopeReadAppCreatedData) {
			approval = socialhub.ApprovalGranted
		}
	}
	readState := func(capability socialhub.Capability, reason, docURL string) socialhub.CapabilityState {
		return socialhub.CapabilityState{
			Capability: capability, Supported: true, Approval: approval,
			Scopes: []string{ScopeReadAppCreatedData}, Reason: reason, DocURL: docURL,
		}
	}
	return socialhub.Capabilities{
		CapabilityAlbumRead:     readState(CapabilityAlbumRead, "list and retrieve albums created by the same OAuth client app", documentationURL+"/v1/albums"),
		CapabilityMediaItemRead: readState(CapabilityMediaItemRead, "list and retrieve media items created by the same OAuth client app", documentationURL+"/v1/mediaItems"),
		CapabilityMediaSearch:   readState(CapabilityMediaSearch, "search only media items and albums created by the same OAuth client app", documentationURL+"/v1/mediaItems/search"),
		socialhub.CapPublish:    {Capability: socialhub.CapPublish, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "album creation and the two-stage media upload flow are outside this read adapter"},
		socialhub.CapFetch:      {Capability: socialhub.CapFetch, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Google Photos entities retain provider semantics through typed app-created-data workflows"},
		socialhub.CapMedia:      {Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "upload bytes and mediaItems.batchCreate are outside this adapter"},
		socialhub.CapReact:      {Capability: socialhub.CapReact, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Google Photos Library API does not expose social reactions in this surface"},
		socialhub.CapMessage:    {Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "sharing scopes and methods were removed after March 31, 2025"},
		socialhub.CapWebhook:    {Capability: socialhub.CapWebhook, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Google Photos Library API does not document signed webhooks"},
	}, nil
}

func (client *Client) Publisher() (socialhub.Publisher, bool)           { return nil, false }
func (client *Client) Fetcher() (socialhub.Fetcher, bool)               { return nil, false }
func (client *Client) MediaUploader() (socialhub.MediaUploader, bool)   { return nil, false }
func (client *Client) Reactor() (socialhub.Reactor, bool)               { return nil, false }
func (client *Client) Messenger() (socialhub.Messenger, bool)           { return nil, false }
func (client *Client) WebhookHandler() (socialhub.WebhookHandler, bool) { return nil, false }
func (client *Client) Close() error                                     { return nil }

// GooglePhotos returns the complete provider-native read workflow.
func (client *Client) GooglePhotos() ReadWorkflow { return client }

var _ socialhub.Client = (*Client)(nil)
