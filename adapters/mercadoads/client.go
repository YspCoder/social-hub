package mercadoads

import (
	"context"
	"sync"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	CapabilityAdvertiserDiscovery socialhub.Capability = "mercadolibre_product_ads_advertiser_discovery"
	CapabilityCampaignRead        socialhub.Capability = "mercadolibre_product_ads_campaign_read"
	CapabilityItemRead            socialhub.Capability = "mercadolibre_product_ads_item_read"
	CapabilityMetricsRead         socialhub.Capability = "mercadolibre_product_ads_metrics_read"
)

// Client exposes one Mercado Libre user's Product Ads reads. Advertiser-scoped
// list operations use the advertiser_id configured for this SDK account.
type Client struct {
	mu           sync.RWMutex
	accountID    socialhub.AccountID
	advertiserID int64
	api          *transport.Client
	approval     socialhub.ApprovalConfig
	tokens       closeableTokenSource
	closed       bool
}

func (client *Client) Platform() socialhub.Platform { return platformName }
func (client *Client) Account() socialhub.AccountID { return client.accountID }

func (client *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	if _, err := client.apiClient("capabilities"); err != nil {
		return nil, err
	}
	approval := socialhub.ApprovalUnknown
	if approvalConfigured(client.approval) {
		approval = socialhub.ApprovalGranted
	}
	configuredAdvertiser := client.advertiserID > 0
	state := func(capability socialhub.Capability, supported bool, reason string) socialhub.CapabilityState {
		return socialhub.CapabilityState{
			Capability: capability, Supported: supported, Approval: approval,
			Reason: reason, DocURL: documentationURL,
		}
	}
	return socialhub.Capabilities{
		CapabilityAdvertiserDiscovery: state(CapabilityAdvertiserDiscovery, true, "lists Product Ads advertisers available to the authorized Mercado Libre user"),
		CapabilityCampaignRead:        state(CapabilityCampaignRead, configuredAdvertiser, advertiserCapabilityReason(configuredAdvertiser, "Campaign reads")),
		CapabilityItemRead:            state(CapabilityItemRead, true, "typed Product Ads item detail reads; advertiser-scoped item lists require advertiser_id"),
		CapabilityMetricsRead:         state(CapabilityMetricsRead, configuredAdvertiser, advertiserCapabilityReason(configuredAdvertiser, "Campaign and item metric lists")),
		socialhub.CapPublish:          {Capability: socialhub.CapPublish, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "the current public Product Ads page documents read and metrics endpoints only"},
		socialhub.CapFetch:            {Capability: socialhub.CapFetch, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "paid-media reads use typed Product Ads workflows"},
		socialhub.CapMedia:            {Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Product Ads uses existing marketplace listings; media upload is outside this API"},
		socialhub.CapReact:            {Capability: socialhub.CapReact, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Mercado Ads has no organic reaction surface"},
		socialhub.CapMessage:          {Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "marketplace messaging is a separate Mercado Libre API"},
		socialhub.CapWebhook:          {Capability: socialhub.CapWebhook, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Mercado Libre notifications are outside this Product Ads adapter"},
	}, nil
}

func advertiserCapabilityReason(configured bool, prefix string) string {
	if configured {
		return prefix + " are bound to the configured advertiser_id; Product Ads enablement is required"
	}
	return prefix + " require account.settings.advertiser_id after advertiser discovery"
}

func (client *Client) Publisher() (socialhub.Publisher, bool)           { return nil, false }
func (client *Client) Fetcher() (socialhub.Fetcher, bool)               { return nil, false }
func (client *Client) MediaUploader() (socialhub.MediaUploader, bool)   { return nil, false }
func (client *Client) Reactor() (socialhub.Reactor, bool)               { return nil, false }
func (client *Client) Messenger() (socialhub.Messenger, bool)           { return nil, false }
func (client *Client) WebhookHandler() (socialhub.WebhookHandler, bool) { return nil, false }
func (client *Client) Close() error {
	if client == nil {
		return nil
	}
	client.mu.Lock()
	tokens := client.tokens
	client.closed = true
	client.api, client.tokens = nil, nil
	client.mu.Unlock()
	if tokens != nil {
		tokens.Close()
	}
	return nil
}

func (client *Client) apiClient(operation string) (*transport.Client, error) {
	if client == nil {
		return nil, platformError(operation, socialhub.CodeConflict, socialhub.ClassPermanent, nil)
	}
	client.mu.RLock()
	defer client.mu.RUnlock()
	if client.closed || client.api == nil {
		return nil, platformError(operation, socialhub.CodeConflict, socialhub.ClassPermanent, nil)
	}
	return client.api, nil
}

func (client *Client) Advertisers() AdvertiserWorkflow { return client }
func (client *Client) Campaigns() CampaignWorkflow     { return client }
func (client *Client) Items() ItemWorkflow             { return client }

var _ socialhub.Client = (*Client)(nil)
