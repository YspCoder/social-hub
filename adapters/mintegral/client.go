package mintegral

import (
	"context"

	mtg "github.com/jageros/mintegral-go"

	"social-hub/pkg/socialhub"
)

const (
	CapabilityAccountRead        socialhub.Capability = "mintegral_account_read"
	CapabilityCampaignManagement socialhub.Capability = "mintegral_campaign_management"
	CapabilityOfferManagement    socialhub.Capability = "mintegral_offer_management"
	CapabilityCreativeManagement socialhub.Capability = "mintegral_creative_management"
	CapabilityAudienceManagement socialhub.Capability = "mintegral_audience_management"
	CapabilityAdvancedReporting  socialhub.Capability = "mintegral_advanced_reporting_v2"
)

// Client exposes one Mintegral AppGrowth account through typed workflows.
type Client struct {
	accountID socialhub.AccountID
	sdk       *mtg.Client
}

func (client *Client) Platform() socialhub.Platform { return platformName }
func (client *Client) Account() socialhub.AccountID { return client.accountID }

func (client *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	return socialhub.Capabilities{
		CapabilityAccountRead: {
			Capability: CapabilityAccountRead, Supported: true, Approval: socialhub.ApprovalRequired,
			Reason: "account balance and currency require an approved Mintegral AppGrowth advertiser account and API keys", DocURL: documentationURL,
		},
		CapabilityCampaignManagement: {
			Capability: CapabilityCampaignManagement, Supported: true, Approval: socialhub.ApprovalRequired,
			Reason: "typed Campaign and App metadata workflows backed by the pinned Mintegral Go SDK", DocURL: documentationURL,
		},
		CapabilityOfferManagement: {
			Capability: CapabilityOfferManagement, Supported: true, Approval: socialhub.ApprovalRequired,
			Reason: "Offer creation, targeting, bids, budgets, tracking, status, and audience assignment", DocURL: documentationURL,
		},
		CapabilityCreativeManagement: {
			Capability: CapabilityCreativeManagement, Supported: true, Approval: socialhub.ApprovalRequired,
			Reason: "Creative Set, Ad, media, playable, and asset-library workflows", DocURL: documentationURL,
		},
		CapabilityAudienceManagement: {
			Capability: CapabilityAudienceManagement, Supported: true, Approval: socialhub.ApprovalRequired,
			Reason: "regional S3 or OSS audience upload plans and audience lifecycle operations", DocURL: documentationURL,
		},
		CapabilityAdvancedReporting: {
			Capability: CapabilityAdvancedReporting, Supported: true, Approval: socialhub.ApprovalRequired,
			Reason: "asynchronous Advanced Performance Report v2 status, TSV streaming, and bounded batch consumption", DocURL: "https://helpcenter.mintegral.com/en/docs/advanced-ad-delivery-report/",
		},
		socialhub.CapPublish: {Capability: socialhub.CapPublish, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "AppGrowth advertising is not organic social publishing"},
		socialhub.CapFetch:   {Capability: socialhub.CapFetch, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "advertising resources use dedicated typed workflows"},
		socialhub.CapMedia:   {Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "ad assets use Mintegral-specific repeatable upload sources"},
		socialhub.CapReact:   {Capability: socialhub.CapReact, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "advertising resources have no organic engagement surface"},
		socialhub.CapMessage: {Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "AppGrowth Open API has no messaging surface"},
		socialhub.CapWebhook: {Capability: socialhub.CapWebhook, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "the integrated Open API does not expose webhooks"},
	}, nil
}

func (client *Client) Publisher() (socialhub.Publisher, bool)           { return nil, false }
func (client *Client) Fetcher() (socialhub.Fetcher, bool)               { return nil, false }
func (client *Client) MediaUploader() (socialhub.MediaUploader, bool)   { return nil, false }
func (client *Client) Reactor() (socialhub.Reactor, bool)               { return nil, false }
func (client *Client) Messenger() (socialhub.Messenger, bool)           { return nil, false }
func (client *Client) WebhookHandler() (socialhub.WebhookHandler, bool) { return nil, false }
func (client *Client) Close() error                                     { return nil }

func (client *Client) Accounts() AccountsWorkflow   { return &AccountsService{client: client} }
func (client *Client) Campaigns() CampaignsWorkflow { return &CampaignsService{client: client} }
func (client *Client) Apps() AppsWorkflow           { return &AppsService{client: client} }
func (client *Client) Offers() OffersWorkflow       { return &OffersService{client: client} }
func (client *Client) Events() EventsWorkflow       { return &EventsService{client: client} }
func (client *Client) CreativeSets() CreativeSetsWorkflow {
	return &CreativeSetsService{client: client}
}
func (client *Client) CreativeAds() CreativeAdsWorkflow { return &CreativeAdsService{client: client} }
func (client *Client) Assets() AssetsWorkflow           { return &AssetsService{client: client} }
func (client *Client) Reports() ReportsWorkflow         { return &ReportsService{client: client} }
func (client *Client) Audiences() AudiencesWorkflow     { return &AudiencesService{client: client} }

var _ socialhub.Client = (*Client)(nil)
