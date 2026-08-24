package bilibililive

import (
	"context"
	"log/slog"
	"net/http"
	"sync"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	// CapabilityProjectLifecycle covers start, end, and project heartbeats.
	CapabilityProjectLifecycle socialhub.Capability = "bilibili_live_project_lifecycle"
	// CapabilityMessageStream covers authenticated WebSocket live-room events.
	CapabilityMessageStream socialhub.Capability = "bilibili_live_message_stream"
)

// ProjectLifecycle is Bilibili's explicit live project session contract.
type ProjectLifecycle interface {
	StartProject(context.Context, string, ...socialhub.CallOption) (*ProjectSession, error)
	EndProject(context.Context, string, ...socialhub.CallOption) error
	Heartbeat(context.Context, string, ...socialhub.CallOption) error
	BatchHeartbeat(context.Context, []string, ...socialhub.CallOption) (*BatchHeartbeatResult, error)
}

// LifecycleProvider exposes the Bilibili-specific project lifecycle.
type LifecycleProvider interface {
	ProjectLifecycle() ProjectLifecycle
}

// MessageSource opens authenticated live-room message streams.
type MessageSource interface {
	ConnectMessages(context.Context, WebSocketInfo, ...StreamOption) (*MessageStream, error)
}

// MessageSourceProvider exposes Bilibili's WebSocket event source.
type MessageSourceProvider interface {
	LiveMessages() MessageSource
}

// Client represents one approved Bilibili Live Open Platform application.
type Client struct {
	accountID  socialhub.AccountID
	appID      int64
	api        *transport.Client
	httpClient *http.Client
	logger     *slog.Logger
	clock      socialhub.Clock

	mu      sync.Mutex
	streams map[*MessageStream]struct{}
	closed  bool
}

func (client *Client) Platform() socialhub.Platform { return "bilibili" }
func (client *Client) Account() socialhub.AccountID { return client.accountID }

func (client *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	return socialhub.Capabilities{
		socialhub.CapPublish: {
			Capability: socialhub.CapPublish, Supported: false, Approval: socialhub.ApprovalUnknown,
			Reason: "Live Open Platform manages interactive project sessions, not social post publication",
		},
		socialhub.CapFetch: {
			Capability: socialhub.CapFetch, Supported: false, Approval: socialhub.ApprovalUnknown,
			Reason: "anchor and room data are session-scoped results rather than common post or user lookup",
		},
		socialhub.CapMedia: {
			Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown,
			Reason: "Live Open Platform does not expose general media upload through this product",
		},
		socialhub.CapReact: {
			Capability: socialhub.CapReact, Supported: false, Approval: socialhub.ApprovalUnknown,
			Reason: "reactions are inbound live events and cannot be mutated through this product",
		},
		socialhub.CapMessage: {
			Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown,
			Reason: "the live message stream is inbound-only and does not satisfy the common Messenger contract",
		},
		socialhub.CapWebhook: {
			Capability: socialhub.CapWebhook, Supported: false, Approval: socialhub.ApprovalUnknown,
			Reason: "live events use an authenticated binary WebSocket protocol rather than HTTP callbacks",
		},
		CapabilityProjectLifecycle: {
			Capability: CapabilityProjectLifecycle, Supported: true, Approval: socialhub.ApprovalUnknown,
			Reason: "v2 project start/end and single or batch project heartbeats; application approval and room allowlists are enforced by Bilibili",
			DocURL: documentationURL,
		},
		CapabilityMessageStream: {
			Capability: CapabilityMessageStream, Supported: true, Approval: socialhub.ApprovalUnknown,
			Reason: "authenticated binary WebSocket events with zlib decoding, heartbeat, cluster failover, and reconnect",
			DocURL: "https://open-live.bilibili.com/document/657d8e34-f926-a133-16c0-300c1afc6e6b",
		},
	}, nil
}

func (client *Client) Publisher() (socialhub.Publisher, bool)         { return nil, false }
func (client *Client) Fetcher() (socialhub.Fetcher, bool)             { return nil, false }
func (client *Client) MediaUploader() (socialhub.MediaUploader, bool) { return nil, false }
func (client *Client) Reactor() (socialhub.Reactor, bool)             { return nil, false }
func (client *Client) Messenger() (socialhub.Messenger, bool)         { return nil, false }
func (client *Client) WebhookHandler() (socialhub.WebhookHandler, bool) {
	return nil, false
}

func (client *Client) ProjectLifecycle() ProjectLifecycle { return client }
func (client *Client) LiveMessages() MessageSource        { return client }

func (client *Client) Close() error {
	client.mu.Lock()
	if client.closed {
		client.mu.Unlock()
		return nil
	}
	client.closed = true
	streams := make([]*MessageStream, 0, len(client.streams))
	for stream := range client.streams {
		streams = append(streams, stream)
	}
	client.mu.Unlock()

	for _, stream := range streams {
		_ = stream.Close()
	}
	return nil
}

func (client *Client) addStream(stream *MessageStream) error {
	client.mu.Lock()
	defer client.mu.Unlock()
	if client.closed {
		return platformError("connect_messages", socialhub.CodeConflict, socialhub.ClassPermanent, nil)
	}
	client.streams[stream] = struct{}{}
	return nil
}

func (client *Client) removeStream(stream *MessageStream) {
	client.mu.Lock()
	delete(client.streams, stream)
	client.mu.Unlock()
}

func (client *Client) ensureOpen(operation string) error {
	client.mu.Lock()
	defer client.mu.Unlock()
	if client.closed {
		return platformError(operation, socialhub.CodeConflict, socialhub.ClassPermanent, nil)
	}
	return nil
}

var _ socialhub.Client = (*Client)(nil)
var _ LifecycleProvider = (*Client)(nil)
var _ MessageSourceProvider = (*Client)(nil)
