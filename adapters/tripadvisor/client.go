package tripadvisor

import (
	"context"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	CapabilityLocationSearch  socialhub.Capability = "tripadvisor_location_search"
	CapabilityLocationDetails socialhub.Capability = "tripadvisor_location_details"
	CapabilityPhotoRead       socialhub.Capability = "tripadvisor_photo_read"
	CapabilityReviewRead      socialhub.Capability = "tripadvisor_review_read"
)

// Client exposes typed Tripadvisor Content API reads for one API key.
type Client struct {
	accountID socialhub.AccountID
	api       *transport.Client
	apiKey    string
	clock     socialhub.Clock
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
		CapabilityLocationSearch:  readState(CapabilityLocationSearch, "location search and nearby discovery", "https://tripadvisor-content-api.readme.io/reference/searchforlocations"),
		CapabilityLocationDetails: readState(CapabilityLocationDetails, "location details available to the configured Content API plan", "https://tripadvisor-content-api.readme.io/reference/getlocationdetails"),
		CapabilityPhotoRead:       readState(CapabilityPhotoRead, "attributed Tripadvisor location photos", "https://tripadvisor-content-api.readme.io/reference/getlocationphotos"),
		CapabilityReviewRead:      readState(CapabilityReviewRead, "Tripadvisor location reviews", "https://tripadvisor-content-api.readme.io/reference/getlocationreviews"),
		socialhub.CapPublish:      {Capability: socialhub.CapPublish, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Content API v1 is read-only in this adapter"},
		socialhub.CapFetch:        {Capability: socialhub.CapFetch, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "locations and reviews use the typed Places workflow rather than generic social posts"},
		socialhub.CapMedia:        {Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "photo reads are provider content, not media upload"},
		socialhub.CapReact:        {Capability: socialhub.CapReact, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Content API v1 does not expose portable reaction writes"},
		socialhub.CapMessage:      {Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Content API v1 does not expose messaging"},
		socialhub.CapWebhook:      {Capability: socialhub.CapWebhook, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "the implemented Content API endpoints are request/response reads"},
	}, nil
}

func (client *Client) Publisher() (socialhub.Publisher, bool)           { return nil, false }
func (client *Client) Fetcher() (socialhub.Fetcher, bool)               { return nil, false }
func (client *Client) MediaUploader() (socialhub.MediaUploader, bool)   { return nil, false }
func (client *Client) Reactor() (socialhub.Reactor, bool)               { return nil, false }
func (client *Client) Messenger() (socialhub.Messenger, bool)           { return nil, false }
func (client *Client) WebhookHandler() (socialhub.WebhookHandler, bool) { return nil, false }
func (client *Client) Close() error                                     { return nil }

// Places returns the bounded Tripadvisor Content API read workflow.
func (client *Client) Places() PlacesWorkflow { return client }

var _ socialhub.Client = (*Client)(nil)
