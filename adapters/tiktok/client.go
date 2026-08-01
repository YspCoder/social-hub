package tiktok

import (
	"context"
	"net/http"
	"sync"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

// Client implements TikTok Display API and webhook capabilities.
type Client struct {
	accountID     socialhub.AccountID
	openID        string
	clientKey     string
	transport     *transport.Client
	httpClient    *http.Client
	scopes        []string
	webhookSecret string
	clock         socialhub.Clock
	uploadMu      sync.Mutex
	uploads       map[string]*videoUpload
	content       *ContentService
}

func (c *Client) Platform() socialhub.Platform { return "tiktok" }
func (c *Client) Account() socialhub.AccountID { return c.accountID }

func (c *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	webhookSupported := c.webhookSecret != ""
	webhookReason := "configure secret_ref or webhook.secret_ref to verify TikTok-Signature"
	if webhookSupported {
		webhookReason = "TikTok webhook payloads are HMAC verified with replay protection"
	}
	return socialhub.Capabilities{
		CapabilityContentPosting: capabilityState(CapabilityContentPosting, true, c.scopes, []string{"video.publish"}, "use CreatorInfo, InitVideo, sequential UploadChunk, and Status", docURL+"content-posting-api-reference-direct-post"),
		socialhub.CapPublish:     {Capability: socialhub.CapPublish, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "TikTok posting requires creator choices and asynchronous media transfer; use ContentWorkflow"},
		socialhub.CapFetch:       capabilityState(socialhub.CapFetch, true, c.scopes, []string{"user.info.basic", "video.list"}, "Display API reads only the authorized user's profile and public videos", docURL+"display-api-overview"),
		socialhub.CapMedia:       {Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "video transfer is part of ContentWorkflow and is not an independent reusable media upload"},
		socialhub.CapReact:       {Capability: socialhub.CapReact, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "TikTok for Developers does not expose general like or comment mutations"},
		socialhub.CapMessage:     {Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "TikTok for Developers does not expose general direct messaging"},
		socialhub.CapWebhook:     {Capability: socialhub.CapWebhook, Supported: webhookSupported, Approval: socialhub.ApprovalUnknown, Reason: webhookReason, DocURL: docURL + "webhooks-overview"},
	}, nil
}

func capabilityState(capability socialhub.Capability, supported bool, granted, required []string, reason, documentation string) socialhub.CapabilityState {
	approval := socialhub.ApprovalUnknown
	if len(granted) > 0 {
		approval = socialhub.ApprovalGranted
		for _, scope := range required {
			if !contains(granted, scope) {
				approval = socialhub.ApprovalRequired
				break
			}
		}
	}
	return socialhub.CapabilityState{Capability: capability, Supported: supported, Approval: approval, Scopes: required, Reason: reason, DocURL: documentation}
}

func (c *Client) Publisher() (socialhub.Publisher, bool)         { return nil, false }
func (c *Client) Fetcher() (socialhub.Fetcher, bool)             { return c, true }
func (c *Client) MediaUploader() (socialhub.MediaUploader, bool) { return nil, false }
func (c *Client) Reactor() (socialhub.Reactor, bool)             { return nil, false }
func (c *Client) Messenger() (socialhub.Messenger, bool)         { return nil, false }
func (c *Client) WebhookHandler() (socialhub.WebhookHandler, bool) {
	if c.webhookSecret == "" {
		return nil, false
	}
	return c, true
}
func (c *Client) Close() error { return nil }

// ContentWorkflow returns TikTok's creator-aware asynchronous posting flow.
func (c *Client) ContentWorkflow() ContentWorkflow { return c.content }

func (c *Client) requireScope(operation, scope string) error {
	if len(c.scopes) == 0 || contains(c.scopes, scope) {
		return nil
	}
	return &socialhub.Error{
		Code: socialhub.CodeApprovalRequired, Class: socialhub.ClassUserAction, Platform: "tiktok", Product: "tiktok-for-developers", Op: operation,
		RequiredScopes: []string{scope}, ApprovalURL: "https://developers.tiktok.com/apps/", PlatformMessage: "configured approval scopes do not include " + scope,
	}
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

var _ socialhub.Client = (*Client)(nil)
var _ socialhub.Fetcher = (*Client)(nil)
