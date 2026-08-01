package dailymotion

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	CapabilityProfile     socialhub.Capability = "profile"
	CapabilityVideo       socialhub.Capability = "video"
	CapabilityVideoUpload socialhub.Capability = "video_upload"
	CapabilityPlaylist    socialhub.Capability = "playlist"
)

// Client implements Dailymotion's common fetcher and typed workflows.
type Client struct {
	accountID  socialhub.AccountID
	profileID  string
	scopes     []string
	api        *transport.Client
	apiBaseURL *url.URL
	httpClient *http.Client
	uploadMu   sync.Mutex
	uploads    map[string]*videoUpload
	upload     *VideoUploadService
}

func (c *Client) Platform() socialhub.Platform { return "dailymotion" }
func (c *Client) Account() socialhub.AccountID { return c.accountID }

func (c *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	return socialhub.Capabilities{
		socialhub.CapFetch:    c.capabilityState(socialhub.CapFetch, []string{ScopeProfileRead, ScopeVideoRead}, "profiles and owned videos; comments are not exposed by API v2", docURL),
		CapabilityProfile:     c.capabilityState(CapabilityProfile, []string{ScopeProfileRead}, "current account, managed profiles, profile updates, and webhook configuration", "https://developers.dailymotion.com/reference/get-profile"),
		CapabilityVideo:       c.capabilityState(CapabilityVideo, []string{ScopeVideoRead}, "video reads, metadata creation, updates, and deletion", "https://developers.dailymotion.com/reference/get-video"),
		CapabilityVideoUpload: c.capabilityState(CapabilityVideoUpload, []string{ScopeVideoManage}, "upload session, multipart file transfer, and video publication", "https://developers.dailymotion.com/docs/upload-videos"),
		CapabilityPlaylist:    c.capabilityState(CapabilityPlaylist, []string{ScopePlaylistRead}, "playlist CRUD and ordered video membership", "https://developers.dailymotion.com/reference/get-playlist"),
		socialhub.CapPublish:  {Capability: socialhub.CapPublish, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Dailymotion video creation requires category, audience, and source fields; use VideoWorkflow or VideoUploadWorkflow"},
		socialhub.CapMedia:    {Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "uploaded bytes are an intermediate video source; use VideoUploadWorkflow"},
		socialhub.CapReact:    {Capability: socialhub.CapReact, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Dailymotion API v2 does not expose reactions or comments"},
		socialhub.CapMessage:  {Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Dailymotion API v2 does not expose direct messaging"},
		socialhub.CapWebhook:  {Capability: socialhub.CapWebhook, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "API v2 exposes webhook configuration but does not publicly document X-DM-Signature verification"},
	}, nil
}

func (c *Client) capabilityState(capability socialhub.Capability, required []string, reason, documentation string) socialhub.CapabilityState {
	approval := socialhub.ApprovalGranted
	for _, scope := range required {
		if !c.hasScope(scope) {
			approval = socialhub.ApprovalRequired
			break
		}
	}
	return socialhub.CapabilityState{Capability: capability, Supported: true, Approval: approval, Scopes: append([]string(nil), required...), Reason: reason, DocURL: documentation}
}

func (c *Client) Publisher() (socialhub.Publisher, bool)           { return nil, false }
func (c *Client) Fetcher() (socialhub.Fetcher, bool)               { return c, true }
func (c *Client) MediaUploader() (socialhub.MediaUploader, bool)   { return nil, false }
func (c *Client) Reactor() (socialhub.Reactor, bool)               { return nil, false }
func (c *Client) Messenger() (socialhub.Messenger, bool)           { return nil, false }
func (c *Client) WebhookHandler() (socialhub.WebhookHandler, bool) { return nil, false }
func (c *Client) Close() error                                     { return nil }

// ProfileWorkflow returns profile and API-webhook configuration operations.
func (c *Client) ProfileWorkflow() ProfileWorkflow { return c }

// VideoWorkflow returns Dailymotion video metadata operations.
func (c *Client) VideoWorkflow() VideoWorkflow { return c }

// VideoUploadWorkflow returns Dailymotion's upload-and-publish lifecycle.
func (c *Client) VideoUploadWorkflow() VideoUploadWorkflow { return c.upload }

// PlaylistWorkflow returns playlist and membership operations.
func (c *Client) PlaylistWorkflow() PlaylistWorkflow { return c }

func (c *Client) requireScopes(operation string, required ...string) error {
	missing := make([]string, 0, len(required))
	for _, scope := range required {
		if !c.hasScope(scope) {
			missing = append(missing, scope)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	return &socialhub.Error{
		Code: socialhub.CodeApprovalRequired, Class: socialhub.ClassUserAction,
		Platform: "dailymotion", Product: productName, Op: operation, RequiredScopes: missing,
		ApprovalURL: "https://developers.dailymotion.com/reference/api-scopes", PlatformMessage: "configured OAuth scopes are incomplete",
	}
}

func (c *Client) hasScope(required string) bool {
	for _, granted := range c.scopes {
		if granted == required || granted == BundleOrganization {
			return true
		}
		if strings.HasSuffix(required, ".read") && granted == strings.TrimSuffix(required, ".read")+".manage" {
			return true
		}
		switch granted {
		case BundlePublic:
			if required == ScopeProfileRead || required == ScopeVideoRead || required == ScopePlaylistRead {
				return true
			}
		case BundleUser, BundlePublisher:
			if required == ScopeAccountRead || required == ScopeAccountManage || required == ScopeProfileRead || required == ScopeProfileManage || required == ScopeVideoRead || required == ScopeVideoManage || required == ScopePlaylistRead || required == ScopePlaylistManage || required == ScopeLiveRead || required == ScopeLiveManage {
				return true
			}
			if granted == BundlePublisher && (required == ScopePlayerRead || required == ScopePlayerManage || required == ScopeAnalyticsManage) {
				return true
			}
		}
	}
	return false
}

func (c *Client) defaultProfile(operation, requested string) (string, error) {
	if requested == "" {
		requested = c.profileID
	}
	if !validResourceID(requested) {
		return "", invalidArgument(operation, "a profile ID is required in the request or account settings")
	}
	return requested, nil
}

var _ socialhub.Client = (*Client)(nil)
var _ socialhub.Fetcher = (*Client)(nil)
