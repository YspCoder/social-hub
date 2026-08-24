package marketing

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"social-hub/pkg/socialhub"
)

var baseInsightFields = []string{
	"account_id", "account_name", "date_start", "date_stop", "impressions", "reach",
	"frequency", "clicks", "spend", "ctr", "cpc", "cpm", "actions", "cost_per_action_type",
}

func (client *Client) GetInsights(ctx context.Context, input InsightsRequest, options ...socialhub.CallOption) (InsightsPage, error) {
	limit, err := validatePage(input.Cursor, input.MaxResults)
	if err != nil {
		return InsightsPage{}, err
	}
	if input.EntityID != "" && !validNumericID(input.EntityID) {
		return InsightsPage{}, invalidArgument("insights_get", "entity ID must be numeric")
	}
	level := input.Level
	if level == "" {
		level = InsightLevelAccount
	}
	if level != InsightLevelAccount && level != InsightLevelCampaign && level != InsightLevelAdSet && level != InsightLevelAd {
		return InsightsPage{}, invalidArgument("insights_get", "level must be account, campaign, adset, or ad")
	}
	fields := input.Fields
	if len(fields) == 0 {
		fields = defaultFieldsForLevel(level)
	}
	for _, field := range append(append([]string(nil), fields...), input.Breakdowns...) {
		if !validFieldName(field) {
			return InsightsPage{}, invalidArgument("insights_get", "fields and breakdowns must be lowercase API identifiers")
		}
	}
	if input.DatePreset != "" && !validFieldName(input.DatePreset) || input.DatePreset != "" && input.TimeRange != nil {
		return InsightsPage{}, invalidArgument("insights_get", "date_preset is invalid or conflicts with time_range")
	}
	if input.TimeRange != nil && (!validDate(input.TimeRange.Since) || !validDate(input.TimeRange.Until) || input.TimeRange.Since > input.TimeRange.Until) {
		return InsightsPage{}, invalidArgument("insights_get", "time_range must contain ordered YYYY-MM-DD dates")
	}
	if input.TimeIncrement < 0 || input.TimeIncrement > 90 {
		return InsightsPage{}, invalidArgument("insights_get", "time_increment must be between 0 and 90 days")
	}
	if err := client.requireRead("insights_get"); err != nil {
		return InsightsPage{}, err
	}
	query := url.Values{
		"fields": {strings.Join(fields, ",")}, "level": {string(level)}, "default_summary": {"true"},
	}
	addPaging(query, input.Cursor, limit)
	if len(input.Breakdowns) > 0 {
		query.Set("breakdowns", strings.Join(input.Breakdowns, ","))
	}
	if input.DatePreset != "" {
		query.Set("date_preset", input.DatePreset)
	}
	if input.TimeRange != nil {
		if err := setJSONForm(query, "time_range", input.TimeRange, "insights_get"); err != nil {
			return InsightsPage{}, err
		}
	}
	if input.TimeIncrement > 0 {
		query.Set("time_increment", strconv.Itoa(input.TimeIncrement))
	}
	parent := client.adAccountResource()
	if input.EntityID != "" {
		parent = url.PathEscape(input.EntityID)
	}
	var response graphPage[Insight]
	if err := client.api.JSON(ctx, http.MethodGet, "/"+parent+"/insights", query, nil, &response, options...); err != nil {
		return InsightsPage{}, err
	}
	return InsightsPage{
		Items: response.Data, NextCursor: response.Paging.Cursors.After, PrevCursor: response.Paging.Cursors.Before,
		HasMore: response.Paging.Next != "", Summary: response.Summary,
	}, nil
}

func defaultFieldsForLevel(level InsightLevel) []string {
	fields := append([]string(nil), baseInsightFields...)
	if level == InsightLevelCampaign || level == InsightLevelAdSet || level == InsightLevelAd {
		fields = append(fields, "campaign_id", "campaign_name")
	}
	if level == InsightLevelAdSet || level == InsightLevelAd {
		fields = append(fields, "adset_id", "adset_name")
	}
	if level == InsightLevelAd {
		fields = append(fields, "ad_id", "ad_name")
	}
	return fields
}
