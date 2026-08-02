package trakt

import (
	"context"
	"net/http"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	CapabilityAuth     socialhub.Capability = "trakt_auth"
	CapabilityCatalog  socialhub.Capability = "trakt_catalog"
	CapabilityProfile  socialhub.Capability = "trakt_profile"
	CapabilitySync     socialhub.Capability = "trakt_sync"
	CapabilityScrobble socialhub.Capability = "trakt_scrobble"
	CapabilityComments socialhub.Capability = "trakt_comments"
)

// Client implements typed Trakt media, sync, scrobble, and comment workflows.
type Client struct {
	accountID     socialhub.AccountID
	clientID      string
	clientSecret  string
	username      string
	authenticated bool
	api           *transport.Client
	httpClient    *http.Client
	clock         socialhub.Clock
	authURL       string
	userAgent     string
}

func (c *Client) Platform() socialhub.Platform { return "trakt" }
func (c *Client) Account() socialhub.AccountID { return c.accountID }

func (c *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	auth := credentialCapability(CapabilityAuth, c.clientSecret != "", "browser exchange, device polling, refresh, and revoke require the client secret")
	sync := credentialCapability(CapabilitySync, c.authenticated, "current-user history, watchlist, and ratings require OAuth")
	scrobble := credentialCapability(CapabilityScrobble, c.authenticated, "media-center scrobbling requires OAuth")
	return socialhub.Capabilities{
		CapabilityAuth:       auth,
		CapabilityCatalog:    {Capability: CapabilityCatalog, Supported: true, Approval: socialhub.ApprovalGranted, Reason: "public movie, show, episode, trending, and search endpoints", DocURL: documentationURL},
		CapabilityProfile:    {Capability: CapabilityProfile, Supported: true, Approval: socialhub.ApprovalGranted, Reason: "public profiles; settings require OAuth", DocURL: documentationURL},
		CapabilitySync:       sync,
		CapabilityScrobble:   scrobble,
		CapabilityComments:   {Capability: CapabilityComments, Supported: true, Approval: socialhub.ApprovalGranted, Reason: "public comment reads; creation and moderation require OAuth", DocURL: documentationURL},
		socialhub.CapPublish: {Capability: socialhub.CapPublish, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Trakt media actions are not portable social posts"},
		socialhub.CapFetch:   {Capability: socialhub.CapFetch, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "movies, shows, episodes, and comments are exposed through typed workflows"},
		socialhub.CapMedia:   {Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Trakt does not accept media uploads"},
		socialhub.CapReact:   {Capability: socialhub.CapReact, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "ratings and comment likes are not mapped to post reactions"},
		socialhub.CapMessage: {Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Trakt API does not expose messaging"},
		socialhub.CapWebhook: {Capability: socialhub.CapWebhook, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Trakt API does not expose signed webhooks"},
	}, nil
}

func credentialCapability(name socialhub.Capability, available bool, reason string) socialhub.CapabilityState {
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

func (c *Client) OAuthWorkflow() OAuthWorkflow       { return c }
func (c *Client) CatalogWorkflow() CatalogWorkflow   { return c }
func (c *Client) UserWorkflow() UserWorkflow         { return c }
func (c *Client) SyncWorkflow() SyncWorkflow         { return c }
func (c *Client) ScrobbleWorkflow() ScrobbleWorkflow { return c }
func (c *Client) CommentWorkflow() CommentWorkflow   { return c }

func (c *Client) requireSecret(operation string) error {
	if c.clientSecret != "" {
		return nil
	}
	return approvalRequired(operation, "a Trakt client secret is required")
}

func (c *Client) requireOAuth(operation string) error {
	if c.authenticated {
		return nil
	}
	return approvalRequired(operation, "an authorized Trakt user access token is required")
}

func approvalRequired(operation, message string) error {
	return &socialhub.Error{
		Code: socialhub.CodeApprovalRequired, Class: socialhub.ClassUserAction,
		Platform: "trakt", Product: productName, Op: operation,
		PlatformMessage: message, ApprovalURL: "https://app.trakt.tv/settings/apps",
	}
}

var _ socialhub.Client = (*Client)(nil)
