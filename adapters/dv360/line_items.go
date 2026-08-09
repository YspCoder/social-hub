package dv360

import (
	"context"
	"net/url"

	"social-hub/pkg/socialhub"
)

var lineItemOrderFields = map[string]struct{}{
	"displayName": {}, "entityStatus": {}, "updateTime": {},
}

type lineItemCreatePayload struct {
	InsertionOrderID       string                       `json:"insertionOrderId"`
	DisplayName            string                       `json:"displayName"`
	LineItemType           LineItemType                 `json:"lineItemType"`
	EntityStatus           EntityStatus                 `json:"entityStatus"`
	Flight                 LineItemFlight               `json:"flight"`
	Budget                 LineItemBudget               `json:"budget"`
	Pacing                 Pacing                       `json:"pacing"`
	PartnerRevenueModel    PartnerRevenueModel          `json:"partnerRevenueModel"`
	BidStrategy            BiddingStrategy              `json:"bidStrategy"`
	FrequencyCap           FrequencyCap                 `json:"frequencyCap"`
	ContainsEUPoliticalAds EUPoliticalAdvertisingStatus `json:"containsEuPoliticalAds"`
}

type lineItemPatchPayload struct {
	DisplayName            *string                       `json:"displayName,omitempty"`
	EntityStatus           *EntityStatus                 `json:"entityStatus,omitempty"`
	Flight                 *LineItemFlight               `json:"flight,omitempty"`
	Budget                 *LineItemBudget               `json:"budget,omitempty"`
	Pacing                 *Pacing                       `json:"pacing,omitempty"`
	PartnerRevenueModel    *PartnerRevenueModel          `json:"partnerRevenueModel,omitempty"`
	BidStrategy            *BiddingStrategy              `json:"bidStrategy,omitempty"`
	FrequencyCap           *FrequencyCap                 `json:"frequencyCap,omitempty"`
	ContainsEUPoliticalAds *EUPoliticalAdvertisingStatus `json:"containsEuPoliticalAds,omitempty"`
}

type duplicateLineItemPayload struct {
	TargetDisplayName      string                       `json:"targetDisplayName"`
	ContainsEUPoliticalAds EUPoliticalAdvertisingStatus `json:"containsEuPoliticalAds"`
}

type duplicateLineItemResponse struct {
	DuplicateLineItemID string `json:"duplicateLineItemId"`
}

func (client *Client) GetLineItem(ctx context.Context, lineItemID string, options ...socialhub.CallOption) (LineItem, error) {
	const operation = "line_item_get"
	if !validID(lineItemID) {
		return LineItem{}, invalidArgument(operation, "line item ID must be a positive string-encoded integer")
	}
	var lineItem LineItem
	if err := client.getJSON(ctx, operation, client.lineItemsPath()+"/"+lineItemID, nil, &lineItem, options...); err != nil {
		return LineItem{}, err
	}
	if err := client.validateLineItem(operation, lineItem); err != nil {
		return LineItem{}, err
	}
	if lineItem.LineItemID != lineItemID {
		return LineItem{}, platformContractError(operation, "DV360 returned a different line item")
	}
	return lineItem, nil
}

func (client *Client) ListLineItems(ctx context.Context, input ListRequest, options ...socialhub.CallOption) (Page[LineItem], error) {
	const operation = "line_item_list"
	if !validPage(input, 200, lineItemOrderFields) {
		return Page[LineItem]{}, invalidArgument(operation, "pagination, filter, or order is invalid")
	}
	var response listLineItemsResponse
	if err := client.getJSON(ctx, operation, client.lineItemsPath(), listQuery(input), &response, options...); err != nil {
		return Page[LineItem]{}, err
	}
	seen := make(map[string]struct{}, len(response.LineItems))
	for _, lineItem := range response.LineItems {
		if err := client.validateLineItem(operation, lineItem); err != nil {
			return Page[LineItem]{}, err
		}
		if _, exists := seen[lineItem.LineItemID]; exists {
			return Page[LineItem]{}, platformContractError(operation, "DV360 returned duplicate line items")
		}
		seen[lineItem.LineItemID] = struct{}{}
	}
	if !validPageToken(response.NextPageToken) {
		return Page[LineItem]{}, platformContractError(operation, "DV360 returned an invalid page token")
	}
	return Page[LineItem]{Items: response.LineItems, NextPageToken: response.NextPageToken}, nil
}

func (client *Client) CreateLineItem(ctx context.Context, input CreateLineItemRequest, options ...socialhub.CallOption) (LineItem, error) {
	const operation = "line_item_create"
	if !validCreateLineItem(input) {
		return LineItem{}, invalidArgument(operation, "line item fields are invalid")
	}
	order, err := client.GetInsertionOrder(ctx, input.InsertionOrderID, options...)
	if err != nil {
		return LineItem{}, withOperation(err, operation)
	}
	if _, err := client.GetCampaign(ctx, order.CampaignID, options...); err != nil {
		return LineItem{}, withOperation(err, operation)
	}
	if !validPacingForBudget(input.Pacing, order.Budget.BudgetUnit) {
		return LineItem{}, invalidArgument(operation, "pacing does not match the parent insertion order budget unit")
	}
	payload := lineItemCreatePayload{
		InsertionOrderID: input.InsertionOrderID, DisplayName: input.DisplayName,
		LineItemType: input.LineItemType, EntityStatus: EntityStatusDraft,
		Flight: input.Flight, Budget: input.Budget, Pacing: input.Pacing,
		PartnerRevenueModel: input.PartnerRevenueModel, BidStrategy: input.BidStrategy,
		FrequencyCap: input.FrequencyCap, ContainsEUPoliticalAds: input.ContainsEUPoliticalAds,
	}
	var lineItem LineItem
	if err := client.postJSON(ctx, operation, client.lineItemsPath(), payload, &lineItem, options...); err != nil {
		return LineItem{}, err
	}
	if err := client.validateLineItem(operation, lineItem); err != nil {
		return LineItem{}, err
	}
	if lineItem.InsertionOrderID != input.InsertionOrderID || lineItem.CampaignID != order.CampaignID ||
		lineItem.DisplayName != input.DisplayName || lineItem.EntityStatus != EntityStatusDraft {
		return LineItem{}, platformContractError(operation, "new DV360 line item was not returned draft under the requested insertion order")
	}
	return lineItem, nil
}

func (client *Client) UpdateLineItem(ctx context.Context, lineItemID string, input UpdateLineItemRequest, options ...socialhub.CallOption) (LineItem, error) {
	const operation = "line_item_update"
	mask, err := validateLineItemPatch(lineItemID, input)
	if err != nil {
		return LineItem{}, withOperation(err, operation)
	}
	current, err := client.GetLineItem(ctx, lineItemID, options...)
	if err != nil {
		return LineItem{}, withOperation(err, operation)
	}
	if !validLineItemType(current.LineItemType) {
		return LineItem{}, unsupportedError(operation, "only standard Display, Video, and Audio RTB line items can be updated")
	}
	if input.EntityStatus != nil && *input.EntityStatus == EntityStatusActive {
		order, err := client.GetInsertionOrder(ctx, current.InsertionOrderID, options...)
		if err != nil {
			return LineItem{}, withOperation(err, operation)
		}
		if order.EntityStatus != EntityStatusActive {
			return LineItem{}, conflictError(operation, "line item cannot be activated while its insertion order is not active")
		}
		campaign, err := client.GetCampaign(ctx, order.CampaignID, options...)
		if err != nil {
			return LineItem{}, withOperation(err, operation)
		}
		if campaign.EntityStatus != EntityStatusActive {
			return LineItem{}, conflictError(operation, "line item cannot be activated while its campaign is not active")
		}
	}
	payload := lineItemPatchPayload{
		DisplayName: input.DisplayName, EntityStatus: input.EntityStatus, Flight: input.Flight,
		Budget: input.Budget, Pacing: input.Pacing, PartnerRevenueModel: input.PartnerRevenueModel,
		BidStrategy: input.BidStrategy, FrequencyCap: input.FrequencyCap,
		ContainsEUPoliticalAds: input.ContainsEUPoliticalAds,
	}
	query := url.Values{"updateMask": {mask}}
	var updated LineItem
	if err := client.patchJSON(ctx, operation, client.lineItemsPath()+"/"+lineItemID, query, payload, &updated, options...); err != nil {
		return LineItem{}, err
	}
	if err := client.validateLineItem(operation, updated); err != nil {
		return LineItem{}, err
	}
	if updated.LineItemID != lineItemID || updated.InsertionOrderID != current.InsertionOrderID ||
		updated.CampaignID != current.CampaignID || !lineItemMatchesPatch(updated, input) {
		return LineItem{}, platformContractError(operation, "DV360 returned a line item that does not match the update")
	}
	return updated, nil
}

func (client *Client) DuplicateLineItem(ctx context.Context, lineItemID string, input DuplicateLineItemRequest, options ...socialhub.CallOption) (LineItem, error) {
	const operation = "line_item_duplicate"
	if !validID(lineItemID) || !validDisplayName(input.TargetDisplayName) || !validPoliticalStatus(input.ContainsEUPoliticalAds) {
		return LineItem{}, invalidArgument(operation, "source ID, target display name, or EU political advertising status is invalid")
	}
	source, err := client.GetLineItem(ctx, lineItemID, options...)
	if err != nil {
		return LineItem{}, withOperation(err, operation)
	}
	if !validLineItemType(source.LineItemType) {
		return LineItem{}, unsupportedError(operation, "YouTube, Demand Gen, and other allowlisted line items cannot be duplicated by this adapter")
	}
	payload := duplicateLineItemPayload(input)
	var response duplicateLineItemResponse
	if err := client.postJSON(ctx, operation, client.lineItemsPath()+"/"+lineItemID+":duplicate", payload, &response, options...); err != nil {
		return LineItem{}, err
	}
	if !validID(response.DuplicateLineItemID) || response.DuplicateLineItemID == lineItemID {
		return LineItem{}, platformContractError(operation, "DV360 returned an invalid duplicate line item ID")
	}
	duplicate, err := client.GetLineItem(ctx, response.DuplicateLineItemID, options...)
	if err != nil {
		return LineItem{}, withOperation(err, operation)
	}
	if duplicate.InsertionOrderID != source.InsertionOrderID || duplicate.CampaignID != source.CampaignID ||
		duplicate.DisplayName != input.TargetDisplayName || duplicate.ContainsEUPoliticalAds != input.ContainsEUPoliticalAds {
		return LineItem{}, platformContractError(operation, "DV360 returned a duplicate that does not match the request")
	}
	return duplicate, nil
}

func validCreateLineItem(input CreateLineItemRequest) bool {
	return validID(input.InsertionOrderID) && validDisplayName(input.DisplayName) && validLineItemType(input.LineItemType) &&
		validLineItemFlight(input.Flight) && input.Budget.BudgetUnit == "" && validLineItemBudget(input.Budget) &&
		validPacing(input.Pacing) && validPartnerRevenue(input.PartnerRevenueModel) &&
		validBiddingStrategy(input.BidStrategy, false) && validFrequencyCap(input.FrequencyCap) &&
		validPoliticalStatus(input.ContainsEUPoliticalAds)
}

func validateLineItemPatch(id string, input UpdateLineItemRequest) (string, error) {
	if !validID(id) {
		return "", invalidArgument("line_item_update", "line item ID is invalid")
	}
	fields := make([]string, 0, 9)
	if input.DisplayName != nil {
		if !validDisplayName(*input.DisplayName) {
			return "", invalidArgument("line_item_update", "display name is invalid")
		}
		fields = append(fields, "displayName")
	}
	if input.EntityStatus != nil {
		if !validUpdateEntityStatus(*input.EntityStatus) {
			return "", invalidArgument("line_item_update", "entity status is invalid; draft cannot be restored")
		}
		fields = append(fields, "entityStatus")
	}
	if input.Flight != nil {
		if !validLineItemFlight(*input.Flight) {
			return "", invalidArgument("line_item_update", "flight is invalid")
		}
		fields = append(fields, "flight")
	}
	if input.Budget != nil {
		if input.Budget.BudgetUnit != "" || !validLineItemBudget(*input.Budget) {
			return "", invalidArgument("line_item_update", "budget is invalid")
		}
		fields = append(fields, "budget")
	}
	if input.Pacing != nil {
		if !validPacing(*input.Pacing) {
			return "", invalidArgument("line_item_update", "pacing is invalid")
		}
		fields = append(fields, "pacing")
	}
	if input.PartnerRevenueModel != nil {
		if !validPartnerRevenue(*input.PartnerRevenueModel) {
			return "", invalidArgument("line_item_update", "partner revenue model is invalid")
		}
		fields = append(fields, "partnerRevenueModel")
	}
	if input.BidStrategy != nil {
		if !validBiddingStrategy(*input.BidStrategy, false) {
			return "", invalidArgument("line_item_update", "bid strategy is invalid")
		}
		fields = append(fields, "bidStrategy")
	}
	if input.FrequencyCap != nil {
		if !validFrequencyCap(*input.FrequencyCap) {
			return "", invalidArgument("line_item_update", "frequency cap is invalid")
		}
		fields = append(fields, "frequencyCap")
	}
	if input.ContainsEUPoliticalAds != nil {
		if !validPoliticalStatus(*input.ContainsEUPoliticalAds) {
			return "", invalidArgument("line_item_update", "EU political advertising status is invalid")
		}
		fields = append(fields, "containsEuPoliticalAds")
	}
	if len(fields) == 0 {
		return "", invalidArgument("line_item_update", "at least one field must be updated")
	}
	return joinMask(fields), nil
}

func (client *Client) validateLineItem(operation string, lineItem LineItem) error {
	if !validID(lineItem.LineItemID) || !validID(lineItem.AdvertiserID) || !validID(lineItem.CampaignID) ||
		!validID(lineItem.InsertionOrderID) || !validDisplayName(lineItem.DisplayName) ||
		!validReadLineItemType(lineItem.LineItemType) || !validReadEntityStatus(lineItem.EntityStatus) {
		return platformContractError(operation, "DV360 returned an invalid line item")
	}
	if lineItem.AdvertiserID != client.advertiserID {
		return ownershipError(operation, "line item")
	}
	return nil
}

func lineItemMatchesPatch(lineItem LineItem, input UpdateLineItemRequest) bool {
	return (input.DisplayName == nil || lineItem.DisplayName == *input.DisplayName) &&
		(input.EntityStatus == nil || lineItem.EntityStatus == *input.EntityStatus) &&
		(input.ContainsEUPoliticalAds == nil || lineItem.ContainsEUPoliticalAds == *input.ContainsEUPoliticalAds)
}

func (client *Client) lineItemsPath() string {
	return "/v4/advertisers/" + client.advertiserID + "/lineItems"
}
