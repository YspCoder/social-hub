package blogger

import (
	"context"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	CapabilityBlogRead    socialhub.Capability = "blogger_blog_read"
	CapabilityPostRead    socialhub.Capability = "blogger_post_read"
	CapabilityCommentRead socialhub.Capability = "blogger_comment_read"
	CapabilityPageRead    socialhub.Capability = "blogger_page_read"
)

// Client exposes provider-native Blogger reads for one OAuth account.
type Client struct {
	accountID   socialhub.AccountID
	scopes      []string
	api         *transport.Client
	accessToken string
	clock       socialhub.Clock
}

func (client *Client) Platform() socialhub.Platform { return platformName }
func (client *Client) Account() socialhub.AccountID { return client.accountID }

func (client *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	approval := socialhub.ApprovalUnknown
	if len(client.scopes) > 0 {
		approval = socialhub.ApprovalRequired
		if hasReadScope(client.scopes) {
			approval = socialhub.ApprovalGranted
		}
	}
	readState := func(capability socialhub.Capability, reason, docURL string) socialhub.CapabilityState {
		return socialhub.CapabilityState{
			Capability: capability, Supported: true, Approval: approval,
			Scopes: []string{ScopeReadOnly}, Reason: reason, DocURL: docURL,
		}
	}
	return socialhub.Capabilities{
		CapabilityBlogRead:    readState(CapabilityBlogRead, "get blogs by ID or URL and list the authenticated user's blogs", documentationURL+"/blogs"),
		CapabilityPostRead:    readState(CapabilityPostRead, "get, list, and search Blogger posts", documentationURL+"/posts"),
		CapabilityCommentRead: readState(CapabilityCommentRead, "get and list comments by post or blog", documentationURL+"/comments"),
		CapabilityPageRead:    readState(CapabilityPageRead, "get and list static Blogger pages", documentationURL+"/pages"),
		socialhub.CapPublish:  {Capability: socialhub.CapPublish, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "insert, update, publish, revert, and delete state transitions are outside this read-only adapter"},
		socialhub.CapFetch:    {Capability: socialhub.CapFetch, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Blogger resources retain provider semantics through the typed Blogger workflow"},
		socialhub.CapMedia:    {Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Blogger API v3 does not define a media upload workflow in this surface"},
		socialhub.CapReact:    {Capability: socialhub.CapReact, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "comment moderation writes require the manage scope and are outside this adapter"},
		socialhub.CapMessage:  {Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Blogger API v3 does not expose direct messaging"},
		socialhub.CapWebhook:  {Capability: socialhub.CapWebhook, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Blogger API v3 does not document a signed webhook contract"},
	}, nil
}

func (client *Client) Publisher() (socialhub.Publisher, bool)           { return nil, false }
func (client *Client) Fetcher() (socialhub.Fetcher, bool)               { return nil, false }
func (client *Client) MediaUploader() (socialhub.MediaUploader, bool)   { return nil, false }
func (client *Client) Reactor() (socialhub.Reactor, bool)               { return nil, false }
func (client *Client) Messenger() (socialhub.Messenger, bool)           { return nil, false }
func (client *Client) WebhookHandler() (socialhub.WebhookHandler, bool) { return nil, false }
func (client *Client) Close() error                                     { return nil }

// Blogger returns the bounded provider-native read workflow.
func (client *Client) Blogger() ReadWorkflow { return client }

func (client *Client) requireReadScope(operation string) error {
	if len(client.scopes) == 0 || hasReadScope(client.scopes) {
		return nil
	}
	return &socialhub.Error{
		Code: socialhub.CodeApprovalRequired, Class: socialhub.ClassUserAction,
		Platform: platformName, Product: productName, Op: operation,
		RequiredScopes: []string{ScopeReadOnly}, ApprovalURL: authorizationURL,
		PlatformMessage: "configured approval scopes do not include a Blogger read scope",
	}
}

func hasReadScope(scopes []string) bool {
	for _, scope := range scopes {
		if scope == ScopeReadOnly || scope == ScopeManageBlogger {
			return true
		}
	}
	return false
}

var _ socialhub.Client = (*Client)(nil)
