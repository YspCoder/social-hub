package criteo

import (
	"context"
	"strings"
	"time"

	"social-hub/pkg/socialhub"
)

const adSetsPath = "/2026-01/marketing-solutions/ad-sets"

func (client *Client) GetAdSet(ctx context.Context, adSetID string, options ...socialhub.CallOption) (AdSet, error) {
	const operation = "ad_set_get"
	if !validID(adSetID) {
		return AdSet{}, invalidArgument(operation, "Ad Set ID must be a positive string-encoded integer")
	}
	var response apiEnvelope[entityResource[AdSet]]
	if err := client.getJSON(ctx, operation, adSetsPath+"/"+adSetID, nil, &response, options...); err != nil {
		return AdSet{}, err
	}
	if err := checkProblems(operation, response.Errors); err != nil {
		return AdSet{}, err
	}
	adSet, err := adSetFromResource(operation, response.Data)
	if err != nil {
		return AdSet{}, err
	}
	if adSet.ID != adSetID {
		return AdSet{}, platformContractError(operation, "Criteo returned a different Ad Set")
	}
	if adSet.AdvertiserID != client.advertiserID {
		return AdSet{}, ownershipError(operation)
	}
	return adSet, nil
}

func (client *Client) SearchAdSets(ctx context.Context, input AdSetSearchRequest, options ...socialhub.CallOption) ([]AdSet, error) {
	const operation = "ad_set_search"
	if !validIDs(input.AdSetIDs, 1000) || !validIDs(input.CampaignIDs, 1000) {
		return nil, invalidArgument(operation, "Ad Set and Campaign IDs must be unique positive string-encoded integers")
	}
	var request adSetSearchEnvelope
	request.Filters.AdvertiserIDs = []string{client.advertiserID}
	request.Filters.AdSetIDs = append([]string(nil), input.AdSetIDs...)
	request.Filters.CampaignIDs = append([]string(nil), input.CampaignIDs...)
	var response apiEnvelope[[]entityResource[AdSet]]
	if err := client.postJSON(ctx, operation, adSetsPath+"/search", request, &response, options...); err != nil {
		return nil, err
	}
	if err := checkProblems(operation, response.Errors); err != nil {
		return nil, err
	}
	result := make([]AdSet, 0, len(response.Data))
	seen := make(map[string]struct{}, len(response.Data))
	for _, resource := range response.Data {
		adSet, err := adSetFromResource(operation, resource)
		if err != nil {
			return nil, err
		}
		if adSet.AdvertiserID != client.advertiserID {
			return nil, ownershipError(operation)
		}
		if _, exists := seen[adSet.ID]; exists {
			return nil, platformContractError(operation, "Criteo returned duplicate Ad Sets")
		}
		seen[adSet.ID] = struct{}{}
		result = append(result, adSet)
	}
	return result, nil
}

func (client *Client) CreateAdSet(ctx context.Context, campaignID string, input CreateAdSetRequest, options ...socialhub.CallOption) (AdSet, error) {
	const operation = "ad_set_create"
	if !validID(campaignID) || !validCreateAdSet(input) {
		return AdSet{}, invalidArgument(operation, "Campaign ID or Ad Set fields are invalid")
	}
	if _, err := client.GetCampaign(ctx, campaignID, options...); err != nil {
		return AdSet{}, withOperation(err, operation)
	}
	attributes := createAdSetAttributes{
		CampaignID: campaignID, Name: input.Name, DatasetID: input.DatasetID,
		Objective: input.Objective, MediaType: input.MediaType, Schedule: input.Schedule,
		Bidding: input.Bidding, Budget: input.Budget, Targeting: input.Targeting,
		TrackingCode: input.TrackingCode, AttributionConfiguration: input.AttributionConfiguration,
	}
	request := struct {
		Data entityResource[createAdSetAttributes] `json:"data"`
	}{Data: entityResource[createAdSetAttributes]{Type: "AdSet", Attributes: attributes}}
	var response apiEnvelope[entityResource[AdSet]]
	if err := client.postJSON(ctx, operation, adSetsPath, request, &response, options...); err != nil {
		return AdSet{}, err
	}
	if err := checkProblems(operation, response.Errors); err != nil {
		return AdSet{}, err
	}
	adSet, err := adSetFromResource(operation, response.Data)
	if err != nil {
		return AdSet{}, err
	}
	if adSet.AdvertiserID != client.advertiserID || adSet.CampaignID != campaignID || adSet.Name != input.Name {
		return AdSet{}, platformContractError(operation, "Criteo returned an Ad Set that does not match the create request")
	}
	if adSet.Schedule.ActivationStatus != ActivationOff || adSet.Schedule.DeliveryStatus != DeliveryDraft {
		return AdSet{}, platformContractError(operation, "new Criteo Ad Set was not returned off and draft")
	}
	return adSet, nil
}

func (client *Client) UpdateAdSet(ctx context.Context, adSetID string, input UpdateAdSetRequest, options ...socialhub.CallOption) (AdSet, error) {
	const operation = "ad_set_update"
	if !validID(adSetID) || !validAdSetPatch(input) {
		return AdSet{}, invalidArgument(operation, "Ad Set ID or patch fields are invalid")
	}
	if _, err := client.GetAdSet(ctx, adSetID, options...); err != nil {
		return AdSet{}, withOperation(err, operation)
	}
	request := adSetPatchEnvelope{Data: []entityResource[adSetPatchAttributes]{{
		Type: "PatchAdSetV24Q3", ID: adSetID,
		Attributes: adSetPatchAttributes{
			Name: input.Name, AttributionConfiguration: input.AttributionConfiguration,
			Bidding: input.Bidding, Budget: input.Budget, Scheduling: input.Schedule,
		},
	}}}
	var response apiEnvelope[[]idResource]
	if err := client.patchJSON(ctx, operation, adSetsPath, request, &response, options...); err != nil {
		return AdSet{}, err
	}
	if err := checkProblems(operation, response.Errors); err != nil {
		return AdSet{}, err
	}
	if err := validateIDResult(operation, response.Data, adSetID, "AdSetIdV24Q3"); err != nil {
		return AdSet{}, err
	}
	updated, err := client.GetAdSet(ctx, adSetID, options...)
	return updated, withOperation(err, operation)
}

func (client *Client) StartAdSet(ctx context.Context, adSetID string, options ...socialhub.CallOption) (AdSet, error) {
	const operation = "ad_set_start"
	current, err := client.GetAdSet(ctx, adSetID, options...)
	if err != nil {
		return AdSet{}, withOperation(err, operation)
	}
	if current.Schedule.ActivationStatus == ActivationOn {
		return current, nil
	}
	if current.Schedule.DeliveryStatus == DeliveryArchived || current.Schedule.DeliveryStatus == DeliveryEnded {
		return AdSet{}, conflictError(operation, "archived or ended Ad Sets cannot be restarted")
	}
	if err := client.changeAdSetActivation(ctx, operation, "start", adSetID, options...); err != nil {
		return AdSet{}, err
	}
	updated, err := client.GetAdSet(ctx, adSetID, options...)
	if err != nil {
		return AdSet{}, withOperation(err, operation)
	}
	if updated.Schedule.ActivationStatus != ActivationOn {
		return AdSet{}, platformContractError(operation, "Criteo did not activate the Ad Set")
	}
	return updated, nil
}

func (client *Client) StopAdSet(ctx context.Context, adSetID string, options ...socialhub.CallOption) (AdSet, error) {
	const operation = "ad_set_stop"
	current, err := client.GetAdSet(ctx, adSetID, options...)
	if err != nil {
		return AdSet{}, withOperation(err, operation)
	}
	if current.Schedule.ActivationStatus == ActivationOff {
		return current, nil
	}
	if err := client.changeAdSetActivation(ctx, operation, "stop", adSetID, options...); err != nil {
		return AdSet{}, err
	}
	updated, err := client.GetAdSet(ctx, adSetID, options...)
	if err != nil {
		return AdSet{}, withOperation(err, operation)
	}
	if updated.Schedule.ActivationStatus != ActivationOff {
		return AdSet{}, platformContractError(operation, "Criteo did not stop the Ad Set")
	}
	return updated, nil
}

func (client *Client) changeAdSetActivation(ctx context.Context, operation, action, adSetID string, options ...socialhub.CallOption) error {
	request := adSetIDEnvelope{Data: []idResource{{Type: "AdSetId", ID: adSetID}}}
	var response apiEnvelope[[]idResource]
	if err := client.postJSON(ctx, operation, adSetsPath+"/"+action, request, &response, options...); err != nil {
		return err
	}
	if err := checkProblems(operation, response.Errors); err != nil {
		return err
	}
	return validateIDResult(operation, response.Data, adSetID, "AdSetId")
}

func adSetFromResource(operation string, resource entityResource[AdSet]) (AdSet, error) {
	if resource.Type != "" && !strings.EqualFold(resource.Type, "AdSet") {
		return AdSet{}, platformContractError(operation, "Criteo returned an invalid Ad Set resource type")
	}
	result := resource.Attributes
	result.ID = resource.ID
	if !validID(result.ID) || !validID(result.AdvertiserID) || !validID(result.CampaignID) || !validID(result.DatasetID) ||
		!validText(result.Name, 1024) || !validObjective(result.Objective) || !validMediaType(result.MediaType) ||
		(result.Schedule.ActivationStatus != ActivationOn && result.Schedule.ActivationStatus != ActivationOff) ||
		!validDeliveryStatus(result.Schedule.DeliveryStatus) || !validReadSchedule(result.Schedule) ||
		!validReadBidding(result.Bidding) || !validReadBudget(result.Budget) || !validReadAttribution(result.AttributionConfiguration) {
		return AdSet{}, platformContractError(operation, "Criteo returned an invalid Ad Set")
	}
	return result, nil
}

func validateIDResult(operation string, resources []idResource, expectedID, expectedType string) error {
	if len(resources) != 1 || resources[0].ID != expectedID || resources[0].Type != expectedType {
		return platformContractError(operation, "Criteo returned an invalid mutation result")
	}
	return nil
}

func validDeliveryStatus(value DeliveryStatus) bool {
	switch value {
	case DeliveryDraft, DeliveryInactive, DeliveryLive, DeliveryNotLive, DeliveryPausing, DeliveryPaused,
		DeliveryScheduled, DeliveryEnded, DeliveryNotDelivering, DeliveryArchived:
		return true
	default:
		return false
	}
}

func validReadSchedule(value AdSetSchedule) bool {
	if value.StartDate.Value != nil && !validDateTime(*value.StartDate.Value) ||
		value.EndDate.Value != nil && !validDateTime(*value.EndDate.Value) {
		return false
	}
	if value.StartDate.Value != nil && value.EndDate.Value != nil {
		start, _ := time.Parse(time.RFC3339Nano, *value.StartDate.Value)
		end, _ := time.Parse(time.RFC3339Nano, *value.EndDate.Value)
		return !end.Before(start)
	}
	return true
}

func validReadBidding(value *AdSetBidding) bool {
	return value == nil || (value.CostController == "" || validCostController(value.CostController)) && validOptionalPositive(value.BidAmount)
}

func validReadBudget(value *AdSetBudget) bool {
	if value == nil || !validOptionalPositive(value.Amount) {
		return value == nil
	}
	if value.DeliverySmoothing != "" && value.DeliverySmoothing != DeliveryAccelerated && value.DeliverySmoothing != DeliveryStandard {
		return false
	}
	if value.DeliveryWeek != "" && value.DeliveryWeek != WeekUndefined && !validDeliveryWeek(value.DeliveryWeek) {
		return false
	}
	switch value.Strategy {
	case "":
		return value.Amount == nil && value.Renewal == ""
	case BudgetCapped:
		return value.Renewal == "" || value.Renewal == BudgetDaily || value.Renewal == BudgetMonthly || value.Renewal == BudgetLifetime || value.Renewal == BudgetWeekly
	case BudgetUncapped:
		return value.Amount == nil && (value.Renewal == "" || value.Renewal == BudgetUndefined)
	default:
		return false
	}
}

func validReadAttribution(value *AttributionConfiguration) bool {
	if value == nil {
		return true
	}
	if value.Method == "" {
		return value.LookbackWindow == ""
	}
	return validAttribution(value)
}

func validCreateAdSet(input CreateAdSetRequest) bool {
	if !validText(input.Name, 1024) || !validID(input.DatasetID) || !validObjective(input.Objective) ||
		!validMediaType(input.MediaType) || !validDateTime(input.Schedule.StartDate) ||
		!validOptionalDateTime(input.Schedule.EndDate) || !validCostController(input.Bidding.CostController) ||
		!validOptionalPositive(input.Bidding.BidAmount) || !validBudget(input.Budget) ||
		!validOptionalText(input.TrackingCode, 4096) || !validFrequency(input.Targeting.FrequencyCapping) {
		return false
	}
	if input.Schedule.EndDate != nil {
		start, _ := time.Parse(time.RFC3339Nano, input.Schedule.StartDate)
		end, _ := time.Parse(time.RFC3339Nano, *input.Schedule.EndDate)
		if end.Before(start) {
			return false
		}
	}
	if input.Targeting.DeliveryLimitations != nil && !validDeliveryLimitations(*input.Targeting.DeliveryLimitations) {
		return false
	}
	if input.Targeting.GeoLocation != nil && (!validTargetingRule(input.Targeting.GeoLocation.Countries) ||
		!validTargetingRule(input.Targeting.GeoLocation.Subdivisions) || !validTargetingRule(input.Targeting.GeoLocation.ZipCodes)) {
		return false
	}
	return validAttribution(input.AttributionConfiguration)
}

func validAttribution(value *AttributionConfiguration) bool {
	if value == nil {
		return true
	}
	switch value.Method {
	case AttributionUnknown, AttributionCriteo, AttributionGoogleAnalyticsLastClick,
		AttributionGoogleAnalyticsDataDriven, AttributionLastClick, AttributionPostClick:
	default:
		return false
	}
	switch value.LookbackWindow {
	case "", LookbackUnknown, Lookback30M, Lookback24H, Lookback7D, Lookback30D:
		return true
	default:
		return false
	}
}

func validAdSetPatch(input UpdateAdSetRequest) bool {
	if input.Name == nil && input.AttributionConfiguration == nil && input.Bidding == nil && input.Budget == nil && input.Schedule == nil {
		return false
	}
	if input.Name != nil && !validText(*input.Name, 1024) {
		return false
	}
	if !validAttribution(input.AttributionConfiguration) {
		return false
	}
	if input.Bidding != nil {
		if input.Bidding.BidAmount == nil || (input.Bidding.BidAmount.Value != nil && !validPositive(*input.Bidding.BidAmount.Value)) {
			return false
		}
	}
	if input.Budget != nil && !validPatchBudget(*input.Budget) {
		return false
	}
	if input.Schedule != nil {
		if input.Schedule.StartDate == nil && input.Schedule.EndDate == nil {
			return false
		}
		for _, value := range []*NullableTime{input.Schedule.StartDate, input.Schedule.EndDate} {
			if value != nil && value.Value != nil && !validDateTime(*value.Value) {
				return false
			}
		}
	}
	return true
}

func validPatchBudget(value PatchAdSetBudget) bool {
	if value.Amount == nil && value.Strategy == nil && value.Renewal == nil && value.DeliverySmoothing == nil && value.DeliveryWeek == nil {
		return false
	}
	if value.Amount != nil && value.Amount.Value != nil && !validPositive(*value.Amount.Value) {
		return false
	}
	if value.Strategy != nil && *value.Strategy != BudgetCapped && *value.Strategy != BudgetUncapped {
		return false
	}
	if value.Renewal != nil && *value.Renewal != BudgetUndefined && *value.Renewal != BudgetDaily &&
		*value.Renewal != BudgetMonthly && *value.Renewal != BudgetLifetime && *value.Renewal != BudgetWeekly {
		return false
	}
	if value.DeliverySmoothing != nil && *value.DeliverySmoothing != DeliveryAccelerated && *value.DeliverySmoothing != DeliveryStandard {
		return false
	}
	return value.DeliveryWeek == nil || *value.DeliveryWeek == WeekUndefined || validDeliveryWeek(*value.DeliveryWeek)
}
