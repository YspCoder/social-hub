package lemmy

import (
	"context"
	"sync"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	CapabilityPosts           socialhub.Capability = "lemmy_posts"
	CapabilityVotes           socialhub.Capability = "lemmy_votes"
	CapabilityPrivateMessages socialhub.Capability = "lemmy_private_messages"
)

// Client operates as one user on one Lemmy instance.
type Client struct {
	accountID socialhub.AccountID
	baseURL   string
	username  string
	api       *transport.Client
	clock     socialhub.Clock

	uploadMu sync.Mutex
	uploads  map[string]*uploadState
	media    map[string]socialhub.Media
}

func (client *Client) Platform() socialhub.Platform { return "lemmy" }
func (client *Client) Account() socialhub.AccountID { return client.accountID }

func (client *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	return socialhub.Capabilities{
		socialhub.CapPublish: {
			Capability: socialhub.CapPublish, Supported: false, Approval: socialhub.ApprovalUnknown,
			Reason: "Lemmy posts require a title and community_id; use PostWorkflow", DocURL: documentationURL,
		},
		socialhub.CapFetch: {
			Capability: socialhub.CapFetch, Supported: true, Approval: socialhub.ApprovalGranted,
			Reason: "people, posts, account/person post pages, and post comments", DocURL: documentationURL,
		},
		socialhub.CapMedia: {
			Capability: socialhub.CapMedia, Supported: true, Approval: socialhub.ApprovalGranted,
			Reason: "single-part image upload through the instance Pictrs endpoint", DocURL: documentationURL,
		},
		socialhub.CapReact: {
			Capability: socialhub.CapReact, Supported: true, Approval: socialhub.ApprovalGranted,
			Reason: "post likes, vote removal, comment creation, and comment deletion", DocURL: documentationURL,
		},
		socialhub.CapMessage: {
			Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown,
			Reason: "API v3 has private-message list and mutation endpoints but no direct get-by-ID endpoint; use PrivateMessageWorkflow", DocURL: documentationURL,
		},
		socialhub.CapWebhook: {
			Capability: socialhub.CapWebhook, Supported: false, Approval: socialhub.ApprovalUnknown,
			Reason: "Lemmy API v3 does not publish a signed inbound webhook contract", DocURL: documentationURL,
		},
		CapabilityPosts: {
			Capability: CapabilityPosts, Supported: true, Approval: socialhub.ApprovalGranted,
			Reason: "typed community post create, update, delete, lookup, and cursor-paginated feeds", DocURL: documentationURL,
		},
		CapabilityVotes: {
			Capability: CapabilityVotes, Supported: true, Approval: socialhub.ApprovalGranted,
			Reason: "typed -1, 0, and 1 votes for posts and comments", DocURL: documentationURL,
		},
		CapabilityPrivateMessages: {
			Capability: CapabilityPrivateMessages, Supported: true, Approval: socialhub.ApprovalGranted,
			Reason: "typed private-message send, list, edit, delete, and read-state operations", DocURL: documentationURL,
		},
	}, nil
}

func (client *Client) Publisher() (socialhub.Publisher, bool)         { return nil, false }
func (client *Client) Fetcher() (socialhub.Fetcher, bool)             { return client, true }
func (client *Client) MediaUploader() (socialhub.MediaUploader, bool) { return client, true }
func (client *Client) Reactor() (socialhub.Reactor, bool)             { return client, true }
func (client *Client) Messenger() (socialhub.Messenger, bool)         { return nil, false }
func (client *Client) WebhookHandler() (socialhub.WebhookHandler, bool) {
	return nil, false
}
func (client *Client) Close() error { return nil }

func (client *Client) PostWorkflow() PostWorkflow                     { return client }
func (client *Client) VoteWorkflow() VoteWorkflow                     { return client }
func (client *Client) PrivateMessageWorkflow() PrivateMessageWorkflow { return client }

var _ socialhub.Client = (*Client)(nil)
var _ socialhub.Fetcher = (*Client)(nil)
var _ socialhub.MediaUploader = (*Client)(nil)
var _ socialhub.Reactor = (*Client)(nil)
var _ PostWorkflow = (*Client)(nil)
var _ VoteWorkflow = (*Client)(nil)
var _ PrivateMessageWorkflow = (*Client)(nil)
