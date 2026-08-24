package ads

import (
	"context"
	"strings"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	CapabilityAdsManagement socialhub.Capability = "x_ads_management"
	CapabilityAdsReporting  socialhub.Capability = "x_ads_reporting"
	standardAccess                               = "standard_access"
)

// Client exposes one X Ads Account's paid-media workflows.
type Client struct {
	accountID    socialhub.AccountID
	adsAccountID string
	api          *transport.Client
	accessLevel  string
}

func (client *Client) Platform() socialhub.Platform { return platformName }
func (client *Client) Account() socialhub.AccountID { return client.accountID }

func (client *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	approval := socialhub.ApprovalUnknown
	if client.accessLevel != "" {
		approval = socialhub.ApprovalRequired
		if strings.EqualFold(client.accessLevel, standardAccess) {
			approval = socialhub.ApprovalGranted
		}
	}
	state := func(capability socialhub.Capability, reason, docURL string) socialhub.CapabilityState {
		return socialhub.CapabilityState{
			Capability: capability, Supported: true, Approval: approval,
			Reason: reason, DocURL: docURL,
		}
	}
	return socialhub.Capabilities{
		CapabilityAdsManagement: state(CapabilityAdsManagement, "account-role-gated Campaign, Line Item, and Promoted Tweet workflows", documentationURL+"/campaign-management"),
		CapabilityAdsReporting:  state(CapabilityAdsReporting, "bounded synchronous Ads analytics", documentationURL+"/analytics"),
		socialhub.CapPublish:    {Capability: socialhub.CapPublish, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "paid ads are not organic posts; use Campaigns(), LineItems(), and PromotedTweets()"},
		socialhub.CapFetch:      {Capability: socialhub.CapFetch, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "advertising reads use typed X Ads resources"},
		socialhub.CapMedia:      {Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "the initial adapter associates existing Tweet IDs"},
		socialhub.CapReact:      {Capability: socialhub.CapReact, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "X Ads API is not an organic engagement product"},
		socialhub.CapMessage:    {Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "X Ads API does not provide general messaging"},
		socialhub.CapWebhook:    {Capability: socialhub.CapWebhook, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "webhooks are outside the initial X Ads adapter contract"},
	}, nil
}

func (client *Client) Publisher() (socialhub.Publisher, bool)         { return nil, false }
func (client *Client) Fetcher() (socialhub.Fetcher, bool)             { return nil, false }
func (client *Client) MediaUploader() (socialhub.MediaUploader, bool) { return nil, false }
func (client *Client) Reactor() (socialhub.Reactor, bool)             { return nil, false }
func (client *Client) Messenger() (socialhub.Messenger, bool)         { return nil, false }
func (client *Client) WebhookHandler() (socialhub.WebhookHandler, bool) {
	return nil, false
}
func (client *Client) Close() error { return nil }

func (client *Client) Accounts() AccountWorkflow             { return client }
func (client *Client) Campaigns() CampaignWorkflow           { return client }
func (client *Client) LineItems() LineItemWorkflow           { return client }
func (client *Client) PromotedTweets() PromotedTweetWorkflow { return client }
func (client *Client) Stats() StatsWorkflow                  { return client }

func (client *Client) accountPath() string { return "/accounts/" + client.adsAccountID }

func (client *Client) resourcePath(resource string) string {
	return client.accountPath() + "/" + resource
}

var _ socialhub.Client = (*Client)(nil)
