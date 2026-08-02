package kitsu

import (
	"context"
	"net/http"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	CapabilityToken   socialhub.Capability = "kitsu_token"
	CapabilityAnime   socialhub.Capability = "kitsu_anime"
	CapabilityManga   socialhub.Capability = "kitsu_manga"
	CapabilityUser    socialhub.Capability = "kitsu_user"
	CapabilityLibrary socialhub.Capability = "kitsu_library"
	CapabilityPost    socialhub.Capability = "kitsu_post"
	CapabilityComment socialhub.Capability = "kitsu_comment"
)

// Client exposes typed Kitsu catalog, tracking, and social workflows.
type Client struct {
	accountID    socialhub.AccountID
	userID       string
	clientID     string
	clientSecret string
	accessToken  string
	api          *transport.Client
	httpClient   *http.Client
	clock        socialhub.Clock
	tokenURL     string
	userAgent    string
}

func (c *Client) Platform() socialhub.Platform { return "kitsu" }
func (c *Client) Account() socialhub.AccountID { return c.accountID }

func (c *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	return socialhub.Capabilities{
		CapabilityToken:      capabilityState(CapabilityToken, c.accessToken != "", "caller-provided bearer tokens and refresh-token rotation; password login is deliberately unsupported"),
		CapabilityAnime:      {Capability: CapabilityAnime, Supported: true, Approval: socialhub.ApprovalGranted, Reason: "public anime search and details", DocURL: documentationURL},
		CapabilityManga:      {Capability: CapabilityManga, Supported: true, Approval: socialhub.ApprovalGranted, Reason: "public manga search and details", DocURL: documentationURL},
		CapabilityUser:       {Capability: CapabilityUser, Supported: true, Approval: socialhub.ApprovalGranted, Reason: "public profiles; self lookup requires a bearer token", DocURL: documentationURL},
		CapabilityLibrary:    capabilityState(CapabilityLibrary, c.accessToken != "", "filtered public reads; owner writes require a bearer token and configured user_id"),
		CapabilityPost:       capabilityState(CapabilityPost, c.accessToken != "", "global public post reads; owner writes require a bearer token and configured user_id"),
		CapabilityComment:    capabilityState(CapabilityComment, c.accessToken != "", "public comment reads; owner writes require a bearer token and configured user_id"),
		socialhub.CapPublish: {Capability: socialhub.CapPublish, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Kitsu posts are exposed through a typed JSON:API workflow"},
		socialhub.CapFetch:   {Capability: socialhub.CapFetch, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Kitsu catalog, library, and global social feeds do not map losslessly to common posts"},
		socialhub.CapMedia:   {Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "standalone media upload is not part of the initial adapter scope"},
		socialhub.CapReact:   {Capability: socialhub.CapReact, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Kitsu like endpoints are outside the initial adapter scope"},
		socialhub.CapMessage: {Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Kitsu does not expose direct messaging through this API surface"},
		socialhub.CapWebhook: {Capability: socialhub.CapWebhook, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Kitsu does not document signed webhooks"},
	}, nil
}

func capabilityState(capability socialhub.Capability, authenticated bool, reason string) socialhub.CapabilityState {
	state := socialhub.CapabilityState{Capability: capability, Supported: true, Approval: socialhub.ApprovalGranted, Reason: reason, DocURL: documentationURL}
	if !authenticated {
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

func (c *Client) TokenWorkflow() TokenWorkflow     { return c }
func (c *Client) AnimeWorkflow() AnimeWorkflow     { return c }
func (c *Client) MangaWorkflow() MangaWorkflow     { return c }
func (c *Client) UserWorkflow() UserWorkflow       { return c }
func (c *Client) LibraryWorkflow() LibraryWorkflow { return c }
func (c *Client) PostWorkflow() PostWorkflow       { return c }
func (c *Client) CommentWorkflow() CommentWorkflow { return c }

func (c *Client) requireUser(operation string) error {
	if c.accessToken == "" {
		return approvalRequired(operation, "a Kitsu bearer token is required")
	}
	if c.userID == "" {
		return approvalRequired(operation, "account.settings.user_id is required for owner mutations")
	}
	return nil
}

func (c *Client) requireToken(operation string) error {
	if c.accessToken == "" {
		return approvalRequired(operation, "a Kitsu bearer token is required")
	}
	return nil
}

func approvalRequired(operation, message string) error {
	return &socialhub.Error{
		Code: socialhub.CodeApprovalRequired, Class: socialhub.ClassUserAction,
		Platform: "kitsu", Product: productName, Op: operation,
		PlatformMessage: message, ApprovalURL: registrationURL,
	}
}

var _ socialhub.Client = (*Client)(nil)
