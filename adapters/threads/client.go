package threads

import (
	"context"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	CapabilityContainerPublish socialhub.Capability = "container_publish"
	CapabilityInsights         socialhub.Capability = "insights"
	CapabilityDiscovery        socialhub.Capability = "public_discovery"
	CapabilityReplyModeration  socialhub.Capability = "reply_moderation"
	CapabilityRepost           socialhub.Capability = "repost_workflow"
	CapabilityPublishingQuota  socialhub.Capability = "publishing_quota"
)

// Client implements common and typed Threads API capabilities for one user.
type Client struct {
	accountID socialhub.AccountID
	userID    string
	transport *transport.Client
	scopes    []string
	clock     socialhub.Clock
}

func (c *Client) Platform() socialhub.Platform { return "threads" }
func (c *Client) Account() socialhub.AccountID { return c.accountID }

func (c *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	return socialhub.Capabilities{
		socialhub.CapPublish:       capabilityState(socialhub.CapPublish, c.scopes, []string{"threads_basic", "threads_content_publish", "threads_delete"}, "text posts, replies, quotes, status, and deletion", docURL+"posts/"),
		socialhub.CapFetch:         capabilityState(socialhub.CapFetch, c.scopes, []string{"threads_basic", "threads_read_replies"}, "authorized profile, own posts, post details, and direct replies", docURL),
		socialhub.CapReact:         capabilityState(socialhub.CapReact, c.scopes, []string{"threads_content_publish", "threads_delete"}, "common comments map to text replies; reaction mutation uses typed repost workflow", docURL+"reply-management/"),
		CapabilityContainerPublish: capabilityState(CapabilityContainerPublish, c.scopes, []string{"threads_content_publish"}, "remote image/video and carousel container lifecycle", docURL+"posts/"),
		CapabilityInsights:         capabilityState(CapabilityInsights, c.scopes, []string{"threads_manage_insights"}, "post and account insights", docURL+"insights/"),
		CapabilityDiscovery:        capabilityState(CapabilityDiscovery, c.scopes, []string{"threads_profile_discovery", "threads_keyword_search", "threads_manage_mentions"}, "public profiles/posts, keyword search, and mentions", docURL),
		CapabilityReplyModeration:  capabilityState(CapabilityReplyModeration, c.scopes, []string{"threads_manage_replies"}, "hide/unhide and pending reply approval", docURL+"reply-management/"),
		CapabilityRepost:           capabilityState(CapabilityRepost, c.scopes, []string{"threads_content_publish"}, "typed repost returns the deletable repost ID", docURL+"posts/"),
		CapabilityPublishingQuota:  capabilityState(CapabilityPublishingQuota, c.scopes, []string{"threads_content_publish"}, "server-provided post, reply, delete, and location-search quota", docURL+"publishing-limit/"),
		socialhub.CapMedia: {
			Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown,
			Reason: "Threads fetches public HTTPS media URLs into publication containers",
		},
		socialhub.CapMessage: {
			Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown,
			Reason: "Threads API does not expose direct messaging",
		},
		socialhub.CapWebhook: {
			Capability: socialhub.CapWebhook, Supported: false, Approval: socialhub.ApprovalUnknown,
			Reason: "webhook payload and signature contracts are not included in the verified official sample surface",
		},
	}, nil
}

func capabilityState(capability socialhub.Capability, granted, required []string, reason, documentation string) socialhub.CapabilityState {
	approval := socialhub.ApprovalUnknown
	if len(granted) > 0 {
		approval = socialhub.ApprovalGranted
		for _, scope := range required {
			if !contains(granted, scope) {
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
func (c *Client) MediaUploader() (socialhub.MediaUploader, bool)   { return nil, false }
func (c *Client) Reactor() (socialhub.Reactor, bool)               { return c, true }
func (c *Client) Messenger() (socialhub.Messenger, bool)           { return nil, false }
func (c *Client) WebhookHandler() (socialhub.WebhookHandler, bool) { return nil, false }
func (c *Client) Close() error                                     { return nil }

func (c *Client) ContainerWorkflow() ContainerWorkflow   { return c }
func (c *Client) InsightsWorkflow() InsightsWorkflow     { return c }
func (c *Client) DiscoveryWorkflow() DiscoveryWorkflow   { return c }
func (c *Client) ModerationWorkflow() ModerationWorkflow { return c }
func (c *Client) RepostWorkflow() RepostWorkflow         { return c }
func (c *Client) PublishingQuotaWorkflow() QuotaWorkflow { return c }

func (c *Client) requireScope(operation string, required ...string) error {
	if len(c.scopes) == 0 {
		return nil
	}
	missing := make([]string, 0, len(required))
	for _, scope := range required {
		if !contains(c.scopes, scope) {
			missing = append(missing, scope)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	return &socialhub.Error{
		Code: socialhub.CodeApprovalRequired, Class: socialhub.ClassUserAction, Platform: "threads", Product: productName,
		Op: operation, RequiredScopes: missing, ApprovalURL: "https://developers.facebook.com/apps/",
		PlatformMessage: "configured approval scopes do not include required Threads permissions",
	}
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

var _ socialhub.Client = (*Client)(nil)
var _ socialhub.Publisher = (*Client)(nil)
var _ socialhub.Fetcher = (*Client)(nil)
var _ socialhub.Reactor = (*Client)(nil)
