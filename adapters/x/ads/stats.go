package ads

import (
	"context"
	"net/url"
	"strings"
	"time"

	"social-hub/pkg/socialhub"
)

type statsResponse struct {
	DataType         string        `json:"data_type"`
	TimeSeriesLength int           `json:"time_series_length"`
	Data             []EntityStats `json:"data"`
}

func (client *Client) GetStats(ctx context.Context, input StatsRequest, options ...socialhub.CallOption) (StatsResult, error) {
	const operation = "stats_get"
	if !client.validStatsRequest(input) {
		return StatsResult{}, invalidArgument(operation, "entity, IDs, time range, granularity, placement, or metric groups are invalid")
	}
	entityIDs := make([]string, len(input.EntityIDs))
	copy(entityIDs, input.EntityIDs)
	groups := make([]string, len(input.MetricGroups))
	for index, group := range input.MetricGroups {
		groups[index] = string(group)
	}
	query := url.Values{
		"entity": {string(input.Entity)}, "entity_ids": {strings.Join(entityIDs, ",")},
		"start_time": {input.StartTime.UTC().Format(time.RFC3339)}, "end_time": {input.EndTime.UTC().Format(time.RFC3339)},
		"granularity": {string(input.Granularity)}, "placement": {string(input.Placement)},
		"metric_groups": {strings.Join(groups, ",")},
	}
	var response statsResponse
	if _, err := client.get(ctx, "/stats"+client.accountPath(), query, &response, options...); err != nil {
		return StatsResult{}, err
	}
	if response.DataType != "stats" || response.TimeSeriesLength < 0 || len(response.Data) != len(input.EntityIDs) {
		return StatsResult{}, platformContractError(operation, "X returned a malformed Stats envelope")
	}
	remaining := stringSet(input.EntityIDs)
	for _, item := range response.Data {
		if _, exists := remaining[item.ID]; !exists {
			return StatsResult{}, platformContractError(operation, "X returned Stats for an unexpected entity")
		}
		delete(remaining, item.ID)
	}
	return StatsResult{
		DataType: response.DataType, TimeSeriesLength: response.TimeSeriesLength,
		Entities: response.Data,
	}, nil
}

func (client *Client) validStatsRequest(input StatsRequest) bool {
	if !validAnalyticsEntity(input.Entity) || !validUniqueAdsIDs(input.EntityIDs, 20) ||
		!hourAligned(input.StartTime) || !hourAligned(input.EndTime) || !input.EndTime.After(input.StartTime) ||
		input.EndTime.Sub(input.StartTime) > 7*24*time.Hour || !validGranularity(input.Granularity) ||
		!validAnalyticsPlacement(input.Placement) || !validMetricGroups(input.MetricGroups) {
		return false
	}
	if input.Entity == AnalyticsAccount {
		return len(input.EntityIDs) == 1 && input.EntityIDs[0] == client.adsAccountID
	}
	return true
}
