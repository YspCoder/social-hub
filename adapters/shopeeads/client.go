package shopeeads

import (
	"context"
	"strings"
	"sync"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	CapabilityBalanceRead        socialhub.Capability = "shopee_ads_balance_read"
	CapabilityRecommendations    socialhub.Capability = "shopee_ads_recommendations"
	CapabilityCampaignRead       socialhub.Capability = "shopee_ads_campaign_read"
	CapabilityPerformanceReports socialhub.Capability = "shopee_ads_performance_reports"
)

// Client exposes read-only workflows for one Shopee shop's Ads account.
type Client struct {
	mu        sync.RWMutex
	accountID socialhub.AccountID
	shopID    int64
	api       *transport.Client
	approval  socialhub.ApprovalConfig
	clock     socialhub.Clock
	tokens    closeableTokenSource
	signer    *shopAuthenticator
	closed    bool
}

func (client *Client) Platform() socialhub.Platform { return platformName }
func (client *Client) Account() socialhub.AccountID { return client.accountID }

func (client *Client) Capabilities(ctx context.Context) (socialhub.Capabilities, error) {
	if ctx == nil {
		return nil, invalidArgument("capabilities", "context is required")
	}
	if _, err := client.apiClient("capabilities"); err != nil {
		return nil, err
	}
	approval := socialhub.ApprovalUnknown
	if strings.TrimSpace(client.approval.AccountType) != "" {
		approval = socialhub.ApprovalGranted
	}
	state := func(capability socialhub.Capability, reason string) socialhub.CapabilityState {
		return socialhub.CapabilityState{
			Capability: capability, Supported: true, Approval: approval,
			Reason: reason, DocURL: documentationURL,
		}
	}
	return socialhub.Capabilities{
		CapabilityBalanceRead:        state(CapabilityBalanceRead, "real-time shop Ads credit balance and shop toggles"),
		CapabilityRecommendations:    state(CapabilityRecommendations, "recommended products and product keywords"),
		CapabilityCampaignRead:       state(CapabilityCampaignRead, "paginated product Campaign IDs and typed setting details"),
		CapabilityPerformanceReports: state(CapabilityPerformanceReports, "shop CPC and product Campaign daily/hourly performance"),
		socialhub.CapPublish:         {Capability: socialhub.CapPublish, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "paid advertising is separate from organic publishing"},
		socialhub.CapFetch:           {Capability: socialhub.CapFetch, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Ads reads use typed shop-scoped workflows"},
		socialhub.CapMedia:           {Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "media upload is outside this initial Ads adapter"},
		socialhub.CapReact:           {Capability: socialhub.CapReact, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Shopee Ads has no organic reaction surface"},
		socialhub.CapMessage:         {Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Shopee chat is a separate API product"},
		socialhub.CapWebhook:         {Capability: socialhub.CapWebhook, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Open Platform push callbacks are outside this Ads adapter"},
	}, nil
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
	tokens, signer := client.tokens, client.signer
	client.api, client.tokens, client.signer, client.closed = nil, nil, nil, true
	client.mu.Unlock()
	if tokens != nil {
		tokens.Close()
	}
	if signer != nil {
		signer.Close()
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

func (client *Client) Balances() BalanceWorkflow               { return client }
func (client *Client) Recommendations() RecommendationWorkflow { return client }
func (client *Client) Campaigns() CampaignWorkflow             { return client }
func (client *Client) Reports() PerformanceReportWorkflow      { return client }

var _ socialhub.Client = (*Client)(nil)
