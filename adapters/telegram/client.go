package telegram

import (
	"context"

	tgbot "github.com/go-telegram/bot"

	"social-hub/pkg/socialhub"
)

// Client implements Telegram's supported capability interfaces.
type Client struct {
	accountID     socialhub.AccountID
	bot           *tgbot.Bot
	clock         socialhub.Clock
	defaultChatID string
	webhookSecret string
	workflow      *BotService
}

func (c *Client) Platform() socialhub.Platform { return "telegram" }
func (c *Client) Account() socialhub.AccountID { return c.accountID }

func (c *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	publishReason := "configure account.settings.default_chat_id to use the common Publisher"
	if c.defaultChatID != "" {
		publishReason = "text posts can be sent to the configured default chat or channel"
	}
	webhookReason := "configure webhook.secret_ref to verify X-Telegram-Bot-Api-Secret-Token"
	if c.webhookSecret != "" {
		webhookReason = "webhook updates are verified with the configured secret token"
	}
	return socialhub.Capabilities{
		socialhub.CapPublish: {Capability: socialhub.CapPublish, Supported: c.defaultChatID != "", Approval: socialhub.ApprovalUnknown, Reason: publishReason, DocURL: docURL + "#sendmessage"},
		socialhub.CapFetch:   {Capability: socialhub.CapFetch, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Bot API does not provide arbitrary message history or message lookup"},
		socialhub.CapMedia:   {Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Telegram uploads media as part of send operations, not through an independent upload lifecycle"},
		socialhub.CapReact:   {Capability: socialhub.CapReact, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "common Reactor lacks the chat identifier required by setMessageReaction"},
		socialhub.CapMessage: {Capability: socialhub.CapMessage, Supported: true, Approval: socialhub.ApprovalUnknown, Reason: "bots can send messages to chats that permitted interaction", DocURL: docURL + "#sendmessage"},
		socialhub.CapWebhook: {Capability: socialhub.CapWebhook, Supported: c.webhookSecret != "", Approval: socialhub.ApprovalUnknown, Reason: webhookReason, DocURL: docURL + "#setwebhook"},
		CapabilityMediaSend:  {Capability: CapabilityMediaSend, Supported: true, Approval: socialhub.ApprovalUnknown, Reason: "typed workflow supports Telegram file IDs, URLs, and bounded multipart uploads", DocURL: docURL + "#sending-files"},
	}, nil
}

func (c *Client) Publisher() (socialhub.Publisher, bool) {
	if c.defaultChatID == "" {
		return nil, false
	}
	return c, true
}

func (c *Client) Fetcher() (socialhub.Fetcher, bool)             { return nil, false }
func (c *Client) MediaUploader() (socialhub.MediaUploader, bool) { return nil, false }
func (c *Client) Reactor() (socialhub.Reactor, bool)             { return nil, false }
func (c *Client) Messenger() (socialhub.Messenger, bool)         { return c, true }

func (c *Client) WebhookHandler() (socialhub.WebhookHandler, bool) {
	if c.webhookSecret == "" {
		return nil, false
	}
	return c, true
}

func (c *Client) Close() error { return nil }

// BotWorkflow returns Telegram-specific bot operations.
func (c *Client) BotWorkflow() BotWorkflow { return c.workflow }

var _ socialhub.Client = (*Client)(nil)
var _ socialhub.Publisher = (*Client)(nil)
var _ socialhub.Messenger = (*Client)(nil)
var _ socialhub.WebhookHandler = (*Client)(nil)
