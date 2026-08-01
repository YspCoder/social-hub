package snapchat

import (
	"context"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

// PublicProfileWorkflow exposes the read-only Snapchat Public Profile flow.
type PublicProfileWorkflow interface {
	Profile(context.Context, string, ...socialhub.CallOption) (*socialhub.User, error)
	MyProfile(context.Context, ...socialhub.CallOption) (*socialhub.User, error)
	SearchProfiles(context.Context, ProfileSearchRequest, ...socialhub.CallOption) (socialhub.Page[socialhub.User], error)
	ListSpotlights(context.Context, SpotlightListRequest, ...socialhub.CallOption) (socialhub.Page[socialhub.Post], error)
	Spotlight(context.Context, string, ...socialhub.CallOption) (*socialhub.Post, error)
}

// Client is one configured Snapchat Public Profile account.
type Client struct {
	accountID socialhub.AccountID
	profileID string
	transport *transport.Client
	scopes    []string
	profiles  *PublicProfileService
}

func (c *Client) Platform() socialhub.Platform { return "snapchat" }
func (c *Client) Account() socialhub.AccountID { return c.accountID }

func (c *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	return socialhub.Capabilities{
		CapabilityPublicProfileWorkflow: capabilityState(CapabilityPublicProfileWorkflow, c.scopes, "typed read-only Public Profile, discovery, and Spotlight workflow; app allowlisting is also required", docURL),
		socialhub.CapPublish:            {Capability: socialhub.CapPublish, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Public Profile publishing requires an encrypted media-container and multipart workflow"},
		socialhub.CapFetch:              {Capability: socialhub.CapFetch, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "use PublicProfileWorkflow; Snapchat does not expose a general timeline or comment fetch contract"},
		socialhub.CapMedia:              {Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "AES-encrypted media containers cannot be represented by the common media uploader"},
		socialhub.CapReact:              {Capability: socialhub.CapReact, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Public Profile API does not expose common reactions or comments"},
		socialhub.CapMessage:            {Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Public Profile messaging is not a general direct-message contract"},
		socialhub.CapWebhook:            {Capability: socialhub.CapWebhook, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "no Public Profile webhook contract is implemented"},
	}, nil
}

func capabilityState(capability socialhub.Capability, granted []string, reason, documentation string) socialhub.CapabilityState {
	approval := socialhub.ApprovalUnknown
	if len(granted) > 0 {
		approval = socialhub.ApprovalRequired
		if contains(granted, requiredScope) {
			approval = socialhub.ApprovalGranted
		}
	}
	return socialhub.CapabilityState{
		Capability: capability, Supported: true, Approval: approval, Scopes: []string{requiredScope},
		Reason: reason, DocURL: documentation,
	}
}

func (c *Client) Publisher() (socialhub.Publisher, bool)           { return nil, false }
func (c *Client) Fetcher() (socialhub.Fetcher, bool)               { return nil, false }
func (c *Client) MediaUploader() (socialhub.MediaUploader, bool)   { return nil, false }
func (c *Client) Reactor() (socialhub.Reactor, bool)               { return nil, false }
func (c *Client) Messenger() (socialhub.Messenger, bool)           { return nil, false }
func (c *Client) WebhookHandler() (socialhub.WebhookHandler, bool) { return nil, false }
func (c *Client) Close() error                                     { return nil }

// PublicProfileWorkflow returns Snapchat's typed read-only workflow.
func (c *Client) PublicProfileWorkflow() PublicProfileWorkflow { return c.profiles }

func (c *Client) requireScope(operation string) error {
	if len(c.scopes) == 0 || contains(c.scopes, requiredScope) {
		return nil
	}
	return &socialhub.Error{
		Code: socialhub.CodeApprovalRequired, Class: socialhub.ClassUserAction, Platform: "snapchat", Product: "snapchat-public-profile", Op: operation,
		RequiredScopes: []string{requiredScope}, ApprovalURL: docURL, PlatformMessage: "configured approval scopes do not include snapchat-profile-api",
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
var _ PublicProfileWorkflow = (*PublicProfileService)(nil)
