package wechatminiprogram

import (
	"context"
	"net/http"
	"sync"
	"time"

	"social-hub/pkg/socialhub"
)

const (
	CapabilityCode2Session         socialhub.Capability = "wechat_mini_program_code2session"
	CapabilityStableAccessToken    socialhub.Capability = "wechat_mini_program_stable_access_token"
	CapabilitySubscriptionMessages socialhub.Capability = "wechat_mini_program_subscription_messages"
	CapabilityPhoneNumberExchange  socialhub.Capability = "wechat_mini_program_phone_number_exchange"
)

// Client exposes typed Mini Program server workflows for one AppID.
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
		CapabilityCode2Session: {
			Capability: CapabilityCode2Session, Supported: true, Approval: socialhub.ApprovalGranted,
			Reason: "server-side exchange of a wx.login code for the Mini Program login state",
			DocURL: code2SessionDocumentationURL,
		},
		CapabilityStableAccessToken: {
			Capability: CapabilityStableAccessToken, Supported: true, Approval: socialhub.ApprovalGranted,
			Reason: "stable access-token retrieval and process-local refresh coordination",
			DocURL: stableTokenDocumentationURL,
		},
		CapabilitySubscriptionMessages: {
			Capability: CapabilitySubscriptionMessages, Supported: true, Approval: socialhub.ApprovalRequired,
			Reason: "sending requires an account template and a matching user subscription grant",
			DocURL: subscriptionDocumentationURL,
		},
		CapabilityPhoneNumberExchange: {
			Capability: CapabilityPhoneNumberExchange, Supported: true, Approval: socialhub.ApprovalRequired,
			Reason: "phone-number exchange requires explicit user consent and an eligible, activated Mini Program account",
			DocURL: phoneNumberDocumentationURL,
		},
		socialhub.CapPublish: {Capability: socialhub.CapPublish, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Mini Program publishing is outside this adapter"},
		socialhub.CapFetch:   {Capability: socialhub.CapFetch, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "login-state and phone exchanges are typed sensitive-data workflows, not social fetches"},
		socialhub.CapMedia:   {Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "media APIs are outside this adapter"},
		socialhub.CapReact:   {Capability: socialhub.CapReact, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "social reactions are outside this adapter"},
		socialhub.CapMessage: {Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "subscription templates use the typed SubscriptionMessages workflow rather than the common message model"},
		socialhub.CapWebhook: {Capability: socialhub.CapWebhook, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "subscription-result webhooks are outside this adapter"},
	}, nil
}

func (client *Client) Publisher() (socialhub.Publisher, bool)           { return nil, false }
func (client *Client) Fetcher() (socialhub.Fetcher, bool)               { return nil, false }
func (client *Client) MediaUploader() (socialhub.MediaUploader, bool)   { return nil, false }
func (client *Client) Reactor() (socialhub.Reactor, bool)               { return nil, false }
func (client *Client) Messenger() (socialhub.Messenger, bool)           { return nil, false }
func (client *Client) WebhookHandler() (socialhub.WebhookHandler, bool) { return nil, false }

func (client *Client) Login() LoginWorkflow                       { return client }
func (client *Client) Credentials() CredentialsWorkflow           { return client }
func (client *Client) SubscriptionMessages() SubscriptionWorkflow { return client }
func (client *Client) PhoneNumbers() PhoneNumberWorkflow          { return client }

// Close discards the client's in-memory copies of credentials and cached token.
func (client *Client) Close() error {
	client.mu.Lock()
	defer client.mu.Unlock()
	client.closed = true
	client.appID, client.appSecret = "", ""
	client.token = StableAccessToken{}
	return nil
}

func (client *Client) credentials(operation string) (string, string, error) {
	client.mu.Lock()
	defer client.mu.Unlock()
	if client.closed || client.appID == "" || client.appSecret == "" {
		return "", "", platformError(operation, socialhub.CodeConflict, socialhub.ClassPermanent, nil)
	}
	return client.appID, client.appSecret, nil
}

var _ socialhub.Client = (*Client)(nil)
