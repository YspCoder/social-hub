package discourse

import (
	"context"
	"strings"
	"sync"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	CapabilityTopics          socialhub.Capability = "discourse_topics"
	CapabilityPrivateMessages socialhub.Capability = "discourse_private_messages"
)

// Client operates on one Discourse instance as one configured API user.
type Client struct {
	accountID     socialhub.AccountID
	baseURL       string
	apiUsername   string
	api           *transport.Client
	webhookSecret string
	clock         socialhub.Clock

	uploadMu sync.Mutex
	uploads  map[string]*uploadState
	media    map[string]socialhub.Media
}

func (client *Client) Platform() socialhub.Platform { return "discourse" }
func (client *Client) Account() socialhub.AccountID { return client.accountID }

func (client *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	apiApproval := socialhub.ApprovalUnknown
	if client.api != nil {
		apiApproval = socialhub.ApprovalGranted
	}
	webhookApproval := socialhub.ApprovalUnknown
	if client.webhookSecret != "" {
		webhookApproval = socialhub.ApprovalGranted
	}
	return socialhub.Capabilities{
		socialhub.CapPublish: {
			Capability: socialhub.CapPublish, Supported: client.api != nil, Approval: apiApproval,
			Reason: "replies to existing Discourse posts; new topics use TopicWorkflow", DocURL: documentationURL,
		},
		socialhub.CapFetch: {
			Capability: socialhub.CapFetch, Supported: client.api != nil, Approval: apiApproval,
			Reason: "users, individual posts, and post replies; the site-wide latest feed uses TopicWorkflow", DocURL: documentationURL,
		},
		socialhub.CapMedia: {
			Capability: socialhub.CapMedia, Supported: client.api != nil, Approval: apiApproval,
			Reason: "synchronous composer uploads", DocURL: documentationURL,
		},
		socialhub.CapReact: {
			Capability: socialhub.CapReact, Supported: client.api != nil, Approval: apiApproval,
			Reason: "post replies, deletion, and likes; unlike is absent from the official OpenAPI contract", DocURL: documentationURL,
		},
		socialhub.CapMessage: {
			Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown,
			Reason: "Discourse private messages are topics and use PrivateMessageWorkflow", DocURL: documentationURL,
		},
		socialhub.CapWebhook: {
			Capability: socialhub.CapWebhook, Supported: client.webhookSecret != "", Approval: webhookApproval,
			Reason: "HMAC-SHA256 verification for Discourse webhook events", DocURL: documentationURL,
		},
		CapabilityTopics: {
			Capability: CapabilityTopics, Supported: client.api != nil, Approval: apiApproval,
			Reason: "typed topic creation, retrieval, and latest-post feed", DocURL: documentationURL,
		},
		CapabilityPrivateMessages: {
			Capability: CapabilityPrivateMessages, Supported: client.api != nil, Approval: apiApproval,
			Reason: "typed private-message topic creation", DocURL: documentationURL,
		},
	}, nil
}

func (client *Client) Publisher() (socialhub.Publisher, bool) {
	if client.api == nil {
		return nil, false
	}
	return client, true
}
func (client *Client) Fetcher() (socialhub.Fetcher, bool) {
	if client.api == nil {
		return nil, false
	}
	return client, true
}
func (client *Client) MediaUploader() (socialhub.MediaUploader, bool) {
	if client.api == nil {
		return nil, false
	}
	return client, true
}
func (client *Client) Reactor() (socialhub.Reactor, bool) {
	if client.api == nil {
		return nil, false
	}
	return client, true
}
func (client *Client) Messenger() (socialhub.Messenger, bool) { return nil, false }
func (client *Client) WebhookHandler() (socialhub.WebhookHandler, bool) {
	if client.webhookSecret == "" {
		return nil, false
	}
	return client, true
}
func (client *Client) Close() error { return nil }

func (client *Client) TopicWorkflow() TopicWorkflow { return client }

func (client *Client) PrivateMessageWorkflow() PrivateMessageWorkflow { return client }

func (client *Client) requireAPI(operation string) (*transport.Client, error) {
	if client.api == nil {
		return nil, &socialhub.Error{
			Code: socialhub.CodeUnauthenticated, Class: socialhub.ClassUserAction, Platform: "discourse", Product: productName,
			Op: operation, PlatformMessage: "configure access_token_ref with a Discourse API key",
		}
	}
	return client.api, nil
}

func validUsername(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > 512 || strings.ContainsAny(value, "/\\?#\x00\r\n") {
		return false
	}
	for _, character := range value {
		if character < 0x20 || character == 0x7f {
			return false
		}
	}
	return true
}

var _ socialhub.Client = (*Client)(nil)
var _ socialhub.Publisher = (*Client)(nil)
var _ socialhub.Fetcher = (*Client)(nil)
var _ socialhub.MediaUploader = (*Client)(nil)
var _ socialhub.Reactor = (*Client)(nil)
var _ socialhub.WebhookHandler = (*Client)(nil)
var _ TopicWorkflow = (*Client)(nil)
var _ PrivateMessageWorkflow = (*Client)(nil)
