package applovinads

import (
	"context"
	"encoding/json"
	"net/url"
	"strconv"
	"strings"

	"social-hub/pkg/socialhub"
)

type CampaignWorkflow interface {
	ListCampaigns(context.Context, ListRequest, ...socialhub.CallOption) ([]Campaign, error)
	CreateCampaign(context.Context, CampaignCreateRequest, ...socialhub.CallOption) (*CampaignRef, error)
	UpdateCampaign(context.Context, CampaignUpdateRequest, ...socialhub.CallOption) (*CampaignRef, error)
	GetCatalogInfo(context.Context, ...socialhub.CallOption) (*CatalogInfo, error)
}

func (client *Client) ListCampaigns(ctx context.Context, input ListRequest, options ...socialhub.CallOption) ([]Campaign, error) {
	if !validList(input) {
		return nil, invalidArgument("campaign_list", "page, size, ids, or hashed_ids is invalid")
	}
	if err := client.requireAccess("campaign_list"); err != nil {
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
	if err := client.getJSON(ctx, "campaign_list", "/campaign/list", query, &raw, options...); err != nil {
		return nil, err
	}
	result := make([]Campaign, len(raw))
	for index := range raw {
		if err := json.Unmarshal(raw[index], &result[index]); err != nil || !validCampaignResponse(result[index], client.accountType) {
			return nil, platformContractError("campaign_list", "Axon returned an invalid Campaign list")
		}
		result[index].Raw = append(json.RawMessage(nil), raw[index]...)
	}
	return result, nil
}

func (client *Client) CreateCampaign(ctx context.Context, input CampaignCreateRequest, options ...socialhub.CallOption) (*CampaignRef, error) {
	payload, err := campaignCreatePayload(input, client.accountType)
	if err != nil {
		return nil, err
	}
	if err := client.requireAccess("campaign_create"); err != nil {
		return nil, err
	}
	var response CampaignRef
	if err := client.postJSON(ctx, "campaign_create", "/campaign/create", payload, &response, options...); err != nil {
		return nil, err
	}
	if !validNumericID(response.ID) {
		return nil, platformContractError("campaign_create", "Axon returned an invalid Campaign ID")
	}
	return &response, nil
}

func campaignCreatePayload(input CampaignCreateRequest, accountType AccountType) (map[string]any, error) {
	switch typed := input.(type) {
	case AppCampaignCreateRequest:
		if accountType != AccountTypeApp {
			return nil, invalidArgument("campaign_create", "APP Campaign input requires an APP account")
		}
		if !validText(typed.Name, 1024) || !validSchedule(typed.StartDate, typed.EndDate, typed.IsContinuousDelivery) ||
			(typed.BiddingStrategy != BiddingTargetGoalCPI && typed.BiddingStrategy != BiddingAutoCPM) ||
			(typed.Platform != PlatformIOS && typed.Platform != PlatformAndroid) || !validText(typed.PackageName, 512) ||
			typed.Platform == PlatformIOS && !validNumericID(typed.ITunesID) || typed.Platform == PlatformAndroid && typed.ITunesID != "" ||
			!validBudget(typed.Budget) || !validAppGoal(typed.Goal, typed.BiddingStrategy) || !validTargeting(typed.Targeting) || !validTracking(typed.Tracking) {
			return nil, invalidArgument("campaign_create", "APP Campaign fields or cross-field constraints are invalid")
		}
		payload := map[string]any{
			"type": AccountTypeApp, "name": typed.Name, "start_date": typed.StartDate,
			"bidding_strategy": typed.BiddingStrategy, "platform": typed.Platform, "package_name": typed.PackageName,
			"budget": typed.Budget, "goal": typed.Goal, "targeting": typed.Targeting, "tracking": typed.Tracking,
		}
		setOptionalCampaignFields(payload, typed.EndDate, typed.IsContinuousDelivery)
		if typed.ITunesID != "" {
			payload["itunes_id"] = typed.ITunesID
		}
		if typed.IsCompositeBannerEnabled != nil {
			payload["is_composite_banner_enabled"] = *typed.IsCompositeBannerEnabled
		}
		return payload, nil
	case *AppCampaignCreateRequest:
		if typed == nil {
			return nil, invalidArgument("campaign_create", "Campaign input is required")
		}
		return campaignCreatePayload(*typed, accountType)
	case WebCampaignCreateRequest:
		if accountType != AccountTypeWeb {
			return nil, invalidArgument("campaign_create", "WEB Campaign input requires a WEB account")
		}
		if !validText(typed.Name, 1024) || !validSchedule(typed.StartDate, typed.EndDate, typed.IsContinuousDelivery) ||
			typed.BiddingStrategy != BiddingAutoCPM || !validAbsoluteURL(typed.WebsiteURL) || !validBudget(typed.Budget) ||
			!validWebGoal(typed.Goal) || !validTargeting(typed.Targeting) || !validWebCampaignOptions(typed) {
			return nil, invalidArgument("campaign_create", "WEB Campaign fields or cross-field constraints are invalid")
		}
		payload := map[string]any{
			"type": AccountTypeWeb, "name": typed.Name, "start_date": typed.StartDate,
			"bidding_strategy": typed.BiddingStrategy, "website_url": typed.WebsiteURL,
			"budget": typed.Budget, "goal": typed.Goal, "targeting": typed.Targeting,
		}
		setOptionalCampaignFields(payload, typed.EndDate, typed.IsContinuousDelivery)
		if typed.IsDynamicAdsEnabled != nil {
			payload["is_dynamic_ads_enabled"] = *typed.IsDynamicAdsEnabled
		}
		if typed.CatalogType != "" {
			payload["catalog_type"] = typed.CatalogType
		}
		if typed.VariantSetID != "" {
			payload["variant_set_id"] = typed.VariantSetID
		}
		if typed.CatalogID != "" {
			payload["catalog_id"] = typed.CatalogID
		}
		if typed.AudienceStrategy != "" {
			payload["audience_strategy"] = typed.AudienceStrategy
		}
		return payload, nil
	case *WebCampaignCreateRequest:
		if typed == nil {
			return nil, invalidArgument("campaign_create", "Campaign input is required")
		}
		return campaignCreatePayload(*typed, accountType)
	default:
		return nil, invalidArgument("campaign_create", "unsupported Campaign input type")
	}
}

func setOptionalCampaignFields(payload map[string]any, endDate string, continuous *bool) {
	if endDate != "" {
		payload["end_date"] = endDate
	}
	if continuous != nil {
		payload["is_continuous_delivery"] = *continuous
	}
}

func validWebCampaignOptions(input WebCampaignCreateRequest) bool {
	if input.CatalogType != "" && input.CatalogType != CatalogDPA && input.CatalogType != CatalogDOA {
		return false
	}
	if input.CatalogType == CatalogDOA && input.CatalogID != "" && !validNumericID(input.CatalogID) {
		return false
	}
	if input.IsDynamicAdsEnabled != nil && *input.IsDynamicAdsEnabled && (input.CatalogID == "" || input.VariantSetID == "") {
		return false
	}
	if input.AudienceStrategy != "" && input.AudienceStrategy != AudienceUniversal && input.AudienceStrategy != AudienceProspecting && input.AudienceStrategy != AudienceDiscovery {
		return false
	}
	return input.AudienceStrategy == "" || input.Goal.GoalType == GoalCPP || input.Goal.GoalType == GoalIAPROAS
}

func (client *Client) UpdateCampaign(ctx context.Context, input CampaignUpdateRequest, options ...socialhub.CallOption) (*CampaignRef, error) {
	payload, err := campaignUpdatePayload(input, client.accountType)
	if err != nil {
		return nil, err
	}
	if err := client.requireAccess("campaign_update"); err != nil {
		return nil, err
	}
	var response CampaignRef
	if err := client.postJSON(ctx, "campaign_update", "/campaign/update", payload, &response, options...); err != nil {
		return nil, err
	}
	expected, _ := payload["id"].(string)
	if response.ID != expected {
		return nil, platformContractError("campaign_update", "Axon returned a Campaign ID that does not match the request")
	}
	return &response, nil
}

func campaignUpdatePayload(input CampaignUpdateRequest, accountType AccountType) (map[string]any, error) {
	switch typed := input.(type) {
	case AppCampaignUpdateRequest:
		if accountType != AccountTypeApp || !validNumericID(typed.ID) || !validUpdateSchedule(typed.EndDate, typed.IsContinuousDelivery) || !validAppCampaignUpdate(typed) {
			return nil, invalidArgument("campaign_update", "APP Campaign patch is invalid")
		}
		payload := map[string]any{"type": AccountTypeApp, "id": typed.ID}
		applyCampaignPatch(payload, typed.Name, typed.Status, typed.EndDate, typed.IsContinuousDelivery, typed.Budget, typed.Goal, typed.Targeting)
		if typed.Tracking != nil {
			payload["tracking"] = *typed.Tracking
		}
		if typed.IsCompositeBannerEnabled != nil {
			payload["is_composite_banner_enabled"] = *typed.IsCompositeBannerEnabled
		}
		return payload, nil
	case *AppCampaignUpdateRequest:
		if typed == nil {
			return nil, invalidArgument("campaign_update", "Campaign patch is required")
		}
		return campaignUpdatePayload(*typed, accountType)
	case WebCampaignUpdateRequest:
		if accountType != AccountTypeWeb || !validNumericID(typed.ID) || !validUpdateSchedule(typed.EndDate, typed.IsContinuousDelivery) || !validWebCampaignUpdate(typed) {
			return nil, invalidArgument("campaign_update", "WEB Campaign patch is invalid")
		}
		payload := map[string]any{"type": AccountTypeWeb, "id": typed.ID}
		applyCampaignPatch(payload, typed.Name, typed.Status, typed.EndDate, typed.IsContinuousDelivery, typed.Budget, typed.Goal, typed.Targeting)
		if typed.WebsiteURL != nil {
			payload["website_url"] = *typed.WebsiteURL
		}
		if typed.IsDynamicAdsEnabled != nil {
			payload["is_dynamic_ads_enabled"] = *typed.IsDynamicAdsEnabled
		}
		if typed.CatalogType != nil {
			payload["catalog_type"] = *typed.CatalogType
		}
		if typed.VariantSetID != nil {
			payload["variant_set_id"] = *typed.VariantSetID
		}
		if typed.CatalogID != nil {
			payload["catalog_id"] = *typed.CatalogID
		}
		if typed.AudienceStrategy != nil {
			payload["audience_strategy"] = *typed.AudienceStrategy
		}
		return payload, nil
	case *WebCampaignUpdateRequest:
		if typed == nil {
			return nil, invalidArgument("campaign_update", "Campaign patch is required")
		}
		return campaignUpdatePayload(*typed, accountType)
	default:
		return nil, invalidArgument("campaign_update", "unsupported Campaign patch type")
	}
}

func applyCampaignPatch(payload map[string]any, name *string, status *Status, endDate *string, continuous *bool, budget *Budget, goal *GoalUpdate, targeting *[]Targeting) {
	if name != nil {
		payload["name"] = *name
	}
	if status != nil {
		payload["status"] = *status
	}
	if endDate != nil {
		payload["end_date"] = *endDate
	}
	if continuous != nil {
		payload["is_continuous_delivery"] = *continuous
	}
	if budget != nil {
		payload["budget"] = *budget
	}
	if goal != nil {
		payload["goal"] = *goal
	}
	if targeting != nil {
		payload["targeting"] = *targeting
	}
}

func validAppCampaignUpdate(input AppCampaignUpdateRequest) bool {
	if input.Name == nil && input.Status == nil && input.EndDate == nil && input.IsContinuousDelivery == nil && input.Budget == nil && input.Goal == nil &&
		input.Targeting == nil && input.Tracking == nil && input.IsCompositeBannerEnabled == nil {
		return false
	}
	if input.Name != nil && !validText(*input.Name, 1024) || input.Status != nil && !validStatus(*input.Status) || input.Budget != nil && !validBudget(*input.Budget) ||
		input.Goal != nil && !validGoalUpdate(*input.Goal, AccountTypeApp) || input.Targeting != nil && !validTargeting(*input.Targeting) {
		return false
	}
	return input.Tracking == nil || validTrackingUpdate(*input.Tracking)
}

func validWebCampaignUpdate(input WebCampaignUpdateRequest) bool {
	if input.Name == nil && input.Status == nil && input.EndDate == nil && input.IsContinuousDelivery == nil && input.Budget == nil && input.Goal == nil &&
		input.Targeting == nil && input.WebsiteURL == nil && input.IsDynamicAdsEnabled == nil && input.CatalogType == nil && input.VariantSetID == nil &&
		input.CatalogID == nil && input.AudienceStrategy == nil {
		return false
	}
	if input.Name != nil && !validText(*input.Name, 1024) || input.Status != nil && !validStatus(*input.Status) || input.Budget != nil && !validBudget(*input.Budget) ||
		input.Goal != nil && !validGoalUpdate(*input.Goal, AccountTypeWeb) || input.Targeting != nil && !validTargeting(*input.Targeting) ||
		input.WebsiteURL != nil && !validAbsoluteURL(*input.WebsiteURL) {
		return false
	}
	if input.CatalogType != nil && *input.CatalogType != CatalogDPA && *input.CatalogType != CatalogDOA ||
		input.CatalogType != nil && *input.CatalogType == CatalogDOA && input.CatalogID != nil && !validNumericID(*input.CatalogID) {
		return false
	}
	return input.AudienceStrategy == nil || *input.AudienceStrategy == AudienceUniversal || *input.AudienceStrategy == AudienceProspecting || *input.AudienceStrategy == AudienceDiscovery
}

func validGoalUpdate(value GoalUpdate, accountType AccountType) bool {
	uniform, perCountry := value.GoalValueForAllCountries != "", len(value.CountryCodeToGoalValue) > 0
	if !uniform && !perCountry && value.ROASDayTarget == "" && value.EventTarget == "" || uniform && perCountry ||
		uniform && !validDecimal(value.GoalValueForAllCountries, true) || perCountry && !validCountryValues(value.CountryCodeToGoalValue, true) {
		return false
	}
	if accountType == AccountTypeApp {
		return value.ROASDayTarget == "" || value.ROASDayTarget == ROASDay7 || value.ROASDayTarget == ROASDay28
	}
	return value.ROASDayTarget == "" || value.ROASDayTarget == ROASDay0 || value.ROASDayTarget == ROASDay7
}

func validTrackingUpdate(value TrackingUpdate) bool {
	return (value.ImpressionURL != "" || value.ClickURL != "") && (value.ImpressionURL == "" || validAbsoluteURL(value.ImpressionURL)) &&
		(value.ClickURL == "" || validAbsoluteURL(value.ClickURL))
}

func validCampaignResponse(value Campaign, accountType AccountType) bool {
	return validNumericID(value.ID) && value.Type == accountType && validText(value.Name, 4096) && value.BiddingStrategy != "" && value.StartDate != "" && value.CreatedAt != ""
}

func (client *Client) GetCatalogInfo(ctx context.Context, options ...socialhub.CallOption) (*CatalogInfo, error) {
	if client.accountType != AccountTypeWeb {
		return nil, invalidArgument("campaign_catalog_info", "catalog discovery is available only for WEB accounts")
	}
	if err := client.requireAccess("campaign_catalog_info"); err != nil {
		return nil, err
	}
	var response CatalogInfo
	if err := client.getJSON(ctx, "campaign_catalog_info", "/campaign/catalog_info", nil, &response, options...); err != nil {
		return nil, err
	}
	for _, catalog := range response.Catalogs {
		if !validOpaque(catalog.ID, 256) || !validText(catalog.Name, 1024) {
			return nil, platformContractError("campaign_catalog_info", "Axon returned invalid catalog metadata")
		}
	}
	return &response, nil
}
