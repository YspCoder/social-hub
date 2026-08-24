package marketing

import (
	"context"
	"net/http"

	"social-hub/pkg/socialhub"
)

type adItem struct {
	SubRequestStatus      string     `json:"sub_request_status"`
	SubRequestErrorReason string     `json:"sub_request_error_reason,omitempty"`
	Errors                []apiError `json:"errors,omitempty"`
	Ad                    *Ad        `json:"ad"`
}

type adResponse struct {
	responseMeta
	Ads    []adItem `json:"ads"`
	Paging paging   `json:"paging"`
}

type createAdPayload struct {
	AdSquadID  string       `json:"ad_squad_id"`
	CreativeID string       `json:"creative_id"`
	Name       string       `json:"name"`
	Type       string       `json:"type"`
	Status     EntityStatus `json:"status"`
}

func (client *Client) ListAds(ctx context.Context, input ListRequest, options ...socialhub.CallOption) (socialhub.Page[Ad], error) {
	const operation = "ads_list"
	if !validPage(input.Cursor, input.Limit, 1000) {
		return socialhub.Page[Ad]{}, invalidArgument(operation, "cursor or limit is invalid")
	}
	path := client.accountResourcePath("ads")
	var response adResponse
	if _, err := client.getJSON(ctx, operation, path, listQuery(input), &response, options...); err != nil {
		return socialhub.Page[Ad]{}, err
	}
	items, err := client.adItems(operation, response)
	if err != nil {
		return socialhub.Page[Ad]{}, err
	}
	cursor, err := client.pageCursor(operation, path, response.Paging.NextLink)
	if err != nil {
		return socialhub.Page[Ad]{}, err
	}
	return socialhub.Page[Ad]{Items: items, NextCursor: cursor, HasMore: cursor != nil}, nil
}

func (client *Client) GetAd(ctx context.Context, id string, options ...socialhub.CallOption) (*Ad, error) {
	const operation = "ad_get"
	if !validUUID(id) {
		return nil, invalidArgument(operation, "Ad ID must be a UUID")
	}
	value, err := client.getAd(ctx, operation, id, options...)
	if err != nil {
		return nil, err
	}
	if _, err := client.GetAdSquad(ctx, value.AdSquadID, options...); err != nil {
		return nil, err
	}
	return value, nil
}

func (client *Client) CreateAd(ctx context.Context, input CreateAdRequest, options ...socialhub.CallOption) (*Ad, error) {
	const operation = "ad_create"
	if !validUUID(input.AdSquadID) || !validUUID(input.CreativeID) || !validText(input.Name, 256) {
		return nil, invalidArgument(operation, "Ad Squad ID, Creative ID, and name are required")
	}
	if _, err := client.GetAdSquad(ctx, input.AdSquadID, options...); err != nil {
		return nil, err
	}
	payload := struct {
		Ads []createAdPayload `json:"ads"`
	}{Ads: []createAdPayload{{
		AdSquadID: input.AdSquadID, CreativeID: input.CreativeID,
		Name: input.Name, Type: "SNAP_AD", Status: StatusPaused,
	}}}
	var response adResponse
	path := "/adsquads/" + input.AdSquadID + "/ads"
	if _, err := client.writeJSON(ctx, operation, http.MethodPost, path, payload, &response, options...); err != nil {
		return nil, err
	}
	created, err := client.singleAd(operation, response, "", input.AdSquadID)
	if err != nil {
		return nil, err
	}
	return client.GetAd(ctx, created.ID, options...)
}

func (client *Client) UpdateAd(ctx context.Context, id string, input UpdateEntityRequest, options ...socialhub.CallOption) (*Ad, error) {
	return client.patchAd(ctx, "ad_update", id, input, options...)
}

func (client *Client) SetAdStatus(ctx context.Context, id string, status EntityStatus, options ...socialhub.CallOption) (*Ad, error) {
	return client.patchAd(ctx, "ad_status", id, UpdateEntityRequest{Status: &status}, options...)
}

func (client *Client) patchAd(ctx context.Context, operation, id string, input UpdateEntityRequest, options ...socialhub.CallOption) (*Ad, error) {
	operations, err := updateOperations(operation, id, input)
	if err != nil {
		return nil, err
	}
	current, err := client.GetAd(ctx, id, options...)
	if err != nil {
		return nil, err
	}
	var response adResponse
	path := "/adsquads/" + current.AdSquadID + "/ads/" + id
	if _, err := client.writeJSON(ctx, operation, http.MethodPatch, path, operations, &response, options...); err != nil {
		return nil, err
	}
	if _, err := client.singleAd(operation, response, id, current.AdSquadID); err != nil {
		return nil, err
	}
	return client.GetAd(ctx, id, options...)
}

func (client *Client) getAd(ctx context.Context, operation, id string, options ...socialhub.CallOption) (*Ad, error) {
	var response adResponse
	if _, err := client.getJSON(ctx, operation, "/ads/"+id, nil, &response, options...); err != nil {
		return nil, err
	}
	return client.singleAd(operation, response, id, "")
}

func (client *Client) adItems(operation string, response adResponse) ([]Ad, error) {
	states := make([]subRequestState, len(response.Ads))
	for index, item := range response.Ads {
		states[index] = subRequestState{Status: item.SubRequestStatus, Reason: item.SubRequestErrorReason, Errors: item.Errors}
	}
	if err := checkResponse(operation, response.responseMeta, states); err != nil {
		return nil, err
	}
	items := make([]Ad, len(response.Ads))
	for index, item := range response.Ads {
		if item.Ad == nil {
			return nil, platformContractError(operation, "Snapchat Ad result omitted the Ad")
		}
		if err := client.validateAd(operation, item.Ad, "", ""); err != nil {
			return nil, err
		}
		items[index] = *item.Ad
	}
	return items, nil
}

func (client *Client) singleAd(operation string, response adResponse, expectedID, expectedAdSquadID string) (*Ad, error) {
	if len(response.Ads) != 1 {
		return nil, platformContractError(operation, "Snapchat did not return exactly one Ad result")
	}
	items, err := client.adItems(operation, response)
	if err != nil {
		return nil, err
	}
	if err := client.validateAd(operation, &items[0], expectedID, expectedAdSquadID); err != nil {
		return nil, err
	}
	return &items[0], nil
}

func (client *Client) validateAd(operation string, value *Ad, expectedID, expectedAdSquadID string) error {
	if !validUUID(value.ID) || expectedID != "" && value.ID != expectedID || !validUUID(value.AdSquadID) ||
		expectedAdSquadID != "" && value.AdSquadID != expectedAdSquadID || !validUUID(value.CreativeID) {
		return platformContractError(operation, "Snapchat returned a missing or mismatched Ad identity")
	}
	if value.AdAccountID != "" && value.AdAccountID != client.adAccountID {
		return platformContractError(operation, "Snapchat returned an Ad owned by another Ad Account")
	}
	if value.AdAccountID == "" {
		value.AdAccountID = client.adAccountID
	}
	return nil
}
