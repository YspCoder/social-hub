package myanimelist

import (
	"context"
	"net/http"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	CapabilityOAuth     socialhub.Capability = "myanimelist_oauth"
	CapabilityAnime     socialhub.Capability = "myanimelist_anime"
	CapabilityManga     socialhub.Capability = "myanimelist_manga"
	CapabilityUser      socialhub.Capability = "myanimelist_user"
	CapabilityAnimeList socialhub.Capability = "myanimelist_anime_list"
	CapabilityMangaList socialhub.Capability = "myanimelist_manga_list"
)

// Client exposes typed MyAnimeList catalog and personal-list workflows.
type Client struct {
	accountID    socialhub.AccountID
	clientID     string
	clientSecret string
	accessToken  string
	scopes       []string
	api          *transport.Client
	httpClient   *http.Client
	clock        socialhub.Clock
	authURL      string
	tokenURL     string
	userAgent    string
}

func (c *Client) Platform() socialhub.Platform { return "myanimelist" }
func (c *Client) Account() socialhub.AccountID { return c.accountID }

func (c *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	hasUser := c.accessToken != ""
	return socialhub.Capabilities{
		CapabilityOAuth:      {Capability: CapabilityOAuth, Supported: true, Approval: socialhub.ApprovalGranted, Reason: "authorization code with plain PKCE and refresh", DocURL: authorizationURL},
		CapabilityAnime:      {Capability: CapabilityAnime, Supported: true, Approval: socialhub.ApprovalGranted, Reason: "search, details, ranking, seasonal catalog, and authorized suggestions", DocURL: documentationURL},
		CapabilityManga:      {Capability: CapabilityManga, Supported: true, Approval: socialhub.ApprovalGranted, Reason: "search, details, and ranking", DocURL: documentationURL},
		CapabilityUser:       tokenCapability(CapabilityUser, hasUser, "the @me profile requires an authorized user token"),
		CapabilityAnimeList:  {Capability: CapabilityAnimeList, Supported: true, Approval: socialhub.ApprovalGranted, Reason: "public list reads use a client ID; @me and mutations require a user token", DocURL: documentationURL},
		CapabilityMangaList:  {Capability: CapabilityMangaList, Supported: true, Approval: socialhub.ApprovalGranted, Reason: "public list reads use a client ID; @me and mutations require a user token", DocURL: documentationURL},
		socialhub.CapPublish: {Capability: socialhub.CapPublish, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "anime and manga list mutations are exposed as typed workflows"},
		socialhub.CapFetch:   {Capability: socialhub.CapFetch, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "catalog works and list entries are not social posts"},
		socialhub.CapMedia:   {Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "MyAnimeList API v2 does not expose media upload"},
		socialhub.CapReact:   {Capability: socialhub.CapReact, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "scores and progress are list status fields, not common reactions"},
		socialhub.CapMessage: {Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "MyAnimeList API v2 does not expose messaging"},
		socialhub.CapWebhook: {Capability: socialhub.CapWebhook, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "MyAnimeList API v2 does not document signed webhooks"},
	}, nil
}

func tokenCapability(capability socialhub.Capability, available bool, reason string) socialhub.CapabilityState {
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
func (c *Client) AnimeWorkflow() AnimeWorkflow         { return c }
func (c *Client) MangaWorkflow() MangaWorkflow         { return c }
func (c *Client) UserWorkflow() UserWorkflow           { return c }
func (c *Client) AnimeListWorkflow() AnimeListWorkflow { return c }
func (c *Client) MangaListWorkflow() MangaListWorkflow { return c }

func (c *Client) requireUser(operation string) error {
	if c.accessToken == "" {
		return approvalRequired(operation, nil, "an authorization-code user token is required")
	}
	return nil
}

func (c *Client) requireWrite(operation string) error {
	if err := c.requireUser(operation); err != nil {
		return err
	}
	if len(c.scopes) > 0 && !containsScope(c.scopes, scopeWriteUsers) {
		return approvalRequired(operation, []string{scopeWriteUsers}, "configured scopes do not include write:users")
	}
	return nil
}

func approvalRequired(operation string, scopes []string, message string) error {
	return &socialhub.Error{
		Code: socialhub.CodeApprovalRequired, Class: socialhub.ClassUserAction,
		Platform: "myanimelist", Product: productName, Op: operation,
		PlatformMessage: message, RequiredScopes: scopes, ApprovalURL: registrationURL,
	}
}

var _ socialhub.Client = (*Client)(nil)
