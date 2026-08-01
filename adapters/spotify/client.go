package spotify

import (
	"context"
	"net/url"
	"strings"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	CapabilityProfile         socialhub.Capability = "spotify_profile"
	CapabilityCatalog         socialhub.Capability = "spotify_catalog"
	CapabilityLibraryRead     socialhub.Capability = "spotify_library_read"
	CapabilityLibraryModify   socialhub.Capability = "spotify_library_modify"
	CapabilityPlaylistRead    socialhub.Capability = "spotify_playlist_read"
	CapabilityPlaylistModify  socialhub.Capability = "spotify_playlist_modify"
	CapabilityPlaybackRead    socialhub.Capability = "spotify_playback_read"
	CapabilityPlaybackControl socialhub.Capability = "spotify_playback_control"
)

// Client implements Spotify's typed Web API workflows.
type Client struct {
	accountID        socialhub.AccountID
	spotifyAccountID string
	accountType      string
	scopes           []string
	api              *transport.Client
	apiBaseURL       *url.URL
	clock            socialhub.Clock
}

func (c *Client) Platform() socialhub.Platform { return "spotify" }
func (c *Client) Account() socialhub.AccountID { return c.accountID }

func (c *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	return socialhub.Capabilities{
		CapabilityProfile:         c.scopeState(CapabilityProfile, nil, "current authenticated user profile", docURL),
		CapabilityCatalog:         c.scopeState(CapabilityCatalog, nil, "single-track catalog reads and track search", docURL),
		CapabilityLibraryRead:     c.scopeState(CapabilityLibraryRead, []string{ScopeUserLibraryRead}, "saved tracks and generic library membership", docURL),
		CapabilityLibraryModify:   {Capability: CapabilityLibraryModify, Supported: true, Approval: socialhub.ApprovalUnknown, Reason: "generic URI library changes use entity-specific scopes", DocURL: docURL},
		CapabilityPlaylistRead:    c.scopeState(CapabilityPlaylistRead, []string{ScopePlaylistReadPrivate}, "current-user playlists and owned/collaborative playlist items", docURL),
		CapabilityPlaylistModify:  c.anyScopeState(CapabilityPlaylistModify, []string{ScopePlaylistModifyPublic, ScopePlaylistModifyPrivate}, "playlist creation, metadata, items, and snapshots", docURL),
		CapabilityPlaybackRead:    c.scopeState(CapabilityPlaybackRead, []string{ScopeUserReadPlaybackState}, "devices, state, and queue", docURL),
		CapabilityPlaybackControl: c.playbackControlState(),
		socialhub.CapPublish:      {Capability: socialhub.CapPublish, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Spotify Web API does not publish user posts or upload audio"},
		socialhub.CapFetch:        {Capability: socialhub.CapFetch, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Spotify catalog and library entities are not social posts; use typed workflows"},
		socialhub.CapMedia:        {Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Spotify Web API does not expose audio upload or download"},
		socialhub.CapReact:        {Capability: socialhub.CapReact, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "library saves are private collection state, not portable social reactions"},
		socialhub.CapMessage:      {Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Spotify Web API does not expose messaging"},
		socialhub.CapWebhook:      {Capability: socialhub.CapWebhook, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Spotify Web API does not expose signed webhooks"},
	}, nil
}

func (c *Client) Publisher() (socialhub.Publisher, bool)           { return nil, false }
func (c *Client) Fetcher() (socialhub.Fetcher, bool)               { return nil, false }
func (c *Client) MediaUploader() (socialhub.MediaUploader, bool)   { return nil, false }
func (c *Client) Reactor() (socialhub.Reactor, bool)               { return nil, false }
func (c *Client) Messenger() (socialhub.Messenger, bool)           { return nil, false }
func (c *Client) WebhookHandler() (socialhub.WebhookHandler, bool) { return nil, false }
func (c *Client) Close() error                                     { return nil }

func (c *Client) ProfileWorkflow() ProfileWorkflow   { return c }
func (c *Client) CatalogWorkflow() CatalogWorkflow   { return c }
func (c *Client) LibraryWorkflow() LibraryWorkflow   { return c }
func (c *Client) PlaylistWorkflow() PlaylistWorkflow { return c }
func (c *Client) PlaybackWorkflow() PlaybackWorkflow { return c }

func (c *Client) scopeState(capability socialhub.Capability, required []string, reason, documentation string) socialhub.CapabilityState {
	approval := socialhub.ApprovalUnknown
	if len(c.scopes) > 0 {
		approval = socialhub.ApprovalGranted
		for _, scope := range required {
			if !contains(c.scopes, scope) {
				approval = socialhub.ApprovalRequired
				break
			}
		}
	}
	return socialhub.CapabilityState{Capability: capability, Supported: true, Approval: approval, Scopes: append([]string(nil), required...), Reason: reason, DocURL: documentation}
}

func (c *Client) anyScopeState(capability socialhub.Capability, allowed []string, reason, documentation string) socialhub.CapabilityState {
	state := c.scopeState(capability, nil, reason, documentation)
	state.Scopes = append([]string(nil), allowed...)
	if len(c.scopes) > 0 {
		state.Approval = socialhub.ApprovalRequired
		for _, scope := range allowed {
			if contains(c.scopes, scope) {
				state.Approval = socialhub.ApprovalGranted
				break
			}
		}
	}
	return state
}

func (c *Client) playbackControlState() socialhub.CapabilityState {
	state := c.scopeState(CapabilityPlaybackControl, []string{ScopeUserModifyPlaybackState}, "Premium-only Spotify Connect playback controls", docURL)
	if c.accountType != "" && !strings.EqualFold(c.accountType, "premium") {
		state.Approval = socialhub.ApprovalRequired
		state.Reason = "playback controls require Spotify Premium"
	}
	return state
}

func (c *Client) requireScopes(operation string, required ...string) error {
	if len(c.scopes) == 0 {
		return nil
	}
	missing := make([]string, 0, len(required))
	for _, scope := range required {
		if !contains(c.scopes, scope) {
			missing = append(missing, scope)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	return &socialhub.Error{
		Code: socialhub.CodeApprovalRequired, Class: socialhub.ClassUserAction,
		Platform: "spotify", Product: productName, Op: operation, RequiredScopes: missing,
		ApprovalURL: "https://developer.spotify.com/dashboard", PlatformMessage: "configured OAuth scopes are incomplete",
	}
}

func (c *Client) requireAnyScope(operation string, allowed ...string) error {
	if len(c.scopes) == 0 {
		return nil
	}
	for _, scope := range allowed {
		if contains(c.scopes, scope) {
			return nil
		}
	}
	return &socialhub.Error{
		Code: socialhub.CodeApprovalRequired, Class: socialhub.ClassUserAction,
		Platform: "spotify", Product: productName, Op: operation, RequiredScopes: append([]string(nil), allowed...),
		ApprovalURL: "https://developer.spotify.com/dashboard", PlatformMessage: "one of the required Spotify scopes must be granted",
	}
}

func (c *Client) requirePremium(operation string) error {
	if c.accountType == "" || strings.EqualFold(c.accountType, "premium") {
		return nil
	}
	return &socialhub.Error{
		Code: socialhub.CodeApprovalRequired, Class: socialhub.ClassUserAction,
		Platform: "spotify", Product: productName, Op: operation,
		ApprovalURL: "https://www.spotify.com/premium/", PlatformMessage: "Spotify Premium is required for playback control",
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
