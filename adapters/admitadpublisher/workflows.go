package admitadpublisher

import (
	"context"
	"net/url"
	"strconv"
	"time"

	"social-hub/pkg/socialhub"
)

func (client *Client) ListProgramsForWebsite(
	ctx context.Context,
	input ListProgramsRequest,
	options ...socialhub.CallOption,
) (ProgramsResponse, error) {
	const operation = "list_programs_for_website"
	if !validListPrograms(input) {
		return ProgramsResponse{}, invalidArgument(operation, "website ID, connection status, program tool, or pagination is invalid")
	}
	query := make(url.Values)
	setOptionalQuery(query, "connection_status", string(input.ConnectionStatus))
	setOptionalQuery(query, "has_tool", string(input.HasTool))
	setPagination(query, input.Offset, input.Limit)
	path := "/advcampaigns/website/" + strconv.FormatInt(input.WebsiteID, 10) + "/"
	var output ProgramsResponse
	metadata, err := client.getJSON(ctx, operation, path, query, scopePrograms, &output, options...)
	if err != nil {
		return ProgramsResponse{}, err
	}
	if output.Results == nil || len(output.Results) > effectivePageLimit(input.Limit) {
		return ProgramsResponse{}, platformContractError(operation, "Admitad returned an invalid or oversized program page")
	}
	programIDs := make(map[int64]struct{}, len(output.Results))
	for _, program := range output.Results {
		programID, valid := positiveExactID(program.ID)
		if !valid {
			return ProgramsResponse{}, platformContractError(operation, "Admitad returned a program without a valid ID")
		}
		if _, duplicate := programIDs[programID]; duplicate {
			return ProgramsResponse{}, platformContractError(operation, "Admitad returned a duplicate program ID")
		}
		programIDs[programID] = struct{}{}
	}
	output.Meta = metadata
	return output, nil
}

func (client *Client) GetProgramForWebsite(
	ctx context.Context,
	input GetProgramRequest,
	options ...socialhub.CallOption,
) (Program, error) {
	const operation = "get_program_for_website"
	if !validGetProgram(input) {
		return Program{}, invalidArgument(operation, "website ID and campaign ID must be positive")
	}
	path := "/advcampaigns/" + strconv.FormatInt(input.CampaignID, 10) + "/website/" + strconv.FormatInt(input.WebsiteID, 10) + "/"
	var output Program
	metadata, err := client.getJSON(ctx, operation, path, nil, scopePrograms, &output, options...)
	if err != nil {
		return Program{}, err
	}
	programID, valid := positiveExactID(output.ID)
	if !valid || programID != input.CampaignID {
		return Program{}, platformContractError(operation, "Admitad returned a program with a missing or mismatched ID")
	}
	output.Meta = metadata
	return output, nil
}

func (client *Client) GenerateDeeplinks(
	ctx context.Context,
	input GenerateDeeplinksRequest,
	options ...socialhub.CallOption,
) (DeeplinksResponse, error) {
	const operation = "generate_deeplinks"
	if !validGenerateDeeplinks(input) {
		return DeeplinksResponse{}, invalidArgument(operation, "website ID, campaign ID, target URLs, or SubID value is invalid")
	}
	query := make(url.Values)
	setOptionalQuery(query, "subid", input.SubID)
	setOptionalQuery(query, "subid1", input.SubID1)
	setOptionalQuery(query, "subid2", input.SubID2)
	setOptionalQuery(query, "subid3", input.SubID3)
	setOptionalQuery(query, "subid4", input.SubID4)
	for _, target := range input.TargetURLs {
		query.Add("ulp", target)
	}
	path := "/deeplink/" + strconv.FormatInt(input.WebsiteID, 10) + "/advcampaign/" + strconv.FormatInt(input.CampaignID, 10) + "/"
	var output DeeplinksResponse
	metadata, err := client.getJSON(ctx, operation, path, query, scopeDeeplinks, &output, options...)
	if err != nil {
		return DeeplinksResponse{}, err
	}
	if output.Links == nil || len(output.Links) > len(input.TargetURLs) {
		return DeeplinksResponse{}, platformContractError(operation, "Admitad returned an invalid or oversized deeplink list")
	}
	for _, link := range output.Links {
		if !validTargetURL(link.Link) {
			return DeeplinksResponse{}, platformContractError(operation, "Admitad returned an invalid deeplink URL")
		}
	}
	output.Meta = metadata
	return output, nil
}

func (client *Client) ListCouponsForWebsite(
	ctx context.Context,
	input ListCouponsRequest,
	options ...socialhub.CallOption,
) (CouponsResponse, error) {
	const operation = "list_coupons_for_website"
	if !validListCoupons(input) {
		return CouponsResponse{}, invalidArgument(operation, "website ID, filters, date window, ordering, or pagination is invalid")
	}
	query := make(url.Values)
	setOptionalInt(query, "campaign", input.CampaignID)
	setOptionalInt(query, "category", input.CategoryID)
	setOptionalInt(query, "campaign_category", input.CampaignCategoryID)
	setOptionalInt(query, "type", input.TypeID)
	setOptionalQuery(query, "search", input.Search)
	setOptionalDate(query, "date_start", input.DateStart)
	setOptionalDate(query, "date_end", input.DateEnd)
	setOptionalQuery(query, "region", input.Region)
	setOptionalQuery(query, "language", input.Language)
	for _, order := range input.OrderBy {
		query.Add("order_by", order)
	}
	setOptionalBool(query, "is_tracking_promocode", input.IsTrackingPromocode)
	setOptionalBool(query, "has_affiliate_link", input.HasAffiliateLink)
	setOptionalQuery(query, "customer_type", string(input.CustomerType))
	setPagination(query, input.Offset, input.Limit)
	path := "/coupons/website/" + strconv.FormatInt(input.WebsiteID, 10) + "/"
	var output CouponsResponse
	metadata, err := client.getJSON(ctx, operation, path, query, scopeCoupons, &output, options...)
	if err != nil {
		return CouponsResponse{}, err
	}
	if output.Results == nil || len(output.Results) > effectivePageLimit(input.Limit) {
		return CouponsResponse{}, platformContractError(operation, "Admitad returned an invalid or oversized coupon page")
	}
	couponIDs := make(map[int64]struct{}, len(output.Results))
	for _, coupon := range output.Results {
		couponID, valid := positiveExactID(coupon.ID)
		if !valid {
			return CouponsResponse{}, platformContractError(operation, "Admitad returned a coupon without a valid ID")
		}
		if _, duplicate := couponIDs[couponID]; duplicate {
			return CouponsResponse{}, platformContractError(operation, "Admitad returned a duplicate coupon ID")
		}
		couponIDs[couponID] = struct{}{}
	}
	output.Meta = metadata
	return output, nil
}

func (client *Client) ListCampaignStatistics(
	ctx context.Context,
	input ListCampaignStatisticsRequest,
	options ...socialhub.CallOption,
) (CampaignStatisticsResponse, error) {
	const operation = "list_campaign_statistics"
	if !validListCampaignStatistics(input) {
		return CampaignStatisticsResponse{}, invalidArgument(operation, "date window, website IDs, campaign IDs, SubID, ordering, or pagination is invalid")
	}
	query := make(url.Values)
	setOptionalDate(query, "date_start", input.DateStart)
	setOptionalDate(query, "date_end", input.DateEnd)
	for _, websiteID := range input.WebsiteIDs {
		query.Add("website", strconv.FormatInt(websiteID, 10))
	}
	for _, campaignID := range input.CampaignIDs {
		query.Add("campaign", strconv.FormatInt(campaignID, 10))
	}
	setOptionalQuery(query, "subid", input.SubID)
	for _, order := range input.OrderBy {
		query.Add("order_by", string(order))
	}
	setPagination(query, input.Offset, input.Limit)
	query.Set("total", "0")
	var output CampaignStatisticsResponse
	metadata, err := client.getJSON(ctx, operation, "/statistics/campaigns/", query, scopeStatistics, &output, options...)
	if err != nil {
		return CampaignStatisticsResponse{}, err
	}
	if output.Results == nil || len(output.Results) > effectivePageLimit(input.Limit) {
		return CampaignStatisticsResponse{}, platformContractError(operation, "Admitad returned an invalid or oversized campaign statistics page")
	}
	campaignIDs := make(map[int64]struct{}, len(output.Results))
	for _, statistic := range output.Results {
		campaignID, valid := positiveExactID(statistic.CampaignID)
		if !valid {
			return CampaignStatisticsResponse{}, platformContractError(operation, "Admitad returned campaign statistics without a valid campaign ID")
		}
		if _, duplicate := campaignIDs[campaignID]; duplicate {
			return CampaignStatisticsResponse{}, platformContractError(operation, "Admitad returned duplicate campaign statistics")
		}
		campaignIDs[campaignID] = struct{}{}
	}
	output.Meta = metadata
	return output, nil
}

func setOptionalQuery(query url.Values, key, value string) {
	if value != "" {
		query.Set(key, value)
	}
}

func setOptionalInt(query url.Values, key string, value int64) {
	if value > 0 {
		query.Set(key, strconv.FormatInt(value, 10))
	}
}

func setOptionalBool(query url.Values, key string, value *bool) {
	if value != nil {
		query.Set(key, strconv.FormatBool(*value))
	}
}

func setOptionalDate(query url.Values, key string, value time.Time) {
	if !value.IsZero() {
		query.Set(key, value.Format("02.01.2006"))
	}
}

func setPagination(query url.Values, offset, limit int) {
	if offset > 0 {
		query.Set("offset", strconv.Itoa(offset))
	}
	if limit > 0 {
		query.Set("limit", strconv.Itoa(limit))
	}
}

var _ PublisherWorkflow = (*Client)(nil)
