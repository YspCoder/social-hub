package lastfm

import (
	"context"
	"net/url"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	CapabilityAuth      socialhub.Capability = "lastfm_auth"
	CapabilityDiscovery socialhub.Capability = "lastfm_discovery"
	CapabilityUser      socialhub.Capability = "lastfm_user"
	CapabilityListening socialhub.Capability = "lastfm_listening"
	CapabilityLibrary   socialhub.Capability = "lastfm_library"
)

// Client implements typed Last.fm discovery, profile, and scrobbling workflows.
type Client struct {
	accountID  socialhub.AccountID
	apiKey     string
	apiSecret  string
	sessionKey string
	username   string
	authURL    *url.URL
	api        *transport.Client
}

func (c *Client) Platform() socialhub.Platform { return "lastfm" }
func (c *Client) Account() socialhub.AccountID { return c.accountID }

func (c *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	auth := capability(CapabilityAuth, c.apiSecret != "", "signed browser authentication requires an API secret")
	write := capability(CapabilityListening, c.apiSecret != "" && c.sessionKey != "", "now-playing and scrobbling require an API secret and user session key")
	library := capability(CapabilityLibrary, c.apiSecret != "" && c.sessionKey != "", "love and unlove require an API secret and user session key")
	return socialhub.Capabilities{
		CapabilityAuth:       auth,
		CapabilityDiscovery:  {Capability: CapabilityDiscovery, Supported: true, Approval: socialhub.ApprovalGranted, Reason: "public track, artist, and album metadata and search", DocURL: documentationURL},
		CapabilityUser:       {Capability: CapabilityUser, Supported: true, Approval: socialhub.ApprovalGranted, Reason: "public profile, recent, top, and loved-track reads", DocURL: documentationURL},
		CapabilityListening:  write,
		CapabilityLibrary:    library,
		socialhub.CapPublish: {Capability: socialhub.CapPublish, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Last.fm does not publish portable social posts"},
		socialhub.CapFetch:   {Capability: socialhub.CapFetch, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "music resources are exposed through typed workflows"},
		socialhub.CapMedia:   {Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Last.fm does not upload or provide audio media"},
		socialhub.CapReact:   {Capability: socialhub.CapReact, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "track love state is not mapped to portable post reactions"},
		socialhub.CapMessage: {Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Last.fm Web Services does not expose messaging"},
		socialhub.CapWebhook: {Capability: socialhub.CapWebhook, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Last.fm Web Services does not expose signed webhooks"},
	}, nil
}

func capability(name socialhub.Capability, available bool, reason string) socialhub.CapabilityState {
	state := socialhub.CapabilityState{Capability: name, Supported: true, Approval: socialhub.ApprovalGranted, Reason: reason, DocURL: documentationURL}
	if !available {
		state.Approval = socialhub.ApprovalRequired
	}
	return state
}

func (c *Client) Publisher() (socialhub.Publisher, bool)           { return nil, false }
func (c *Client) Fetcher() (socialhub.Fetcher, bool)               { return nil, false }
func (c *Client) MediaUploader() (socialhub.MediaUploader, bool)   { return nil, false }
func (c *Client) Reactor() (socialhub.Reactor, bool)               { return nil, false }
func (c *Client) Messenger() (socialhub.Messenger, bool)           { return nil, false }
func (c *Client) WebhookHandler() (socialhub.WebhookHandler, bool) { return nil, false }
func (c *Client) Close() error                                     { return nil }

func (c *Client) AuthWorkflow() AuthWorkflow           { return c }
func (c *Client) DiscoveryWorkflow() DiscoveryWorkflow { return c }
func (c *Client) UserWorkflow() UserWorkflow           { return c }
func (c *Client) ListeningWorkflow() ListeningWorkflow { return c }
func (c *Client) LibraryWorkflow() LibraryWorkflow     { return c }

func (c *Client) requireSecret(operation string) error {
	if c.apiSecret != "" {
		return nil
	}
	return approvalRequired(operation, "a Last.fm API secret is required")
}

func (c *Client) requireSession(operation string) error {
	if c.apiSecret != "" && c.sessionKey != "" {
		return nil
	}
	return approvalRequired(operation, "a Last.fm API secret and authorized session key are required")
}

func approvalRequired(operation, message string) error {
	return &socialhub.Error{
		Code: socialhub.CodeApprovalRequired, Class: socialhub.ClassUserAction,
		Platform: "lastfm", Product: productName, Op: operation,
		PlatformMessage: message, ApprovalURL: "https://www.last.fm/api/authentication",
	}
}

var _ socialhub.Client = (*Client)(nil)
