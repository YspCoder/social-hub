package peertube

import (
	"context"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	// CapabilityVideoWorkflow identifies PeerTube's video lifecycle, including upload.
	CapabilityVideoWorkflow socialhub.Capability = "video_workflow"
	// CapabilityChannelWorkflow identifies PeerTube video-channel discovery.
	CapabilityChannelWorkflow socialhub.Capability = "channel_workflow"
)

// Client implements PeerTube's supported common and typed workflows.
type Client struct {
	accountID   socialhub.AccountID
	accountName string
	instanceURL string
	transport   *transport.Client
	roles       []string
	clock       socialhub.Clock
}

func (c *Client) Platform() socialhub.Platform { return "peertube" }
func (c *Client) Account() socialhub.AccountID { return c.accountID }

func (c *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	return socialhub.Capabilities{
		socialhub.CapFetch:        capabilityState(socialhub.CapFetch, c.roles, "profiles, global/account videos, and top-level comment threads", docURL),
		socialhub.CapReact:        capabilityState(socialhub.CapReact, c.roles, "video ratings and comment creation; typed deletion requires the video ID", docURL),
		CapabilityVideoWorkflow:   capabilityState(CapabilityVideoWorkflow, c.roles, "legacy streaming upload plus video update and deletion", docURL+"#tag/Video-Upload"),
		CapabilityChannelWorkflow: {Capability: CapabilityChannelWorkflow, Supported: true, Approval: socialhub.ApprovalGranted, Reason: "public video-channel discovery", DocURL: docURL + "#tag/Video-Channels"},
		socialhub.CapPublish:      {Capability: socialhub.CapPublish, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "video publication requires PeerTube-specific channel and privacy fields"},
		socialhub.CapMedia:        {Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "a PeerTube upload creates a video instead of an independent media object"},
		socialhub.CapMessage:      {Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "PeerTube REST API does not expose direct messages"},
		socialhub.CapWebhook:      {Capability: socialhub.CapWebhook, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "no public signed webhook contract is exposed for account clients"},
	}, nil
}

func capabilityState(capability socialhub.Capability, roles []string, reason, documentation string) socialhub.CapabilityState {
	approval := socialhub.ApprovalUnknown
	if len(roles) > 0 {
		approval = socialhub.ApprovalRequired
		if roleGranted(roles, "user") {
			approval = socialhub.ApprovalGranted
		}
	}
	return socialhub.CapabilityState{Capability: capability, Supported: true, Approval: approval, Scopes: []string{"user"}, Reason: reason, DocURL: documentation}
}

func roleGranted(granted []string, required string) bool {
	for _, role := range granted {
		if role == required || role == "admin" || (required == "user" && role == "moderator") {
			return true
		}
	}
	return false
}

func (c *Client) requireUser(operation string) error {
	if len(c.roles) == 0 || roleGranted(c.roles, "user") {
		return nil
	}
	return &socialhub.Error{
		Code: socialhub.CodeApprovalRequired, Class: socialhub.ClassUserAction, Platform: "peertube", Product: productName,
		Op: operation, RequiredScopes: []string{"user"}, ApprovalURL: c.instanceURL + "/my-account/applications",
		PlatformMessage: "the configured PeerTube role does not grant user operations",
	}
}

func (c *Client) Publisher() (socialhub.Publisher, bool)           { return nil, false }
func (c *Client) Fetcher() (socialhub.Fetcher, bool)               { return c, true }
func (c *Client) MediaUploader() (socialhub.MediaUploader, bool)   { return nil, false }
func (c *Client) Reactor() (socialhub.Reactor, bool)               { return c, true }
func (c *Client) Messenger() (socialhub.Messenger, bool)           { return nil, false }
func (c *Client) WebhookHandler() (socialhub.WebhookHandler, bool) { return nil, false }
func (c *Client) Close() error                                     { return nil }

func (c *Client) VideoWorkflow() VideoWorkflow     { return c }
func (c *Client) ChannelWorkflow() ChannelWorkflow { return c }
func (c *Client) CommentWorkflow() CommentWorkflow { return c }

var _ socialhub.Client = (*Client)(nil)
var _ socialhub.Fetcher = (*Client)(nil)
var _ socialhub.Reactor = (*Client)(nil)
var _ VideoWorkflow = (*Client)(nil)
var _ ChannelWorkflow = (*Client)(nil)
var _ CommentWorkflow = (*Client)(nil)
