package forem

import (
	"context"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	CapabilityArticles  socialhub.Capability = "forem_articles"
	CapabilityReactions socialhub.Capability = "forem_reactions"
)

// Client operates on one Forem account and instance.
type Client struct {
	accountID socialhub.AccountID
	baseURL   string
	api       *transport.Client
	clock     socialhub.Clock
}

func (client *Client) Platform() socialhub.Platform { return "forem" }
func (client *Client) Account() socialhub.AccountID { return client.accountID }

func (client *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	return socialhub.Capabilities{
		socialhub.CapPublish: {
			Capability: socialhub.CapPublish, Supported: false, Approval: socialhub.ApprovalUnknown,
			Reason: "Forem article publishing requires a title and article metadata; use ArticleWorkflow", DocURL: documentationURL,
		},
		socialhub.CapFetch: {
			Capability: socialhub.CapFetch, Supported: true, Approval: socialhub.ApprovalGranted,
			Reason: "users, articles, account article pages, and threaded article comments", DocURL: documentationURL,
		},
		socialhub.CapMedia: {
			Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown,
			Reason: "Forem API V1 accepts remote article image URLs but has no public media upload endpoint", DocURL: documentationURL,
		},
		socialhub.CapReact: {
			Capability: socialhub.CapReact, Supported: true, Approval: socialhub.ApprovalGranted,
			Reason: "idempotent Article likes; other categories and target types use ReactionWorkflow", DocURL: documentationURL,
		},
		socialhub.CapMessage: {
			Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown,
			Reason: "Forem API V1 exposes no direct messaging contract", DocURL: documentationURL,
		},
		socialhub.CapWebhook: {
			Capability: socialhub.CapWebhook, Supported: false, Approval: socialhub.ApprovalUnknown,
			Reason: "Forem API V1 does not document a signed inbound webhook verification contract", DocURL: documentationURL,
		},
		CapabilityArticles: {
			Capability: CapabilityArticles, Supported: true, Approval: socialhub.ApprovalGranted,
			Reason: "typed article creation, updates, draft/published listing, and unpublish", DocURL: documentationURL,
		},
		CapabilityReactions: {
			Capability: CapabilityReactions, Supported: true, Approval: socialhub.ApprovalGranted,
			Reason: "typed Forem reaction categories across Article, Comment, and User targets", DocURL: documentationURL,
		},
	}, nil
}

func (client *Client) Publisher() (socialhub.Publisher, bool)         { return nil, false }
func (client *Client) Fetcher() (socialhub.Fetcher, bool)             { return client, true }
func (client *Client) MediaUploader() (socialhub.MediaUploader, bool) { return nil, false }
func (client *Client) Reactor() (socialhub.Reactor, bool)             { return client, true }
func (client *Client) Messenger() (socialhub.Messenger, bool)         { return nil, false }
func (client *Client) WebhookHandler() (socialhub.WebhookHandler, bool) {
	return nil, false
}
func (client *Client) Close() error { return nil }

func (client *Client) ArticleWorkflow() ArticleWorkflow   { return client }
func (client *Client) ReactionWorkflow() ReactionWorkflow { return client }

var _ socialhub.Client = (*Client)(nil)
var _ socialhub.Fetcher = (*Client)(nil)
var _ socialhub.Reactor = (*Client)(nil)
var _ ArticleWorkflow = (*Client)(nil)
var _ ReactionWorkflow = (*Client)(nil)
