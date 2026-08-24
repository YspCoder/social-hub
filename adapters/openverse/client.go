package openverse

import (
	"context"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	CapabilityImageSearch socialhub.Capability = "openverse_image_search"
	CapabilityImageRead   socialhub.Capability = "openverse_image_read"
	CapabilityAudioSearch socialhub.Capability = "openverse_audio_search"
	CapabilityAudioRead   socialhub.Capability = "openverse_audio_read"
)

// Client exposes typed Openverse image and audio discovery for one logical
// account. The account may be anonymous or use an externally managed token.
type Client struct {
	accountID     socialhub.AccountID
	api           *transport.Client
	authenticated bool
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
		CapabilityImageSearch: readState(CapabilityImageSearch, "bounded Openverse image search with stable filters", documentationURL),
		CapabilityImageRead:   readState(CapabilityImageRead, "Openverse image metadata, source, creator, license, and attribution", documentationURL),
		CapabilityAudioSearch: readState(CapabilityAudioSearch, "bounded Openverse audio search with stable filters", documentationURL),
		CapabilityAudioRead:   readState(CapabilityAudioRead, "Openverse audio metadata, alternatives, source, creator, license, and attribution", documentationURL),
		socialhub.CapPublish:  {Capability: socialhub.CapPublish, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Openverse API v1 is a media discovery API, not a publishing API"},
		socialhub.CapFetch:    {Capability: socialhub.CapFetch, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Openverse media uses typed image and audio workflows rather than social posts"},
		socialhub.CapMedia:    {Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "the adapter returns media metadata and never uploads or downloads media bytes"},
		socialhub.CapReact:    {Capability: socialhub.CapReact, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Openverse does not expose reactions for discovered media"},
		socialhub.CapMessage:  {Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Openverse does not expose messaging"},
		socialhub.CapWebhook:  {Capability: socialhub.CapWebhook, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "the implemented Openverse workflows are request/response based"},
	}, nil
}

func (client *Client) Publisher() (socialhub.Publisher, bool)           { return nil, false }
func (client *Client) Fetcher() (socialhub.Fetcher, bool)               { return nil, false }
func (client *Client) MediaUploader() (socialhub.MediaUploader, bool)   { return nil, false }
func (client *Client) Reactor() (socialhub.Reactor, bool)               { return nil, false }
func (client *Client) Messenger() (socialhub.Messenger, bool)           { return nil, false }
func (client *Client) WebhookHandler() (socialhub.WebhookHandler, bool) { return nil, false }
func (client *Client) Close() error                                     { return nil }

// Images returns the bounded image discovery workflow.
func (client *Client) Images() ImagesWorkflow { return client }

// Audio returns the bounded audio discovery workflow.
func (client *Client) Audio() AudioWorkflow { return client }

var _ socialhub.Client = (*Client)(nil)
