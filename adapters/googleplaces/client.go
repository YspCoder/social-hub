package googleplaces

import (
	"context"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	CapabilityTextSearch   socialhub.Capability = "google_places_text_search"
	CapabilityNearbySearch socialhub.Capability = "google_places_nearby_search"
	CapabilityPlaceDetails socialhub.Capability = "google_places_place_details"
	CapabilityPhotoMedia   socialhub.Capability = "google_places_photo_media"
)

// Client exposes typed Places API (New) reads for one restricted API key.
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
		CapabilityTextSearch:   readState(CapabilityTextSearch, "Places API (New) Text Search with explicit fields", "https://developers.google.com/maps/documentation/places/web-service/text-search"),
		CapabilityNearbySearch: readState(CapabilityNearbySearch, "Places API (New) Nearby Search within a circle", "https://developers.google.com/maps/documentation/places/web-service/nearby-search"),
		CapabilityPlaceDetails: readState(CapabilityPlaceDetails, "Places API (New) Place Details with explicit fields", "https://developers.google.com/maps/documentation/places/web-service/place-details"),
		CapabilityPhotoMedia:   readState(CapabilityPhotoMedia, "photo metadata and non-redirecting short-lived media URL retrieval", "https://developers.google.com/maps/documentation/places/web-service/place-photos"),
		socialhub.CapPublish:   {Capability: socialhub.CapPublish, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Places API (New) is read-only in this adapter"},
		socialhub.CapFetch:     {Capability: socialhub.CapFetch, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "place resources use the typed Places workflow rather than generic social posts"},
		socialhub.CapMedia:     {Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "photo media is read-only and is not an upload capability"},
		socialhub.CapReact:     {Capability: socialhub.CapReact, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Places API (New) does not expose portable reaction writes"},
		socialhub.CapMessage:   {Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Places API (New) does not expose messaging"},
		socialhub.CapWebhook:   {Capability: socialhub.CapWebhook, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "the implemented Places API operations are request/response reads"},
	}, nil
}

func (client *Client) Publisher() (socialhub.Publisher, bool)           { return nil, false }
func (client *Client) Fetcher() (socialhub.Fetcher, bool)               { return nil, false }
func (client *Client) MediaUploader() (socialhub.MediaUploader, bool)   { return nil, false }
func (client *Client) Reactor() (socialhub.Reactor, bool)               { return nil, false }
func (client *Client) Messenger() (socialhub.Messenger, bool)           { return nil, false }
func (client *Client) WebhookHandler() (socialhub.WebhookHandler, bool) { return nil, false }
func (client *Client) Close() error                                     { return nil }

// Places returns the bounded Places API (New) v1 workflow.
func (client *Client) Places() PlacesWorkflow { return client }

var _ socialhub.Client = (*Client)(nil)
