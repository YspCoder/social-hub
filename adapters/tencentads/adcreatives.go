package tencentads

import (
	"context"
	"net/http"
	"net/url"
	"strconv"

	"social-hub/pkg/socialhub"
)

var adCreativeFields = []string{
	"adcreative_id", "campaign_id", "adcreative_name", "promoted_object_type", "promoted_object_id",
	"adcreative_template_id", "page_type", "deep_link_url", "created_time", "last_modified_time", "is_deleted",
}

func (client *Client) ListAdCreatives(ctx context.Context, input ListAdCreativesRequest, options ...socialhub.CallOption) (NumberPage[AdCreative], error) {
	const operation = "adcreative_list"
	page, pageSize, err := validateList(input.Fields, input.Filtering, input.Page, input.PageSize)
	if err != nil {
		return NumberPage[AdCreative]{}, err
	}
	fields := input.Fields
	if len(fields) == 0 {
		fields = adCreativeFields
	}
	fields = appendRequiredFields(fields, "adcreative_id", "campaign_id")
	query := url.Values{
		"account_id": {strconv.FormatInt(client.advertiserID, 10)}, "page": {strconv.Itoa(page)},
		"page_size": {strconv.Itoa(pageSize)}, "is_deleted": {strconv.FormatBool(input.IncludeDeleted)},
	}
	if err := setJSONQuery(query, "fields", fields, operation); err != nil {
		return NumberPage[AdCreative]{}, err
	}
	if len(input.Filtering) > 0 {
		if err := setJSONQuery(query, "filtering", input.Filtering, operation); err != nil {
			return NumberPage[AdCreative]{}, err
		}
	}
	var response apiEnvelope[struct {
		List     []AdCreative `json:"list"`
		PageInfo *pageInfo    `json:"page_info"`
	}]
	header, err := client.requestJSON(ctx, operation, http.MethodGet, "/adcreatives/get", query, nil, &response, options...)
	if err != nil {
		return NumberPage[AdCreative]{}, err
	}
	data, err := requireEnvelope(operation, response, header)
	if err != nil {
		return NumberPage[AdCreative]{}, err
	}
	if err := validatePageInfo(operation, data.PageInfo); err != nil {
		return NumberPage[AdCreative]{}, err
	}
	for index := range data.List {
		if err := requireResourceID(operation, 0, data.List[index].ID); err != nil || !validID(data.List[index].CampaignID) {
			return NumberPage[AdCreative]{}, platformContractError(operation, "Tencent Ads returned an invalid ad creative or campaign ID")
		}
		if err := requireAccount(operation, client.advertiserID, data.List[index].AccountID); err != nil {
			return NumberPage[AdCreative]{}, err
		}
		data.List[index].AccountID = client.advertiserID
	}
	return numberPage(data.List, data.PageInfo), nil
}

func (client *Client) CreateAdCreative(ctx context.Context, input CreateAdCreativeRequest, options ...socialhub.CallOption) (*AdCreative, error) {
	const operation = "adcreative_create"
	if !validID(input.CampaignID) || !validRequiredText(input.Name, 512) || !validEnum(string(input.PromotedObjectType)) || !validID(input.TemplateID) {
		return nil, invalidArgument(operation, "campaign, name, promoted object type, and template ID are required")
	}
	fixed := map[string]any{
		"account_id": client.advertiserID, "campaign_id": input.CampaignID, "adcreative_name": input.Name,
		"promoted_object_type": input.PromotedObjectType, "adcreative_template_id": input.TemplateID,
	}
	body, err := mergeFields(operation, fixed, input.Fields, "adcreative_id", "adcreative_name", "campaign_id", "adcreative_template_id")
	if err != nil {
		return nil, err
	}
	var response apiEnvelope[struct {
		AdCreativeID int64 `json:"adcreative_id"`
	}]
	header, err := client.requestJSON(ctx, operation, http.MethodPost, "/adcreatives/add", nil, body, &response, options...)
	if err != nil {
		return nil, err
	}
	data, err := requireEnvelope(operation, response, header)
	if err != nil {
		return nil, err
	}
	if err := requireResourceID(operation, 0, data.AdCreativeID); err != nil {
		return nil, err
	}
	return &AdCreative{
		ID: data.AdCreativeID, AccountID: client.advertiserID, CampaignID: input.CampaignID,
		Name: input.Name, PromotedObjectType: input.PromotedObjectType, TemplateID: input.TemplateID,
	}, nil
}

func (client *Client) UpdateAdCreative(ctx context.Context, adCreativeID int64, input UpdateAdCreativeRequest, options ...socialhub.CallOption) error {
	const operation = "adcreative_update"
	if !validID(adCreativeID) || input.Name == nil && len(input.Fields) == 0 {
		return invalidArgument(operation, "an ad creative ID and at least one patch field are required")
	}
	if input.Name != nil && !validRequiredText(*input.Name, 512) {
		return invalidArgument(operation, "ad creative name is invalid")
	}
	fixed := map[string]any{"account_id": client.advertiserID, "adcreative_id": adCreativeID}
	if input.Name != nil {
		fixed["adcreative_name"] = *input.Name
	}
	body, err := mergeFields(operation, fixed, input.Fields, "adcreative_id", "adcreative_name", "campaign_id")
	if err != nil {
		return err
	}
	var response apiEnvelope[struct {
		AdCreativeID int64 `json:"adcreative_id"`
	}]
	header, err := client.requestJSON(ctx, operation, http.MethodPost, "/adcreatives/update", nil, body, &response, options...)
	if err != nil {
		return err
	}
	data, err := requireEnvelope(operation, response, header)
	if err != nil {
		return err
	}
	return requireResourceID(operation, adCreativeID, data.AdCreativeID)
}
