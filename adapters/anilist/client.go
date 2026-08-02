package anilist

import (
	"context"
	"net/http"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	CapabilityOAuth     socialhub.Capability = "anilist_oauth"
	CapabilityMedia     socialhub.Capability = "anilist_media"
	CapabilityUser      socialhub.Capability = "anilist_user"
	CapabilityMediaList socialhub.Capability = "anilist_media_list"
	CapabilityActivity  socialhub.Capability = "anilist_activity"
)

// Client exposes typed AniList catalog, tracking, and social workflows.
type Client struct {
	accountID    socialhub.AccountID
	clientID     string
	clientSecret string
	accessToken  string
	api          *transport.Client
	httpClient   *http.Client
	clock        socialhub.Clock
	authURL      string
	tokenURL     string
	userAgent    string
}

func (c *Client) Platform() socialhub.Platform { return "anilist" }
func (c *Client) Account() socialhub.AccountID { return c.accountID }

func (c *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	hasOAuthApp := c.clientID != ""
	return socialhub.Capabilities{
		CapabilityOAuth:      capabilityState(CapabilityOAuth, hasOAuthApp, "authorization code and implicit grants; no scopes or refresh tokens"),
		CapabilityMedia:      {Capability: CapabilityMedia, Supported: true, Approval: socialhub.ApprovalGranted, Reason: "public anime and manga catalog, search, trending, and seasonal discovery", DocURL: documentationURL},
		CapabilityUser:       {Capability: CapabilityUser, Supported: true, Approval: socialhub.ApprovalGranted, Reason: "public profiles; Viewer requires a user token", DocURL: documentationURL},
		CapabilityMediaList:  {Capability: CapabilityMediaList, Supported: true, Approval: socialhub.ApprovalGranted, Reason: "public list reads; mutations require a user token", DocURL: documentationURL},
		CapabilityActivity:   {Capability: CapabilityActivity, Supported: true, Approval: socialhub.ApprovalGranted, Reason: "public activity reads; text, reply, and like mutations require a user token", DocURL: documentationURL},
		socialhub.CapPublish: {Capability: socialhub.CapPublish, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "AniList activities are exposed through typed workflows"},
		socialhub.CapFetch:   {Capability: socialhub.CapFetch, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "media tracking and activity unions do not map losslessly to common posts"},
		socialhub.CapMedia:   {Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "AniList API v2 does not expose media upload"},
		socialhub.CapReact:   {Capability: socialhub.CapReact, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "AniList exposes non-idempotent toggle-like mutations"},
		socialhub.CapMessage: {Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "private message activities are outside the initial adapter scope"},
		socialhub.CapWebhook: {Capability: socialhub.CapWebhook, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "AniList API v2 does not document signed webhooks"},
	}, nil
}

func capabilityState(capability socialhub.Capability, available bool, reason string) socialhub.CapabilityState {
	state := socialhub.CapabilityState{Capability: capability, Supported: true, Approval: socialhub.ApprovalGranted, Reason: reason, DocURL: documentationURL}
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

func (c *Client) OAuthWorkflow() OAuthWorkflow         { return c }
func (c *Client) MediaWorkflow() MediaWorkflow         { return c }
func (c *Client) UserWorkflow() UserWorkflow           { return c }
func (c *Client) MediaListWorkflow() MediaListWorkflow { return c }
func (c *Client) ActivityWorkflow() ActivityWorkflow   { return c }

func (c *Client) requireOAuthApp(operation string, secret bool) error {
	if c.clientID == "" || (secret && c.clientSecret == "") {
		return approvalRequired(operation, "a registered AniList OAuth client is required")
	}
	return nil
}

func (c *Client) requireUser(operation string) error {
	if c.accessToken == "" {
		return approvalRequired(operation, "an AniList user access token is required")
	}
	return nil
}

func approvalRequired(operation, message string) error {
	return &socialhub.Error{
		Code: socialhub.CodeApprovalRequired, Class: socialhub.ClassUserAction,
		Platform: "anilist", Product: productName, Op: operation,
		PlatformMessage: message, ApprovalURL: registrationURL,
	}
}

var _ socialhub.Client = (*Client)(nil)
