package tvmaze

import (
	"context"
	"net/url"
	"time"

	"social-hub/pkg/socialhub"
)

func (c *Client) ListSchedule(ctx context.Context, request ScheduleRequest, options ...socialhub.CallOption) ([]Episode, error) {
	if request.Country != "" && !validCountry(request.Country) {
		return nil, invalidArgument("list_schedule", "country must be an uppercase two-letter code")
	}
	if request.Date != nil && !validDate(*request.Date) {
		return nil, invalidArgument("list_schedule", "date must be set when supplied")
	}
	query := url.Values{}
	if request.Country != "" {
		query.Set("country", request.Country)
	}
	if request.Date != nil {
		query.Set("date", request.Date.Format(time.DateOnly))
	}
	var episodes []Episode
	_, err := requestJSON(ctx, c.api, "list_schedule", "/schedule", query, &episodes, options...)
	return episodes, err
}

func (c *Client) ListWebSchedule(ctx context.Context, request WebScheduleRequest, options ...socialhub.CallOption) ([]Episode, error) {
	if request.Country != nil && *request.Country != "" && !validCountry(*request.Country) {
		return nil, invalidArgument("list_web_schedule", "country must be empty or an uppercase two-letter code")
	}
	if request.Date != nil && !validDate(*request.Date) {
		return nil, invalidArgument("list_web_schedule", "date must be set when supplied")
	}
	query := url.Values{}
	if request.Country != nil {
		query.Set("country", *request.Country)
	}
	if request.Date != nil {
		query.Set("date", request.Date.Format(time.DateOnly))
	}
	var episodes []Episode
	_, err := requestJSON(ctx, c.api, "list_web_schedule", "/schedule/web", query, &episodes, options...)
	return episodes, err
}
