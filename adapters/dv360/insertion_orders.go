package dv360

import (
	"context"
	"net/url"

	"social-hub/pkg/socialhub"
)

var insertionOrderOrderFields = map[string]struct{}{
	"displayName": {}, "entityStatus": {}, "updateTime": {},
}

type insertionOrderCreatePayload struct {
	CampaignID            string                `json:"campaignId"`
	DisplayName           string                `json:"displayName"`
	EntityStatus          EntityStatus          `json:"entityStatus"`
	InsertionOrderType    InsertionOrderType    `json:"insertionOrderType,omitempty"`
	Pacing                Pacing                `json:"pacing"`
	FrequencyCap          FrequencyCap          `json:"frequencyCap"`
	Budget                InsertionOrderBudget  `json:"budget"`
	KPI                   KPI                   `json:"kpi"`
	OptimizationObjective OptimizationObjective `json:"optimizationObjective"`
	BidStrategy           *BiddingStrategy      `json:"bidStrategy,omitempty"`
}

type insertionOrderPatchPayload struct {
	DisplayName           *string                `json:"displayName,omitempty"`
	EntityStatus          *EntityStatus          `json:"entityStatus,omitempty"`
	Pacing                *Pacing                `json:"pacing,omitempty"`
	FrequencyCap          *FrequencyCap          `json:"frequencyCap,omitempty"`
	Budget                *InsertionOrderBudget  `json:"budget,omitempty"`
	KPI                   *KPI                   `json:"kpi,omitempty"`
	OptimizationObjective *OptimizationObjective `json:"optimizationObjective,omitempty"`
	BidStrategy           *BiddingStrategy       `json:"bidStrategy,omitempty"`
}

func (client *Client) GetInsertionOrder(ctx context.Context, insertionOrderID string, options ...socialhub.CallOption) (InsertionOrder, error) {
	const operation = "insertion_order_get"
	if !validID(insertionOrderID) {
		return InsertionOrder{}, invalidArgument(operation, "insertion order ID must be a positive string-encoded integer")
	}
	var order InsertionOrder
	if err := client.getJSON(ctx, operation, client.insertionOrdersPath()+"/"+insertionOrderID, nil, &order, options...); err != nil {
		return InsertionOrder{}, err
	}
	if err := client.validateInsertionOrder(operation, order); err != nil {
		return InsertionOrder{}, err
	}
	if order.InsertionOrderID != insertionOrderID {
		return InsertionOrder{}, platformContractError(operation, "DV360 returned a different insertion order")
	}
	return order, nil
}

func (client *Client) ListInsertionOrders(ctx context.Context, input ListRequest, options ...socialhub.CallOption) (Page[InsertionOrder], error) {
	const operation = "insertion_order_list"
	if !validPage(input, 100, insertionOrderOrderFields) {
		return Page[InsertionOrder]{}, invalidArgument(operation, "pagination, filter, or order is invalid")
	}
	var response listInsertionOrdersResponse
	if err := client.getJSON(ctx, operation, client.insertionOrdersPath(), listQuery(input), &response, options...); err != nil {
		return Page[InsertionOrder]{}, err
	}
	seen := make(map[string]struct{}, len(response.InsertionOrders))
	for _, order := range response.InsertionOrders {
		if err := client.validateInsertionOrder(operation, order); err != nil {
			return Page[InsertionOrder]{}, err
		}
		if _, exists := seen[order.InsertionOrderID]; exists {
			return Page[InsertionOrder]{}, platformContractError(operation, "DV360 returned duplicate insertion orders")
		}
		seen[order.InsertionOrderID] = struct{}{}
	}
	if !validPageToken(response.NextPageToken) {
		return Page[InsertionOrder]{}, platformContractError(operation, "DV360 returned an invalid page token")
	}
	return Page[InsertionOrder]{Items: response.InsertionOrders, NextPageToken: response.NextPageToken}, nil
}

func (client *Client) CreateInsertionOrder(ctx context.Context, input CreateInsertionOrderRequest, options ...socialhub.CallOption) (InsertionOrder, error) {
	const operation = "insertion_order_create"
	if !validCreateInsertionOrder(input) {
		return InsertionOrder{}, invalidArgument(operation, "insertion order fields are invalid")
	}
	if _, err := client.GetCampaign(ctx, input.CampaignID, options...); err != nil {
		return InsertionOrder{}, withOperation(err, operation)
	}
	payload := insertionOrderCreatePayload{
		CampaignID: input.CampaignID, DisplayName: input.DisplayName, EntityStatus: EntityStatusDraft,
		InsertionOrderType: input.InsertionOrderType, Pacing: input.Pacing, FrequencyCap: input.FrequencyCap,
		Budget: input.Budget, KPI: input.KPI, OptimizationObjective: input.OptimizationObjective,
		BidStrategy: input.BidStrategy,
	}
	var order InsertionOrder
	if err := client.postJSON(ctx, operation, client.insertionOrdersPath(), payload, &order, options...); err != nil {
		return InsertionOrder{}, err
	}
	if err := client.validateInsertionOrder(operation, order); err != nil {
		return InsertionOrder{}, err
	}
	if order.CampaignID != input.CampaignID || order.DisplayName != input.DisplayName || order.EntityStatus != EntityStatusDraft {
		return InsertionOrder{}, platformContractError(operation, "new DV360 insertion order was not returned draft under the requested campaign")
	}
	return order, nil
}

func (client *Client) UpdateInsertionOrder(ctx context.Context, insertionOrderID string, input UpdateInsertionOrderRequest, options ...socialhub.CallOption) (InsertionOrder, error) {
	const operation = "insertion_order_update"
	mask, err := validateInsertionOrderPatch(insertionOrderID, input)
	if err != nil {
		return InsertionOrder{}, withOperation(err, operation)
	}
	current, err := client.GetInsertionOrder(ctx, insertionOrderID, options...)
	if err != nil {
		return InsertionOrder{}, withOperation(err, operation)
	}
	if input.EntityStatus != nil && *input.EntityStatus == EntityStatusActive {
		campaign, err := client.GetCampaign(ctx, current.CampaignID, options...)
		if err != nil {
			return InsertionOrder{}, withOperation(err, operation)
		}
		if campaign.EntityStatus != EntityStatusActive {
			return InsertionOrder{}, conflictError(operation, "insertion order cannot be activated while its campaign is not active")
		}
	}
	payload := insertionOrderPatchPayload{
		DisplayName: input.DisplayName, EntityStatus: input.EntityStatus, Pacing: input.Pacing,
		FrequencyCap: input.FrequencyCap, Budget: input.Budget, KPI: input.KPI,
		OptimizationObjective: input.OptimizationObjective, BidStrategy: input.BidStrategy,
	}
	query := url.Values{"updateMask": {mask}}
	var updated InsertionOrder
	if err := client.patchJSON(ctx, operation, client.insertionOrdersPath()+"/"+insertionOrderID, query, payload, &updated, options...); err != nil {
		return InsertionOrder{}, err
	}
	if err := client.validateInsertionOrder(operation, updated); err != nil {
		return InsertionOrder{}, err
	}
	if updated.InsertionOrderID != insertionOrderID || updated.CampaignID != current.CampaignID || !insertionOrderMatchesPatch(updated, input) {
		return InsertionOrder{}, platformContractError(operation, "DV360 returned an insertion order that does not match the update")
	}
	return updated, nil
}

func validCreateInsertionOrder(input CreateInsertionOrderRequest) bool {
	if !validID(input.CampaignID) || !validDisplayName(input.DisplayName) || !validInsertionOrderType(input.InsertionOrderType) ||
		!validInsertionOrderBudget(input.Budget) || !validPacingForBudget(input.Pacing, input.Budget.BudgetUnit) ||
		!validFrequencyCap(input.FrequencyCap) || !validKPI(input.KPI) || !validOptimizationObjective(input.OptimizationObjective) {
		return false
	}
	return input.BidStrategy == nil || validBiddingStrategy(*input.BidStrategy, true)
}

func validateInsertionOrderPatch(id string, input UpdateInsertionOrderRequest) (string, error) {
	if !validID(id) {
		return "", invalidArgument("insertion_order_update", "insertion order ID is invalid")
	}
	fields := make([]string, 0, 8)
	if input.DisplayName != nil {
		if !validDisplayName(*input.DisplayName) {
			return "", invalidArgument("insertion_order_update", "display name is invalid")
		}
		fields = append(fields, "displayName")
	}
	if input.EntityStatus != nil {
		if !validUpdateEntityStatus(*input.EntityStatus) {
			return "", invalidArgument("insertion_order_update", "entity status is invalid; draft cannot be restored")
		}
		fields = append(fields, "entityStatus")
	}
	if input.Pacing != nil {
		if !validPacing(*input.Pacing) {
			return "", invalidArgument("insertion_order_update", "pacing is invalid")
		}
		fields = append(fields, "pacing")
	}
	if input.FrequencyCap != nil {
		if !validFrequencyCap(*input.FrequencyCap) {
			return "", invalidArgument("insertion_order_update", "frequency cap is invalid")
		}
		fields = append(fields, "frequencyCap")
	}
	if input.Budget != nil {
		if !validInsertionOrderBudget(*input.Budget) {
			return "", invalidArgument("insertion_order_update", "budget is invalid")
		}
		fields = append(fields, "budget")
	}
	if input.KPI != nil {
		if !validKPI(*input.KPI) {
			return "", invalidArgument("insertion_order_update", "KPI is invalid")
		}
		fields = append(fields, "kpi")
	}
	if input.OptimizationObjective != nil {
		if !validOptimizationObjective(*input.OptimizationObjective) {
			return "", invalidArgument("insertion_order_update", "optimization objective is invalid")
		}
		fields = append(fields, "optimizationObjective")
	}
	if input.BidStrategy != nil {
		if !validBiddingStrategy(*input.BidStrategy, true) {
			return "", invalidArgument("insertion_order_update", "bid strategy is invalid")
		}
		fields = append(fields, "bidStrategy")
	}
	if len(fields) == 0 {
		return "", invalidArgument("insertion_order_update", "at least one field must be updated")
	}
	return joinMask(fields), nil
}

func (client *Client) validateInsertionOrder(operation string, order InsertionOrder) error {
	if !validID(order.InsertionOrderID) || !validID(order.AdvertiserID) || !validID(order.CampaignID) ||
		!validDisplayName(order.DisplayName) || !validReadEntityStatus(order.EntityStatus) {
		return platformContractError(operation, "DV360 returned an invalid insertion order")
	}
	if order.AdvertiserID != client.advertiserID {
		return ownershipError(operation, "insertion order")
	}
	return nil
}

func insertionOrderMatchesPatch(order InsertionOrder, input UpdateInsertionOrderRequest) bool {
	return (input.DisplayName == nil || order.DisplayName == *input.DisplayName) &&
		(input.EntityStatus == nil || order.EntityStatus == *input.EntityStatus)
}

func (client *Client) insertionOrdersPath() string {
	return "/v4/advertisers/" + client.advertiserID + "/insertionOrders"
}
