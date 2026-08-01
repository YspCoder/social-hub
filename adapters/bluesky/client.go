package bluesky

import (
	"context"
	"sync"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	// CapabilityHomeTimeline identifies Bluesky's authenticated home timeline.
	CapabilityHomeTimeline socialhub.Capability = "home_timeline"
	// CapabilityPostRecord identifies the typed Bluesky post-record workflow.
	CapabilityPostRecord socialhub.Capability = "post_record"
)

// TimelineWorkflow exposes Bluesky's authenticated home timeline.
type TimelineWorkflow interface {
	Home(context.Context, TimelineRequest, ...socialhub.CallOption) (socialhub.Page[socialhub.Post], error)
}

// PostRecordWorkflow exposes Bluesky fields that the common post request does
// not carry, including languages, alt text, aspect ratio, and record keys.
type PostRecordWorkflow interface {
	CreateRecord(context.Context, PostRecordRequest, ...socialhub.CallOption) (*socialhub.Post, error)
}

// Client implements Bluesky's common and typed capabilities for one repo.
type Client struct {
	accountID  socialhub.AccountID
	serviceURL string
	repo       string
	transport  *transport.Client
	clock      socialhub.Clock
	uploadMu   sync.Mutex
	uploads    map[string]*uploadSession
	blobs      map[string]blobRef
}

func (c *Client) Platform() socialhub.Platform { return "bluesky" }
func (c *Client) Account() socialhub.AccountID { return c.accountID }

func (c *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	return socialhub.Capabilities{
		socialhub.CapPublish:   capabilityState(socialhub.CapPublish, "post, reply, quote, and deletion through repository records", docURL+"tutorials/creating-a-post"),
		socialhub.CapFetch:     capabilityState(socialhub.CapFetch, "profiles, posts, author feeds, and thread replies", docURL+"category/tutorials"),
		socialhub.CapMedia:     capabilityState(socialhub.CapMedia, "single-part image and simple MP4 blob uploads", docURL+"tutorials/video"),
		socialhub.CapReact:     capabilityState(socialhub.CapReact, "Like and Repost repository records", docURL+"tutorials/like-repost"),
		CapabilityHomeTimeline: capabilityState(CapabilityHomeTimeline, "authenticated reverse-chronological timeline", docURL+"tutorials/viewing-feeds"),
		CapabilityPostRecord:   capabilityState(CapabilityPostRecord, "typed languages, alt text, aspect ratio, and deterministic record key", docURL+"advanced-guides/posts"),
		socialhub.CapMessage: {
			Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown,
			Reason: "Bluesky chat is a separately authorized proxied service and is not part of the public repository contract",
		},
		socialhub.CapWebhook: {
			Capability: socialhub.CapWebhook, Supported: false, Approval: socialhub.ApprovalUnknown,
			Reason: "AT Protocol exposes repository event streams rather than signed HTTP webhooks",
		},
	}, nil
}

func capabilityState(capability socialhub.Capability, reason, documentation string) socialhub.CapabilityState {
	return socialhub.CapabilityState{
		Capability: capability, Supported: true, Approval: socialhub.ApprovalUnknown,
		Reason: reason, DocURL: documentation,
	}
}

func (c *Client) Publisher() (socialhub.Publisher, bool)           { return c, true }
func (c *Client) Fetcher() (socialhub.Fetcher, bool)               { return c, true }
func (c *Client) MediaUploader() (socialhub.MediaUploader, bool)   { return c, true }
func (c *Client) Reactor() (socialhub.Reactor, bool)               { return c, true }
func (c *Client) Messenger() (socialhub.Messenger, bool)           { return nil, false }
func (c *Client) WebhookHandler() (socialhub.WebhookHandler, bool) { return nil, false }
func (c *Client) Close() error                                     { return nil }

func (c *Client) TimelineWorkflow() TimelineWorkflow     { return c }
func (c *Client) PostRecordWorkflow() PostRecordWorkflow { return c }

var _ socialhub.Client = (*Client)(nil)
var _ socialhub.Publisher = (*Client)(nil)
var _ socialhub.Fetcher = (*Client)(nil)
var _ socialhub.MediaUploader = (*Client)(nil)
var _ socialhub.Reactor = (*Client)(nil)
var _ TimelineWorkflow = (*Client)(nil)
var _ PostRecordWorkflow = (*Client)(nil)
