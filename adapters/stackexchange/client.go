package stackexchange

import (
	"context"
	"strings"
	"sync"
	"time"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

// CapabilityQnA identifies Stack Exchange's question-and-answer workflow.
const CapabilityQnA socialhub.Capability = "stackexchange_qna"

// Client implements public reads, typed Q&A, and human-initiated interactions.
type Client struct {
	accountID socialhub.AccountID
	site      string
	userID    string
	api       *transport.Client
	hasToken  bool
	scopes    []string
	clock     socialhub.Clock
	userAgent string

	rateMu  sync.Mutex
	quota   Quota
	backoff map[string]time.Time
}

func (client *Client) Platform() socialhub.Platform { return "stackexchange" }
func (client *Client) Account() socialhub.AccountID { return client.accountID }

func (client *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	return socialhub.Capabilities{
		CapabilityQnA:        capabilityState(CapabilityQnA, true, client.hasToken, client.scopes, []string{"write_access"}, "public question search plus token-gated question and answer creation"),
		socialhub.CapPublish: {Capability: socialhub.CapPublish, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "use QnAWorkflow; common posts cannot express question titles, tags, or answer targets"},
		socialhub.CapFetch: {
			Capability: socialhub.CapFetch, Supported: true, Approval: socialhub.ApprovalGranted,
			Reason: "public users, posts, user questions, and post comments", DocURL: documentationURL,
		},
		socialhub.CapMedia:   {Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Stack Exchange API v2.3 does not expose media upload"},
		socialhub.CapReact:   capabilityState(socialhub.CapReact, client.hasToken, client.hasToken, client.scopes, []string{"write_access"}, "comments and explicit human-initiated upvote or undo"),
		socialhub.CapMessage: {Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Stack Exchange API v2.3 does not expose direct messaging"},
		socialhub.CapWebhook: {Capability: socialhub.CapWebhook, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Stack Exchange API v2.3 does not expose signed webhooks"},
	}, nil
}

func capabilityState(capability socialhub.Capability, supported, hasToken bool, granted, required []string, reason string) socialhub.CapabilityState {
	approval := socialhub.ApprovalUnknown
	if !hasToken {
		approval = socialhub.ApprovalRequired
	} else if len(granted) > 0 {
		approval = socialhub.ApprovalGranted
		for _, scope := range required {
			if !contains(granted, scope) {
				approval = socialhub.ApprovalRequired
				break
			}
		}
	}
	return socialhub.CapabilityState{
		Capability: capability, Supported: supported, Approval: approval, Scopes: append([]string(nil), required...),
		Reason: reason, DocURL: documentationURL,
	}
}

func (client *Client) Publisher() (socialhub.Publisher, bool)         { return nil, false }
func (client *Client) Fetcher() (socialhub.Fetcher, bool)             { return client, true }
func (client *Client) MediaUploader() (socialhub.MediaUploader, bool) { return nil, false }
func (client *Client) Reactor() (socialhub.Reactor, bool) {
	if !client.hasToken {
		return nil, false
	}
	return client, true
}
func (client *Client) Messenger() (socialhub.Messenger, bool)           { return nil, false }
func (client *Client) WebhookHandler() (socialhub.WebhookHandler, bool) { return nil, false }
func (client *Client) Close() error                                     { return nil }

// QnAWorkflow returns Stack Exchange question creation, answer creation, and search.
func (client *Client) QnAWorkflow() QnAWorkflow { return client }

// Quota returns the latest quota and backoff values observed in an API wrapper.
func (client *Client) Quota() Quota {
	client.rateMu.Lock()
	defer client.rateMu.Unlock()
	return client.quota
}

func (client *Client) requireWrite(operation string) error {
	if !client.hasToken {
		return &socialhub.Error{
			Code: socialhub.CodeUnauthenticated, Class: socialhub.ClassUserAction, Platform: "stackexchange", Product: productName,
			Op: operation, PlatformMessage: "configure access_token_ref with an OAuth user token",
		}
	}
	if len(client.scopes) == 0 || contains(client.scopes, "write_access") {
		return nil
	}
	return &socialhub.Error{
		Code: socialhub.CodeApprovalRequired, Class: socialhub.ClassUserAction, Platform: "stackexchange", Product: productName,
		Op: operation, RequiredScopes: []string{"write_access"}, ApprovalURL: documentationURL + "/authentication",
		PlatformMessage: "configured approval scopes do not include write_access",
	}
}

func (client *Client) requireActor(operation, actorID string) error {
	if actorID == "" {
		return nil
	}
	if client.userID == "" || actorID != client.userID {
		return invalidArgument(operation, "actor must be the configured Stack Exchange user ID")
	}
	return nil
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) == target {
			return true
		}
	}
	return false
}

var _ socialhub.Client = (*Client)(nil)
var _ socialhub.Fetcher = (*Client)(nil)
var _ socialhub.Reactor = (*Client)(nil)
