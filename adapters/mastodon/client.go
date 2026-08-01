package mastodon

import (
	"context"
	"sync"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	// CapabilityHomeTimeline identifies Mastodon's authenticated home timeline.
	CapabilityHomeTimeline socialhub.Capability = "home_timeline"
	// CapabilityInstanceDiscovery identifies Mastodon instance metadata discovery.
	CapabilityInstanceDiscovery socialhub.Capability = "instance_discovery"
)

// TimelineWorkflow exposes Mastodon's authenticated home timeline.
type TimelineWorkflow interface {
	Home(context.Context, TimelineRequest, ...socialhub.CallOption) (socialhub.Page[socialhub.Post], error)
}

// InstanceWorkflow exposes server capabilities and limits for one instance.
type InstanceWorkflow interface {
	Instance(context.Context, ...socialhub.CallOption) (*InstanceInfo, error)
}

// Client implements Mastodon's supported common and typed capabilities.
type Client struct {
	accountID   socialhub.AccountID
	instanceURL string
	userID      string
	transport   *transport.Client
	scopes      []string
	clock       socialhub.Clock
	uploadMu    sync.Mutex
	uploads     map[string]*uploadSession
}

func (c *Client) Platform() socialhub.Platform { return "mastodon" }
func (c *Client) Account() socialhub.AccountID { return c.accountID }

func (c *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	return socialhub.Capabilities{
		socialhub.CapPublish:        capabilityState(socialhub.CapPublish, c.scopes, []string{"write:statuses"}, "status publishing, replies, and API-v7 quote IDs", docURL+"statuses/"),
		socialhub.CapFetch:          capabilityState(socialhub.CapFetch, c.scopes, []string{"read:accounts", "read:statuses"}, "profiles, account statuses, status lookup, and thread context", docURL+"accounts/"),
		socialhub.CapMedia:          capabilityState(socialhub.CapMedia, c.scopes, []string{"write:media"}, "single-part asynchronous media attachments", docURL+"media/"),
		socialhub.CapReact:          capabilityState(socialhub.CapReact, c.scopes, []string{"write:favourites", "write:statuses"}, "favourite, boost, and reply operations", docURL+"statuses/"),
		CapabilityHomeTimeline:      capabilityState(CapabilityHomeTimeline, c.scopes, []string{"read:statuses"}, "authenticated chronological home timeline", docURL+"timelines/"),
		CapabilityInstanceDiscovery: {Capability: CapabilityInstanceDiscovery, Supported: true, Approval: socialhub.ApprovalGranted, Reason: "public instance metadata and configured limits", DocURL: docURL + "instance/"},
		socialhub.CapMessage:        {Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "direct-visibility statuses do not provide the common conversation message contract"},
		socialhub.CapWebhook:        {Capability: socialhub.CapWebhook, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Mastodon webhooks are an instance administration feature"},
	}, nil
}

func capabilityState(capability socialhub.Capability, granted, required []string, reason, documentation string) socialhub.CapabilityState {
	approval := socialhub.ApprovalUnknown
	if len(granted) > 0 {
		approval = socialhub.ApprovalGranted
		for _, scope := range required {
			if !scopeGranted(granted, scope) {
				approval = socialhub.ApprovalRequired
				break
			}
		}
	}
	return socialhub.CapabilityState{Capability: capability, Supported: true, Approval: approval, Scopes: required, Reason: reason, DocURL: documentation}
}

func (c *Client) Publisher() (socialhub.Publisher, bool)           { return c, true }
func (c *Client) Fetcher() (socialhub.Fetcher, bool)               { return c, true }
func (c *Client) MediaUploader() (socialhub.MediaUploader, bool)   { return c, true }
func (c *Client) Reactor() (socialhub.Reactor, bool)               { return c, true }
func (c *Client) Messenger() (socialhub.Messenger, bool)           { return nil, false }
func (c *Client) WebhookHandler() (socialhub.WebhookHandler, bool) { return nil, false }
func (c *Client) Close() error                                     { return nil }

func (c *Client) TimelineWorkflow() TimelineWorkflow { return c }
func (c *Client) InstanceWorkflow() InstanceWorkflow { return c }

func (c *Client) requireScopes(operation string, required ...string) error {
	if len(c.scopes) == 0 {
		return nil
	}
	var missing []string
	for _, scope := range required {
		if !scopeGranted(c.scopes, scope) {
			missing = append(missing, scope)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	return &socialhub.Error{
		Code: socialhub.CodeApprovalRequired, Class: socialhub.ClassUserAction, Platform: "mastodon", Product: "mastodon-rest-api", Op: operation,
		RequiredScopes: missing, ApprovalURL: c.instanceURL + "/settings/applications", PlatformMessage: "configured OAuth scopes are incomplete",
	}
}

func (c *Client) requireAnyScope(operation string, required ...string) error {
	if len(c.scopes) == 0 {
		return nil
	}
	for _, scope := range required {
		if scopeGranted(c.scopes, scope) {
			return nil
		}
	}
	return &socialhub.Error{
		Code: socialhub.CodeApprovalRequired, Class: socialhub.ClassUserAction, Platform: "mastodon", Product: "mastodon-rest-api", Op: operation,
		RequiredScopes: required, ApprovalURL: c.instanceURL + "/settings/applications", PlatformMessage: "one of the required OAuth scopes must be granted",
	}
}

func scopeGranted(granted []string, required string) bool {
	for _, scope := range granted {
		if scope == required {
			return true
		}
		if required == "profile" && scope == "read:accounts" {
			return true
		}
		for _, parent := range []string{"read", "write", "follow"} {
			if scope == parent && len(required) > len(parent) && required[:len(parent)+1] == parent+":" {
				return true
			}
		}
	}
	return false
}

var _ socialhub.Client = (*Client)(nil)
var _ socialhub.Publisher = (*Client)(nil)
var _ socialhub.Fetcher = (*Client)(nil)
var _ socialhub.MediaUploader = (*Client)(nil)
var _ socialhub.Reactor = (*Client)(nil)
var _ TimelineWorkflow = (*Client)(nil)
var _ InstanceWorkflow = (*Client)(nil)
