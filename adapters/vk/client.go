package vk

import (
	"context"
	"net/http"
	"sync"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	CapabilityWall     socialhub.Capability = "vk_wall"
	CapabilityCallback socialhub.Capability = "vk_callback"
)

// Client implements the VK capabilities supported by one configured token.
type Client struct {
	accountID        socialhub.AccountID
	ownerID          int64
	groupID          int64
	tokenKind        TokenKind
	api              *transport.Client
	httpClient       *http.Client
	clock            socialhub.Clock
	callbackSecret   string
	allowHTTPUploads bool

	uploadMu sync.Mutex
	uploads  map[string]*uploadState
	media    map[string]socialhub.Media
}

func (c *Client) Platform() socialhub.Platform { return "vk" }
func (c *Client) Account() socialhub.AccountID { return c.accountID }

func (c *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	publish := c.tokenKind == TokenUser
	media := c.tokenKind == TokenUser
	react := c.tokenKind == TokenUser || c.tokenKind == TokenCommunity
	message := c.tokenKind == TokenUser || c.tokenKind == TokenCommunity
	webhook := c.groupID > 0 && c.callbackSecret != ""
	return socialhub.Capabilities{
		socialhub.CapPublish: {Capability: socialhub.CapPublish, Supported: publish, Approval: socialhub.ApprovalUnknown, Reason: tokenReason(publish, "wall publishing and deletion require a user access token", "user-token wall publishing, reposts, and deletion"), DocURL: methodDoc("wall.post")},
		socialhub.CapFetch:   capability(socialhub.CapFetch, "public users, communities, walls, posts, and comments visible to the configured token", "wall.get"),
		socialhub.CapMedia:   {Capability: socialhub.CapMedia, Supported: media, Approval: socialhub.ApprovalUnknown, Reason: tokenReason(media, "wall photo upload requires a user access token", "three-step wall photo upload using a server-owned upload URL"), DocURL: methodDoc("photos.getWallUploadServer")},
		socialhub.CapReact:   {Capability: socialhub.CapReact, Supported: react, Approval: socialhub.ApprovalUnknown, Reason: tokenReason(react, "service tokens cannot create interactions", "likes require a user token; wall comments also accept community tokens"), DocURL: methodDoc("likes.add")},
		socialhub.CapMessage: {Capability: socialhub.CapMessage, Supported: message, Approval: socialhub.ApprovalUnknown, Reason: tokenReason(message, "service tokens cannot access messages", "messages.send and messages.getById; community messaging must be enabled in VK"), DocURL: methodDoc("messages.send")},
		socialhub.CapWebhook: {Capability: socialhub.CapWebhook, Supported: webhook, Approval: socialhub.ApprovalUnknown, Reason: tokenReason(webhook, "configure a community owner_id and webhook.secret_ref", "Callback API body-secret verification and event decoding"), DocURL: "https://dev.vk.ru/ru/api/callback/getting-started"},
		CapabilityWall:       {Capability: CapabilityWall, Supported: publish, Approval: socialhub.ApprovalUnknown, Reason: tokenReason(publish, "typed wall publishing and repost controls require a user access token", "typed wall publishing and repost controls"), DocURL: methodDoc("wall.post")},
		CapabilityCallback:   {Capability: CapabilityCallback, Supported: c.groupID > 0, Approval: socialhub.ApprovalUnknown, Reason: tokenReason(c.groupID > 0, "a community owner_id is required", "Callback API confirmation-code workflow"), DocURL: methodDoc("groups.getCallbackConfirmationCode")},
	}, nil
}

func capability(name socialhub.Capability, reason, method string) socialhub.CapabilityState {
	return socialhub.CapabilityState{Capability: name, Supported: true, Approval: socialhub.ApprovalUnknown, Reason: reason, DocURL: methodDoc(method)}
}

func tokenReason(supported bool, unavailable, available string) string {
	if supported {
		return available
	}
	return unavailable
}

func methodDoc(method string) string { return "https://dev.vk.ru/ru/method/" + method }

func (c *Client) Publisher() (socialhub.Publisher, bool) { return c, c.tokenKind == TokenUser }
func (c *Client) Fetcher() (socialhub.Fetcher, bool)     { return c, true }
func (c *Client) MediaUploader() (socialhub.MediaUploader, bool) {
	return c, c.tokenKind == TokenUser
}
func (c *Client) Reactor() (socialhub.Reactor, bool) {
	return c, c.tokenKind == TokenUser || c.tokenKind == TokenCommunity
}
func (c *Client) Messenger() (socialhub.Messenger, bool) {
	return c, c.tokenKind == TokenUser || c.tokenKind == TokenCommunity
}
func (c *Client) WebhookHandler() (socialhub.WebhookHandler, bool) {
	if c.groupID == 0 || c.callbackSecret == "" {
		return nil, false
	}
	return c, true
}
func (c *Client) Close() error { return nil }

func (c *Client) WallWorkflow() WallWorkflow         { return c }
func (c *Client) CallbackWorkflow() CallbackWorkflow { return c }

var _ socialhub.Client = (*Client)(nil)
var _ socialhub.Publisher = (*Client)(nil)
var _ socialhub.Fetcher = (*Client)(nil)
var _ socialhub.MediaUploader = (*Client)(nil)
var _ socialhub.Reactor = (*Client)(nil)
var _ socialhub.Messenger = (*Client)(nil)
var _ socialhub.WebhookHandler = (*Client)(nil)
