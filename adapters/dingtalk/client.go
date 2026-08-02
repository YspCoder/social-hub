package dingtalk

import (
	"context"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	// CapabilityContacts exposes UnionID-based internal contact lookup.
	CapabilityContacts socialhub.Capability = "dingtalk_contacts"
	// CapabilityRobotMessages exposes application bot group and OTO sends.
	CapabilityRobotMessages socialhub.Capability = "dingtalk_robot_messages"
	// CapabilityAppToken exposes explicit refresh for managed app credentials.
	CapabilityAppToken socialhub.Capability = "dingtalk_app_token"
)

// Client implements the supported DingTalk capabilities for one application.
type Client struct {
	accountID    socialhub.AccountID
	robotCode    string
	api          *transport.Client
	tokenManager *appTokenSource
	scopes       []string
}

func (c *Client) Platform() socialhub.Platform { return "dingtalk" }
func (c *Client) Account() socialhub.AccountID { return c.accountID }

func (c *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	contactApproval := socialhub.ApprovalUnknown
	if contains(c.scopes, "Contact.User.Read") {
		contactApproval = socialhub.ApprovalGranted
	}
	robotSupported := c.robotCode != ""
	robotReason := "configure account.settings.robot_code for application bot sends"
	if robotSupported {
		robotReason = "application bot group sends and batches of up to 100 OTO recipients"
	}
	tokenSupported := c.tokenManager != nil
	tokenReason := "static access_token_ref is caller-managed"
	if tokenSupported {
		tokenReason = "cached application token refresh using ClientId, ClientSecret, and CorpId"
	}
	return socialhub.Capabilities{
		socialhub.CapPublish:    {Capability: socialhub.CapPublish, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "DingTalk application bot messages are not social posts"},
		socialhub.CapFetch:      {Capability: socialhub.CapFetch, Supported: true, Approval: contactApproval, Scopes: []string{"Contact.User.Read"}, Reason: "reads internal users by UnionID", DocURL: "https://open.dingtalk.com/document/orgapp-server/query-user-details-based-on-the-unionid"},
		socialhub.CapMedia:      {Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "media workflows are not implemented in this adapter version"},
		socialhub.CapReact:      {Capability: socialhub.CapReact, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "application bots do not expose the common social reaction contract"},
		socialhub.CapMessage:    {Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "robot APIs send messages but do not implement common Messenger.GetMessage"},
		socialhub.CapWebhook:    {Capability: socialhub.CapWebhook, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "DingTalk Stream event reception is not implemented in this adapter version"},
		CapabilityContacts:      {Capability: CapabilityContacts, Supported: true, Approval: contactApproval, Scopes: []string{"Contact.User.Read"}, Reason: "preserves DingTalk contact fields while mapping the common User", DocURL: "https://open.dingtalk.com/document/orgapp-server/query-user-details-based-on-the-unionid"},
		CapabilityRobotMessages: {Capability: CapabilityRobotMessages, Supported: robotSupported, Approval: socialhub.ApprovalUnknown, Reason: robotReason, DocURL: "https://open.dingtalk.com/document/orgapp-server/robot-message-types-and-data-format"},
		CapabilityAppToken:      {Capability: CapabilityAppToken, Supported: tokenSupported, Approval: socialhub.ApprovalUnknown, Reason: tokenReason, DocURL: "https://open-dingtalk.github.io/developerpedia/docs/develop/permission/single_to_multi/new_get_app_token/"},
	}, nil
}

func (c *Client) Publisher() (socialhub.Publisher, bool)           { return nil, false }
func (c *Client) Fetcher() (socialhub.Fetcher, bool)               { return c, true }
func (c *Client) MediaUploader() (socialhub.MediaUploader, bool)   { return nil, false }
func (c *Client) Reactor() (socialhub.Reactor, bool)               { return nil, false }
func (c *Client) Messenger() (socialhub.Messenger, bool)           { return nil, false }
func (c *Client) WebhookHandler() (socialhub.WebhookHandler, bool) { return nil, false }
func (c *Client) Close() error                                     { return nil }

// ContactWorkflow returns DingTalk-specific contact operations.
func (c *Client) ContactWorkflow() ContactWorkflow { return c }

// RobotWorkflow returns DingTalk application bot operations.
func (c *Client) RobotWorkflow() RobotWorkflow { return c }

// AuthWorkflow returns explicit app-token operations.
func (c *Client) AuthWorkflow() AuthWorkflow { return c }

func (c *Client) responseError(ctx context.Context, operation string, response apiError) error {
	err := response.Err(operation, 0, nil)
	if err == nil {
		return nil
	}
	if c.tokenManager != nil && isTokenError(err) {
		c.tokenManager.Invalidate(ctx)
	}
	return err
}

var _ socialhub.Client = (*Client)(nil)
var _ socialhub.Fetcher = (*Client)(nil)
var _ ContactWorkflow = (*Client)(nil)
var _ RobotWorkflow = (*Client)(nil)
var _ AuthWorkflow = (*Client)(nil)
