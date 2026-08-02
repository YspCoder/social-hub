package deviantart

import (
	"context"
	"strings"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	CapabilityIdentity    socialhub.Capability = "deviantart_identity"
	CapabilityDeviations  socialhub.Capability = "deviantart_deviations"
	CapabilityStatuses    socialhub.Capability = "deviantart_statuses"
	CapabilityComments    socialhub.Capability = "deviantart_comments"
	CapabilityCollections socialhub.Capability = "deviantart_collections"
)

// Client exposes normalized DeviantArt operations and typed workflows.
type Client struct {
	accountID socialhub.AccountID
	username  string
	userID    string
	scopes    []string
	api       *transport.Client
	clock     socialhub.Clock
	userAgent string
}

func (client *Client) Platform() socialhub.Platform { return "deviantart" }
func (client *Client) Account() socialhub.AccountID { return client.accountID }

func (client *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	return socialhub.Capabilities{
		socialhub.CapPublish:  client.capability(socialhub.CapPublish, []string{"user.manage"}, "text Status publishing; Deviations require the separate Sta.sh workflow", documentationURL+"reference/user_statuses_post"),
		socialhub.CapFetch:    client.capability(socialhub.CapFetch, []string{"browse"}, "public profiles, Deviations, galleries, and Deviation comments", documentationURL+"reference"),
		socialhub.CapMedia:    {Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "DeviantArt media publishing is a two-stage Sta.sh submit and publish workflow"},
		socialhub.CapReact:    client.capability(socialhub.CapReact, []string{"browse", "comment.post", "collection"}, "Deviation comments and favourite toggling", documentationURL+"reference/comments_post_deviation"),
		socialhub.CapMessage:  {Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "DeviantArt notification feeds are not a direct-message contract"},
		socialhub.CapWebhook:  {Capability: socialhub.CapWebhook, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "DeviantArt API v1 does not expose signed webhooks"},
		CapabilityIdentity:    client.capability(CapabilityIdentity, []string{"basic", "user"}, "authorized-user identity", documentationURL+"reference/user_whoami"),
		CapabilityDeviations:  client.capability(CapabilityDeviations, []string{"browse"}, "Deviation, gallery, profile Post, and comment reads", documentationURL+"reference/deviation_single"),
		CapabilityStatuses:    client.capability(CapabilityStatuses, []string{"user.manage"}, "Status publishing", documentationURL+"reference/user_statuses_post"),
		CapabilityComments:    client.capability(CapabilityComments, []string{"browse", "comment.post"}, "Deviation comment reads and writes", documentationURL+"reference/comments_post_deviation"),
		CapabilityCollections: client.capability(CapabilityCollections, []string{"browse", "collection"}, "favourite and unfavourite Deviations", documentationURL+"reference/collections_fave"),
	}, nil
}

func (client *Client) capability(capability socialhub.Capability, required []string, reason, docURL string) socialhub.CapabilityState {
	approval := socialhub.ApprovalUnknown
	if len(client.scopes) > 0 {
		approval = socialhub.ApprovalGranted
		for _, scope := range required {
			if !scopeGranted(client.scopes, scope) {
				approval = socialhub.ApprovalRequired
				break
			}
		}
	}
	return socialhub.CapabilityState{
		Capability: capability, Supported: client.api != nil, Approval: approval,
		Scopes: append([]string(nil), required...), Reason: reason, DocURL: docURL,
	}
}

func (client *Client) Publisher() (socialhub.Publisher, bool)         { return client, client.api != nil }
func (client *Client) Fetcher() (socialhub.Fetcher, bool)             { return client, client.api != nil }
func (client *Client) MediaUploader() (socialhub.MediaUploader, bool) { return nil, false }
func (client *Client) Reactor() (socialhub.Reactor, bool)             { return client, client.api != nil }
func (client *Client) Messenger() (socialhub.Messenger, bool)         { return nil, false }
func (client *Client) WebhookHandler() (socialhub.WebhookHandler, bool) {
	return nil, false
}
func (client *Client) Close() error { return nil }

func (client *Client) UserWorkflow() UserWorkflow             { return client }
func (client *Client) DeviationWorkflow() DeviationWorkflow   { return client }
func (client *Client) GalleryWorkflow() GalleryWorkflow       { return client }
func (client *Client) StatusWorkflow() StatusWorkflow         { return client }
func (client *Client) CommentWorkflow() CommentWorkflow       { return client }
func (client *Client) CollectionWorkflow() CollectionWorkflow { return client }

func (client *Client) requireAPI(operation string) (*transport.Client, error) {
	if client.api == nil {
		return nil, &socialhub.Error{
			Code: socialhub.CodeUnauthenticated, Class: socialhub.ClassUserAction, Platform: "deviantart", Product: productName,
			Op: operation, PlatformMessage: "configure access_token_ref with a DeviantArt OAuth token",
		}
	}
	return client.api, nil
}

func (client *Client) requireScopes(operation string, required ...string) error {
	if len(client.scopes) == 0 {
		return nil
	}
	missing := make([]string, 0, len(required))
	for _, scope := range required {
		if !scopeGranted(client.scopes, scope) {
			missing = append(missing, scope)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	return &socialhub.Error{
		Code: socialhub.CodeApprovalRequired, Class: socialhub.ClassUserAction, Platform: "deviantart", Product: productName,
		Op: operation, RequiredScopes: missing, ApprovalURL: defaultAuthURL,
		PlatformMessage: "configured approval scopes do not include required DeviantArt permissions",
	}
}

func (client *Client) validActor(actor string) bool {
	actor = strings.TrimSpace(actor)
	return actor == "" || client.userID != "" && actor == client.userID
}

var _ socialhub.Client = (*Client)(nil)
var _ socialhub.Publisher = (*Client)(nil)
var _ socialhub.Fetcher = (*Client)(nil)
var _ socialhub.Reactor = (*Client)(nil)
