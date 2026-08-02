package listenbrainz

import (
	"context"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	CapabilityAuth          socialhub.Capability = "listenbrainz_auth"
	CapabilityListening     socialhub.Capability = "listenbrainz_listening"
	CapabilitySubmission    socialhub.Capability = "listenbrainz_submission"
	CapabilityFeedback      socialhub.Capability = "listenbrainz_feedback"
	CapabilityFeedbackWrite socialhub.Capability = "listenbrainz_feedback_write"
	CapabilityPlaylist      socialhub.Capability = "listenbrainz_playlist"
)

// Client exposes typed listening-history, feedback, and playlist workflows.
type Client struct {
	accountID socialhub.AccountID
	username  string
	token     string
	api       *transport.Client
}

func (c *Client) Platform() socialhub.Platform { return "listenbrainz" }
func (c *Client) Account() socialhub.AccountID { return c.accountID }

func (c *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	return socialhub.Capabilities{
		CapabilityAuth: tokenCapability(CapabilityAuth, c.token != "", "token validation requires a ListenBrainz user token"),
		CapabilityListening: {
			Capability: CapabilityListening, Supported: true, Approval: socialhub.ApprovalGranted,
			Reason: "public user search, listening history, playing-now, and listen-count reads", DocURL: documentationURL,
		},
		CapabilitySubmission: tokenCapability(CapabilitySubmission, c.token != "", "listen submission and deletion require a user token"),
		CapabilityFeedback: {
			Capability: CapabilityFeedback, Supported: true, Approval: socialhub.ApprovalGranted,
			Reason: "public recording-feedback reads", DocURL: documentationURL,
		},
		CapabilityFeedbackWrite: tokenCapability(CapabilityFeedbackWrite, c.token != "", "feedback submission requires a user token"),
		CapabilityPlaylist: {
			Capability: CapabilityPlaylist, Supported: true, Approval: socialhub.ApprovalGranted,
			Reason: "public JSPF playlist search and reads", DocURL: documentationURL,
		},
		socialhub.CapPublish: {Capability: socialhub.CapPublish, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "listens and playlists are not portable social posts"},
		socialhub.CapFetch:   {Capability: socialhub.CapFetch, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "ListenBrainz resources are exposed through typed workflows"},
		socialhub.CapMedia:   {Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "ListenBrainz stores listening metadata, not uploaded audio"},
		socialhub.CapReact:   {Capability: socialhub.CapReact, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "recording feedback is not mapped to portable post reactions"},
		socialhub.CapMessage: {Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "ListenBrainz does not expose direct messaging"},
		socialhub.CapWebhook: {Capability: socialhub.CapWebhook, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "ListenBrainz does not document signed user webhooks"},
	}, nil
}

func tokenCapability(name socialhub.Capability, available bool, reason string) socialhub.CapabilityState {
	state := socialhub.CapabilityState{
		Capability: name, Supported: true, Approval: socialhub.ApprovalGranted,
		Reason: reason, DocURL: documentationURL,
	}
	if !available {
		state.Approval = socialhub.ApprovalRequired
	}
	return state
}

func (c *Client) Publisher() (socialhub.Publisher, bool)           { return nil, false }
func (c *Client) Fetcher() (socialhub.Fetcher, bool)               { return nil, false }
func (c *Client) MediaUploader() (socialhub.MediaUploader, bool)   { return nil, false }
func (c *Client) Reactor() (socialhub.Reactor, bool)               { return nil, false }
func (c *Client) Messenger() (socialhub.Messenger, bool)           { return nil, false }
func (c *Client) WebhookHandler() (socialhub.WebhookHandler, bool) { return nil, false }
func (c *Client) Close() error                                     { return nil }

func (c *Client) AuthWorkflow() AuthWorkflow           { return c }
func (c *Client) ListeningWorkflow() ListeningWorkflow { return c }
func (c *Client) FeedbackWorkflow() FeedbackWorkflow   { return c }
func (c *Client) PlaylistWorkflow() PlaylistWorkflow   { return c }

func (c *Client) requireToken(operation string) error {
	if c.token != "" {
		return nil
	}
	return &socialhub.Error{
		Code: socialhub.CodeApprovalRequired, Class: socialhub.ClassUserAction,
		Platform: "listenbrainz", Product: productName, Op: operation,
		PlatformMessage: "a ListenBrainz user token is required", ApprovalURL: approvalURL,
	}
}

func (c *Client) resolveUsername(operation, username string) (string, error) {
	if username == "" {
		username = c.username
	}
	if !validUsername(username) {
		return "", invalidArgument(operation, "username is required and must be a bounded path-safe value")
	}
	return username, nil
}

var _ socialhub.Client = (*Client)(nil)
