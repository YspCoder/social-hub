package spotify

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

func (c *Client) GetTrack(ctx context.Context, trackID, market string, options ...socialhub.CallOption) (*Track, error) {
	if !validSpotifyID(trackID) {
		return nil, invalidArgument("get_track", "a valid Spotify track ID is required")
	}
	if !validMarket(market) {
		return nil, invalidArgument("get_track", "market must be an uppercase ISO 3166-1 alpha-2 code")
	}
	query := url.Values{}
	if market != "" {
		query.Set("market", market)
	}
	var response spotifyTrack
	if _, err := c.requestJSON(ctx, http.MethodGet, "/tracks/"+escapedID(trackID), query, nil, &response, options...); err != nil {
		return nil, err
	}
	mapped, err := mapTrack(response)
	if err != nil {
		return nil, err
	}
	if mapped.ID != trackID {
		return nil, mappingError("get_track", "Spotify track response ID did not match the request")
	}
	return &mapped, nil
}

func (c *Client) SearchTracks(ctx context.Context, input SearchTracksRequest, options ...socialhub.CallOption) (socialhub.Page[Track], error) {
	queryText := strings.TrimSpace(input.Query)
	if queryText == "" || utf8.RuneCountInString(queryText) > 1_000 {
		return socialhub.Page[Track]{}, invalidArgument("search_tracks", "query must contain at most 1000 characters")
	}
	if !validMarket(input.Market) {
		return socialhub.Page[Track]{}, invalidArgument("search_tracks", "market must be an uppercase ISO 3166-1 alpha-2 code")
	}
	query, err := pageQuery("search_tracks", input.Cursor, input.MaxResults, 10)
	if err != nil {
		return socialhub.Page[Track]{}, err
	}
	if offset := query.Get("offset"); offset != "" {
		value, _ := strconv.Atoi(offset)
		if value > 1_000 {
			return socialhub.Page[Track]{}, invalidArgument("search_tracks", "cursor exceeds Spotify's maximum search offset of 1000")
		}
	}
	query.Set("q", queryText)
	query.Set("type", "track")
	if input.Market != "" {
		query.Set("market", input.Market)
	}
	var response spotifySearchResponse
	if _, err := c.requestJSON(ctx, http.MethodGet, "/search", query, nil, &response, options...); err != nil {
		return socialhub.Page[Track]{}, err
	}
	items := make([]Track, 0, len(response.Tracks.Items))
	for _, item := range response.Tracks.Items {
		mapped, err := mapTrack(item)
		if err != nil {
			return socialhub.Page[Track]{}, err
		}
		items = append(items, mapped)
	}
	next, err := pageCursor(response.Tracks.Next, c.apiBaseURL)
	if err != nil {
		return socialhub.Page[Track]{}, err
	}
	if next != nil {
		offset, _ := strconv.Atoi(*next)
		if offset > 1_000 {
			next = nil
		}
	}
	return socialhub.Page[Track]{Items: items, NextCursor: next, HasMore: next != nil}, nil
}

var _ CatalogWorkflow = (*Client)(nil)
