package simkl

import (
	"context"
	"net/http"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	CapabilityOAuth    socialhub.Capability = "simkl_oauth"
	CapabilityCatalog  socialhub.Capability = "simkl_catalog"
	CapabilityTrending socialhub.Capability = "simkl_trending"
	CapabilityUser     socialhub.Capability = "simkl_user"
	CapabilitySync     socialhub.Capability = "simkl_sync"
	CapabilityScrobble socialhub.Capability = "simkl_scrobble"
)

// Client exposes typed Simkl catalog, tracking, and scrobble workflows.
type Client struct {
	accountID    socialhub.AccountID
	clientID     string
	clientSecret string
	accessToken  string
	api          *transport.Client
	userAPI      *transport.Client
	cdn          *transport.Client
	httpClient   *http.Client
	clock        socialhub.Clock
	authURL      string
	tokenURL     string
	userAgent    string
}

func (c *Client) Platform() socialhub.Platform { return "simkl" }
func (c *Client) Account() socialhub.AccountID { return c.accountID }

func (c *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	return socialhub.Capabilities{
		CapabilityOAuth:      credentialCapability(CapabilityOAuth, c.clientID != "", "OAuth confidential, PKCE S256, and PIN flows require a registered client_id"),
		CapabilityCatalog:    credentialCapability(CapabilityCatalog, c.clientID != "", "search and movie, TV, and anime details require client_id"),
		CapabilityTrending:   {Capability: CapabilityTrending, Supported: true, Approval: socialhub.ApprovalGranted, Reason: "public CDN trending files; Simkl attribution and deep links are required", DocURL: documentationURL},
		CapabilityUser:       credentialCapability(CapabilityUser, c.clientID != "" && c.accessToken != "", "current-user settings require client_id and a user access token"),
		CapabilitySync:       credentialCapability(CapabilitySync, c.clientID != "" && c.accessToken != "", "incremental library and batched writes require client_id and a user access token"),
		CapabilityScrobble:   credentialCapability(CapabilityScrobble, c.clientID != "" && c.accessToken != "", "playback scrobbling requires client_id and a user access token"),
		socialhub.CapPublish: {Capability: socialhub.CapPublish, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Simkl tracking mutations are not social posts"},
		socialhub.CapFetch:   {Capability: socialhub.CapFetch, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "catalog and library resources are exposed through typed workflows"},
		socialhub.CapMedia:   {Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Simkl does not accept media uploads"},
		socialhub.CapReact:   {Capability: socialhub.CapReact, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "ratings do not map losslessly to common post reactions"},
		socialhub.CapMessage: {Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Simkl API does not expose direct messaging"},
		socialhub.CapWebhook: {Capability: socialhub.CapWebhook, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Simkl API does not document signed webhooks"},
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

func (c *Client) OAuthWorkflow() OAuthWorkflow       { return c }
func (c *Client) CatalogWorkflow() CatalogWorkflow   { return c }
func (c *Client) TrendingWorkflow() TrendingWorkflow { return c }
func (c *Client) UserWorkflow() UserWorkflow         { return c }
func (c *Client) SyncWorkflow() SyncWorkflow         { return c }
func (c *Client) ScrobbleWorkflow() ScrobbleWorkflow { return c }

func (c *Client) requireClientID(operation string) error {
	if c.clientID != "" {
		return nil
	}
	return approvalRequired(operation, "a registered Simkl client_id is required")
}

func (c *Client) requireOAuth(operation string) error {
	if err := c.requireClientID(operation); err != nil {
		return err
	}
	if c.accessToken != "" {
		return nil
	}
	return approvalRequired(operation, "an authorized Simkl user access token is required")
}

func (c *Client) requireSecret(operation string) error {
	if err := c.requireClientID(operation); err != nil {
		return err
	}
	if c.clientSecret != "" {
		return nil
	}
	return approvalRequired(operation, "a Simkl client_secret is required for the confidential OAuth flow")
}

func approvalRequired(operation, message string) error {
	return &socialhub.Error{
		Code: socialhub.CodeApprovalRequired, Class: socialhub.ClassUserAction,
		Platform: "simkl", Product: productName, Op: operation,
		PlatformMessage: message, ApprovalURL: developerPortalURL,
	}
}

var _ socialhub.Client = (*Client)(nil)
