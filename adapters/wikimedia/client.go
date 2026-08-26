package wikimedia

import (
	"context"
	"net/http"

	"social-hub/pkg/socialhub"
)

const (
	CapabilityKnowledgeSearch socialhub.Capability = "wikimedia_knowledge_search"
	CapabilityPageRead        socialhub.Capability = "wikimedia_page_read"
	CapabilityMediaRead       socialhub.Capability = "wikimedia_media_read"
)

// Client exposes anonymous MediaWiki REST v1 reads for one Wikimedia site.
type Client struct {
	accountID  socialhub.AccountID
	project    Project
	language   string
	baseURL    string
	userAgent  string
	httpClient *http.Client
	clock      socialhub.Clock
}

func (client *Client) Platform() socialhub.Platform { return platformName }
func (client *Client) Account() socialhub.AccountID { return client.accountID }

func (client *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	return socialhub.Capabilities{
		CapabilityKnowledgeSearch: {
			Capability: CapabilityKnowledgeSearch, Supported: true, Approval: socialhub.ApprovalGranted,
			Reason: "anonymous full-text page search on the configured Wikipedia or Commons wiki", DocURL: documentationURL,
		},
		CapabilityPageRead: {
			Capability: CapabilityPageRead, Supported: true, Approval: socialhub.ApprovalGranted,
			Reason: "anonymous page metadata and latest revision identity", DocURL: documentationURL,
		},
		CapabilityMediaRead: {
			Capability: CapabilityMediaRead, Supported: true, Approval: socialhub.ApprovalGranted,
			Reason: "anonymous page media, file metadata, and standard thumbnail derivatives", DocURL: documentationURL,
		},
		socialhub.CapPublish: {Capability: socialhub.CapPublish, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "write and OAuth contracts are intentionally excluded"},
		socialhub.CapFetch:   {Capability: socialhub.CapFetch, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "knowledge resources use typed MediaWiki workflows"},
		socialhub.CapMedia:   {Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "media is read-only and does not implement upload sessions"},
		socialhub.CapReact:   {Capability: socialhub.CapReact, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "reactions are outside MediaWiki REST page reads"},
		socialhub.CapMessage: {Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "messaging is outside MediaWiki REST page reads"},
		socialhub.CapWebhook: {Capability: socialhub.CapWebhook, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "MediaWiki REST v1 does not define signed webhooks for these resources"},
	}, nil
}

func (client *Client) Publisher() (socialhub.Publisher, bool)           { return nil, false }
func (client *Client) Fetcher() (socialhub.Fetcher, bool)               { return nil, false }
func (client *Client) MediaUploader() (socialhub.MediaUploader, bool)   { return nil, false }
func (client *Client) Reactor() (socialhub.Reactor, bool)               { return nil, false }
func (client *Client) Messenger() (socialhub.Messenger, bool)           { return nil, false }
func (client *Client) WebhookHandler() (socialhub.WebhookHandler, bool) { return nil, false }
func (client *Client) Close() error                                     { return nil }

// Knowledge returns the bounded MediaWiki REST v1 read workflow.
func (client *Client) Knowledge() KnowledgeWorkflow { return client }

var _ socialhub.Client = (*Client)(nil)
