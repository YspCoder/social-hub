package matrix

import (
	"context"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	CapabilityEvents socialhub.Capability = "matrix_events"
	CapabilityMedia  socialhub.Capability = "matrix_media"
	CapabilitySync   socialhub.Capability = "matrix_sync"
)

// Client implements Matrix's common capabilities and typed workflows.
type Client struct {
	accountID     socialhub.AccountID
	homeserverURL string
	userID        string
	deviceID      string
	defaultRoomID string
	api           *transport.Client
	clock         socialhub.Clock
}

func (client *Client) Platform() socialhub.Platform { return "matrix" }
func (client *Client) Account() socialhub.AccountID { return client.accountID }

func (client *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	defaultRoom := client.defaultRoomID != ""
	return socialhub.Capabilities{
		socialhub.CapPublish: {Capability: socialhub.CapPublish, Supported: defaultRoom, Approval: socialhub.ApprovalUnknown, Reason: defaultRoomReason(defaultRoom, "text events can be published to the configured default room"), DocURL: eventDocURL},
		socialhub.CapFetch:   {Capability: socialhub.CapFetch, Supported: true, Approval: socialhub.ApprovalUnknown, Reason: "profiles, composite room-event lookup, default-room history, and thread replies", DocURL: messagesDocURL},
		socialhub.CapMedia:   {Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Matrix uses a single raw-body upload; use MediaWorkflow rather than the resumable common lifecycle"},
		socialhub.CapReact:   {Capability: socialhub.CapReact, Supported: true, Approval: socialhub.ApprovalUnknown, Reason: "like maps to an m.reaction thumbs-up annotation; comments map to m.thread replies; removal requires the reaction event ID", DocURL: relationsDocURL},
		socialhub.CapMessage: {Capability: socialhub.CapMessage, Supported: true, Approval: socialhub.ApprovalUnknown, Reason: "plain unencrypted room text messages and replies", DocURL: eventDocURL},
		socialhub.CapWebhook: {Capability: socialhub.CapWebhook, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Matrix clients receive events through /sync, not a signed webhook callback"},
		CapabilityEvents:     {Capability: CapabilityEvents, Supported: true, Approval: socialhub.ApprovalUnknown, Reason: "typed room text, media, reaction, lookup, redaction, and history operations", DocURL: eventDocURL},
		CapabilityMedia:      {Capability: CapabilityMedia, Supported: true, Approval: socialhub.ApprovalUnknown, Reason: "authenticated raw-body upload returning an mxc URI", DocURL: mediaDocURL},
		CapabilitySync:       {Capability: CapabilitySync, Supported: true, Approval: socialhub.ApprovalUnknown, Reason: "incremental client sync with joined-room timelines", DocURL: syncDocURL},
	}, nil
}

func defaultRoomReason(configured bool, supported string) string {
	if configured {
		return supported
	}
	return "configure account.settings.default_room_id for the common Publisher"
}

func (client *Client) Publisher() (socialhub.Publisher, bool) {
	if client.defaultRoomID == "" {
		return nil, false
	}
	return client, true
}
func (client *Client) Fetcher() (socialhub.Fetcher, bool)             { return client, true }
func (client *Client) MediaUploader() (socialhub.MediaUploader, bool) { return nil, false }
func (client *Client) Reactor() (socialhub.Reactor, bool)             { return client, true }
func (client *Client) Messenger() (socialhub.Messenger, bool)         { return client, true }
func (client *Client) WebhookHandler() (socialhub.WebhookHandler, bool) {
	return nil, false
}
func (client *Client) Close() error { return nil }

// EventWorkflow returns typed Matrix room event operations.
func (client *Client) EventWorkflow() EventWorkflow { return client }

// MediaWorkflow returns Matrix's raw media upload operation.
func (client *Client) MediaWorkflow() MediaWorkflow { return client }

// SyncWorkflow returns Matrix incremental sync.
func (client *Client) SyncWorkflow() SyncWorkflow { return client }

var _ socialhub.Client = (*Client)(nil)
var _ socialhub.Publisher = (*Client)(nil)
var _ socialhub.Fetcher = (*Client)(nil)
var _ socialhub.Reactor = (*Client)(nil)
var _ socialhub.Messenger = (*Client)(nil)
