package misskey

import (
	"context"
	"sync"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	CapabilityHomeTimeline socialhub.Capability = "home_timeline"
	CapabilityMiAuth       socialhub.Capability = "miauth"
)

// Client implements Misskey's supported common and typed capabilities.
type Client struct {
	accountID       socialhub.AccountID
	instanceURL     string
	userID          string
	defaultReaction string
	api             *transport.Client
	permissions     []string
	clock           socialhub.Clock
	uploadMu        sync.Mutex
	uploads         map[string]*uploadSession
}

func (c *Client) Platform() socialhub.Platform { return "misskey" }
func (c *Client) Account() socialhub.AccountID { return c.accountID }

func (c *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	return socialhub.Capabilities{
		socialhub.CapPublish:   capabilityState(socialhub.CapPublish, c.permissions, []string{"write:notes"}, "notes, replies, quotes, Renotes, CW, and visibility", docURL),
		socialhub.CapFetch:     capabilityState(socialhub.CapFetch, c.permissions, nil, "users, notes, replies, and user timelines", docURL),
		socialhub.CapMedia:     capabilityState(socialhub.CapMedia, c.permissions, []string{"write:drive", "read:drive"}, "single-part uploads to the instance Drive", docURL),
		socialhub.CapReact:     capabilityState(socialhub.CapReact, c.permissions, []string{"write:reactions", "write:notes"}, "emoji reactions and typed Renotes", docURL),
		CapabilityHomeTimeline: capabilityState(CapabilityHomeTimeline, c.permissions, []string{"read:account"}, "authenticated home timeline", docURL),
		CapabilityMiAuth: {
			Capability: CapabilityMiAuth, Supported: true, Approval: socialhub.ApprovalGranted,
			Reason: "per-instance MiAuth authorization without app registration", DocURL: docURL + "token/miauth/",
		},
		socialhub.CapMessage: {
			Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown,
			Reason: "the newer Misskey Chat API is not exposed by this initial adapter",
		},
		socialhub.CapWebhook: {
			Capability: socialhub.CapWebhook, Supported: false, Approval: socialhub.ApprovalUnknown,
			Reason: "Misskey provides a WebSocket Streaming API rather than signed HTTP webhooks",
		},
	}, nil
}

func capabilityState(capability socialhub.Capability, granted, required []string, reason, documentation string) socialhub.CapabilityState {
	approval := socialhub.ApprovalUnknown
	if len(required) == 0 {
		approval = socialhub.ApprovalGranted
	} else if len(granted) > 0 {
		approval = socialhub.ApprovalGranted
		for _, permission := range required {
			if !permissionGranted(granted, permission) {
				approval = socialhub.ApprovalRequired
				break
			}
		}
	}
	return socialhub.CapabilityState{
		Capability: capability, Supported: true, Approval: approval,
		Scopes: append([]string(nil), required...), Reason: reason, DocURL: documentation,
	}
}

func (c *Client) Publisher() (socialhub.Publisher, bool)           { return c, true }
func (c *Client) Fetcher() (socialhub.Fetcher, bool)               { return c, true }
func (c *Client) MediaUploader() (socialhub.MediaUploader, bool)   { return c, true }
func (c *Client) Reactor() (socialhub.Reactor, bool)               { return c, true }
func (c *Client) Messenger() (socialhub.Messenger, bool)           { return nil, false }
func (c *Client) WebhookHandler() (socialhub.WebhookHandler, bool) { return nil, false }
func (c *Client) Close() error                                     { return nil }

func (c *Client) NoteWorkflow() NoteWorkflow         { return c }
func (c *Client) TimelineWorkflow() TimelineWorkflow { return c }
func (c *Client) DriveWorkflow() DriveWorkflow       { return c }
func (c *Client) InstanceWorkflow() InstanceWorkflow { return c }

func (c *Client) requirePermissions(operation string, required ...string) error {
	if len(c.permissions) == 0 {
		return nil
	}
	var missing []string
	for _, permission := range required {
		if !permissionGranted(c.permissions, permission) {
			missing = append(missing, permission)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	return &socialhub.Error{
		Code: socialhub.CodeApprovalRequired, Class: socialhub.ClassUserAction,
		Platform: "misskey", Product: productName, Op: operation, RequiredScopes: missing,
		ApprovalURL: c.instanceURL + "/settings/apps", PlatformMessage: "configured MiAuth/OAuth permissions are incomplete",
	}
}

func permissionGranted(granted []string, required string) bool {
	for _, permission := range granted {
		if permission == required {
			return true
		}
	}
	return false
}

var _ socialhub.Client = (*Client)(nil)
var _ socialhub.Publisher = (*Client)(nil)
var _ socialhub.Fetcher = (*Client)(nil)
var _ socialhub.MediaUploader = (*Client)(nil)
var _ socialhub.Reactor = (*Client)(nil)
var _ NoteWorkflow = (*Client)(nil)
var _ TimelineWorkflow = (*Client)(nil)
var _ DriveWorkflow = (*Client)(nil)
var _ InstanceWorkflow = (*Client)(nil)
