package marketing

import (
	"context"
	"net/http"
	"strings"
	"time"

	"social-hub/pkg/socialhub"
)

type adSquadItem struct {
	SubRequestStatus      string     `json:"sub_request_status"`
	SubRequestErrorReason string     `json:"sub_request_error_reason,omitempty"`
	Errors                []apiError `json:"errors,omitempty"`
	AdSquad               *AdSquad   `json:"adsquad"`
}

type adSquadResponse struct {
	responseMeta
	AdSquads []adSquadItem `json:"adsquads"`
	Paging   paging        `json:"paging"`
}

type createAdSquadPayload struct {
	CampaignID         string       `json:"campaign_id"`
	Name               string       `json:"name"`
	Status             EntityStatus `json:"status"`
	Type               string       `json:"type"`
	PlacementV2        PlacementV2  `json:"placement_v2"`
	OptimizationGoal   string       `json:"optimization_goal"`
	BillingEvent       string       `json:"billing_event"`
	BidStrategy        string       `json:"bid_strategy"`
	BidMicro           int64        `json:"bid_micro"`
	DailyBudgetMicro   int64        `json:"daily_budget_micro"`
	DeliveryConstraint string       `json:"delivery_constraint"`
	Targeting          Targeting    `json:"targeting"`
	StartTime          string       `json:"start_time"`
}

func (client *Client) ListAdSquads(ctx context.Context, input ListRequest, options ...socialhub.CallOption) (socialhub.Page[AdSquad], error) {
	const operation = "ad_squads_list"
	if !validPage(input.Cursor, input.Limit, 1000) {
		return socialhub.Page[AdSquad]{}, invalidArgument(operation, "cursor or limit is invalid")
	}
	path := client.accountResourcePath("adsquads")
	var response adSquadResponse
	if _, err := client.getJSON(ctx, operation, path, listQuery(input), &response, options...); err != nil {
		return socialhub.Page[AdSquad]{}, err
	}
	items, err := client.adSquadItems(operation, response)
	if err != nil {
		return socialhub.Page[AdSquad]{}, err
	}
	cursor, err := client.pageCursor(operation, path, response.Paging.NextLink)
	if err != nil {
		return socialhub.Page[AdSquad]{}, err
	}
	return socialhub.Page[AdSquad]{Items: items, NextCursor: cursor, HasMore: cursor != nil}, nil
}

func (client *Client) GetAdSquad(ctx context.Context, id string, options ...socialhub.CallOption) (*AdSquad, error) {
	const operation = "ad_squad_get"
	if !validUUID(id) {
		return nil, invalidArgument(operation, "Ad Squad ID must be a UUID")
	}
	value, err := client.getAdSquad(ctx, operation, id, options...)
	if err != nil {
		return nil, err
	}
	if _, err := client.GetCampaign(ctx, value.CampaignID, options...); err != nil {
		return nil, err
	}
	return value, nil
}

func (client *Client) CreateAdSquad(ctx context.Context, input CreateAdSquadRequest, options ...socialhub.CallOption) (*AdSquad, error) {
	const operation = "ad_squad_create"
	if !validUUID(input.CampaignID) || !validText(input.Name, 256) || input.BidMicro <= 0 || input.DailyBudgetMicro <= 0 ||
		!validCountryCodes(input.CountryCodes) || input.StartTime.IsZero() {
		return nil, invalidArgument(operation, "Campaign ID, name, bid, daily budget, countries, or start time is invalid")
	}
	if _, err := client.GetCampaign(ctx, input.CampaignID, options...); err != nil {
		return nil, err
	}
	geos := make([]GeoTarget, len(input.CountryCodes))
	for index, code := range input.CountryCodes {
		geos[index] = GeoTarget{CountryCode: strings.ToLower(code)}
	}
	payload := struct {
		AdSquads []createAdSquadPayload `json:"adsquads"`
	}{AdSquads: []createAdSquadPayload{{
		CampaignID: input.CampaignID, Name: input.Name, Status: StatusPaused, Type: "SNAP_ADS",
		PlacementV2: PlacementV2{Config: "AUTOMATIC"}, OptimizationGoal: "IMPRESSIONS", BillingEvent: "IMPRESSION",
		BidStrategy: "LOWEST_COST_WITH_MAX_BID", BidMicro: input.BidMicro, DailyBudgetMicro: input.DailyBudgetMicro,
		DeliveryConstraint: "DAILY_BUDGET", Targeting: Targeting{Geos: geos}, StartTime: input.StartTime.UTC().Format(time.RFC3339),
	}}}
	var response adSquadResponse
	path := "/campaigns/" + input.CampaignID + "/adsquads"
	if _, err := client.writeJSON(ctx, operation, http.MethodPost, path, payload, &response, options...); err != nil {
		return nil, err
	}
	created, err := client.singleAdSquad(operation, response, "", input.CampaignID)
	if err != nil {
		return nil, err
	}
	return client.GetAdSquad(ctx, created.ID, options...)
}

func (client *Client) UpdateAdSquad(ctx context.Context, id string, input UpdateEntityRequest, options ...socialhub.CallOption) (*AdSquad, error) {
	return client.patchAdSquad(ctx, "ad_squad_update", id, input, options...)
}

func (client *Client) SetAdSquadStatus(ctx context.Context, id string, status EntityStatus, options ...socialhub.CallOption) (*AdSquad, error) {
	return client.patchAdSquad(ctx, "ad_squad_status", id, UpdateEntityRequest{Status: &status}, options...)
}

func (client *Client) patchAdSquad(ctx context.Context, operation, id string, input UpdateEntityRequest, options ...socialhub.CallOption) (*AdSquad, error) {
	operations, err := updateOperations(operation, id, input)
	if err != nil {
		return nil, err
	}
	current, err := client.GetAdSquad(ctx, id, options...)
	if err != nil {
		return nil, err
	}
	var response adSquadResponse
	path := "/campaigns/" + current.CampaignID + "/adsquads/" + id
	if _, err := client.writeJSON(ctx, operation, http.MethodPatch, path, operations, &response, options...); err != nil {
		return nil, err
	}
	if _, err := client.singleAdSquad(operation, response, id, current.CampaignID); err != nil {
		return nil, err
	}
	return client.GetAdSquad(ctx, id, options...)
}

func (client *Client) getAdSquad(ctx context.Context, operation, id string, options ...socialhub.CallOption) (*AdSquad, error) {
	var response adSquadResponse
	if _, err := client.getJSON(ctx, operation, "/adsquads/"+id, nil, &response, options...); err != nil {
		return nil, err
	}
	return client.singleAdSquad(operation, response, id, "")
}

func (client *Client) adSquadItems(operation string, response adSquadResponse) ([]AdSquad, error) {
	states := make([]subRequestState, len(response.AdSquads))
	for index, item := range response.AdSquads {
		states[index] = subRequestState{Status: item.SubRequestStatus, Reason: item.SubRequestErrorReason, Errors: item.Errors}
	}
	if err := checkResponse(operation, response.responseMeta, states); err != nil {
		return nil, err
	}
	items := make([]AdSquad, len(response.AdSquads))
	for index, item := range response.AdSquads {
		if item.AdSquad == nil {
			return nil, platformContractError(operation, "Snapchat Ad Squad result omitted the Ad Squad")
		}
		if err := client.validateAdSquad(operation, item.AdSquad, "", ""); err != nil {
			return nil, err
		}
		items[index] = *item.AdSquad
	}
	return items, nil
}

func (client *Client) singleAdSquad(operation string, response adSquadResponse, expectedID, expectedCampaignID string) (*AdSquad, error) {
	if len(response.AdSquads) != 1 {
		return nil, platformContractError(operation, "Snapchat did not return exactly one Ad Squad result")
	}
	items, err := client.adSquadItems(operation, response)
	if err != nil {
		return nil, err
	}
	if err := client.validateAdSquad(operation, &items[0], expectedID, expectedCampaignID); err != nil {
		return nil, err
	}
	return &items[0], nil
}

func (client *Client) validateAdSquad(operation string, value *AdSquad, expectedID, expectedCampaignID string) error {
	if !validUUID(value.ID) || expectedID != "" && value.ID != expectedID || !validUUID(value.CampaignID) ||
		expectedCampaignID != "" && value.CampaignID != expectedCampaignID {
		return platformContractError(operation, "Snapchat returned a missing or mismatched Ad Squad identity")
	}
	if value.AdAccountID != "" && value.AdAccountID != client.adAccountID {
		return platformContractError(operation, "Snapchat returned an Ad Squad owned by another Ad Account")
	}
	if value.AdAccountID == "" {
		value.AdAccountID = client.adAccountID
	}
	return nil
}

func validCountryCodes(values []string) bool {
	if len(values) == 0 || len(values) > 250 {
		return false
	}
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		lower := strings.ToLower(value)
		if len(lower) != 2 || lower[0] < 'a' || lower[0] > 'z' || lower[1] < 'a' || lower[1] > 'z' {
			return false
		}
		if _, found := seen[lower]; found {
			return false
		}
		seen[lower] = struct{}{}
	}
	return true
}
