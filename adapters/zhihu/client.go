package zhihu

import (
	"context"
	"net/http"
	"strconv"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

type requestAuthenticator struct{ clock socialhub.Clock }

func (a requestAuthenticator) Authenticate(request *http.Request, token socialhub.Token) error {
	request.Header.Set("Authorization", "Bearer "+token.AccessToken)
	request.Header.Set("X-Request-Timestamp", strconv.FormatInt(a.clock.Now().Unix(), 10))
	request.Header.Set("Content-Type", "application/json")
	return nil
}

// Client implements Zhihu's supported read capability.
type Client struct {
	accountID  socialhub.AccountID
	transport  *transport.Client
	clock      socialhub.Clock
	oauthToken string
	approved   bool
	search     *SearchService
}

func (c *Client) Platform() socialhub.Platform { return "zhihu" }
func (c *Client) Account() socialhub.AccountID { return c.accountID }

func (c *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	approval := socialhub.ApprovalRequired
	reason := "Data Open Platform access is in invite testing and requires an Access Secret"
	if c.approved {
		approval = socialhub.ApprovalGranted
		reason = "configured Access Secret can read the account's documented public-range data"
	}
	return socialhub.Capabilities{
		CapabilitySearch:     {Capability: CapabilitySearch, Supported: true, Approval: approval, Reason: reason, DocURL: docURL},
		CapabilityHotList:    {Capability: CapabilityHotList, Supported: true, Approval: approval, Reason: reason, DocURL: docURL},
		socialhub.CapFetch:   {Capability: socialhub.CapFetch, Supported: true, Approval: approval, Reason: "Fetcher exposes only the documented user contents list; arbitrary post, user, and comment reads are unavailable", DocURL: docURL},
		socialhub.CapPublish: {Capability: socialhub.CapPublish, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "the documented Data Open Platform exposes no content publication API"},
		socialhub.CapMedia:   {Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "the documented Data Open Platform exposes no media upload API"},
		socialhub.CapReact:   {Capability: socialhub.CapReact, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "the documented Data Open Platform exposes no reaction or comment write API"},
		socialhub.CapMessage: {Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "the documented Data Open Platform exposes no private-message API"},
		socialhub.CapWebhook: {Capability: socialhub.CapWebhook, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "the documented Data Open Platform exposes no webhook contract"},
	}, nil
}

func (c *Client) Publisher() (socialhub.Publisher, bool)           { return nil, false }
func (c *Client) Fetcher() (socialhub.Fetcher, bool)               { return c, true }
func (c *Client) MediaUploader() (socialhub.MediaUploader, bool)   { return nil, false }
func (c *Client) Reactor() (socialhub.Reactor, bool)               { return nil, false }
func (c *Client) Messenger() (socialhub.Messenger, bool)           { return nil, false }
func (c *Client) WebhookHandler() (socialhub.WebhookHandler, bool) { return nil, false }
func (c *Client) Close() error                                     { return nil }

// SearchWorkflow returns typed Zhihu search and hot-list operations.
func (c *Client) SearchWorkflow() SearchWorkflow { return c.search }

func (c *Client) requireApproval(operation string) error {
	if c.approved {
		return nil
	}
	return approvalError(operation, "Data Open Platform access is in invite testing; configure approved=true only after an Access Secret is granted")
}

var _ socialhub.Client = (*Client)(nil)
var _ socialhub.Fetcher = (*Client)(nil)
