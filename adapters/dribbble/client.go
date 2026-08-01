package dribbble

import (
	"context"
	"net/url"
	"strings"
	"sync"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	CapabilityShots       socialhub.Capability = "dribbble_shots"
	CapabilityProjects    socialhub.Capability = "dribbble_projects"
	CapabilityAttachments socialhub.Capability = "dribbble_attachments"
)

// Client implements Dribbble's profile reads and typed publishing workflows.
type Client struct {
	accountID socialhub.AccountID
	userID    string
	scopes    []string
	api       *transport.Client
	baseURL   *url.URL
	clock     socialhub.Clock

	rateMu sync.RWMutex
	rate   RateLimit
}

func (client *Client) Platform() socialhub.Platform { return "dribbble" }
func (client *Client) Account() socialhub.AccountID { return client.accountID }

func (client *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	return socialhub.Capabilities{
		CapabilityShots:       client.capability(CapabilityShots, []string{"public", "upload"}, "owned Shot reads plus typed asynchronous image publishing, updates, and deletion", documentationURL+"shots/"),
		CapabilityProjects:    client.capability(CapabilityProjects, []string{"public", "upload"}, "owned Project listing and CRUD", documentationURL+"projects/"),
		CapabilityAttachments: client.capability(CapabilityAttachments, []string{"upload"}, "typed asynchronous attachment upload and deletion", documentationURL+"attachments/"),
		socialhub.CapPublish:  {Capability: socialhub.CapPublish, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "use ShotWorkflow; Dribbble requires an image file, title, and publishing-specific metadata"},
		socialhub.CapFetch:    client.capability(socialhub.CapFetch, []string{"public"}, "authorized user, individual Shots, and the authorized user's Shot list; API v2 has no comments", documentationURL),
		socialhub.CapMedia:    {Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Dribbble accepts media only inside Shot or Attachment multipart operations"},
		socialhub.CapReact:    {Capability: socialhub.CapReact, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Dribbble API v2 removed comments, likes, and rebounds as interaction endpoints"},
		socialhub.CapMessage:  {Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Dribbble API v2 does not expose messaging"},
		socialhub.CapWebhook:  {Capability: socialhub.CapWebhook, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Dribbble API v2 does not expose signed webhooks"},
	}, nil
}

func (client *Client) capability(capability socialhub.Capability, required []string, reason, docURL string) socialhub.CapabilityState {
	approval := socialhub.ApprovalUnknown
	if len(client.scopes) > 0 {
		approval = socialhub.ApprovalGranted
		for _, scope := range required {
			if !client.hasScope(scope) {
				approval = socialhub.ApprovalRequired
				break
			}
		}
	}
	return socialhub.CapabilityState{Capability: capability, Supported: true, Approval: approval, Scopes: append([]string(nil), required...), Reason: reason, DocURL: docURL}
}

func (client *Client) Publisher() (socialhub.Publisher, bool)           { return nil, false }
func (client *Client) Fetcher() (socialhub.Fetcher, bool)               { return client, true }
func (client *Client) MediaUploader() (socialhub.MediaUploader, bool)   { return nil, false }
func (client *Client) Reactor() (socialhub.Reactor, bool)               { return nil, false }
func (client *Client) Messenger() (socialhub.Messenger, bool)           { return nil, false }
func (client *Client) WebhookHandler() (socialhub.WebhookHandler, bool) { return nil, false }
func (client *Client) Close() error                                     { return nil }

func (client *Client) ShotWorkflow() ShotWorkflow             { return client }
func (client *Client) ProjectWorkflow() ProjectWorkflow       { return client }
func (client *Client) AttachmentWorkflow() AttachmentWorkflow { return client }

// RateLimit returns the latest server-provided per-minute quota snapshot.
func (client *Client) RateLimit() RateLimit {
	client.rateMu.RLock()
	defer client.rateMu.RUnlock()
	return client.rate
}

func (client *Client) requireScopes(operation string, required ...string) error {
	if len(client.scopes) == 0 {
		return nil
	}
	missing := make([]string, 0, len(required))
	for _, scope := range required {
		if !client.hasScope(scope) {
			missing = append(missing, scope)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	return &socialhub.Error{
		Code: socialhub.CodeApprovalRequired, Class: socialhub.ClassUserAction, Platform: "dribbble", Product: productName,
		Op: operation, RequiredScopes: missing, ApprovalURL: documentationURL + "oauth/",
		PlatformMessage: "configured approval scopes do not include required Dribbble permissions",
	}
}

func (client *Client) hasScope(target string) bool {
	for _, scope := range client.scopes {
		if strings.TrimSpace(scope) == target {
			return true
		}
	}
	return false
}

var _ socialhub.Client = (*Client)(nil)
var _ socialhub.Fetcher = (*Client)(nil)
