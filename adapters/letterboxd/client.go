package letterboxd

import (
	"context"
	"net/http"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	CapabilityOAuth         socialhub.Capability = "letterboxd_oauth"
	CapabilityCatalog       socialhub.Capability = "letterboxd_catalog"
	CapabilityMembers       socialhub.Capability = "letterboxd_members"
	CapabilityLogEntries    socialhub.Capability = "letterboxd_log_entries"
	CapabilityRelationships socialhub.Capability = "letterboxd_relationships"
)

// Client exposes typed Letterboxd film-social workflows.
type Client struct {
	accountID    socialhub.AccountID
	clientID     string
	clientSecret string
	tokenKind    TokenKind
	accessToken  string
	scopes       []string
	api          *transport.Client
	httpClient   *http.Client
	clock        socialhub.Clock
	authURL      string
	tokenURL     string
	revokeURL    string
	userAgent    string
}

func (c *Client) Platform() socialhub.Platform { return "letterboxd" }
func (c *Client) Account() socialhub.AccountID { return c.accountID }

func (c *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	hasToken := c.accessToken != ""
	hasUser := hasToken && c.tokenKind == TokenUser
	return socialhub.Capabilities{
		CapabilityOAuth:         {Capability: CapabilityOAuth, Supported: true, Approval: socialhub.ApprovalGranted, Reason: "client credentials, authorization code, refresh, and RFC 7009 revocation", DocURL: documentationURL},
		CapabilityCatalog:       tokenCapability(CapabilityCatalog, hasToken, "search and film discovery require an approved Letterboxd API token"),
		CapabilityMembers:       tokenCapability(CapabilityMembers, hasToken, "public member reads require an API token; /me requires a user token"),
		CapabilityLogEntries:    tokenCapability(CapabilityLogEntries, hasToken, "public log-entry reads require an API token; writes require a user token"),
		CapabilityRelationships: tokenCapability(CapabilityRelationships, hasUser, "like, rate, watch, and watchlist writes require an authorized user token"),
		socialhub.CapPublish:    {Capability: socialhub.CapPublish, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "diary entries and reviews are exposed as typed log-entry workflows"},
		socialhub.CapFetch:      {Capability: socialhub.CapFetch, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "films, members, activity, and reviews do not share the common post model"},
		socialhub.CapMedia:      {Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "third-party Letterboxd API access does not expose media upload"},
		socialhub.CapReact:      {Capability: socialhub.CapReact, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "film and review relationships use typed object semantics"},
		socialhub.CapMessage:    {Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Letterboxd API does not expose direct messaging"},
		socialhub.CapWebhook:    {Capability: socialhub.CapWebhook, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Letterboxd API does not document signed webhooks"},
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

func (c *Client) OAuthWorkflow() OAuthWorkflow               { return c }
func (c *Client) CatalogWorkflow() CatalogWorkflow           { return c }
func (c *Client) MemberWorkflow() MemberWorkflow             { return c }
func (c *Client) LogEntryWorkflow() LogEntryWorkflow         { return c }
func (c *Client) RelationshipWorkflow() RelationshipWorkflow { return c }

func (c *Client) requireToken(operation string) error {
	if c.accessToken != "" {
		return nil
	}
	return approvalRequired(operation, nil, "an approved Letterboxd API access token is required")
}

func (c *Client) requireUser(operation string) error {
	if c.accessToken == "" || c.tokenKind != TokenUser {
		return approvalRequired(operation, nil, "an authorization-code user token is required")
	}
	return nil
}

func (c *Client) requireContentModify(operation string) error {
	if err := c.requireUser(operation); err != nil {
		return err
	}
	if len(c.scopes) > 0 && !containsScope(c.scopes, "content:modify") {
		return approvalRequired(operation, []string{"content:modify"}, "configured scopes do not include content:modify")
	}
	return nil
}

func approvalRequired(operation string, scopes []string, message string) error {
	return &socialhub.Error{
		Code: socialhub.CodeApprovalRequired, Class: socialhub.ClassUserAction,
		Platform: "letterboxd", Product: productName, Op: operation,
		PlatformMessage: message, RequiredScopes: scopes, ApprovalURL: approvalURL,
	}
}

var _ socialhub.Client = (*Client)(nil)
