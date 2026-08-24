package wechatstore

import (
	"context"
	"net/http"
	"sync"
	"time"

	"social-hub/pkg/socialhub"
)

const (
	CapabilityStableAccessToken socialhub.Capability = "wechat_store_stable_access_token"
	CapabilityStoreInfoRead     socialhub.Capability = "wechat_store_info_read"
	CapabilityProductRead       socialhub.Capability = "wechat_store_product_read"
)

// Client exposes typed, read-only workflows for one self-managed WeChat Store.
type Client struct {
	accountID  socialhub.AccountID
	httpClient *http.Client
	clock      socialhub.Clock

	mu               sync.Mutex
	appID            string
	appSecret        string
	token            StableAccessToken
	lastForceRefresh time.Time
	closed           bool
}

func (client *Client) Platform() socialhub.Platform { return platformName }
func (client *Client) Account() socialhub.AccountID { return client.accountID }

func (client *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	return socialhub.Capabilities{
		CapabilityStableAccessToken: {
			Capability: CapabilityStableAccessToken, Supported: true, Approval: socialhub.ApprovalGranted,
			Reason: "stable access-token retrieval for a self-managed store AppID",
			DocURL: stableTokenDocumentationURL,
		},
		CapabilityStoreInfoRead: {
			Capability: CapabilityStoreInfoRead, Supported: true, Approval: socialhub.ApprovalRequired,
			Reason: "the AppID must belong to an eligible WeChat Store with self-managed API access",
			DocURL: storeInfoDocumentationURL,
		},
		CapabilityProductRead: {
			Capability: CapabilityProductRead, Supported: true, Approval: socialhub.ApprovalRequired,
			Reason: "product reads remain subject to store, category, product, brand, and platform permission state",
			DocURL: productDocumentationURL,
		},
		socialhub.CapPublish: {Capability: socialhub.CapPublish, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "product and store writes are outside this adapter"},
		socialhub.CapFetch:   {Capability: socialhub.CapFetch, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "store catalog reads use typed commerce workflows rather than the normalized social feed contract"},
		socialhub.CapMedia:   {Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "media upload is outside this adapter"},
		socialhub.CapReact:   {Capability: socialhub.CapReact, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "social reactions are outside this adapter"},
		socialhub.CapMessage: {Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "customer service and messaging are outside this adapter"},
		socialhub.CapWebhook: {Capability: socialhub.CapWebhook, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "store event callbacks are outside this adapter"},
	}, nil
}

func (client *Client) Publisher() (socialhub.Publisher, bool)           { return nil, false }
func (client *Client) Fetcher() (socialhub.Fetcher, bool)               { return nil, false }
func (client *Client) MediaUploader() (socialhub.MediaUploader, bool)   { return nil, false }
func (client *Client) Reactor() (socialhub.Reactor, bool)               { return nil, false }
func (client *Client) Messenger() (socialhub.Messenger, bool)           { return nil, false }
func (client *Client) WebhookHandler() (socialhub.WebhookHandler, bool) { return nil, false }

func (client *Client) Credentials() CredentialsWorkflow { return client }
func (client *Client) Store() StoreWorkflow             { return client }
func (client *Client) Catalog() CatalogWorkflow         { return client }

// Close discards the client's in-memory copies of credentials and cached token.
func (client *Client) Close() error {
	client.mu.Lock()
	defer client.mu.Unlock()
	client.closed = true
	client.appID, client.appSecret = "", ""
	client.token = StableAccessToken{}
	return nil
}

var _ socialhub.Client = (*Client)(nil)
