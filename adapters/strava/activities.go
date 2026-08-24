package strava

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"social-hub/pkg/socialhub"
)

func (client *Client) CreateManualActivity(ctx context.Context, input ManualActivityRequest, options ...socialhub.CallOption) (*Activity, error) {
	if !validText(input.Name, 4096, false) || !validSportType(input.SportType) || input.StartDateLocal.IsZero() ||
		input.ElapsedTime <= 0 || input.ElapsedTime%time.Second != 0 || !validText(input.Description, 100_000, true) {
		return nil, invalidArgument("activity_create", "name, documented sport type, local start time, and positive whole-second elapsed time are required")
	}
	if input.DistanceMeters != nil && (*input.DistanceMeters < 0 || *input.DistanceMeters > 1_000_000_000) {
		return nil, invalidArgument("activity_create", "distance must be between 0 and 1,000,000,000 meters")
	}
	if err := client.requireScopes("activity_create", "activity:write"); err != nil {
		return nil, err
	}
	values := url.Values{
		"name": {input.Name}, "sport_type": {string(input.SportType)},
		"start_date_local": {input.StartDateLocal.Format(time.RFC3339)},
		"elapsed_time":     {strconv.FormatInt(int64(input.ElapsedTime/time.Second), 10)},
	}
	if input.Description != "" {
		values.Set("description", input.Description)
	}
	if input.DistanceMeters != nil {
		values.Set("distance", strconv.FormatFloat(*input.DistanceMeters, 'f', -1, 64))
	}
	if input.Trainer != nil {
		values.Set("trainer", boolForm(*input.Trainer))
	}
	if input.Commute != nil {
		values.Set("commute", boolForm(*input.Commute))
	}
	var response activityWire
	if err := client.form(ctx, "/activities", values, &response, options...); err != nil {
		return nil, err
	}
	return client.validateActivity("activity_create", response, "")
}

func (client *Client) UpdateActivity(ctx context.Context, activityID string, input ActivityUpdateRequest, options ...socialhub.CallOption) (*Activity, error) {
	if !validResourceID(activityID) {
		return nil, invalidArgument("activity_update", "activity ID must be a positive decimal Strava ID")
	}
	body := make(map[string]any)
	if input.Name != nil {
		if !validText(*input.Name, 4096, false) {
			return nil, invalidArgument("activity_update", "name must be non-empty and bounded")
		}
		body["name"] = *input.Name
	}
	if input.SportType != nil {
		if !validSportType(*input.SportType) {
			return nil, invalidArgument("activity_update", "sport_type is not documented by Strava")
		}
		body["sport_type"] = *input.SportType
	}
	if input.Description != nil {
		if !validText(*input.Description, 100_000, true) {
			return nil, invalidArgument("activity_update", "description is too large or invalid")
		}
		body["description"] = *input.Description
	}
	if input.Trainer != nil {
		body["trainer"] = *input.Trainer
	}
	if input.Commute != nil {
		body["commute"] = *input.Commute
	}
	if input.HideFromHome != nil {
		body["hide_from_home"] = *input.HideFromHome
	}
	if input.GearID != nil {
		if *input.GearID != "none" && !validOpaque(*input.GearID, 256) {
			return nil, invalidArgument("activity_update", "gear_id must be a bounded gear ID or none")
		}
		body["gear_id"] = *input.GearID
	}
	if len(body) == 0 {
		return nil, invalidArgument("activity_update", "at least one mutable activity field is required")
	}
	if err := client.requireScopes("activity_update", "activity:write"); err != nil {
		return nil, err
	}
	var response activityWire
	path := "/activities/" + url.PathEscape(activityID)
	if err := client.api.JSON(ctx, http.MethodPut, path, nil, body, &response, options...); err != nil {
		return nil, err
	}
	return client.validateActivity("activity_update", response, activityID)
}
