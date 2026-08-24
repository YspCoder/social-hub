package steam

import (
	"context"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	CapabilityPlayerSummaries socialhub.Capability = "steam_player_summaries"
	CapabilityAppNews         socialhub.Capability = "steam_app_news"
)

// Client exposes typed Steam Web API reads for one optional user Web API key.
type Client struct {
	accountID     socialhub.AccountID
	authenticated *transport.Client
	public        *transport.Client
	webAPIKey     string
	clock         socialhub.Clock
}

func (client *Client) Platform() socialhub.Platform { return platformName }
func (client *Client) Account() socialhub.AccountID { return client.accountID }

func (client *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	playerSupported := client.authenticated != nil
	playerApproval := socialhub.ApprovalUnknown
	playerReason := "configure access_token_ref with an ordinary Steam user Web API key"
	if playerSupported {
		playerApproval = socialhub.ApprovalGranted
		playerReason = "batched player-summary reads for profiles visible to the configured Steam Web API key"
	}
	return socialhub.Capabilities{
		CapabilityPlayerSummaries: {
			Capability: CapabilityPlayerSummaries, Supported: playerSupported, Approval: playerApproval,
			Reason: playerReason, DocURL: "https://partner.steamgames.com/doc/webapi/ISteamUser#GetPlayerSummaries",
		},
		CapabilityAppNews: {
			Capability: CapabilityAppNews, Supported: true, Approval: socialhub.ApprovalGranted,
			Reason: "public application news reads without a Web API key", DocURL: "https://partner.steamgames.com/doc/webapi/ISteamNews#GetNewsForApp",
		},
		socialhub.CapPublish: {Capability: socialhub.CapPublish, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "publishing is outside this read-only adapter"},
		socialhub.CapFetch:   {Capability: socialhub.CapFetch, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Steam players and app news retain provider semantics through the typed workflow"},
		socialhub.CapMedia:   {Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Steam Web API media upload is outside this adapter"},
		socialhub.CapReact:   {Capability: socialhub.CapReact, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Steam interaction writes are outside this adapter"},
		socialhub.CapMessage: {Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Steam messaging is outside this adapter"},
		socialhub.CapWebhook: {Capability: socialhub.CapWebhook, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Steam Web API does not define a webhook contract in this surface"},
	}, nil
}

func (client *Client) Publisher() (socialhub.Publisher, bool)           { return nil, false }
func (client *Client) Fetcher() (socialhub.Fetcher, bool)               { return nil, false }
func (client *Client) MediaUploader() (socialhub.MediaUploader, bool)   { return nil, false }
func (client *Client) Reactor() (socialhub.Reactor, bool)               { return nil, false }
func (client *Client) Messenger() (socialhub.Messenger, bool)           { return nil, false }
func (client *Client) WebhookHandler() (socialhub.WebhookHandler, bool) { return nil, false }
func (client *Client) Close() error                                     { return nil }

// Steam returns the bounded typed Web API workflow.
func (client *Client) Steam() ReadWorkflow { return client }

func (client *Client) requireAuthenticated(operation string) (*transport.Client, error) {
	if client.authenticated == nil {
		return nil, &socialhub.Error{
			Code: socialhub.CodeUnauthenticated, Class: socialhub.ClassUserAction,
			Platform: platformName, Product: productName, Op: operation,
			PlatformMessage: "configure access_token_ref with an ordinary Steam user Web API key",
			ApprovalURL:     userKeyURL,
		}
	}
	return client.authenticated, nil
}

var _ socialhub.Client = (*Client)(nil)
