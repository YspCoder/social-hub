package outbrain

import (
	"context"
	"net/url"
	"strconv"
	"strings"

	"social-hub/pkg/socialhub"
)

func (client *Client) CampaignPerformance(ctx context.Context, input CampaignReportRequest, options ...socialhub.CallOption) (CampaignReport, error) {
	if !validCampaignReportRequest(input) {
		return CampaignReport{}, invalidArgument("campaign_performance", "report filters are invalid")
	}
	query := reportBaseQuery(input.From, input.To, input.Limit, input.Offset, input.Filter)
	setOptional(query, "sort", input.Sort)
	setBool(query, "includeArchivedCampaigns", input.IncludeArchivedCampaigns)
	setOptional(query, "budgetId", input.BudgetID)
	setIDs(query, "campaignId", input.CampaignIDs)
	setBool(query, "includeConversionDetails", input.IncludeConversionDetails)
	setBool(query, "conversionsByClickDate", input.ConversionsByClickDate)
	setBool(query, "includeViewedImpressions", input.IncludeViewedImpressions)
	setOptional(query, "timezone", input.Timezone)
	setBool(query, "enabledCampaignsOnly", input.EnabledCampaignsOnly)
	var report CampaignReport
	path := "reports/marketers/" + url.PathEscape(client.marketerID) + "/campaigns"
	if err := client.getJSON(ctx, "campaign_performance", path, query, &report, options...); err != nil {
		return CampaignReport{}, err
	}
	if report.TotalResults < len(report.Results) {
		return CampaignReport{}, platformContractError("campaign_performance", "report totalResults is smaller than results")
	}
	for _, row := range report.Results {
		if !validPathID(row.Metadata.ID) {
			return CampaignReport{}, platformContractError("campaign_performance", "report contains an invalid Campaign ID")
		}
	}
	return report, nil
}

func (client *Client) PromotedContentPerformance(ctx context.Context, input PromotedContentReportRequest, options ...socialhub.CallOption) (PromotedContentReport, error) {
	if !validPromotedContentReportRequest(input) {
		return PromotedContentReport{}, invalidArgument("promoted_content_performance", "report filters are invalid")
	}
	query := reportBaseQuery(input.From, input.To, input.Limit, input.Offset, input.Filter)
	setOptional(query, "sort", input.Sort)
	setBool(query, "includeArchivedCampaigns", input.IncludeArchivedCampaigns)
	setOptional(query, "budgetId", input.BudgetID)
	setIDs(query, "campaignId", input.CampaignIDs)
	setOptional(query, "promotedLinkId", input.PromotedLinkID)
	setOptional(query, "sequenceId", input.SequenceID)
	setBool(query, "includeConversionDetails", input.IncludeConversionDetails)
	setBool(query, "conversionsByClickDate", input.ConversionsByClickDate)
	setBool(query, "enabledCampaignsOnly", input.EnabledCampaignsOnly)
	var report PromotedContentReport
	path := "reports/marketers/" + url.PathEscape(client.marketerID) + "/promotedContent"
	if err := client.getJSON(ctx, "promoted_content_performance", path, query, &report, options...); err != nil {
		return PromotedContentReport{}, err
	}
	if report.TotalResults < len(report.Results) {
		return PromotedContentReport{}, platformContractError("promoted_content_performance", "report totalResults is smaller than results")
	}
	for _, row := range report.Results {
		if !validPathID(row.Metadata.CampaignID) || (row.Metadata.ID == "" && row.Metadata.SequenceID == "") {
			return PromotedContentReport{}, platformContractError("promoted_content_performance", "report contains invalid Promoted Content metadata")
		}
	}
	return report, nil
}

func (client *Client) CampaignPeriodicPerformance(ctx context.Context, input CampaignPeriodicReportRequest, options ...socialhub.CallOption) (CampaignPeriodicReport, error) {
	if !validCampaignPeriodicReportRequest(input) {
		return CampaignPeriodicReport{}, invalidArgument("campaign_periodic_performance", "report filters are invalid")
	}
	query := reportBaseQuery(input.From, input.To, input.Limit, input.Offset, input.Filter)
	setIDs(query, "campaignId", input.CampaignIDs)
	setOptional(query, "breakdown", input.Breakdown)
	setBool(query, "includeArchivedCampaigns", input.IncludeArchivedCampaigns)
	setBool(query, "includeConversionDetails", input.IncludeConversionDetails)
	setBool(query, "conversionsByClickDate", input.ConversionsByClickDate)
	setBool(query, "includeViewedImpressions", input.IncludeViewedImpressions)
	setBool(query, "enabledCampaignsOnly", input.EnabledCampaignsOnly)
	var report CampaignPeriodicReport
	path := "reports/marketers/" + url.PathEscape(client.marketerID) + "/campaigns/periodic"
	if err := client.getJSON(ctx, "campaign_periodic_performance", path, query, &report, options...); err != nil {
		return CampaignPeriodicReport{}, err
	}
	if report.TotalCampaigns != len(report.CampaignResults) {
		return CampaignPeriodicReport{}, platformContractError("campaign_periodic_performance", "totalCampaigns does not match results")
	}
	for _, result := range report.CampaignResults {
		if !validPathID(result.CampaignID) || result.TotalResults < len(result.Results) || !validPeriodicRows(result.Results) {
			return CampaignPeriodicReport{}, platformContractError("campaign_periodic_performance", "invalid periodic Campaign result")
		}
	}
	return report, nil
}

func (client *Client) PromotedContentPeriodicPerformance(ctx context.Context, input PromotedContentPeriodicReportRequest, options ...socialhub.CallOption) (PromotedContentPeriodicReport, error) {
	if !validPromotedContentPeriodicReportRequest(input) {
		return PromotedContentPeriodicReport{}, invalidArgument("promoted_content_periodic_performance", "report filters are invalid")
	}
	if _, err := client.GetCampaign(ctx, input.CampaignID, options...); err != nil {
		return PromotedContentPeriodicReport{}, err
	}
	query := reportBaseQuery(input.From, input.To, input.Limit, input.Offset, input.Filter)
	setOptional(query, "breakdown", input.Breakdown)
	setBool(query, "includeConversionDetails", input.IncludeConversionDetails)
	setBool(query, "conversionsByClickDate", input.ConversionsByClickDate)
	setBool(query, "enabledCampaignsOnly", input.EnabledCampaignsOnly)
	var report PromotedContentPeriodicReport
	path := "reports/marketers/" + url.PathEscape(client.marketerID) + "/campaigns/" + url.PathEscape(input.CampaignID) + "/periodicContent"
	if err := client.getJSON(ctx, "promoted_content_periodic_performance", path, query, &report, options...); err != nil {
		return PromotedContentPeriodicReport{}, err
	}
	if report.TotalPromotedLinks != len(report.PromotedLinkResults) {
		return PromotedContentPeriodicReport{}, platformContractError("promoted_content_periodic_performance", "totalPromotedLinks does not match results")
	}
	for _, result := range report.PromotedLinkResults {
		if !validPathID(result.PromotedLinkID) || result.TotalResults < len(result.Results) || !validPeriodicRows(result.Results) {
			return PromotedContentPeriodicReport{}, platformContractError("promoted_content_periodic_performance", "invalid periodic PromotedLink result")
		}
	}
	return report, nil
}

func reportBaseQuery(from, to string, limit, offset int, filter string) url.Values {
	query := url.Values{"from": {from}, "to": {to}}
	if limit > 0 {
		query.Set("limit", strconv.Itoa(limit))
	}
	if offset > 0 {
		query.Set("offset", strconv.Itoa(offset))
	}
	setOptional(query, "filter", filter)
	return query
}

func setOptional(query url.Values, name, value string) {
	if value != "" {
		query.Set(name, value)
	}
}

func setBool(query url.Values, name string, value bool) {
	if value {
		query.Set(name, "true")
	}
}

func setIDs(query url.Values, name string, values []string) {
	if len(values) > 0 {
		query.Set(name, strings.Join(values, ","))
	}
}

func validCampaignReportRequest(input CampaignReportRequest) bool {
	return validDateWindow(input.From, input.To) && validPage(input.Limit, input.Offset, 1000) && validFilter(input.Filter) &&
		validReportSort(input.Sort, campaignReportSortFields) && validOptionalID(input.BudgetID) && validIDs(input.CampaignIDs) &&
		validTimezone(input.Timezone)
}

func validPromotedContentReportRequest(input PromotedContentReportRequest) bool {
	return validDateWindow(input.From, input.To) && validPage(input.Limit, input.Offset, 1000) && validFilter(input.Filter) &&
		validReportSort(input.Sort, promotedContentSortFields) && validOptionalID(input.BudgetID) && validIDs(input.CampaignIDs) &&
		validOptionalID(input.PromotedLinkID) && validOptionalID(input.SequenceID)
}

func validCampaignPeriodicReportRequest(input CampaignPeriodicReportRequest) bool {
	return validDateWindow(input.From, input.To) && validPage(input.Limit, input.Offset, 1000) && validFilter(input.Filter) &&
		validIDs(input.CampaignIDs) && validBreakdown(input.Breakdown, true)
}

func validPromotedContentPeriodicReportRequest(input PromotedContentPeriodicReportRequest) bool {
	return validPathID(input.CampaignID) && validDateWindow(input.From, input.To) && validPage(input.Limit, input.Offset, 1000) &&
		validFilter(input.Filter) && validBreakdown(input.Breakdown, false)
}

func validOptionalID(value string) bool { return value == "" || validPathID(value) }

func validReportSort(value string, allowed map[string]struct{}) bool {
	if value == "" {
		return true
	}
	field := strings.TrimPrefix(value, "-")
	_, found := allowed[field]
	return found
}

func validBreakdown(value string, campaign bool) bool {
	if value == "" {
		return true
	}
	allowed := map[string]struct{}{"daily": {}, "weekly": {}, "monthly": {}}
	if campaign {
		allowed["hourOfDay"] = struct{}{}
		allowed["dayOfWeek"] = struct{}{}
		allowed["dayOfWeekByHour"] = struct{}{}
	}
	_, found := allowed[value]
	return found
}

func validPeriodicRows(rows []PeriodicRow) bool {
	for _, row := range rows {
		if row.Metadata.ID == "" || !validDate(row.Metadata.FromDate) || !validDate(row.Metadata.ToDate) {
			return false
		}
	}
	return true
}

var campaignReportSortFields = map[string]struct{}{
	"impressions": {}, "clicks": {}, "ctr": {}, "spend": {}, "ecpc": {}, "conversions": {},
	"conversionRate": {}, "cpa": {}, "name": {}, "enabled": {}, "creativeFormat": {},
	"budgetStartDate": {}, "budgetEndDate": {}, "budgetAmount": {},
}

var promotedContentSortFields = map[string]struct{}{
	"impressions": {}, "clicks": {}, "ctr": {}, "spend": {}, "ecpc": {}, "conversions": {},
	"conversionRate": {}, "cpa": {}, "text": {}, "creationTime": {}, "lastModified": {},
	"url": {}, "status": {}, "approvalStatus": {}, "enabled": {}, "campaignName": {},
	"cpcAdjustment": {}, "creativeFormat": {},
}
