package applovinads

import (
	"context"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	CapabilityCampaignManagement socialhub.Capability = "applovin_growth_campaign_management"
	CapabilityCreativeSetManager socialhub.Capability = "applovin_growth_creative_set_management"
	CapabilityAssetManager       socialhub.Capability = "applovin_growth_asset_management"
)

// Client exposes one Axon advertiser account's paid-media workflows.
type Client struct {
	accountID     socialhub.AccountID
	axonAccountID string
	accountType   AccountType
	api           *transport.Client
	approved      bool
}

func (client *Client) Platform() socialhub.Platform { return platformName }
func (client *Client) Account() socialhub.AccountID { return client.accountID }

func (client *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	approval := socialhub.ApprovalRequired
	if client.approved {
		approval = socialhub.ApprovalGranted
	}
	state := func(capability socialhub.Capability, reason, docURL string) socialhub.CapabilityState {
		return socialhub.CapabilityState{
			Capability: capability, Supported: true, Approval: approval, Scopes: []string{approvalScope},
			Reason: reason, DocURL: docURL,
		}
	}
	return socialhub.Capabilities{
		CapabilityCampaignManagement: state(CapabilityCampaignManagement, "APP/WEB Campaign list, create, update, and WEB catalog discovery; API access is whitelist-only", documentationURL),
		CapabilityCreativeSetManager: state(CapabilityCreativeSetManager, "Creative Set list, create, update, clone, and cross-Campaign association workflows", documentationURL+"#creative-sets"),
		CapabilityAssetManager:       state(CapabilityAssetManager, "bounded streaming Asset upload, processing status, library reads, and Creative Set association workflows", documentationURL+"#assets"),
		socialhub.CapPublish:         {Capability: socialhub.CapPublish, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "paid Campaigns are not organic social posts; use Campaigns()"},
		socialhub.CapFetch:           {Capability: socialhub.CapFetch, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "advertising reads use typed Axon resources"},
		socialhub.CapMedia:           {Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "paid creative Assets use the typed Assets() workflow"},
		socialhub.CapReact:           {Capability: socialhub.CapReact, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Campaign Management API has no organic engagement surface"},
		socialhub.CapMessage:         {Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Campaign Management API has no messaging surface"},
		socialhub.CapWebhook:         {Capability: socialhub.CapWebhook, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Campaign Management API v1 does not expose webhooks"},
	}, nil
}

func (client *Client) Publisher() (socialhub.Publisher, bool)           { return nil, false }
func (client *Client) Fetcher() (socialhub.Fetcher, bool)               { return nil, false }
func (client *Client) MediaUploader() (socialhub.MediaUploader, bool)   { return nil, false }
func (client *Client) Reactor() (socialhub.Reactor, bool)               { return nil, false }
func (client *Client) Messenger() (socialhub.Messenger, bool)           { return nil, false }
func (client *Client) WebhookHandler() (socialhub.WebhookHandler, bool) { return nil, false }
func (client *Client) Close() error                                     { return nil }

func (client *Client) Campaigns() CampaignWorkflow       { return client }
func (client *Client) CreativeSets() CreativeSetWorkflow { return client }
func (client *Client) Assets() AssetWorkflow             { return client }

func (client *Client) requireAccess(operation string) error {
	if client.approved {
		return nil
	}
	return &socialhub.Error{
		Code: socialhub.CodeApprovalRequired, Class: socialhub.ClassUserAction,
		Platform: platformName, Product: productName, Op: operation,
		RequiredScopes: []string{approvalScope}, ApprovalURL: approvalURL,
		PlatformMessage: "Axon Campaign Management API is whitelist-only; add campaign_management_api to approval.scopes after AppLovin grants access",
	}
}

var _ socialhub.Client = (*Client)(nil)
