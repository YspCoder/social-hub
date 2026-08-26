package etsy

import (
	"context"
	"strings"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	CapabilityShopRead       socialhub.Capability = "etsy_shop_read"
	CapabilityListingRead    socialhub.Capability = "etsy_listing_read"
	CapabilityDraftListing   socialhub.Capability = "etsy_draft_listing_create"
	CapabilityListingImage   socialhub.Capability = "etsy_listing_image_manage"
	CapabilityInventoryRead  socialhub.Capability = "etsy_inventory_read"
	CapabilityInventoryWrite socialhub.Capability = "etsy_inventory_write"
)

// Client exposes Etsy seller workflows for one configured shop.
type Client struct {
	accountID socialhub.AccountID
	shopID    int64
	api       *transport.Client
	approval  socialhub.ApprovalConfig
	clock     socialhub.Clock
	hasOAuth  bool
}

func (client *Client) Platform() socialhub.Platform { return platformName }
func (client *Client) Account() socialhub.AccountID { return client.accountID }

func (client *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	appApproval := socialhub.ApprovalUnknown
	if strings.TrimSpace(client.approval.AccountType) != "" {
		appApproval = socialhub.ApprovalGranted
	}
	writeApproval := appApproval
	if !client.hasOAuth {
		writeApproval = socialhub.ApprovalRequired
	}
	return socialhub.Capabilities{
		CapabilityShopRead: {
			Capability: CapabilityShopRead, Supported: true, Approval: appApproval,
			Reason: "public shop retrieval requires an approved Etsy API key", DocURL: documentationURL,
		},
		CapabilityListingRead: {
			Capability: CapabilityListingRead, Supported: true, Approval: appApproval,
			Reason: "public listing retrieval; private shop listing reads require listings_r", DocURL: documentationURL,
		},
		CapabilityDraftListing: {
			Capability: CapabilityDraftListing, Supported: true, Approval: writeApproval,
			Reason: "creates draft listings only and requires listings_w", DocURL: documentationURL,
		},
		CapabilityListingImage: {
			Capability: CapabilityListingImage, Supported: true, Approval: writeApproval,
			Reason: "image reads are public; assignment and upload require listings_w", DocURL: documentationURL,
		},
		CapabilityInventoryRead: {
			Capability: CapabilityInventoryRead, Supported: true, Approval: writeApproval,
			Reason: "listing inventory reads require listings_r", DocURL: documentationURL,
		},
		CapabilityInventoryWrite: {
			Capability: CapabilityInventoryWrite, Supported: true, Approval: writeApproval,
			Reason: "full inventory replacement requires listings_w", DocURL: documentationURL,
		},
		socialhub.CapPublish: {Capability: socialhub.CapPublish, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Etsy listings are commerce resources, not organic social posts"},
		socialhub.CapFetch:   {Capability: socialhub.CapFetch, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "commerce reads use typed Etsy workflows"},
		socialhub.CapMedia:   {Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "listing images do not implement the generic resumable media contract"},
		socialhub.CapReact:   {Capability: socialhub.CapReact, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "these seller workflows expose no reactions"},
		socialhub.CapMessage: {Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "these seller workflows expose no messaging"},
		socialhub.CapWebhook: {Capability: socialhub.CapWebhook, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "webhooks are outside the bounded listing surface"},
	}, nil
}

func (client *Client) Publisher() (socialhub.Publisher, bool)           { return nil, false }
func (client *Client) Fetcher() (socialhub.Fetcher, bool)               { return nil, false }
func (client *Client) MediaUploader() (socialhub.MediaUploader, bool)   { return nil, false }
func (client *Client) Reactor() (socialhub.Reactor, bool)               { return nil, false }
func (client *Client) Messenger() (socialhub.Messenger, bool)           { return nil, false }
func (client *Client) WebhookHandler() (socialhub.WebhookHandler, bool) { return nil, false }
func (client *Client) Close() error                                     { return nil }

// Listings returns the bounded Etsy listing workflow.
func (client *Client) Listings() ListingWorkflow { return client }

var _ socialhub.Client = (*Client)(nil)
