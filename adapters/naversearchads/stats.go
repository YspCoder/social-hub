package naversearchads

import (
	"context"
	"encoding/json"
	"net/url"

	"social-hub/pkg/socialhub"
)

const maximumStatsQueryBytes = 32 << 10

func (client *Client) Stats(ctx context.Context, input StatQuery, options ...socialhub.CallOption) (StatResponse, error) {
	const operation = "stats"
	if err := validateStatQuery(input); err != nil {
		return StatResponse{}, err
	}
	fields, err := json.Marshal(input.Fields)
	if err != nil {
		return StatResponse{}, platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	query := url.Values{"fields": {string(fields)}}
	if len(input.IDs) == 1 {
		query.Set("id", input.IDs[0])
	} else {
		query["ids"] = append([]string(nil), input.IDs...)
	}
	if input.TimeRange != nil {
		timeRange, err := json.Marshal(input.TimeRange)
		if err != nil {
			return StatResponse{}, platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
		query.Set("timeRange", string(timeRange))
	} else {
		query.Set("datePreset", string(input.DatePreset))
	}
	increment := input.TimeIncrement
	if increment == "" {
		if len(input.IDs) == 1 {
			increment = TimeIncrementDaily
		} else {
			increment = TimeIncrementAllDays
		}
	}
	query.Set("timeIncrement", string(increment))
	if input.Breakdown != "" {
		query.Set("breakdown", string(input.Breakdown))
	}
	if len(query.Encode()) > maximumStatsQueryBytes {
		return StatResponse{}, invalidArgument(operation, "Stats query exceeds 32 KiB")
	}
	var response StatResponse
	if err := client.getJSON(ctx, operation, "/stats", query, &response, options...); err != nil {
		return StatResponse{}, err
	}
	if increment == TimeIncrementDaily {
		if response.Daily == nil || response.Summary != nil || len(response.Daily.Data) > 100_000 {
			return StatResponse{}, platformContractError(operation, "NAVER returned an invalid daily Stats response")
		}
	} else if response.Summary == nil || response.Daily != nil || len(response.Summary.Data) > len(input.IDs) {
		return StatResponse{}, platformContractError(operation, "NAVER returned an invalid summary Stats response")
	}
	return response, nil
}
