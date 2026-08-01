package discord

import (
	"context"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

// Client implements Discord's supported common capability interfaces.
type Client struct {
	accountID        socialhub.AccountID
	transport        *transport.Client
	cdnURL           string
	userAgent        string
	defaultChannelID string
	workflow         *BotService
}

func (c *Client) Platform() socialhub.Platform { return "discord" }
func (c *Client) Account() socialhub.AccountID { return c.accountID }

func (c *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	publishReason := "configure account.settings.default_channel_id to use the common Publisher"
	if c.defaultChannelID != "" {
		publishReason = "text posts are messages in the configured default channel"
	}
	return socialhub.Capabilities{
		socialhub.CapPublish: {Capability: socialhub.CapPublish, Supported: c.defaultChannelID != "", Approval: socialhub.ApprovalUnknown, Reason: publishReason, DocURL: "https://docs.discord.com/developers/resources/message#create-message"},
		socialhub.CapFetch:   {Capability: socialhub.CapFetch, Supported: true, Approval: socialhub.ApprovalUnknown, Reason: "supports user lookup, composite message lookup, and default-channel history; comment listing is unavailable", DocURL: "https://docs.discord.com/developers/resources/message#get-channel-message"},
		socialhub.CapMedia:   {Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Discord attachments are created with messages and have no independent common upload lifecycle"},
		socialhub.CapReact:   {Capability: socialhub.CapReact, Supported: true, Approval: socialhub.ApprovalUnknown, Reason: "common like maps to a thumbs-up reaction; common comments map to message replies", DocURL: "https://docs.discord.com/developers/resources/message#create-reaction"},
		socialhub.CapMessage: {Capability: socialhub.CapMessage, Supported: true, Approval: socialhub.ApprovalUnknown, Reason: "bots can send messages to accessible guild, thread, and DM channels", DocURL: "https://docs.discord.com/developers/resources/message#create-message"},
		socialhub.CapWebhook: {Capability: socialhub.CapWebhook, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Discord dispatches bot events over Gateway WebSocket; interaction endpoints are a separate signed HTTP workflow"},
		CapabilityGateway:    {Capability: CapabilityGateway, Supported: true, Approval: socialhub.ApprovalUnknown, Reason: "typed workflow exposes Gateway Bot discovery without owning a persistent connection", DocURL: "https://docs.discord.com/developers/events/gateway#get-gateway-bot"},
	}, nil
}

func (c *Client) Publisher() (socialhub.Publisher, bool) {
	if c.defaultChannelID == "" {
		return nil, false
	}
	return c, true
}

func (c *Client) Fetcher() (socialhub.Fetcher, bool)             { return c, true }
func (c *Client) MediaUploader() (socialhub.MediaUploader, bool) { return nil, false }
func (c *Client) Reactor() (socialhub.Reactor, bool)             { return c, true }
func (c *Client) Messenger() (socialhub.Messenger, bool)         { return c, true }
func (c *Client) WebhookHandler() (socialhub.WebhookHandler, bool) {
	return nil, false
}
func (c *Client) Close() error { return nil }

// BotWorkflow returns Discord-specific bot operations.
func (c *Client) BotWorkflow() BotWorkflow { return c.workflow }

var _ socialhub.Client = (*Client)(nil)
var _ socialhub.Publisher = (*Client)(nil)
var _ socialhub.Fetcher = (*Client)(nil)
var _ socialhub.Reactor = (*Client)(nil)
var _ socialhub.Messenger = (*Client)(nil)
