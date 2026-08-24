package googlebooks

import (
	"context"
	"net/http"
	"sync"

	"social-hub/pkg/socialhub"
)

const (
	CapabilityVolumeSearch socialhub.Capability = "google_books_volume_search"
	CapabilityVolumeRead   socialhub.Capability = "google_books_volume_read"
)

// Client exposes public Google Books Volume reads for one API identity.
type Client struct {
	accountID  socialhub.AccountID
	httpClient *http.Client
	clock      socialhub.Clock

	mu          sync.RWMutex
	apiKey      string
	accessToken string
	closed      bool
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
		CapabilityVolumeSearch: readState(CapabilityVolumeSearch, "search the public Volume catalog with documented filters and offset pagination", volumeListURL),
		CapabilityVolumeRead:   readState(CapabilityVolumeRead, "retrieve public publication metadata for one Volume ID", volumeGetURL),
		socialhub.CapPublish:   {Capability: socialhub.CapPublish, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Google Books public Volume reads do not publish content"},
		socialhub.CapFetch:     {Capability: socialhub.CapFetch, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "book catalog resources use the typed Volumes workflow rather than social posts or feeds"},
		socialhub.CapMedia:     {Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "image links are metadata; this adapter does not upload or download media"},
		socialhub.CapReact:     {Capability: socialhub.CapReact, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "ratings and reviews are outside this public-read adapter"},
		socialhub.CapMessage:   {Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Google Books Volume resources do not provide messaging"},
		socialhub.CapWebhook:   {Capability: socialhub.CapWebhook, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "public Volume reads are request/response based"},
	}, nil
}

func (client *Client) Publisher() (socialhub.Publisher, bool)           { return nil, false }
func (client *Client) Fetcher() (socialhub.Fetcher, bool)               { return nil, false }
func (client *Client) MediaUploader() (socialhub.MediaUploader, bool)   { return nil, false }
func (client *Client) Reactor() (socialhub.Reactor, bool)               { return nil, false }
func (client *Client) Messenger() (socialhub.Messenger, bool)           { return nil, false }
func (client *Client) WebhookHandler() (socialhub.WebhookHandler, bool) { return nil, false }

func (client *Client) Volumes() VolumesWorkflow { return client }

func (client *Client) Close() error {
	client.mu.Lock()
	defer client.mu.Unlock()
	client.closed = true
	client.apiKey, client.accessToken = "", ""
	return nil
}

func (client *Client) credentials(operation string) (string, string, error) {
	client.mu.RLock()
	defer client.mu.RUnlock()
	if client.closed || client.apiKey == "" && client.accessToken == "" {
		return "", "", platformError(operation, socialhub.CodeConflict, socialhub.ClassPermanent, nil)
	}
	return client.apiKey, client.accessToken, nil
}

var _ socialhub.Client = (*Client)(nil)
