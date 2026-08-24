package oceanengine

import (
	"context"
	"net/http"
	"net/url"
	"strconv"

	"social-hub/pkg/socialhub"
)

var promotionListFields = []string{
	"promotion_id", "advertiser_id", "project_id", "promotion_name", "opt_status", "status",
	"status_first", "status_second", "budget", "budget_mode", "bid", "cpa_bid",
	"promotion_create_time", "promotion_modify_time",
}

func (client *Client) CreatePromotion(ctx context.Context, input CreatePromotionRequest, options ...socialhub.CallOption) (*Promotion, error) {
	if !validID(input.ProjectID) || !validRequiredText(input.Name, 512) {
		return nil, invalidArgument("promotion_create", "project_id and name are required")
	}
	body, err := mergeFields("promotion_create", map[string]any{
		"advertiser_id": client.advertiserID, "project_id": input.ProjectID,
		"name": input.Name, "operation": OperationDisable,
	}, input.Fields)
	if err != nil {
		return nil, err
	}
	type responseData struct {
		PromotionID *int64 `json:"promotion_id"`
	}
	var response apiEnvelope[responseData]
	if err := client.api.JSON(ctx, http.MethodPost, "/open_api/v3.0/promotion/create/", nil, body, &response, options...); err != nil {
		return nil, err
	}
	data, err := requireEnvelope("promotion_create", response)
	if err != nil {
		return nil, err
	}
	if data.PromotionID == nil || !validID(*data.PromotionID) {
		return nil, platformContractError("promotion_create", "Ocean Engine returned an invalid promotion_id")
	}
	return &Promotion{
		ID: *data.PromotionID, AdvertiserID: client.advertiserID,
		ProjectID: input.ProjectID, Name: input.Name, OptStatus: string(OperationDisable),
	}, nil
}

func (client *Client) ListPromotions(ctx context.Context, input ListPromotionsRequest, options ...socialhub.CallOption) (NumberPage[Promotion], error) {
	page, pageSize, err := validatePage(input.Page, input.PageSize, 100)
	if err != nil {
		return NumberPage[Promotion]{}, err
	}
	if !validateFields(input.Fields) || !validateIDs(input.Filter.IDs) ||
		input.Filter.ProjectID < 0 || input.Filter.Name != "" && !validRequiredText(input.Filter.Name, 512) ||
		input.Filter.Status != "" && !validEnum(input.Filter.Status) {
		return NumberPage[Promotion]{}, invalidArgument("promotion_list", "fields or filtering values are invalid")
	}
	fields := appendRequiredFields(input.Fields, promotionListFields)
	query := url.Values{
		"advertiser_id": {strconv.FormatInt(client.advertiserID, 10)},
		"page":          {strconv.Itoa(page)}, "page_size": {strconv.Itoa(pageSize)},
	}
	if err := setJSONQuery(query, "fields", fields, "promotion_list"); err != nil {
		return NumberPage[Promotion]{}, err
	}
	filtering := map[string]any{}
	if len(input.Filter.IDs) > 0 {
		filtering["ids"] = input.Filter.IDs
	}
	if input.Filter.ProjectID > 0 {
		filtering["project_id"] = input.Filter.ProjectID
	}
	if input.Filter.Name != "" {
		filtering["name"] = input.Filter.Name
	}
	if input.Filter.Status != "" {
		filtering["status"] = input.Filter.Status
	}
	if len(filtering) > 0 {
		if err := setJSONQuery(query, "filtering", filtering, "promotion_list"); err != nil {
			return NumberPage[Promotion]{}, err
		}
	}
	type responseData struct {
		List     []Promotion `json:"list"`
		PageInfo *pageInfo   `json:"page_info"`
	}
	var response apiEnvelope[responseData]
	if err := client.api.JSON(ctx, http.MethodGet, "/open_api/v3.0/promotion/list/", query, nil, &response, options...); err != nil {
		return NumberPage[Promotion]{}, err
	}
	data, err := requireEnvelope("promotion_list", response)
	if err != nil {
		return NumberPage[Promotion]{}, err
	}
	if err := validatePageInfo("promotion_list", data.PageInfo); err != nil {
		return NumberPage[Promotion]{}, err
	}
	for _, promotion := range data.List {
		if !validID(promotion.ID) || promotion.AdvertiserID != client.advertiserID || !validID(promotion.ProjectID) {
			return NumberPage[Promotion]{}, platformContractError("promotion_list", "Ocean Engine returned an invalid or cross-account promotion")
		}
	}
	return numberPage(data.List, data.PageInfo), nil
}

func (client *Client) UpdatePromotion(ctx context.Context, promotionID int64, input UpdatePromotionRequest, options ...socialhub.CallOption) error {
	if !validID(promotionID) || !validRequiredText(input.Name, 512) {
		return invalidArgument("promotion_update", "promotion_id and name are required")
	}
	body, err := mergeFields("promotion_update", map[string]any{
		"advertiser_id": client.advertiserID, "promotion_id": promotionID, "name": input.Name,
	}, input.Fields)
	if err != nil {
		return err
	}
	type responseData struct {
		PromotionID *int64                  `json:"promotion_id"`
		ErrorList   []providerMutationError `json:"error_list"`
	}
	var response apiEnvelope[responseData]
	if err := client.api.JSON(ctx, http.MethodPost, "/open_api/v3.0/promotion/update/", nil, body, &response, options...); err != nil {
		return err
	}
	data, err := requireEnvelope("promotion_update", response)
	if err != nil {
		return err
	}
	if len(data.ErrorList) > 0 {
		return mutationError("promotion_update", data.ErrorList[0], response.RequestID)
	}
	if data.PromotionID == nil || *data.PromotionID != promotionID {
		return platformContractError("promotion_update", "Ocean Engine did not confirm the requested promotion_id")
	}
	return nil
}

func (client *Client) SetPromotionStatus(ctx context.Context, promotionID int64, operation Operation, options ...socialhub.CallOption) error {
	if !validID(promotionID) || !validOperation(operation) {
		return invalidArgument("promotion_status_update", "a promotion_id and ENABLE or DISABLE operation are required")
	}
	body := map[string]any{
		"advertiser_id": client.advertiserID,
		"data":          []map[string]any{{"promotion_id": promotionID, "opt_status": operation}},
	}
	type responseData struct {
		PromotionIDs []int64 `json:"promotion_ids"`
		Errors       []struct {
			PromotionID  int64  `json:"promotion_id"`
			ErrorMessage string `json:"error_message"`
		} `json:"errors"`
	}
	var response apiEnvelope[responseData]
	if err := client.api.JSON(ctx, http.MethodPost, "/open_api/v3.0/promotion/status/update/", nil, body, &response, options...); err != nil {
		return err
	}
	data, err := requireEnvelope("promotion_status_update", response)
	if err != nil {
		return err
	}
	if len(data.Errors) > 0 {
		return mutationError("promotion_status_update", providerMutationError{ErrorMessage: data.Errors[0].ErrorMessage}, response.RequestID)
	}
	if !containsID(data.PromotionIDs, promotionID) {
		return platformContractError("promotion_status_update", "Ocean Engine did not confirm the requested promotion_id")
	}
	return nil
}
