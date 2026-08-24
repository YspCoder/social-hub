package github

import (
	"context"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	CapabilityUserRead         socialhub.Capability = "github_user_read"
	CapabilityRepositoryRead   socialhub.Capability = "github_repository_read"
	CapabilityIssueRead        socialhub.Capability = "github_issue_read"
	CapabilityIssueCommentRead socialhub.Capability = "github_issue_comment_read"
)

// Client exposes typed GitHub REST read workflows for one access token.
type Client struct {
	accountID socialhub.AccountID
	scopes    []string
	api       *transport.Client
	clock     socialhub.Clock
}

func (client *Client) Platform() socialhub.Platform { return platformName }
func (client *Client) Account() socialhub.AccountID { return client.accountID }

func (client *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	readState := func(capability socialhub.Capability, reason, docURL string) socialhub.CapabilityState {
		return socialhub.CapabilityState{
			Capability: capability, Supported: true, Approval: socialhub.ApprovalUnknown,
			Scopes: append([]string(nil), client.scopes...), Reason: reason, DocURL: docURL,
		}
	}
	return socialhub.Capabilities{
		CapabilityUserRead:         readState(CapabilityUserRead, "authenticated and public user profile reads; returned fields depend on token access", "https://docs.github.com/en/rest/users/users"),
		CapabilityRepositoryRead:   readState(CapabilityRepositoryRead, "repository discovery and detail reads within token visibility", "https://docs.github.com/en/rest/repos/repos"),
		CapabilityIssueRead:        readState(CapabilityIssueRead, "repository issue and pull-request issue representations within token visibility", "https://docs.github.com/en/rest/issues/issues"),
		CapabilityIssueCommentRead: readState(CapabilityIssueCommentRead, "issue and pull-request conversation comment reads within token visibility", "https://docs.github.com/en/rest/issues/comments"),
		socialhub.CapPublish:       {Capability: socialhub.CapPublish, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "writes are outside this read-only adapter"},
		socialhub.CapFetch:         {Capability: socialhub.CapFetch, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "GitHub users, repositories, issues, and comments retain provider semantics through typed workflows"},
		socialhub.CapMedia:         {Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "media and release-asset upload are outside this adapter"},
		socialhub.CapReact:         {Capability: socialhub.CapReact, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "reactions and issue mutations are outside this adapter"},
		socialhub.CapMessage:       {Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "GitHub REST does not expose direct social messaging in this surface"},
		socialhub.CapWebhook:       {Capability: socialhub.CapWebhook, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "webhook management and signature verification are outside this adapter"},
	}, nil
}

func (client *Client) Publisher() (socialhub.Publisher, bool)           { return nil, false }
func (client *Client) Fetcher() (socialhub.Fetcher, bool)               { return nil, false }
func (client *Client) MediaUploader() (socialhub.MediaUploader, bool)   { return nil, false }
func (client *Client) Reactor() (socialhub.Reactor, bool)               { return nil, false }
func (client *Client) Messenger() (socialhub.Messenger, bool)           { return nil, false }
func (client *Client) WebhookHandler() (socialhub.WebhookHandler, bool) { return nil, false }
func (client *Client) Close() error                                     { return nil }

// GitHub returns the bounded REST read workflow.
func (client *Client) GitHub() ReadWorkflow { return client }

var _ socialhub.Client = (*Client)(nil)
