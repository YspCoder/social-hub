package amap

import (
	"context"
	"sync"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const CapabilityPlaces socialhub.Capability = "amap_places_v5"

// Client exposes typed, read-only Amap Place Search v5 operations.
type Client struct {
	mu        sync.RWMutex
	accountID socialhub.AccountID
	api       *transport.Client
	clock     socialhub.Clock
	secrets   []string
	closed    bool
}

func (client *Client) Platform() socialhub.Platform { return platformName }
func (client *Client) Account() socialhub.AccountID { return client.accountID }

func (client *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	client.mu.RLock()
	defer client.mu.RUnlock()
	if client.closed {
		return nil, platformError("capabilities", socialhub.CodeConflict, socialhub.ClassPermanent, nil)
	}
	return socialhub.Capabilities{
		CapabilityPlaces: {
			Capability: CapabilityPlaces, Supported: true, Approval: socialhub.ApprovalRequired,
			Reason: "Amap Web Service key, developer certification, quota, and applicable technical-service license required",
			DocURL: documentationURL,
		},
		socialhub.CapPublish: {Capability: socialhub.CapPublish, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Amap Place Search v5 is read-only"},
		socialhub.CapFetch:   {Capability: socialhub.CapFetch, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "places retain provider semantics through PlacesWorkflow"},
		socialhub.CapMedia:   {Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "place photos are optional read data, not uploadable media"},
		socialhub.CapReact:   {Capability: socialhub.CapReact, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Place Search v5 has no reaction surface"},
		socialhub.CapMessage: {Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Place Search v5 has no messaging surface"},
		socialhub.CapWebhook: {Capability: socialhub.CapWebhook, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "these place reads are request/response based"},
	}, nil
}

func (client *Client) Publisher() (socialhub.Publisher, bool)           { return nil, false }
func (client *Client) Fetcher() (socialhub.Fetcher, bool)               { return nil, false }
func (client *Client) MediaUploader() (socialhub.MediaUploader, bool)   { return nil, false }
func (client *Client) Reactor() (socialhub.Reactor, bool)               { return nil, false }
func (client *Client) Messenger() (socialhub.Messenger, bool)           { return nil, false }
func (client *Client) WebhookHandler() (socialhub.WebhookHandler, bool) { return nil, false }

func (client *Client) Close() error {
	client.mu.Lock()
	defer client.mu.Unlock()
	if client.closed {
		return nil
	}
	for index := range client.secrets {
		client.secrets[index] = ""
	}
	client.api = nil
	client.clock = nil
	client.secrets = nil
	client.closed = true
	return nil
}

// Places returns the Amap-specific place discovery workflow.
func (client *Client) Places() PlacesWorkflow { return client }

var _ socialhub.Client = (*Client)(nil)
