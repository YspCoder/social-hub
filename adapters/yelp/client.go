package yelp

import (
	"context"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	CapabilityBusinessSearch socialhub.Capability = "yelp_business_search"
	CapabilityBusinessRead   socialhub.Capability = "yelp_business_read"
	CapabilityReviewRead     socialhub.Capability = "yelp_review_read"
	CapabilityCategoryRead   socialhub.Capability = "yelp_category_read"
)

// Client exposes typed Yelp Places read workflows for one private API key.
type Client struct {
	accountID socialhub.AccountID
	api       *transport.Client
	apiKey    string
}

func (client *Client) Platform() socialhub.Platform { return platformName }
func (client *Client) Account() socialhub.AccountID { return client.accountID }

func (client *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	readState := func(capability socialhub.Capability, reason, docURL string) socialhub.CapabilityState {
		return socialhub.CapabilityState{
			Capability: capability, Supported: true, Approval: socialhub.ApprovalGranted,
			Reason: reason, DocURL: docURL,
		}
	}
	return socialhub.Capabilities{
		CapabilityBusinessSearch: readState(CapabilityBusinessSearch, "business discovery by location or coordinates", "https://docs.developer.yelp.com/reference/v3_business_search"),
		CapabilityBusinessRead:   readState(CapabilityBusinessRead, "public business details available to the configured Places plan", "https://docs.developer.yelp.com/reference/v3_business_info"),
		CapabilityReviewRead:     readState(CapabilityReviewRead, "review excerpts for a business", "https://docs.developer.yelp.com/reference/v3_business_reviews"),
		CapabilityCategoryRead:   readState(CapabilityCategoryRead, "Yelp category taxonomy reads", "https://docs.developer.yelp.com/reference/v3_all_categories"),
		socialhub.CapPublish:     {Capability: socialhub.CapPublish, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Yelp Places is read-only in this adapter"},
		socialhub.CapFetch:       {Capability: socialhub.CapFetch, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Yelp business, review, and category semantics use typed workflows"},
		socialhub.CapMedia:       {Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "media upload is outside Yelp Places reads"},
		socialhub.CapReact:       {Capability: socialhub.CapReact, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "reactions and review writes are outside Yelp Places reads"},
		socialhub.CapMessage:     {Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Yelp messaging is outside the implemented public Places surface"},
		socialhub.CapWebhook:     {Capability: socialhub.CapWebhook, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "these Yelp Places endpoints do not define signed webhooks"},
	}, nil
}

func (client *Client) Publisher() (socialhub.Publisher, bool)           { return nil, false }
func (client *Client) Fetcher() (socialhub.Fetcher, bool)               { return nil, false }
func (client *Client) MediaUploader() (socialhub.MediaUploader, bool)   { return nil, false }
func (client *Client) Reactor() (socialhub.Reactor, bool)               { return nil, false }
func (client *Client) Messenger() (socialhub.Messenger, bool)           { return nil, false }
func (client *Client) WebhookHandler() (socialhub.WebhookHandler, bool) { return nil, false }
func (client *Client) Close() error                                     { return nil }

// Places returns the bounded Yelp Places API v3 read workflow.
func (client *Client) Places() PlacesWorkflow { return client }

var _ socialhub.Client = (*Client)(nil)
