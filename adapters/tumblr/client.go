package tumblr

import (
	"context"
	"strings"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	CapabilityNPF        socialhub.Capability = "tumblr_npf"
	CapabilityTimeline   socialhub.Capability = "tumblr_timeline"
	CapabilityEngagement socialhub.Capability = "tumblr_engagement"
)

// Client implements Tumblr public reads, NPF publishing, and typed workflows.
type Client struct {
	accountID      socialhub.AccountID
	blogIdentifier string
	public         *transport.Client
	user           *transport.Client
	scopes         []string
	clock          socialhub.Clock
}

func (c *Client) Platform() socialhub.Platform { return "tumblr" }
func (c *Client) Account() socialhub.AccountID { return c.accountID }

func (c *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	return socialhub.Capabilities{
		socialhub.CapPublish: capabilityState(socialhub.CapPublish, c.user != nil, c.scopes, []string{"write"}, "NPF text publishing through the configured blog"),
		socialhub.CapFetch: {
			Capability: socialhub.CapFetch, Supported: true, Approval: socialhub.ApprovalGranted,
			Reason: "public blog profiles, NPF posts, and conversation notes via the configured API key", DocURL: documentationURL,
		},
		socialhub.CapMedia:   {Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Tumblr media is uploaded inline with an NPF post; use NPFWorkflow"},
		socialhub.CapReact:   {Capability: socialhub.CapReact, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Tumblr likes and reblogs require a reblog key and source blog context; use EngagementWorkflow or NPFWorkflow"},
		socialhub.CapMessage: {Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Tumblr API v2 does not expose direct messaging"},
		socialhub.CapWebhook: {Capability: socialhub.CapWebhook, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Tumblr API v2 does not publish a webhook contract"},
		CapabilityNPF:        capabilityState(CapabilityNPF, c.user != nil, c.scopes, []string{"basic", "write"}, "NPF create, edit, fetch, reblog, scheduling, and inline media upload"),
		CapabilityTimeline: {
			Capability: CapabilityTimeline, Supported: true, Approval: approvalState(c.scopes, []string{"basic"}), Scopes: []string{"basic"},
			Reason: "public tagged discovery and authenticated dashboard reads", DocURL: documentationURL,
		},
		CapabilityEngagement: capabilityState(CapabilityEngagement, c.user != nil, c.scopes, []string{"write"}, "typed like, unlike, follow, and unfollow actions"),
	}, nil
}

func capabilityState(capability socialhub.Capability, supported bool, granted, required []string, reason string) socialhub.CapabilityState {
	approval := socialhub.ApprovalUnknown
	if supported {
		approval = approvalState(granted, required)
	}
	return socialhub.CapabilityState{
		Capability: capability, Supported: supported, Approval: approval,
		Scopes: append([]string(nil), required...), Reason: reason, DocURL: documentationURL,
	}
}

func approvalState(granted, required []string) socialhub.ApprovalState {
	if len(granted) == 0 {
		return socialhub.ApprovalUnknown
	}
	for _, scope := range required {
		if !contains(granted, scope) {
			return socialhub.ApprovalRequired
		}
	}
	return socialhub.ApprovalGranted
}

func (c *Client) Publisher() (socialhub.Publisher, bool) {
	if c.user == nil {
		return nil, false
	}
	return c, true
}
func (c *Client) Fetcher() (socialhub.Fetcher, bool)             { return c, true }
func (c *Client) MediaUploader() (socialhub.MediaUploader, bool) { return nil, false }
func (c *Client) Reactor() (socialhub.Reactor, bool)             { return nil, false }
func (c *Client) Messenger() (socialhub.Messenger, bool)         { return nil, false }
func (c *Client) WebhookHandler() (socialhub.WebhookHandler, bool) {
	return nil, false
}
func (c *Client) Close() error { return nil }

func (c *Client) NPFWorkflow() NPFWorkflow               { return c }
func (c *Client) TimelineWorkflow() TimelineWorkflow     { return c }
func (c *Client) EngagementWorkflow() EngagementWorkflow { return c }

func (c *Client) requireUser(operation string) (*transport.Client, error) {
	if c.user == nil {
		return nil, &socialhub.Error{
			Code: socialhub.CodeUnauthenticated, Class: socialhub.ClassUserAction, Platform: "tumblr", Product: productName,
			Op: operation, PlatformMessage: "configure access_token_ref with an OAuth2 user token",
		}
	}
	return c.user, nil
}

func (c *Client) requireScopes(operation string, required ...string) error {
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
		Code: socialhub.CodeApprovalRequired, Class: socialhub.ClassUserAction, Platform: "tumblr", Product: productName,
		Op: operation, RequiredScopes: missing, ApprovalURL: "https://www.tumblr.com/oauth/apps",
		PlatformMessage: "configured approval scopes do not include required Tumblr permissions",
	}
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) == target {
			return true
		}
	}
	return false
}

func (c *Client) selectedBlog(value string) (string, error) {
	if strings.TrimSpace(value) == "" {
		return c.blogIdentifier, nil
	}
	if !validBlogIdentifier(value) {
		return "", invalidArgument("blog", "blog identifier is invalid")
	}
	return strings.TrimSpace(value), nil
}

var _ socialhub.Client = (*Client)(nil)
var _ socialhub.Publisher = (*Client)(nil)
var _ socialhub.Fetcher = (*Client)(nil)
