package soundcloud

import (
	"context"
	"net/url"
	"strings"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	CapabilityTrackUpload  socialhub.Capability = "track_upload"
	CapabilityActivityFeed socialhub.Capability = "activity_feed"
)

// Client implements SoundCloud's supported common and typed capabilities.
type Client struct {
	accountID   socialhub.AccountID
	userURN     string
	accountType string
	api         *transport.Client
	apiBaseURL  *url.URL
	clock       socialhub.Clock
	tracks      *TrackUploadService
}

func (c *Client) Platform() socialhub.Platform { return "soundcloud" }
func (c *Client) Account() socialhub.AccountID { return c.accountID }

func (c *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	uploadApproval := socialhub.ApprovalUnknown
	if strings.EqualFold(c.accountType, "artist_pro") {
		uploadApproval = socialhub.ApprovalGranted
	}
	return socialhub.Capabilities{
		socialhub.CapFetch:     {Capability: socialhub.CapFetch, Supported: true, Approval: socialhub.ApprovalUnknown, Reason: "users, tracks, user tracks, and track comments", DocURL: docURL},
		socialhub.CapReact:     {Capability: socialhub.CapReact, Supported: true, Approval: socialhub.ApprovalUnknown, Reason: "track likes, repost creation, and comments; repost and comment deletion are unavailable", DocURL: docURL},
		CapabilityActivityFeed: {Capability: CapabilityActivityFeed, Supported: true, Approval: socialhub.ApprovalUnknown, Reason: "authenticated feed retaining track, playlist, and repost activity types", DocURL: docURL},
		CapabilityTrackUpload:  {Capability: CapabilityTrackUpload, Supported: true, Approval: uploadApproval, Reason: "streaming multipart track creation, metadata update, and deletion; app registration currently requires Artist Pro", DocURL: "https://developers.soundcloud.com/docs/api/guide#uploading-tracks"},
		socialhub.CapPublish:   {Capability: socialhub.CapPublish, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "a SoundCloud track is created together with its audio bytes; use TrackUploadWorkflow"},
		socialhub.CapMedia:     {Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "SoundCloud does not expose an independent audio media resource; use TrackUploadWorkflow"},
		socialhub.CapMessage:   {Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "SoundCloud Public API does not expose direct messaging"},
		socialhub.CapWebhook:   {Capability: socialhub.CapWebhook, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "SoundCloud Public API does not expose signed webhooks"},
	}, nil
}

func (c *Client) Publisher() (socialhub.Publisher, bool)           { return nil, false }
func (c *Client) Fetcher() (socialhub.Fetcher, bool)               { return c, true }
func (c *Client) MediaUploader() (socialhub.MediaUploader, bool)   { return nil, false }
func (c *Client) Reactor() (socialhub.Reactor, bool)               { return c, true }
func (c *Client) Messenger() (socialhub.Messenger, bool)           { return nil, false }
func (c *Client) WebhookHandler() (socialhub.WebhookHandler, bool) { return nil, false }
func (c *Client) Close() error                                     { return nil }

// TrackUploadWorkflow returns SoundCloud's typed track lifecycle.
func (c *Client) TrackUploadWorkflow() TrackUploadWorkflow { return c.tracks }

// ActivityWorkflow returns SoundCloud's typed authenticated feed reader.
func (c *Client) ActivityWorkflow() ActivityWorkflow { return c }

var _ socialhub.Client = (*Client)(nil)
var _ socialhub.Fetcher = (*Client)(nil)
var _ socialhub.Reactor = (*Client)(nil)
