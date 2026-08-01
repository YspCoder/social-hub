package linkedin

import (
	"context"
	"net/http"
	"strings"
	"sync"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

// Client implements the common capabilities supported by LinkedIn REST APIs.
type Client struct {
	accountID   socialhub.AccountID
	authorURN   string
	transport   *transport.Client
	httpClient  *http.Client
	accessToken string
	scopes      []string
	clock       socialhub.Clock
	uploadMu    sync.Mutex
	uploads     map[string]*imageUpload
}

func (c *Client) Platform() socialhub.Platform { return "linkedin" }
func (c *Client) Account() socialhub.AccountID { return c.accountID }

func (c *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	readScope, writeScope := "r_member_social", "w_member_social"
	if c.organizationAuthor() {
		readScope, writeScope = "r_organization_social", "w_organization_social"
	}
	return socialhub.Capabilities{
		socialhub.CapPublish: capabilityState(socialhub.CapPublish, c.scopes, []string{writeScope}, "Posts API supports text, existing media URNs, and reshares", docURL+"community-management/shares/posts-api"),
		socialhub.CapFetch:   capabilityState(socialhub.CapFetch, c.scopes, []string{"openid", "profile", readScope}, "post reads require restricted Community Management access; member profile uses OpenID Connect", docURL+"community-management/shares/posts-api"),
		socialhub.CapMedia:   capabilityState(socialhub.CapMedia, c.scopes, []string{writeScope}, "initial adapter supports Images API single-file uploads", docURL+"community-management/shares/images-api"),
		socialhub.CapReact:   capabilityState(socialhub.CapReact, c.scopes, []string{writeScope}, "LIKE and top-level comment creation are supported; common comment deletion lacks LinkedIn root context", docURL+"community-management/shares/reactions-api"),
		socialhub.CapMessage: {Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "LinkedIn messaging is not a general-purpose public API"},
		socialhub.CapWebhook: {Capability: socialhub.CapWebhook, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "LinkedIn event subscriptions are separate restricted product workflows"},
	}, nil
}

func capabilityState(capability socialhub.Capability, granted, required []string, reason, documentation string) socialhub.CapabilityState {
	approval := socialhub.ApprovalUnknown
	if len(granted) > 0 {
		approval = socialhub.ApprovalGranted
		for _, scope := range required {
			if !contains(granted, scope) {
				approval = socialhub.ApprovalRequired
				break
			}
		}
	}
	return socialhub.CapabilityState{Capability: capability, Supported: true, Approval: approval, Scopes: required, Reason: reason, DocURL: documentation}
}

func (c *Client) Publisher() (socialhub.Publisher, bool)           { return c, true }
func (c *Client) Fetcher() (socialhub.Fetcher, bool)               { return c, true }
func (c *Client) MediaUploader() (socialhub.MediaUploader, bool)   { return c, true }
func (c *Client) Reactor() (socialhub.Reactor, bool)               { return c, true }
func (c *Client) Messenger() (socialhub.Messenger, bool)           { return nil, false }
func (c *Client) WebhookHandler() (socialhub.WebhookHandler, bool) { return nil, false }
func (c *Client) Close() error                                     { return nil }

func (c *Client) organizationAuthor() bool {
	return strings.HasPrefix(c.authorURN, "urn:li:organization:")
}

func (c *Client) socialScope(write bool) string {
	prefix := "r_"
	if write {
		prefix = "w_"
	}
	if c.organizationAuthor() {
		return prefix + "organization_social"
	}
	return prefix + "member_social"
}

func (c *Client) requireScopes(operation string, required ...string) error {
	if len(c.scopes) == 0 {
		return nil
	}
	for _, scope := range required {
		if !contains(c.scopes, scope) {
			return &socialhub.Error{
				Code: socialhub.CodeApprovalRequired, Class: socialhub.ClassUserAction, Platform: "linkedin", Product: "linkedin-rest", Op: operation,
				RequiredScopes: []string{scope}, ApprovalURL: "https://www.linkedin.com/developers/apps", PlatformMessage: "configured approval scopes do not include " + scope,
			}
		}
	}
	return nil
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
var _ socialhub.Publisher = (*Client)(nil)
var _ socialhub.Fetcher = (*Client)(nil)
var _ socialhub.MediaUploader = (*Client)(nil)
var _ socialhub.Reactor = (*Client)(nil)
