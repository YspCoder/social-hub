package wordpresscom

import (
	"context"
	"strings"
	"sync"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	CapabilityPosts socialhub.Capability = "wordpress_posts"
	CapabilitySite  socialhub.Capability = "wordpress_site"
	CapabilityMedia socialhub.Capability = "wordpress_media_library"
)

// Client implements WordPress.com site reads and authenticated content management.
type Client struct {
	accountID socialhub.AccountID
	site      string
	userID    string
	public    *transport.Client
	user      *transport.Client
	scopes    []string
	clock     socialhub.Clock

	uploadMu sync.Mutex
	uploads  map[string]*uploadState
}

func (client *Client) Platform() socialhub.Platform { return "wordpress.com" }
func (client *Client) Account() socialhub.AccountID { return client.accountID }

func (client *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	return socialhub.Capabilities{
		socialhub.CapPublish: capabilityState(socialhub.CapPublish, client.user != nil, client.scopes, []string{"posts"}, "common and typed site Post publishing"),
		socialhub.CapFetch: {
			Capability: socialhub.CapFetch, Supported: true, Approval: socialhub.ApprovalGranted,
			Reason: "public site Posts and Comments; OAuth also enables the current user and private content", DocURL: documentationURL,
		},
		socialhub.CapMedia:   capabilityState(socialhub.CapMedia, client.user != nil, client.scopes, []string{"media"}, "single-part streaming uploads to the site media library"),
		socialhub.CapReact:   capabilityState(socialhub.CapReact, client.user != nil, client.scopes, []string{"posts", "comments"}, "Post likes plus comment and reply lifecycle"),
		socialhub.CapMessage: {Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "WordPress.com REST API does not expose direct messaging"},
		socialhub.CapWebhook: {Capability: socialhub.CapWebhook, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "webhook registration does not define a signed inbound contract for the common handler"},
		CapabilityPosts:      capabilityState(CapabilityPosts, client.user != nil, client.scopes, []string{"posts"}, "title, excerpt, status, scheduling, taxonomy, discussion, update, and restore"),
		CapabilitySite: {
			Capability: CapabilitySite, Supported: true, Approval: socialhub.ApprovalGranted,
			Reason: "typed metadata for the configured WordPress.com or Jetpack-connected site", DocURL: documentationURL,
		},
		CapabilityMedia: capabilityState(CapabilityMedia, client.user != nil, client.scopes, []string{"media"}, "media upload, status lookup, and deletion"),
	}, nil
}

func capabilityState(capability socialhub.Capability, supported bool, granted, required []string, reason string) socialhub.CapabilityState {
	approval := socialhub.ApprovalUnknown
	if supported {
		approval = approvalState(granted, required)
	}
	return socialhub.CapabilityState{
		Capability: capability, Supported: supported, Approval: approval,
		Scopes: append([]string(nil), required...), Reason: reason, DocURL: documentationURL,
	}
}

func approvalState(granted, required []string) socialhub.ApprovalState {
	if len(granted) == 0 {
		return socialhub.ApprovalUnknown
	}
	for _, scope := range required {
		if !scopeGranted(granted, scope) {
			return socialhub.ApprovalRequired
		}
	}
	return socialhub.ApprovalGranted
}

func (client *Client) Publisher() (socialhub.Publisher, bool) {
	if client.user == nil {
		return nil, false
	}
	return client, true
}

func (client *Client) Fetcher() (socialhub.Fetcher, bool) { return client, true }

func (client *Client) MediaUploader() (socialhub.MediaUploader, bool) {
	if client.user == nil {
		return nil, false
	}
	return client, true
}

func (client *Client) Reactor() (socialhub.Reactor, bool) {
	if client.user == nil {
		return nil, false
	}
	return client, true
}

func (client *Client) Messenger() (socialhub.Messenger, bool)           { return nil, false }
func (client *Client) WebhookHandler() (socialhub.WebhookHandler, bool) { return nil, false }
func (client *Client) Close() error                                     { return nil }

func (client *Client) PostWorkflow() PostWorkflow                 { return client }
func (client *Client) SiteWorkflow() SiteWorkflow                 { return client }
func (client *Client) MediaLibraryWorkflow() MediaLibraryWorkflow { return client }

func (client *Client) readAPI() *transport.Client {
	if client.user != nil {
		return client.user
	}
	return client.public
}

func (client *Client) requireUser(operation string) (*transport.Client, error) {
	if client.user == nil {
		return nil, &socialhub.Error{
			Code: socialhub.CodeUnauthenticated, Class: socialhub.ClassUserAction, Platform: "wordpress.com", Product: productName,
			Op: operation, PlatformMessage: "configure access_token_ref with a WordPress.com OAuth2 token",
		}
	}
	return client.user, nil
}

func (client *Client) requireScopes(operation string, required ...string) error {
	if len(client.scopes) == 0 {
		return nil
	}
	missing := make([]string, 0, len(required))
	for _, scope := range required {
		if !scopeGranted(client.scopes, scope) {
			missing = append(missing, scope)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	return &socialhub.Error{
		Code: socialhub.CodeApprovalRequired, Class: socialhub.ClassUserAction, Platform: "wordpress.com", Product: productName,
		Op: operation, RequiredScopes: missing, ApprovalURL: defaultAuthURL,
		PlatformMessage: "configured approval scopes do not include required WordPress.com permissions",
	}
}

func scopeGranted(scopes []string, target string) bool {
	for _, scope := range scopes {
		scope = strings.TrimSpace(scope)
		if scope == "global" || scope == target || target == "users" && scope == "auth" {
			return true
		}
	}
	return false
}

var _ socialhub.Client = (*Client)(nil)
var _ socialhub.Publisher = (*Client)(nil)
var _ socialhub.Fetcher = (*Client)(nil)
var _ socialhub.MediaUploader = (*Client)(nil)
var _ socialhub.Reactor = (*Client)(nil)
