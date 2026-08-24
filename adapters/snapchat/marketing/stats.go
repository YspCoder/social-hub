package marketing

import (
	"context"
	"net/url"
	"strings"
	"time"

	"social-hub/pkg/socialhub"
)

type totalStatItem struct {
	SubRequestStatus      string     `json:"sub_request_status"`
	SubRequestErrorReason string     `json:"sub_request_error_reason,omitempty"`
	Errors                []apiError `json:"errors,omitempty"`
	TotalStat             *TotalStat `json:"total_stat"`
}

type timeseriesStatItem struct {
	SubRequestStatus      string          `json:"sub_request_status"`
	SubRequestErrorReason string          `json:"sub_request_error_reason,omitempty"`
	Errors                []apiError      `json:"errors,omitempty"`
	TimeseriesStat        *TimeseriesStat `json:"timeseries_stat"`
}

type statsResponse struct {
	responseMeta
	TotalStats      []totalStatItem      `json:"total_stats"`
	TimeseriesStats []timeseriesStatItem `json:"timeseries_stats"`
	Paging          paging               `json:"paging"`
}

func (client *Client) GetAccountStats(ctx context.Context, input StatsRequest, options ...socialhub.CallOption) (StatsResult, error) {
	const operation = "account_stats"
	if !validStatsRequest(input) {
		return StatsResult{}, invalidArgument(operation, "granularity, time range, fields, cursor, or limit is invalid")
	}
	query := url.Values{"granularity": {string(input.Granularity)}}
	if len(input.Fields) > 0 {
		query.Set("fields", strings.Join(input.Fields, ","))
	}
	if input.StartTime != nil {
		query.Set("start_time", input.StartTime.UTC().Format(time.RFC3339))
		query.Set("end_time", input.EndTime.UTC().Format(time.RFC3339))
	}
	if input.Cursor != "" {
		query.Set("cursor", input.Cursor)
	}
	if input.Limit > 0 {
		query.Set("limit", fmtInt(input.Limit))
	}
	path := client.accountResourcePath("stats")
	var response statsResponse
	if _, err := client.getJSON(ctx, operation, path, query, &response, options...); err != nil {
		return StatsResult{}, err
	}
	states := make([]subRequestState, 0, len(response.TotalStats)+len(response.TimeseriesStats))
	for _, item := range response.TotalStats {
		states = append(states, subRequestState{Status: item.SubRequestStatus, Reason: item.SubRequestErrorReason, Errors: item.Errors})
	}
	for _, item := range response.TimeseriesStats {
		states = append(states, subRequestState{Status: item.SubRequestStatus, Reason: item.SubRequestErrorReason, Errors: item.Errors})
	}
	if err := checkResponse(operation, response.responseMeta, states); err != nil {
		return StatsResult{}, err
	}
	result := StatsResult{
		Totals: make([]TotalStat, len(response.TotalStats)), Timeseries: make([]TimeseriesStat, len(response.TimeseriesStats)),
	}
	for index, item := range response.TotalStats {
		if item.TotalStat == nil || item.TotalStat.ID != client.adAccountID {
			return StatsResult{}, platformContractError(operation, "Snapchat returned total statistics for another resource")
		}
		result.Totals[index] = *item.TotalStat
	}
	for index, item := range response.TimeseriesStats {
		if item.TimeseriesStat == nil || item.TimeseriesStat.ID != client.adAccountID {
			return StatsResult{}, platformContractError(operation, "Snapchat returned timeseries statistics for another resource")
		}
		result.Timeseries[index] = *item.TimeseriesStat
	}
	cursor, err := client.pageCursor(operation, path, response.Paging.NextLink)
	if err != nil {
		return StatsResult{}, err
	}
	result.NextCursor, result.HasMore = cursor, cursor != nil
	return result, nil
}

func validStatsRequest(input StatsRequest) bool {
	if !validGranularity(input.Granularity) || len(input.Fields) > 0 && !validFields(input.Fields) ||
		(input.Cursor != "" && !validOpaque(input.Cursor, 16384)) || input.Limit < 0 || input.Limit > 200 {
		return false
	}
	if input.Granularity == GranularityDay || input.Granularity == GranularityHour {
		if input.StartTime == nil || input.EndTime == nil {
			return false
		}
	}
	if (input.StartTime == nil) != (input.EndTime == nil) {
		return false
	}
	if input.StartTime != nil {
		return hourAligned(*input.StartTime) && hourAligned(*input.EndTime) && input.EndTime.After(*input.StartTime)
	}
	return true
}
