package producthunt

import (
	"context"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	CapabilityPostRead       socialhub.Capability = "producthunt_post_read"
	CapabilityTopicRead      socialhub.Capability = "producthunt_topic_read"
	CapabilityCollectionRead socialhub.Capability = "producthunt_collection_read"
	CapabilityCommentRead    socialhub.Capability = "producthunt_comment_read"
	CapabilityUserRead       socialhub.Capability = "producthunt_user_read"
)

// Client exposes typed Product Hunt read workflows for one access token.
type Client struct {
	accountID   socialhub.AccountID
	scopes      []string
	api         *transport.Client
	accessToken string
}

func (client *Client) Platform() socialhub.Platform { return platformName }
func (client *Client) Account() socialhub.AccountID { return client.accountID }

func (client *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	publicApproval := socialhub.ApprovalGranted
	if len(client.scopes) > 0 && !containsScope(client.scopes, "public") {
		publicApproval = socialhub.ApprovalRequired
	}
	publicState := func(capability socialhub.Capability, reason string) socialhub.CapabilityState {
		return socialhub.CapabilityState{
			Capability: capability, Supported: true, Approval: publicApproval,
			Scopes: []string{"public"}, Reason: reason, DocURL: graphQLDocsURL,
		}
	}
	return socialhub.Capabilities{
		CapabilityPostRead:       publicState(CapabilityPostRead, "public post discovery and detail reads"),
		CapabilityTopicRead:      publicState(CapabilityTopicRead, "public topic discovery and detail reads"),
		CapabilityCollectionRead: publicState(CapabilityCollectionRead, "published collection discovery and detail reads"),
		CapabilityCommentRead:    publicState(CapabilityCommentRead, "public comment and post-comment reads"),
		CapabilityUserRead:       publicState(CapabilityUserRead, "public profile reads; Viewer requires user context"),
		socialhub.CapPublish:     {Capability: socialhub.CapPublish, Supported: false, Approval: socialhub.ApprovalRequired, Reason: "third-party writes require Product Hunt approval and are outside this read adapter"},
		socialhub.CapFetch:       {Capability: socialhub.CapFetch, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Product Hunt entities are exposed through typed GraphQL workflows"},
		socialhub.CapMedia:       {Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Product Hunt API v2 does not document media upload for this surface"},
		socialhub.CapReact:       {Capability: socialhub.CapReact, Supported: false, Approval: socialhub.ApprovalRequired, Reason: "write mutations require Product Hunt approval and are outside this read adapter"},
		socialhub.CapMessage:     {Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Product Hunt API v2 does not expose direct messaging"},
		socialhub.CapWebhook:     {Capability: socialhub.CapWebhook, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Product Hunt API v2 does not document signed first-party webhooks"},
	}, nil
}

func (client *Client) Publisher() (socialhub.Publisher, bool)           { return nil, false }
func (client *Client) Fetcher() (socialhub.Fetcher, bool)               { return nil, false }
func (client *Client) MediaUploader() (socialhub.MediaUploader, bool)   { return nil, false }
func (client *Client) Reactor() (socialhub.Reactor, bool)               { return nil, false }
func (client *Client) Messenger() (socialhub.Messenger, bool)           { return nil, false }
func (client *Client) WebhookHandler() (socialhub.WebhookHandler, bool) { return nil, false }
func (client *Client) Close() error                                     { return nil }

func (client *Client) ProductHunt() ReadWorkflow { return client }

var _ socialhub.Client = (*Client)(nil)
