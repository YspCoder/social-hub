package gitlab

import (
	"context"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	CapabilityUserRead    socialhub.Capability = "gitlab_user_read"
	CapabilityProjectRead socialhub.Capability = "gitlab_project_read"
	CapabilityIssueRead   socialhub.Capability = "gitlab_issue_read"
	CapabilityNoteRead    socialhub.Capability = "gitlab_issue_note_read"
)

// Client exposes typed GitLab REST v4 read workflows for one access token.
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
		CapabilityUserRead:    readState(CapabilityUserRead, "current and signed-in-visible user profile reads", "https://docs.gitlab.com/api/users/"),
		CapabilityProjectRead: readState(CapabilityProjectRead, "project discovery and detail reads within token visibility", "https://docs.gitlab.com/api/projects/"),
		CapabilityIssueRead:   readState(CapabilityIssueRead, "project issue reads including confidential issues visible to the token", "https://docs.gitlab.com/api/issues/"),
		CapabilityNoteRead:    readState(CapabilityNoteRead, "project issue note and comment reads within token visibility", "https://docs.gitlab.com/api/notes/"),
		socialhub.CapPublish:  {Capability: socialhub.CapPublish, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "writes are outside this read-only adapter"},
		socialhub.CapFetch:    {Capability: socialhub.CapFetch, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "GitLab users, projects, issues, and notes retain provider semantics through typed workflows"},
		socialhub.CapMedia:    {Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "uploads and package or release assets are outside this adapter"},
		socialhub.CapReact:    {Capability: socialhub.CapReact, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "award emoji and issue mutations are outside this adapter"},
		socialhub.CapMessage:  {Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "GitLab REST v4 does not expose direct social messaging in this surface"},
		socialhub.CapWebhook:  {Capability: socialhub.CapWebhook, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "webhook management and verification are outside this adapter"},
	}, nil
}

func (client *Client) Publisher() (socialhub.Publisher, bool)           { return nil, false }
func (client *Client) Fetcher() (socialhub.Fetcher, bool)               { return nil, false }
func (client *Client) MediaUploader() (socialhub.MediaUploader, bool)   { return nil, false }
func (client *Client) Reactor() (socialhub.Reactor, bool)               { return nil, false }
func (client *Client) Messenger() (socialhub.Messenger, bool)           { return nil, false }
func (client *Client) WebhookHandler() (socialhub.WebhookHandler, bool) { return nil, false }
func (client *Client) Close() error                                     { return nil }

// GitLab returns the bounded REST v4 read workflow.
func (client *Client) GitLab() ReadWorkflow { return client }

var _ socialhub.Client = (*Client)(nil)
