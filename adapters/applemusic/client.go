package applemusic

import (
	"context"
	"net/url"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	CapabilityStorefront socialhub.Capability = "applemusic_storefront"
	CapabilityCatalog    socialhub.Capability = "applemusic_catalog"
	CapabilityLibrary    socialhub.Capability = "applemusic_library"
	CapabilityPlaylist   socialhub.Capability = "applemusic_playlist"
	CapabilityHistory    socialhub.Capability = "applemusic_history"
)

// Client implements Apple Music's typed catalog and user-library workflows.
type Client struct {
	accountID      socialhub.AccountID
	storefront     string
	musicUserToken string
	api            *transport.Client
	apiBaseURL     *url.URL
}

func (c *Client) Platform() socialhub.Platform { return "applemusic" }
func (c *Client) Account() socialhub.AccountID { return c.accountID }

func (c *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	personal := socialhub.CapabilityState{
		Supported: true, Approval: socialhub.ApprovalGranted,
		Reason: "requires a Music User Token obtained through MusicKit user authorization", DocURL: documentationURL,
	}
	if c.musicUserToken == "" {
		personal.Approval = socialhub.ApprovalRequired
	}
	return socialhub.Capabilities{
		CapabilityStorefront: {Capability: CapabilityStorefront, Supported: true, Approval: socialhub.ApprovalGranted, Reason: "public storefront discovery; current-user storefront requires a Music User Token", DocURL: documentationURL},
		CapabilityCatalog:    {Capability: CapabilityCatalog, Supported: true, Approval: socialhub.ApprovalGranted, Reason: "catalog resources, search, and charts", DocURL: documentationURL},
		CapabilityLibrary:    withCapability(personal, CapabilityLibrary, "personal library reads, search, and catalog additions require a Music User Token"),
		CapabilityPlaylist:   withCapability(personal, CapabilityPlaylist, "library playlist creation and track additions require a Music User Token"),
		CapabilityHistory:    withCapability(personal, CapabilityHistory, "recently played resources require a Music User Token"),
		socialhub.CapPublish: {Capability: socialhub.CapPublish, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Apple Music API does not publish social posts"},
		socialhub.CapFetch:   {Capability: socialhub.CapFetch, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "music catalog and library resources are exposed through typed workflows"},
		socialhub.CapMedia:   {Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Apple Music API does not expose media upload or audio download"},
		socialhub.CapReact:   {Capability: socialhub.CapReact, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "library state is not mapped to portable social reactions"},
		socialhub.CapMessage: {Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Apple Music API does not expose messaging"},
		socialhub.CapWebhook: {Capability: socialhub.CapWebhook, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Apple Music API does not expose signed webhooks"},
	}, nil
}

func withCapability(state socialhub.CapabilityState, capability socialhub.Capability, reason string) socialhub.CapabilityState {
	state.Capability, state.Reason = capability, reason
	return state
}

func (c *Client) Publisher() (socialhub.Publisher, bool)           { return nil, false }
func (c *Client) Fetcher() (socialhub.Fetcher, bool)               { return nil, false }
func (c *Client) MediaUploader() (socialhub.MediaUploader, bool)   { return nil, false }
func (c *Client) Reactor() (socialhub.Reactor, bool)               { return nil, false }
func (c *Client) Messenger() (socialhub.Messenger, bool)           { return nil, false }
func (c *Client) WebhookHandler() (socialhub.WebhookHandler, bool) { return nil, false }
func (c *Client) Close() error                                     { return nil }

func (c *Client) StorefrontWorkflow() StorefrontWorkflow { return c }
func (c *Client) CatalogWorkflow() CatalogWorkflow       { return c }
func (c *Client) LibraryWorkflow() LibraryWorkflow       { return c }
func (c *Client) PlaylistWorkflow() PlaylistWorkflow     { return c }
func (c *Client) HistoryWorkflow() HistoryWorkflow       { return c }

func (c *Client) requireMusicUserToken(operation string) error {
	if c.musicUserToken != "" {
		return nil
	}
	return &socialhub.Error{
		Code: socialhub.CodeApprovalRequired, Class: socialhub.ClassUserAction,
		Platform: "applemusic", Product: productName, Op: operation,
		PlatformMessage: "a Music User Token obtained through MusicKit authorization is required",
		ApprovalURL:     "https://developer.apple.com/documentation/applemusicapi/user-authentication-for-musickit",
	}
}

var _ socialhub.Client = (*Client)(nil)
