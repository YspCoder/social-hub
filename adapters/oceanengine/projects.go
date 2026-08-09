package oceanengine

import (
	"context"
	"net/http"
	"net/url"
	"strconv"

	"social-hub/pkg/socialhub"
)

var projectListFields = []string{
	"project_id", "advertiser_id", "name", "ad_type", "landing_type", "marketing_goal",
	"opt_status", "status", "status_first", "status_second", "project_create_time", "project_modify_time",
}

func (client *Client) CreateProject(ctx context.Context, input CreateProjectRequest, options ...socialhub.CallOption) (*Project, error) {
	if !validRequiredText(input.Name, 512) || (input.AdType != AdTypeAll && input.AdType != AdTypeSearch) ||
		!validEnum(string(input.LandingType)) || !validEnum(string(input.MarketingGoal)) {
		return nil, invalidArgument("project_create", "name, ad_type, landing_type, and marketing_goal are required")
	}
	if input.DeliveryRange.InventoryCatalog != InventoryCatalogManual && input.DeliveryRange.InventoryCatalog != InventoryCatalogUniversalSmart {
		return nil, invalidArgument("project_create", "delivery_range.inventory_catalog is invalid")
	}
	if !validEnums(input.DeliveryRange.InventoryType) ||
		input.DeliveryRange.UnionVideoType != "" && !validEnum(input.DeliveryRange.UnionVideoType) {
		return nil, invalidArgument("project_create", "delivery_range contains an invalid inventory enum")
	}
	if !validEnum(string(input.DeliverySetting.BidType)) || !validEnum(string(input.DeliverySetting.BudgetMode)) ||
		input.DeliverySetting.DeepBidType != "" && !validEnum(input.DeliverySetting.DeepBidType) ||
		input.DeliverySetting.ScheduleType != "" && !validEnum(input.DeliverySetting.ScheduleType) ||
		!validNonNegative(input.DeliverySetting.Bid, input.DeliverySetting.Budget, input.DeliverySetting.CPABid, input.DeliverySetting.DeepCPABid, input.DeliverySetting.ROIGoal) {
		return nil, invalidArgument("project_create", "delivery_setting contains an invalid bid, budget, or enum")
	}
	body, err := mergeFields("project_create", map[string]any{
		"advertiser_id":    client.advertiserID,
		"name":             input.Name,
		"ad_type":          input.AdType,
		"landing_type":     input.LandingType,
		"marketing_goal":   input.MarketingGoal,
		"delivery_range":   input.DeliveryRange,
		"delivery_setting": input.DeliverySetting,
		"operation":        OperationDisable,
	}, input.Fields)
	if err != nil {
		return nil, err
	}
	type responseData struct {
		ProjectID *int64 `json:"project_id"`
	}
	var response apiEnvelope[responseData]
	if err := client.api.JSON(ctx, http.MethodPost, "/open_api/v3.0/project/create/", nil, body, &response, options...); err != nil {
		return nil, err
	}
	data, err := requireEnvelope("project_create", response)
	if err != nil {
		return nil, err
	}
	if data.ProjectID == nil || !validID(*data.ProjectID) {
		return nil, platformContractError("project_create", "Ocean Engine returned an invalid project_id")
	}
	return &Project{
		ID: *data.ProjectID, AdvertiserID: client.advertiserID, Name: input.Name,
		AdType: input.AdType, LandingType: input.LandingType, MarketingGoal: input.MarketingGoal,
		OptStatus: string(OperationDisable),
	}, nil
}

func (client *Client) ListProjects(ctx context.Context, input ListProjectsRequest, options ...socialhub.CallOption) (NumberPage[Project], error) {
	page, pageSize, err := validatePage(input.Page, input.PageSize, 100)
	if err != nil {
		return NumberPage[Project]{}, err
	}
	if !validateFields(input.Fields) || !validateIDs(input.Filter.IDs) ||
		input.Filter.Name != "" && !validRequiredText(input.Filter.Name, 512) ||
		input.Filter.Status != "" && !validEnum(input.Filter.Status) {
		return NumberPage[Project]{}, invalidArgument("project_list", "fields or filtering values are invalid")
	}
	fields := appendRequiredFields(input.Fields, projectListFields)
	query := url.Values{
		"advertiser_id": {strconv.FormatInt(client.advertiserID, 10)},
		"page":          {strconv.Itoa(page)}, "page_size": {strconv.Itoa(pageSize)},
	}
	if err := setJSONQuery(query, "fields", fields, "project_list"); err != nil {
		return NumberPage[Project]{}, err
	}
	filtering := map[string]any{}
	if len(input.Filter.IDs) > 0 {
		filtering["ids"] = input.Filter.IDs
	}
	if input.Filter.Name != "" {
		filtering["name"] = input.Filter.Name
	}
	if input.Filter.Status != "" {
		filtering["status"] = input.Filter.Status
	}
	if len(filtering) > 0 {
		if err := setJSONQuery(query, "filtering", filtering, "project_list"); err != nil {
			return NumberPage[Project]{}, err
		}
	}
	type responseData struct {
		List     []Project `json:"list"`
		PageInfo *pageInfo `json:"page_info"`
	}
	var response apiEnvelope[responseData]
	if err := client.api.JSON(ctx, http.MethodGet, "/open_api/v3.0/project/list/", query, nil, &response, options...); err != nil {
		return NumberPage[Project]{}, err
	}
	data, err := requireEnvelope("project_list", response)
	if err != nil {
		return NumberPage[Project]{}, err
	}
	if err := validatePageInfo("project_list", data.PageInfo); err != nil {
		return NumberPage[Project]{}, err
	}
	for _, project := range data.List {
		if !validID(project.ID) || project.AdvertiserID != client.advertiserID {
			return NumberPage[Project]{}, platformContractError("project_list", "Ocean Engine returned an invalid or cross-account project")
		}
	}
	return numberPage(data.List, data.PageInfo), nil
}

func (client *Client) UpdateProject(ctx context.Context, projectID int64, input UpdateProjectRequest, options ...socialhub.CallOption) error {
	if !validID(projectID) || input.Name == nil && len(input.Fields) == 0 {
		return invalidArgument("project_update", "a project_id and at least one patch field are required")
	}
	fixed := map[string]any{"advertiser_id": client.advertiserID, "project_id": projectID}
	if input.Name != nil {
		if !validRequiredText(*input.Name, 512) {
			return invalidArgument("project_update", "name is invalid")
		}
		fixed["name"] = *input.Name
	}
	body, err := mergeFields("project_update", fixed, input.Fields)
	if err != nil {
		return err
	}
	type responseData struct {
		ProjectID *int64                  `json:"project_id"`
		ErrorList []providerMutationError `json:"error_list"`
	}
	var response apiEnvelope[responseData]
	if err := client.api.JSON(ctx, http.MethodPost, "/open_api/v3.0/project/update/", nil, body, &response, options...); err != nil {
		return err
	}
	data, err := requireEnvelope("project_update", response)
	if err != nil {
		return err
	}
	if len(data.ErrorList) > 0 {
		return mutationError("project_update", data.ErrorList[0], response.RequestID)
	}
	if data.ProjectID == nil || *data.ProjectID != projectID {
		return platformContractError("project_update", "Ocean Engine did not confirm the requested project_id")
	}
	return nil
}

func (client *Client) SetProjectStatus(ctx context.Context, projectID int64, operation Operation, options ...socialhub.CallOption) error {
	if !validID(projectID) || !validOperation(operation) {
		return invalidArgument("project_status_update", "a project_id and ENABLE or DISABLE operation are required")
	}
	body := map[string]any{
		"advertiser_id": client.advertiserID,
		"data":          []map[string]any{{"project_id": projectID, "opt_status": operation}},
	}
	type responseData struct {
		ProjectIDs []int64 `json:"project_ids"`
		Errors     []struct {
			ProjectID    int64  `json:"project_id"`
			ErrorMessage string `json:"error_message"`
		} `json:"errors"`
	}
	var response apiEnvelope[responseData]
	if err := client.api.JSON(ctx, http.MethodPost, "/open_api/v3.0/project/status/update/", nil, body, &response, options...); err != nil {
		return err
	}
	data, err := requireEnvelope("project_status_update", response)
	if err != nil {
		return err
	}
	if len(data.Errors) > 0 {
		return mutationError("project_status_update", providerMutationError{ErrorMessage: data.Errors[0].ErrorMessage}, response.RequestID)
	}
	return nil
}

func validNonNegative(values ...*float64) bool {
	for _, value := range values {
		if value != nil && *value < 0 {
			return false
		}
	}
	return true
}

func validEnums(values []string) bool {
	for _, value := range values {
		if !validEnum(value) {
			return false
		}
	}
	return true
}
