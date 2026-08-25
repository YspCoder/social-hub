package thetradedesk

import (
	"context"
	"net/http"
	"net/url"

	"social-hub/pkg/socialhub"
)

const (
	advertiserPath = "/v3/advertiser/"
	campaignPath   = "/v3/campaign"
)

func (client *Client) GetAdvertiser(ctx context.Context, options ...socialhub.CallOption) (*Advertiser, error) {
	const operation = "advertiser_get"
	callContext, cancel, err := resolveCallContext(ctx, operation, options)
	if err != nil {
		return nil, err
	}
	defer cancel()
	var advertiser Advertiser
	meta, err := client.doJSON(
		callContext, operation, http.MethodGet, advertiserPath+url.PathEscape(client.advertiserID),
		nil, nil, &advertiser, false,
	)
	if err != nil {
		return nil, err
	}
	advertiser.Meta = meta
	if advertiser.ID != client.advertiserID || !validID(advertiser.ID) || !validText(advertiser.Name, 64) ||
		advertiser.Availability != "" && !validAvailability(advertiser.Availability) {
		return nil, platformContractError(operation, "Platform API returned an invalid or different advertiser")
	}
	return &advertiser, nil
}

func (client *Client) GetCampaign(ctx context.Context, campaignID string, options ...socialhub.CallOption) (*Campaign, error) {
	const operation = "campaign_get"
	if !validID(campaignID) {
		return nil, invalidArgument(operation, "campaign ID is invalid")
	}
	callContext, cancel, err := resolveCallContext(ctx, operation, options)
	if err != nil {
		return nil, err
	}
	defer cancel()
	return client.getCampaign(callContext, campaignID, operation)
}

func (client *Client) getCampaign(ctx context.Context, campaignID, operation string) (*Campaign, error) {
	var campaign Campaign
	meta, err := client.doJSON(
		ctx, operation, http.MethodGet, campaignPath+"/"+url.PathEscape(campaignID),
		nil, nil, &campaign, false,
	)
	if err != nil {
		return nil, err
	}
	campaign.Meta = meta
	if !validCampaignResponse(campaign) || campaign.ID != campaignID || campaign.AdvertiserID != client.advertiserID {
		return nil, platformContractError(operation, "Platform API returned an invalid, different, or cross-advertiser Campaign")
	}
	return &campaign, nil
}

type campaignQueryWire struct {
	AdvertiserID   string         `json:"AdvertiserId"`
	PageSize       int32          `json:"PageSize"`
	PageStartIndex int32          `json:"PageStartIndex"`
	Availabilities []Availability `json:"Availabilities,omitempty"`
	SearchTerms    []string       `json:"SearchTerms,omitempty"`
	SortFields     []CampaignSort `json:"SortFields,omitempty"`
}

type campaignPageWire struct {
	Result               []Campaign `json:"Result"`
	ResultCount          *int64     `json:"ResultCount"`
	TotalFilteredCount   *int64     `json:"TotalFilteredCount"`
	TotalUnfilteredCount *int64     `json:"TotalUnfilteredCount"`
}

func (client *Client) QueryCampaigns(ctx context.Context, input CampaignQuery, options ...socialhub.CallOption) (*CampaignPage, error) {
	const operation = "campaign_query"
	if !validCampaignQuery(input) {
		return nil, invalidArgument(operation, "pagination, availability filters, search terms, or sort fields are invalid")
	}
	callContext, cancel, err := resolveCallContext(ctx, operation, options)
	if err != nil {
		return nil, err
	}
	defer cancel()
	pageSize := int32(100)
	if input.PageSize != nil {
		pageSize = *input.PageSize
	}
	request := campaignQueryWire{
		AdvertiserID: client.advertiserID, PageSize: pageSize, PageStartIndex: input.PageStartIndex,
		Availabilities: append([]Availability(nil), input.Availabilities...),
		SearchTerms:    append([]string(nil), input.SearchTerms...),
		SortFields:     append([]CampaignSort(nil), input.SortFields...),
	}
	var response campaignPageWire
	meta, err := client.doJSON(
		callContext, operation, http.MethodPost, campaignPath+"/query/advertiser",
		nil, request, &response, false,
	)
	if err != nil {
		return nil, err
	}
	if response.Result == nil || len(response.Result) > int(pageSize) {
		return nil, platformContractError(operation, "Platform API returned a missing or oversized Campaign page")
	}
	if response.ResultCount != nil && (*response.ResultCount < 0 || *response.ResultCount != int64(len(response.Result))) ||
		response.TotalFilteredCount != nil && *response.TotalFilteredCount < int64(len(response.Result)) ||
		response.TotalUnfilteredCount != nil && *response.TotalUnfilteredCount < int64(len(response.Result)) ||
		response.TotalFilteredCount != nil && response.TotalUnfilteredCount != nil &&
			*response.TotalUnfilteredCount < *response.TotalFilteredCount {
		return nil, platformContractError(operation, "Platform API returned inconsistent Campaign page counts")
	}
	seen := make(map[string]struct{}, len(response.Result))
	for index := range response.Result {
		campaign := &response.Result[index]
		campaign.Meta = meta
		if !validCampaignResponse(*campaign) || campaign.AdvertiserID != client.advertiserID {
			return nil, platformContractError(operation, "Platform API returned an invalid or cross-advertiser Campaign")
		}
		if _, exists := seen[campaign.ID]; exists {
			return nil, platformContractError(operation, "Platform API returned duplicate Campaigns")
		}
		seen[campaign.ID] = struct{}{}
	}
	return &CampaignPage{
		Campaigns: response.Result, ResultCount: response.ResultCount,
		TotalFilteredCount:   response.TotalFilteredCount,
		TotalUnfilteredCount: response.TotalUnfilteredCount, Meta: meta,
	}, nil
}

type campaignCreateWire struct {
	AdvertiserID                       string                           `json:"AdvertiserId"`
	CampaignName                       string                           `json:"CampaignName"`
	CampaignConversionReportingColumns []ConversionReportingColumnInput `json:"CampaignConversionReportingColumns"`
	PrimaryGoal                        CampaignGoal                     `json:"PrimaryGoal"`
	Description                        string                           `json:"Description,omitempty"`
	Budget                             *Money                           `json:"Budget,omitempty"`
	BudgetInImpressions                *int64                           `json:"BudgetInImpressions,omitempty"`
	DailyBudget                        *Money                           `json:"DailyBudget,omitempty"`
	DailyBudgetInImpressions           *int64                           `json:"DailyBudgetInImpressions,omitempty"`
	StartDate                          string                           `json:"StartDate"`
	EndDate                            *string                          `json:"EndDate,omitempty"`
	TimeZone                           string                           `json:"TimeZone,omitempty"`
	PacingMode                         PacingMode                       `json:"PacingMode,omitempty"`
	CampaignType                       CampaignType                     `json:"CampaignType,omitempty"`
	Version                            CampaignVersion                  `json:"Version,omitempty"`
	BudgetingVersion                   CampaignBudgetingVersion         `json:"BudgetingVersion,omitempty"`
	PrimaryChannel                     Channel                          `json:"PrimaryChannel,omitempty"`
	SeedID                             string                           `json:"SeedId,omitempty"`
	PurchaseOrderNumber                *string                          `json:"PurchaseOrderNumber,omitempty"`
}

func (client *Client) CreateCampaign(ctx context.Context, input CreateCampaignRequest, options ...socialhub.CallOption) (*Campaign, error) {
	const operation = "campaign_create"
	if !validCreateCampaign(input) {
		return nil, invalidArgument(operation, "Campaign fields, Kokai version, primary channel, primary goal, or single-flight budget are invalid")
	}
	callContext, cancel, err := resolveCallContext(ctx, operation, options)
	if err != nil {
		return nil, err
	}
	defer cancel()
	columns := append([]ConversionReportingColumnInput(nil), input.ConversionReportingColumns...)
	if columns == nil {
		columns = []ConversionReportingColumnInput{}
	}
	request := campaignCreateWire{
		AdvertiserID: client.advertiserID, CampaignName: input.Name,
		CampaignConversionReportingColumns: columns,
		PrimaryGoal:                        input.PrimaryGoal,
		Description:                        input.Description, Budget: input.Budget,
		BudgetInImpressions: input.BudgetInImpressions, DailyBudget: input.DailyBudget,
		DailyBudgetInImpressions: input.DailyBudgetInImpressions, StartDate: input.StartDate,
		EndDate: input.EndDate, TimeZone: input.TimeZone, PacingMode: input.PacingMode,
		CampaignType: input.Type, Version: input.Version, BudgetingVersion: input.BudgetingVersion,
		PrimaryChannel: input.PrimaryChannel, SeedID: input.SeedID,
		PurchaseOrderNumber: input.PurchaseOrderNumber,
	}
	var campaign Campaign
	meta, err := client.doJSON(
		callContext, operation, http.MethodPost, campaignPath, nil, request, &campaign, true,
	)
	if err != nil {
		return nil, err
	}
	campaign.Meta = meta
	if !validCampaignResponse(campaign) || campaign.AdvertiserID != client.advertiserID || campaign.Name != input.Name ||
		campaign.Version != CampaignVersionKokai || campaign.PrimaryChannel != input.PrimaryChannel {
		return &campaign, outcomeUnknownError(operation, platformContractError(operation, "created Campaign response did not match the request"), meta.RequestID, client.requestIDs)
	}
	return &campaign, nil
}

func (client *Client) UpdateCampaign(ctx context.Context, campaignID string, input UpdateCampaignRequest, options ...socialhub.CallOption) (*Campaign, error) {
	const operation = "campaign_update"
	if !validID(campaignID) || !validUpdateCampaign(input) {
		return nil, invalidArgument(operation, "campaign ID or partial update fields are invalid")
	}
	callContext, cancel, err := resolveCallContext(ctx, operation, options)
	if err != nil {
		return nil, err
	}
	defer cancel()
	if _, err := client.getCampaign(callContext, campaignID, operation); err != nil {
		return nil, err
	}
	payload := map[string]any{"CampaignId": campaignID}
	setCampaignPatch(payload, input)
	var campaign Campaign
	meta, err := client.doJSON(
		callContext, operation, http.MethodPut, campaignPath, nil, payload, &campaign, true,
	)
	if err != nil {
		return nil, err
	}
	campaign.Meta = meta
	if !validCampaignResponse(campaign) || campaign.ID != campaignID || campaign.AdvertiserID != client.advertiserID {
		return &campaign, outcomeUnknownError(operation, platformContractError(operation, "updated Campaign response was invalid or did not match the request"), meta.RequestID, client.requestIDs)
	}
	return &campaign, nil
}

func setCampaignPatch(payload map[string]any, input UpdateCampaignRequest) {
	setPointer(payload, "CampaignName", input.Name)
	setPointer(payload, "Description", input.Description)
	setPointer(payload, "Availability", input.Availability)
	setPointer(payload, "Budget", input.Budget)
	setNullable(payload, "BudgetInImpressions", input.BudgetInImpressions, input.ClearBudgetInImpressions)
	setNullable(payload, "DailyBudget", input.DailyBudget, input.ClearDailyBudget)
	setNullable(payload, "DailyBudgetInImpressions", input.DailyBudgetInImpressions, input.ClearDailyBudgetInImpressions)
	setPointer(payload, "StartDate", input.StartDate)
	setNullable(payload, "EndDate", input.EndDate, input.ClearEndDate)
	setPointer(payload, "TimeZone", input.TimeZone)
	setPointer(payload, "PacingMode", input.PacingMode)
	setPointer(payload, "PrimaryChannel", input.PrimaryChannel)
	setPointer(payload, "SeedId", input.SeedID)
	setNullable(payload, "PurchaseOrderNumber", input.PurchaseOrderNumber, input.ClearPurchaseOrderNumber)
	if input.ConversionReportingColumns != nil {
		payload["CampaignConversionReportingColumns"] = *input.ConversionReportingColumns
	}
}

func setPointer[T any](payload map[string]any, name string, value *T) {
	if value != nil {
		payload[name] = *value
	}
}

func setNullable[T any](payload map[string]any, name string, value *T, clear bool) {
	if clear {
		payload[name] = nil
	} else if value != nil {
		payload[name] = *value
	}
}
