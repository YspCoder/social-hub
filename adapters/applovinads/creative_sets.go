package applovinads

import (
	"context"
	"encoding/json"
	"net/url"
	"strconv"
	"strings"

	"social-hub/pkg/socialhub"
)

type CreativeSetWorkflow interface {
	ListCreativeSets(context.Context, ListRequest, ...socialhub.CallOption) ([]CreativeSet, error)
	ListCreativeSetsByCampaign(context.Context, []string, int, int, ...socialhub.CallOption) (*CreativeSetsByCampaign, error)
	CreateCreativeSet(context.Context, CreativeSetCreateRequest, ...socialhub.CallOption) (*CreativeSetRef, error)
	UpdateCreativeSet(context.Context, CreativeSetUpdateRequest, ...socialhub.CallOption) (*CreativeSetRef, error)
	CloneCreativeSet(context.Context, CloneCreativeSetRequest, ...socialhub.CallOption) (*CreativeSetRef, error)
	AddCreativeSetsToCampaigns(context.Context, CreativeSetAssociationRequest, ...socialhub.CallOption) error
	RemoveCreativeSetFromCampaigns(context.Context, CreativeSetRemovalRequest, ...socialhub.CallOption) error
	RemoveCreativeSetsFromAllCampaigns(context.Context, []int64, ...socialhub.CallOption) error
}

func (client *Client) ListCreativeSets(ctx context.Context, input ListRequest, options ...socialhub.CallOption) ([]CreativeSet, error) {
	if !validList(input) {
		return nil, invalidArgument("creative_set_list", "page, size, ids, or hashed_ids is invalid")
	}
	if err := client.requireAccess("creative_set_list"); err != nil {
		return nil, err
	}
	page, size := normalizedPage(input.Page, input.Size)
	query := url.Values{"page": {strconv.Itoa(page)}, "size": {strconv.Itoa(size)}}
	if len(input.IDs) > 0 {
		query.Set("ids", strings.Join(input.IDs, ","))
	}
	if len(input.HashedIDs) > 0 {
		query.Set("hashed_ids", strings.Join(input.HashedIDs, ","))
	}
	var raw []json.RawMessage
	if err := client.getJSON(ctx, "creative_set_list", "/creative_set/list", query, &raw, options...); err != nil {
		return nil, err
	}
	result := make([]CreativeSet, len(raw))
	for index := range raw {
		if err := json.Unmarshal(raw[index], &result[index]); err != nil || !validCreativeSetResponse(result[index], client.accountType) {
			return nil, platformContractError("creative_set_list", "Axon returned an invalid Creative Set list")
		}
		result[index].Raw = append(json.RawMessage(nil), raw[index]...)
	}
	return result, nil
}

func (client *Client) ListCreativeSetsByCampaign(ctx context.Context, campaignIDs []string, page, size int, options ...socialhub.CallOption) (*CreativeSetsByCampaign, error) {
	if len(campaignIDs) == 0 || len(campaignIDs) > maximumListSize || !validStringIDs(campaignIDs, true) || page < 0 || size < 0 || size > maximumListSize {
		return nil, invalidArgument("creative_set_list_by_campaign", "campaign ids, page, or size is invalid")
	}
	if err := client.requireAccess("creative_set_list_by_campaign"); err != nil {
		return nil, err
	}
	page, size = normalizedPage(page, size)
	query := url.Values{"ids": {strings.Join(campaignIDs, ",")}, "page": {strconv.Itoa(page)}, "size": {strconv.Itoa(size)}}
	var response CreativeSetsByCampaign
	if err := client.getJSON(ctx, "creative_set_list_by_campaign", "/creative_set/list_by_campaign_id", query, &response, options...); err != nil {
		return nil, err
	}
	if response.CampaignCount < 0 || response.CreativeSetCount < 0 || int64(len(response.Campaigns)) != response.CampaignCount {
		return nil, platformContractError("creative_set_list_by_campaign", "Axon returned invalid association counts")
	}
	var count int64
	for campaignID, sets := range response.Campaigns {
		if !validNumericID(campaignID) {
			return nil, platformContractError("creative_set_list_by_campaign", "Axon returned an invalid Campaign ID")
		}
		for _, set := range sets {
			if !validCreativeSetResponse(set, client.accountType) {
				return nil, platformContractError("creative_set_list_by_campaign", "Axon returned an invalid Creative Set")
			}
			count++
		}
	}
	if count != response.CreativeSetCount {
		return nil, platformContractError("creative_set_list_by_campaign", "Axon returned inconsistent Creative Set counts")
	}
	return &response, nil
}

func (client *Client) CreateCreativeSet(ctx context.Context, input CreativeSetCreateRequest, options ...socialhub.CallOption) (*CreativeSetRef, error) {
	payload, err := creativeSetCreatePayload(input, client.accountType)
	if err != nil {
		return nil, err
	}
	if err := client.requireAccess("creative_set_create"); err != nil {
		return nil, err
	}
	var response CreativeSetRef
	if err := client.postJSON(ctx, "creative_set_create", "/creative_set/create", payload, &response, options...); err != nil {
		return nil, err
	}
	if !validCreativeSetRef(response) {
		return nil, platformContractError("creative_set_create", "Axon returned an invalid Creative Set reference")
	}
	return &response, nil
}

func creativeSetCreatePayload(input CreativeSetCreateRequest, accountType AccountType) (map[string]any, error) {
	switch typed := input.(type) {
	case AppCreativeSetCreateRequest:
		if accountType != AccountTypeApp || !validOptionalNumericID(typed.CampaignID) || !validText(typed.Name, 1024) ||
			!validAssetRefs(typed.Assets, accountType) || !validLanguages(typed.Languages) || !validCountries(typed.Countries, accountType) ||
			typed.ProductPage != "" && !validText(typed.ProductPage, 4096) {
			return nil, invalidArgument("creative_set_create", "APP Creative Set fields are invalid")
		}
		payload := creativeSetBasePayload(accountType, typed.CampaignID, typed.Name, typed.Assets, typed.Languages, typed.Countries)
		if typed.ProductPage != "" {
			payload["product_page"] = typed.ProductPage
		}
		return payload, nil
	case *AppCreativeSetCreateRequest:
		if typed == nil {
			return nil, invalidArgument("creative_set_create", "Creative Set input is required")
		}
		return creativeSetCreatePayload(*typed, accountType)
	case WebCreativeSetCreateRequest:
		if accountType != AccountTypeWeb || !validOptionalNumericID(typed.CampaignID) || !validText(typed.Name, 1024) ||
			!validAssetRefs(typed.Assets, accountType) || !validLanguages(typed.Languages) || !validCountries(typed.Countries, accountType) ||
			typed.CreativeSetURL != "" && !validAbsoluteURL(typed.CreativeSetURL) {
			return nil, invalidArgument("creative_set_create", "WEB Creative Set fields are invalid")
		}
		payload := creativeSetBasePayload(accountType, typed.CampaignID, typed.Name, typed.Assets, typed.Languages, typed.Countries)
		if typed.CreativeSetURL != "" {
			payload["creative_set_url"] = typed.CreativeSetURL
		}
		return payload, nil
	case *WebCreativeSetCreateRequest:
		if typed == nil {
			return nil, invalidArgument("creative_set_create", "Creative Set input is required")
		}
		return creativeSetCreatePayload(*typed, accountType)
	default:
		return nil, invalidArgument("creative_set_create", "unsupported Creative Set input type")
	}
}

func creativeSetBasePayload(accountType AccountType, campaignID, name string, assets []AssetRef, languages, countries []string) map[string]any {
	payload := map[string]any{"type": accountType, "name": name, "assets": assets}
	if campaignID != "" {
		payload["campaign_id"] = campaignID
	}
	if languages != nil {
		payload["languages"] = languages
	}
	if countries != nil {
		payload["countries"] = countries
	}
	return payload
}

func (client *Client) UpdateCreativeSet(ctx context.Context, input CreativeSetUpdateRequest, options ...socialhub.CallOption) (*CreativeSetRef, error) {
	payload, err := creativeSetUpdatePayload(input, client.accountType)
	if err != nil {
		return nil, err
	}
	if err := client.requireAccess("creative_set_update"); err != nil {
		return nil, err
	}
	var response CreativeSetRef
	if err := client.postJSON(ctx, "creative_set_update", "/creative_set/update", payload, &response, options...); err != nil {
		return nil, err
	}
	if expected, _ := payload["id"].(string); !validCreativeSetRef(response) || response.ID != expected {
		return nil, platformContractError("creative_set_update", "Axon returned a Creative Set reference that does not match the request")
	}
	return &response, nil
}

func creativeSetUpdatePayload(input CreativeSetUpdateRequest, accountType AccountType) (map[string]any, error) {
	switch typed := input.(type) {
	case AppCreativeSetUpdateRequest:
		if accountType != AccountTypeApp || !validCreativeSetUpdateCommon(typed.ID, typed.CampaignID, typed.Name, typed.Assets, typed.Languages, typed.Countries, typed.Status, accountType) ||
			typed.ProductPage != nil && *typed.ProductPage != "" && !validText(*typed.ProductPage, 4096) ||
			typed.Name == nil && typed.Assets == nil && typed.Languages == nil && typed.Countries == nil && typed.ProductPage == nil && typed.Status == nil {
			return nil, invalidArgument("creative_set_update", "APP Creative Set patch is invalid")
		}
		payload := creativeSetUpdateBase(accountType, typed.ID, typed.CampaignID, typed.Name, typed.Assets, typed.Languages, typed.Countries, typed.Status)
		if typed.ProductPage != nil {
			payload["product_page"] = *typed.ProductPage
		}
		return payload, nil
	case *AppCreativeSetUpdateRequest:
		if typed == nil {
			return nil, invalidArgument("creative_set_update", "Creative Set patch is required")
		}
		return creativeSetUpdatePayload(*typed, accountType)
	case WebCreativeSetUpdateRequest:
		if accountType != AccountTypeWeb || !validCreativeSetUpdateCommon(typed.ID, typed.CampaignID, typed.Name, typed.Assets, typed.Languages, typed.Countries, typed.Status, accountType) ||
			typed.CreativeSetURL != nil && *typed.CreativeSetURL != "" && !validAbsoluteURL(*typed.CreativeSetURL) ||
			typed.Name == nil && typed.Assets == nil && typed.Languages == nil && typed.Countries == nil && typed.CreativeSetURL == nil && typed.Status == nil {
			return nil, invalidArgument("creative_set_update", "WEB Creative Set patch is invalid")
		}
		payload := creativeSetUpdateBase(accountType, typed.ID, typed.CampaignID, typed.Name, typed.Assets, typed.Languages, typed.Countries, typed.Status)
		if typed.CreativeSetURL != nil {
			payload["creative_set_url"] = *typed.CreativeSetURL
		}
		return payload, nil
	case *WebCreativeSetUpdateRequest:
		if typed == nil {
			return nil, invalidArgument("creative_set_update", "Creative Set patch is required")
		}
		return creativeSetUpdatePayload(*typed, accountType)
	default:
		return nil, invalidArgument("creative_set_update", "unsupported Creative Set patch type")
	}
}

func validCreativeSetUpdateCommon(id, campaignID string, name *string, assets *[]AssetRef, languages, countries *[]string, status *Status, accountType AccountType) bool {
	return validNumericID(id) && validOptionalNumericID(campaignID) && (name == nil || validText(*name, 1024)) &&
		(assets == nil || validAssetRefs(*assets, accountType)) && (languages == nil || validLanguages(*languages)) &&
		(countries == nil || validCountries(*countries, accountType)) && (status == nil || validStatus(*status))
}

func creativeSetUpdateBase(accountType AccountType, id, campaignID string, name *string, assets *[]AssetRef, languages, countries *[]string, status *Status) map[string]any {
	payload := map[string]any{"type": accountType, "id": id}
	if campaignID != "" {
		payload["campaign_id"] = campaignID
	}
	if name != nil {
		payload["name"] = *name
	}
	if assets != nil {
		payload["assets"] = *assets
	}
	if languages != nil {
		payload["languages"] = *languages
	}
	if countries != nil {
		payload["countries"] = *countries
	}
	if status != nil {
		payload["status"] = *status
	}
	return payload
}

func (client *Client) CloneCreativeSet(ctx context.Context, input CloneCreativeSetRequest, options ...socialhub.CallOption) (*CreativeSetRef, error) {
	if !validNumericID(input.CampaignID) || !validNumericID(input.CreativeSetID) {
		return nil, invalidArgument("creative_set_clone", "Campaign ID and Creative Set ID must be positive numeric IDs")
	}
	if err := client.requireAccess("creative_set_clone"); err != nil {
		return nil, err
	}
	payload := map[string]any{"campaign_id": input.CampaignID, "creative_set_id": input.CreativeSetID, "status": StatusPaused}
	var response CreativeSetRef
	if err := client.postJSON(ctx, "creative_set_clone", "/creative_set/clone", payload, &response, options...); err != nil {
		return nil, err
	}
	if !validCreativeSetRef(response) {
		return nil, platformContractError("creative_set_clone", "Axon returned an invalid cloned Creative Set reference")
	}
	return &response, nil
}

func (client *Client) AddCreativeSetsToCampaigns(ctx context.Context, input CreativeSetAssociationRequest, options ...socialhub.CallOption) error {
	if !validPositiveIDs(input.CampaignIDs, 20) || !validPositiveIDs(input.CreativeSetIDs, 50) {
		return invalidArgument("creative_set_add_to_campaigns", "Campaign IDs or Creative Set IDs are invalid")
	}
	if err := client.requireAccess("creative_set_add_to_campaigns"); err != nil {
		return err
	}
	return client.postJSON(ctx, "creative_set_add_to_campaigns", "/creative_set/add-to-campaigns", input, nil, options...)
}

func (client *Client) RemoveCreativeSetFromCampaigns(ctx context.Context, input CreativeSetRemovalRequest, options ...socialhub.CallOption) error {
	if !validPositiveIDs(input.CampaignIDs, 20) || input.CreativeSetID <= 0 {
		return invalidArgument("creative_set_remove_from_campaigns", "Campaign IDs or Creative Set ID are invalid")
	}
	if err := client.requireAccess("creative_set_remove_from_campaigns"); err != nil {
		return err
	}
	return client.postJSON(ctx, "creative_set_remove_from_campaigns", "/creative_set/remove-from-campaigns", input, nil, options...)
}

func (client *Client) RemoveCreativeSetsFromAllCampaigns(ctx context.Context, creativeSetIDs []int64, options ...socialhub.CallOption) error {
	if !validPositiveIDs(creativeSetIDs, 50) {
		return invalidArgument("creative_set_remove_from_all_campaigns", "Creative Set IDs are invalid")
	}
	if err := client.requireAccess("creative_set_remove_from_all_campaigns"); err != nil {
		return err
	}
	return client.postJSON(ctx, "creative_set_remove_from_all_campaigns", "/creative_set/remove-from-all-campaigns", map[string]any{"creative_set_ids": creativeSetIDs}, nil, options...)
}

func validOptionalNumericID(value string) bool { return value == "" || validNumericID(value) }

func validCreativeSetRef(value CreativeSetRef) bool {
	return validNumericID(value.ID) && (value.Version == "" || value.Version == "V1" || value.Version == "V2")
}

func validCreativeSetResponse(value CreativeSet, accountType AccountType) bool {
	return validNumericID(value.ID) && value.Type == accountType && validText(value.Name, 4096) && value.Assets != nil
}
