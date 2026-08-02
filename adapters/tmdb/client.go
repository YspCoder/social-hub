package tmdb

import (
	"context"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	CapabilityAuth    socialhub.Capability = "tmdb_auth"
	CapabilityCatalog socialhub.Capability = "tmdb_catalog"
	CapabilityAccount socialhub.Capability = "tmdb_account"
	CapabilityLibrary socialhub.Capability = "tmdb_library"
	CapabilityRating  socialhub.Capability = "tmdb_rating"
)

// Client implements typed TMDB catalog, account, library, and rating workflows.
type Client struct {
	accountID      socialhub.AccountID
	tmdbAccountID  int64
	sessionID      string
	guestSessionID string
	api            *transport.Client
	authURL        string
}

func (c *Client) Platform() socialhub.Platform { return "tmdb" }
func (c *Client) Account() socialhub.AccountID { return c.accountID }

func (c *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	hasAccount := c.sessionID != "" && c.tmdbAccountID > 0
	hasRating := c.sessionID != "" || c.guestSessionID != ""
	return socialhub.Capabilities{
		CapabilityAuth:       {Capability: CapabilityAuth, Supported: true, Approval: socialhub.ApprovalGranted, Reason: "request-token, user session, and guest session workflows", DocURL: documentationURL},
		CapabilityCatalog:    {Capability: CapabilityCatalog, Supported: true, Approval: socialhub.ApprovalGranted, Reason: "movie, TV, person, search, trending, popular, and image configuration reads", DocURL: documentationURL},
		CapabilityAccount:    credentialCapability(CapabilityAccount, hasAccount, "account details require an approved user session and account ID"),
		CapabilityLibrary:    credentialCapability(CapabilityLibrary, hasAccount, "favorites, watchlist, and rated lists require an approved user session and account ID"),
		CapabilityRating:     credentialCapability(CapabilityRating, hasRating, "ratings require a user session or guest session"),
		socialhub.CapPublish: {Capability: socialhub.CapPublish, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "TMDB metadata and account actions are not portable social posts"},
		socialhub.CapFetch:   {Capability: socialhub.CapFetch, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "media entities are exposed through typed workflows"},
		socialhub.CapMedia:   {Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "TMDB API v3 does not expose general media upload"},
		socialhub.CapReact:   {Capability: socialhub.CapReact, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "favorites and ratings are not mapped to post reactions"},
		socialhub.CapMessage: {Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "TMDB API does not expose messaging"},
		socialhub.CapWebhook: {Capability: socialhub.CapWebhook, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "TMDB API does not expose signed webhooks"},
	}, nil
}

func credentialCapability(capability socialhub.Capability, available bool, reason string) socialhub.CapabilityState {
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

func (c *Client) AuthWorkflow() AuthWorkflow       { return c }
func (c *Client) CatalogWorkflow() CatalogWorkflow { return c }
func (c *Client) AccountWorkflow() AccountWorkflow { return c }
func (c *Client) LibraryWorkflow() LibraryWorkflow { return c }

func (c *Client) requireAccount(operation string) error {
	if c.sessionID != "" && c.tmdbAccountID > 0 {
		return nil
	}
	return approvalRequired(operation, "an approved TMDB user session and account ID are required")
}

func (c *Client) requireRating(operation string) error {
	if c.sessionID != "" || c.guestSessionID != "" {
		return nil
	}
	return approvalRequired(operation, "a TMDB user session or guest session is required")
}

func approvalRequired(operation, message string) error {
	return &socialhub.Error{
		Code: socialhub.CodeApprovalRequired, Class: socialhub.ClassUserAction,
		Platform: "tmdb", Product: productName, Op: operation,
		PlatformMessage: message, ApprovalURL: "https://www.themoviedb.org/settings/api",
	}
}

var _ socialhub.Client = (*Client)(nil)
