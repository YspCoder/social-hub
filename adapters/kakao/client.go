package kakao

import (
	"context"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	// CapabilityTalkFriends exposes Kakao Talk friend discovery and sends.
	CapabilityTalkFriends socialhub.Capability = "kakao_talk_friends"
	// CapabilityTemplates exposes default-text and custom-template messages.
	CapabilityTemplates socialhub.Capability = "kakao_message_templates"
)

// Client implements the supported Kakao Login and Kakao Talk capabilities for
// one authorized service user.
type Client struct {
	accountID             socialhub.AccountID
	userID                string
	defaultLinkURL        string
	friendMessageApproved bool
	api                   *transport.Client
}

func (c *Client) Platform() socialhub.Platform { return "kakao" }
func (c *Client) Account() socialhub.AccountID { return c.accountID }

func (c *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	messageReason := "configure account.settings.default_link_url to enable the common text Messenger"
	if c.defaultLinkURL != "" {
		messageReason = "sends linked text templates to the authorized user or approved friends"
	}
	friendApproval := socialhub.ApprovalRequired
	friendReason := "Kakao Talk friend list and friend messaging require additional platform permission"
	if c.friendMessageApproved {
		friendApproval = socialhub.ApprovalGranted
		friendReason = "retrieves service-linked friends and sends to at most five selected UUIDs"
	}
	return socialhub.Capabilities{
		socialhub.CapPublish:  {Capability: socialhub.CapPublish, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Kakao Talk messages are not social posts"},
		socialhub.CapFetch:    {Capability: socialhub.CapFetch, Supported: true, Approval: socialhub.ApprovalUnknown, Scopes: []string{"profile_nickname", "profile_image"}, Reason: "retrieves the authorized Kakao Login user; post and comment reads are unavailable", DocURL: docURL + "#retrieve-user-information"},
		socialhub.CapMedia:    {Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "message images are remote URLs in templates, not independent uploaded media"},
		socialhub.CapReact:    {Capability: socialhub.CapReact, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Kakao Login and Talk Message APIs expose no generic reactions"},
		socialhub.CapMessage:  {Capability: socialhub.CapMessage, Supported: c.defaultLinkURL != "", Approval: socialhub.ApprovalUnknown, Scopes: []string{"talk_message"}, Reason: messageReason, DocURL: "https://developers.kakao.com/docs/en/kakaotalk-message/rest-api"},
		socialhub.CapWebhook:  {Capability: socialhub.CapWebhook, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Kakao Login and Share callbacks are separate product-specific workflows"},
		CapabilityTalkFriends: {Capability: CapabilityTalkFriends, Supported: true, Approval: friendApproval, Scopes: []string{"friends", "talk_message"}, Reason: friendReason, DocURL: "https://developers.kakao.com/docs/en/kakaotalk-social/rest-api"},
		CapabilityTemplates:   {Capability: CapabilityTemplates, Supported: true, Approval: socialhub.ApprovalUnknown, Scopes: []string{"talk_message"}, Reason: "sends default text and app-managed custom templates", DocURL: "https://developers.kakao.com/docs/en/kakaotalk-message/rest-api"},
	}, nil
}

func (c *Client) Publisher() (socialhub.Publisher, bool)         { return nil, false }
func (c *Client) Fetcher() (socialhub.Fetcher, bool)             { return c, true }
func (c *Client) MediaUploader() (socialhub.MediaUploader, bool) { return nil, false }
func (c *Client) Reactor() (socialhub.Reactor, bool)             { return nil, false }
func (c *Client) Messenger() (socialhub.Messenger, bool) {
	if c.defaultLinkURL == "" {
		return nil, false
	}
	return c, true
}
func (c *Client) WebhookHandler() (socialhub.WebhookHandler, bool) { return nil, false }
func (c *Client) Close() error                                     { return nil }

// UserWorkflow returns current-user operations.
func (c *Client) UserWorkflow() UserWorkflow { return c }

// FriendWorkflow returns Kakao Talk friend-list operations.
func (c *Client) FriendWorkflow() FriendWorkflow { return c }

// MessageWorkflow returns typed Kakao Talk template operations.
func (c *Client) MessageWorkflow() MessageWorkflow { return c }

func (c *Client) requireFriendApproval(operation string) error {
	if c.friendMessageApproved {
		return nil
	}
	return &socialhub.Error{
		Code: socialhub.CodeApprovalRequired, Class: socialhub.ClassUserAction, Platform: "kakao", Product: productName,
		Op: operation, ApprovalURL: approvalURL, PlatformMessage: "Kakao Talk friend-list and message permission is required",
	}
}

var _ socialhub.Client = (*Client)(nil)
var _ socialhub.Fetcher = (*Client)(nil)
var _ socialhub.Messenger = (*Client)(nil)
