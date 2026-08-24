package tiktokresearch

import (
	"context"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	CapabilityVideoQuery  socialhub.Capability = "tiktok_research_video_query"
	CapabilityUserRead    socialhub.Capability = "tiktok_research_user_read"
	CapabilityCommentRead socialhub.Capability = "tiktok_research_comment_read"
)

const (
	videoQueryDocsURL = "https://developers.tiktok.com/doc/research-api-specs-query-videos"
	userInfoDocsURL   = "https://developers.tiktok.com/doc/research-api-specs-query-user-info"
	commentsDocsURL   = "https://developers.tiktok.com/doc/research-api-specs-query-video-comments"
)

// Client exposes typed TikTok Research API reads for one approved project.
type Client struct {
	accountID socialhub.AccountID
	approved  bool
	api       *transport.Client
	clock     socialhub.Clock
}

func (client *Client) Platform() socialhub.Platform { return platformName }
func (client *Client) Account() socialhub.AccountID { return client.accountID }

func (client *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	approval := socialhub.ApprovalRequired
	if client.approved {
		approval = socialhub.ApprovalGranted
	}
	researchState := func(capability socialhub.Capability, reason, docURL string) socialhub.CapabilityState {
		return socialhub.CapabilityState{
			Capability: capability, Supported: true, Approval: approval,
			Scopes: []string{RequiredScope}, Reason: reason, DocURL: docURL,
		}
	}
	return socialhub.Capabilities{
		CapabilityVideoQuery:  researchState(CapabilityVideoQuery, "query archived public videos for an approved non-commercial research project", videoQueryDocsURL),
		CapabilityUserRead:    researchState(CapabilityUserRead, "read public user profile fields for approved research", userInfoDocsURL),
		CapabilityCommentRead: researchState(CapabilityCommentRead, "list anonymized public video comments and replies for approved research", commentsDocsURL),
		socialhub.CapPublish:  {Capability: socialhub.CapPublish, Supported: false, Approval: socialhub.ApprovalRequired, Reason: "Research API v2 is read-only"},
		socialhub.CapFetch:    {Capability: socialhub.CapFetch, Supported: false, Approval: socialhub.ApprovalRequired, Reason: "research datasets use typed Research API workflows rather than the normalized social feed contract"},
		socialhub.CapMedia:    {Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalRequired, Reason: "Research API v2 does not upload media"},
		socialhub.CapReact:    {Capability: socialhub.CapReact, Supported: false, Approval: socialhub.ApprovalRequired, Reason: "Research API v2 does not create interactions"},
		socialhub.CapMessage:  {Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalRequired, Reason: "Research API v2 does not expose messaging"},
		socialhub.CapWebhook:  {Capability: socialhub.CapWebhook, Supported: false, Approval: socialhub.ApprovalRequired, Reason: "the implemented Research API is request/response based"},
	}, nil
}

func (client *Client) Publisher() (socialhub.Publisher, bool)           { return nil, false }
func (client *Client) Fetcher() (socialhub.Fetcher, bool)               { return nil, false }
func (client *Client) MediaUploader() (socialhub.MediaUploader, bool)   { return nil, false }
func (client *Client) Reactor() (socialhub.Reactor, bool)               { return nil, false }
func (client *Client) Messenger() (socialhub.Messenger, bool)           { return nil, false }
func (client *Client) WebhookHandler() (socialhub.WebhookHandler, bool) { return nil, false }
func (client *Client) Close() error                                     { return nil }

func (client *Client) Research() ResearchWorkflow { return client }

var _ socialhub.Client = (*Client)(nil)
