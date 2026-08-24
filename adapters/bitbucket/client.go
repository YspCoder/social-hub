package bitbucket

import (
	"context"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	CapabilityCurrentUserRead        socialhub.Capability = "bitbucket_current_user_read"
	CapabilityWorkspaceRead          socialhub.Capability = "bitbucket_workspace_read"
	CapabilityRepositoryRead         socialhub.Capability = "bitbucket_repository_read"
	CapabilityPullRequestRead        socialhub.Capability = "bitbucket_pull_request_read"
	CapabilityPullRequestCommentRead socialhub.Capability = "bitbucket_pull_request_comment_read"
)

// Client exposes typed Bitbucket Cloud read workflows for one credential.
type Client struct {
	accountID socialhub.AccountID
	scopes    []string
	api       *transport.Client
	clock     socialhub.Clock
}

func (client *Client) Platform() socialhub.Platform { return platformName }
func (client *Client) Account() socialhub.AccountID { return client.accountID }

func (client *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	readState := func(capability socialhub.Capability, scopes []string, reason, docURL string) socialhub.CapabilityState {
		approval := socialhub.ApprovalUnknown
		if len(client.scopes) > 0 {
			approval = socialhub.ApprovalRequired
			for _, scope := range scopes {
				if containsScope(client.scopes, scope) {
					approval = socialhub.ApprovalGranted
					break
				}
			}
		}
		return socialhub.CapabilityState{
			Capability: capability, Supported: true, Approval: approval,
			Scopes: append([]string(nil), scopes...), Reason: reason, DocURL: docURL,
		}
	}
	return socialhub.Capabilities{
		CapabilityCurrentUserRead:        readState(CapabilityCurrentUserRead, []string{"account", "read:user:bitbucket"}, "current credential owner profile reads", documentationURL+"api-group-users/#api-user-get"),
		CapabilityWorkspaceRead:          readState(CapabilityWorkspaceRead, []string{"account", "read:workspace:bitbucket"}, "accessible workspace discovery and workspace detail reads", documentationURL+"api-group-workspaces/"),
		CapabilityRepositoryRead:         readState(CapabilityRepositoryRead, []string{"repository", "read:repository:bitbucket"}, "repository discovery and detail reads within credential visibility", documentationURL+"api-group-repositories/"),
		CapabilityPullRequestRead:        readState(CapabilityPullRequestRead, []string{"pullrequest", "read:pullrequest:bitbucket"}, "pull request discovery and detail reads", documentationURL+"api-group-pullrequests/"),
		CapabilityPullRequestCommentRead: readState(CapabilityPullRequestCommentRead, []string{"pullrequest", "read:pullrequest:bitbucket"}, "pull request comment discovery and detail reads", documentationURL+"api-group-pullrequests/"),
		socialhub.CapPublish:             {Capability: socialhub.CapPublish, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "writes are outside this read-only adapter"},
		socialhub.CapFetch:               {Capability: socialhub.CapFetch, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Bitbucket entities retain provider semantics through typed workflows"},
		socialhub.CapMedia:               {Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "downloads and artifact upload are outside this adapter"},
		socialhub.CapReact:               {Capability: socialhub.CapReact, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "approvals and comment mutations are outside this read-only adapter"},
		socialhub.CapMessage:             {Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Bitbucket Cloud REST does not expose direct social messaging in this surface"},
		socialhub.CapWebhook:             {Capability: socialhub.CapWebhook, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "webhook management and verification are outside this adapter"},
	}, nil
}

func (client *Client) Publisher() (socialhub.Publisher, bool)           { return nil, false }
func (client *Client) Fetcher() (socialhub.Fetcher, bool)               { return nil, false }
func (client *Client) MediaUploader() (socialhub.MediaUploader, bool)   { return nil, false }
func (client *Client) Reactor() (socialhub.Reactor, bool)               { return nil, false }
func (client *Client) Messenger() (socialhub.Messenger, bool)           { return nil, false }
func (client *Client) WebhookHandler() (socialhub.WebhookHandler, bool) { return nil, false }
func (client *Client) Close() error                                     { return nil }

// Bitbucket returns the bounded provider-native read workflow.
func (client *Client) Bitbucket() ReadWorkflow { return client }

var _ socialhub.Client = (*Client)(nil)
