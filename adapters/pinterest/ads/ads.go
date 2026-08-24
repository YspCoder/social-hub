package ads

import (
	"context"
	"net/http"

	"social-hub/pkg/socialhub"
)

type adPage struct {
	Items    []Ad   `json:"items"`
	Bookmark string `json:"bookmark"`
}

type adMutationResource struct {
	ID               string       `json:"id,omitempty"`
	AdGroupID        string       `json:"ad_group_id,omitempty"`
	PinID            string       `json:"pin_id,omitempty"`
	Name             string       `json:"name,omitempty"`
	CreativeType     CreativeType `json:"creative_type,omitempty"`
	Status           EntityStatus `json:"status,omitempty"`
	DestinationURL   *string      `json:"destination_url,omitempty"`
	ClickTrackingURL *string      `json:"click_tracking_url,omitempty"`
	ViewTrackingURL  *string      `json:"view_tracking_url,omitempty"`
}

func (client *Client) ListAds(ctx context.Context, input ListAdsRequest, options ...socialhub.CallOption) (socialhub.Page[Ad], error) {
	const operation = "ads_list"
	if !validIDs(input.IDs) || !validIDs(input.CampaignIDs) || !validIDs(input.AdGroupIDs) ||
		!validStatuses(input.Statuses) || !validPage(input.Cursor, input.MaxResults) {
		return socialhub.Page[Ad]{}, invalidArgument(operation, "Ad filters, bookmark, or page size are invalid")
	}
	query := listQuery(input.Cursor, input.MaxResults)
	addQueryValues(query, "ad_ids", input.IDs)
	addQueryValues(query, "campaign_ids", input.CampaignIDs)
	addQueryValues(query, "ad_group_ids", input.AdGroupIDs)
	addStatusValues(query, input.Statuses)
	var response adPage
	if _, err := client.getJSON(ctx, operation, client.resourcePath("ads"), query, &response, options...); err != nil {
		return socialhub.Page[Ad]{}, err
	}
	for index := range response.Items {
		if err := client.validateAd(operation, &response.Items[index], ""); err != nil {
			return socialhub.Page[Ad]{}, err
		}
	}
	return toPage(response.Items, response.Bookmark), nil
}

func (client *Client) GetAd(ctx context.Context, id string, options ...socialhub.CallOption) (*Ad, error) {
	const operation = "ad_get"
	if !validID(id) {
		return nil, invalidArgument(operation, "Ad ID is invalid")
	}
	var response Ad
	if _, err := client.getJSON(ctx, operation, client.resourcePath("ads/"+id), nil, &response, options...); err != nil {
		return nil, err
	}
	if err := client.validateAd(operation, &response, id); err != nil {
		return nil, err
	}
	return &response, nil
}

func (client *Client) CreateAd(ctx context.Context, input CreateAdRequest, options ...socialhub.CallOption) (*Ad, error) {
	const operation = "ad_create"
	if !validID(input.AdGroupID) || !validID(input.PinID) || !validOptionalText(input.Name, 255) || !validCreativeType(input.CreativeType) ||
		!validHTTPURL(input.DestinationURL) || !validHTTPURL(input.ClickTrackingURL) || !validHTTPURL(input.ViewTrackingURL) {
		return nil, invalidArgument(operation, "Ad Group, Pin, name, creative type, or URL is invalid")
	}
	resource := adMutationResource{
		AdGroupID: input.AdGroupID, PinID: input.PinID, Name: input.Name,
		CreativeType: input.CreativeType, Status: StatusPaused,
	}
	setOptionalString(&resource.DestinationURL, input.DestinationURL)
	setOptionalString(&resource.ClickTrackingURL, input.ClickTrackingURL)
	setOptionalString(&resource.ViewTrackingURL, input.ViewTrackingURL)
	return client.mutateAd(ctx, operation, http.MethodPost, resource, "", options...)
}

func (client *Client) UpdateAd(ctx context.Context, id string, input UpdateAdRequest, options ...socialhub.CallOption) (*Ad, error) {
	const operation = "ad_update"
	if !validID(id) || input.Name == nil && input.DestinationURL == nil && input.ClickTrackingURL == nil && input.ViewTrackingURL == nil {
		return nil, invalidArgument(operation, "Ad ID and at least one update are required")
	}
	if input.Name != nil && !validOptionalText(*input.Name, 255) || input.DestinationURL != nil && !validHTTPURL(*input.DestinationURL) ||
		input.ClickTrackingURL != nil && !validHTTPURL(*input.ClickTrackingURL) || input.ViewTrackingURL != nil && !validHTTPURL(*input.ViewTrackingURL) {
		return nil, invalidArgument(operation, "one or more Ad update fields are invalid")
	}
	resource := adMutationResource{
		ID: id, DestinationURL: input.DestinationURL,
		ClickTrackingURL: input.ClickTrackingURL, ViewTrackingURL: input.ViewTrackingURL,
	}
	if input.Name != nil {
		resource.Name = *input.Name
	}
	return client.mutateAd(ctx, operation, http.MethodPatch, resource, id, options...)
}

func (client *Client) SetAdStatus(ctx context.Context, id string, status EntityStatus, options ...socialhub.CallOption) (*Ad, error) {
	if !validID(id) || !validMutationStatus(status) {
		return nil, invalidArgument("ad_status", "Ad ID and ACTIVE or PAUSED status are required")
	}
	return client.mutateAd(ctx, "ad_status", http.MethodPatch, adMutationResource{ID: id, Status: status}, id, options...)
}

func (client *Client) ArchiveAd(ctx context.Context, id string, options ...socialhub.CallOption) error {
	if !validID(id) {
		return invalidArgument("ad_archive", "Ad ID is invalid")
	}
	_, err := client.mutateAd(ctx, "ad_archive", http.MethodPatch, adMutationResource{ID: id, Status: StatusArchived}, id, options...)
	return err
}

func (client *Client) mutateAd(ctx context.Context, operation, method string, resource adMutationResource, expected string, options ...socialhub.CallOption) (*Ad, error) {
	var response batchResponse[Ad]
	metadata, err := client.writeJSON(ctx, operation, method, client.resourcePath("ads"), []adMutationResource{resource}, &response, options...)
	if err != nil {
		return nil, err
	}
	return requireBatchResult(operation, response, metadata, func(ad *Ad) error { return client.validateAd(operation, ad, expected) })
}

func (client *Client) validateAd(operation string, ad *Ad, expected string) error {
	if !validID(ad.ID) || expected != "" && ad.ID != expected || !validID(ad.CampaignID) || !validID(ad.AdGroupID) || !validID(ad.PinID) {
		return platformContractError(operation, "Pinterest returned an invalid or mismatched Ad, Campaign, Ad Group, or Pin ID")
	}
	if ad.AdAccountID != "" && ad.AdAccountID != client.adAccountID {
		return platformContractError(operation, "Pinterest returned an Ad owned by another Ad Account")
	}
	if ad.AdAccountID == "" {
		ad.AdAccountID = client.adAccountID
	}
	return nil
}

func setOptionalString(target **string, value string) {
	if value != "" {
		copy := value
		*target = &copy
	}
}
