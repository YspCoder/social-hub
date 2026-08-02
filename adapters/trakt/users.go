package trakt

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"social-hub/pkg/socialhub"
)

func (c *Client) GetProfile(ctx context.Context, username, extended string, options ...socialhub.CallOption) (*Profile, error) {
	username, err := c.resolveUsername(username, "get_profile")
	if err != nil || !validExtended(extended) {
		if err != nil {
			return nil, err
		}
		return nil, invalidArgument("get_profile", "extended mode is invalid")
	}
	query := url.Values{}
	setExtended(query, extended)
	var response Profile
	if _, err := c.requestJSON(ctx, http.MethodGet, "/users/"+escaped(username), query, nil, &response, options...); err != nil {
		return nil, err
	}
	return &response, nil
}

func (c *Client) GetSettings(ctx context.Context, options ...socialhub.CallOption) (*UserSettings, error) {
	if err := c.requireOAuth("get_settings"); err != nil {
		return nil, err
	}
	var response UserSettings
	if _, err := c.requestJSON(ctx, http.MethodGet, "/users/settings", nil, nil, &response, options...); err != nil {
		return nil, err
	}
	return &response, nil
}

func (c *Client) ListHistory(ctx context.Context, input HistoryRequest, options ...socialhub.CallOption) (socialhub.Page[HistoryItem], error) {
	username, err := c.resolveUsername(input.Username, "list_history")
	if err != nil {
		return socialhub.Page[HistoryItem]{}, err
	}
	page, err := validatePage(input.Cursor, input.MaxResults)
	if err != nil || !validExtended(input.Extended) || !validHistoryType(input.Type) ||
		(!input.StartAt.IsZero() && !input.EndAt.IsZero() && input.StartAt.After(input.EndAt)) {
		if err != nil {
			return socialhub.Page[HistoryItem]{}, err
		}
		return socialhub.Page[HistoryItem]{}, invalidArgument("list_history", "history filter is invalid")
	}
	path := "/users/" + escaped(username) + "/history"
	if input.Type != "" {
		path += "/" + pluralMediaType(input.Type)
	}
	query := url.Values{}
	setPage(query, page, input.MaxResults)
	setExtended(query, input.Extended)
	setTimeRange(query, input.StartAt, input.EndAt)
	var response []HistoryItem
	metadata, err := c.requestJSON(ctx, http.MethodGet, path, query, nil, &response, options...)
	if err != nil {
		return socialhub.Page[HistoryItem]{}, err
	}
	return pageFromMetadata(response, page, metadata), nil
}

func (c *Client) ListWatchlist(ctx context.Context, input WatchlistRequest, options ...socialhub.CallOption) (socialhub.Page[WatchlistItem], error) {
	username, err := c.resolveUsername(input.Username, "list_watchlist")
	if err != nil {
		return socialhub.Page[WatchlistItem]{}, err
	}
	page, err := validatePage(input.Cursor, input.MaxResults)
	if err != nil || !validExtended(input.Extended) || (input.Type != MediaMovie && input.Type != MediaShow) || !validSort(input.Sort) {
		if err != nil {
			return socialhub.Page[WatchlistItem]{}, err
		}
		return socialhub.Page[WatchlistItem]{}, invalidArgument("list_watchlist", "watchlist filter is invalid")
	}
	sort := input.Sort
	if sort == "" {
		sort = "rank"
	}
	path := "/users/" + escaped(username) + "/watchlist/" + pluralMediaType(input.Type) + "/" + sort
	query := url.Values{}
	setPage(query, page, input.MaxResults)
	setExtended(query, input.Extended)
	var response []WatchlistItem
	metadata, err := c.requestJSON(ctx, http.MethodGet, path, query, nil, &response, options...)
	if err != nil {
		return socialhub.Page[WatchlistItem]{}, err
	}
	return pageFromMetadata(response, page, metadata), nil
}

func (c *Client) ListRatings(ctx context.Context, input RatingsRequest, options ...socialhub.CallOption) (socialhub.Page[RatingItem], error) {
	username, err := c.resolveUsername(input.Username, "list_ratings")
	if err != nil {
		return socialhub.Page[RatingItem]{}, err
	}
	page, err := validatePage(input.Cursor, input.MaxResults)
	if err != nil || !validExtended(input.Extended) || !validRatingType(input.Type) || input.Rating < 0 || input.Rating > 10 {
		if err != nil {
			return socialhub.Page[RatingItem]{}, err
		}
		return socialhub.Page[RatingItem]{}, invalidArgument("list_ratings", "ratings filter is invalid")
	}
	path := "/users/" + escaped(username) + "/ratings/" + pluralMediaType(input.Type)
	if input.Rating > 0 {
		path += "/" + strconv.Itoa(input.Rating)
	}
	query := url.Values{}
	setPage(query, page, input.MaxResults)
	setExtended(query, input.Extended)
	var response []RatingItem
	metadata, err := c.requestJSON(ctx, http.MethodGet, path, query, nil, &response, options...)
	if err != nil {
		return socialhub.Page[RatingItem]{}, err
	}
	return pageFromMetadata(response, page, metadata), nil
}

func (c *Client) resolveUsername(value, operation string) (string, error) {
	value = firstNonEmpty(value, c.username)
	if value == "" && c.authenticated {
		value = "me"
	}
	if !validIdentifier(value, 255) {
		return "", invalidArgument(operation, "username is required")
	}
	return value, nil
}

func validHistoryType(value MediaType) bool {
	return value == "" || value == MediaMovie || value == MediaShow || value == MediaEpisode
}

func validRatingType(value MediaType) bool {
	return value == MediaMovie || value == MediaShow || value == MediaSeason || value == MediaEpisode
}

func pluralMediaType(value MediaType) string {
	switch value {
	case MediaMovie:
		return "movies"
	case MediaShow:
		return "shows"
	case MediaSeason:
		return "seasons"
	case MediaEpisode:
		return "episodes"
	default:
		return ""
	}
}

func validSort(value string) bool {
	switch value {
	case "", "rank", "added", "released", "title":
		return true
	default:
		return false
	}
}

func setTimeRange(query url.Values, start, end time.Time) {
	if !start.IsZero() {
		query.Set("start_at", start.UTC().Format(time.RFC3339))
	}
	if !end.IsZero() {
		query.Set("end_at", end.UTC().Format(time.RFC3339))
	}
}
