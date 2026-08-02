package tmdb

import (
	"context"
	"net/http"
	"net/url"
	"strconv"

	"social-hub/pkg/socialhub"
)

func (c *Client) SetFavorite(ctx context.Context, target MediaTarget, favorite bool, options ...socialhub.CallOption) (*StatusResponse, error) {
	query, err := c.accountQuery("set_favorite")
	if err != nil {
		return nil, err
	}
	if !validTarget(target) {
		return nil, invalidArgument("set_favorite", "movie or TV target is invalid")
	}
	body := map[string]any{"media_type": target.MediaType, "media_id": target.MediaID, "favorite": favorite}
	return c.writeStatus(ctx, http.MethodPost, "/account/"+strconv.FormatInt(c.tmdbAccountID, 10)+"/favorite", query, body, "set_favorite", options...)
}

func (c *Client) SetWatchlist(ctx context.Context, target MediaTarget, watchlist bool, options ...socialhub.CallOption) (*StatusResponse, error) {
	query, err := c.accountQuery("set_watchlist")
	if err != nil {
		return nil, err
	}
	if !validTarget(target) {
		return nil, invalidArgument("set_watchlist", "movie or TV target is invalid")
	}
	body := map[string]any{"media_type": target.MediaType, "media_id": target.MediaID, "watchlist": watchlist}
	return c.writeStatus(ctx, http.MethodPost, "/account/"+strconv.FormatInt(c.tmdbAccountID, 10)+"/watchlist", query, body, "set_watchlist", options...)
}

func (c *Client) SetRating(ctx context.Context, input RatingRequest, options ...socialhub.CallOption) (*StatusResponse, error) {
	query, err := c.ratingQuery("set_rating")
	if err != nil {
		return nil, err
	}
	if !validTarget(input.Target) || !validRating(input.Value) {
		return nil, invalidArgument("set_rating", "movie or TV target and a half-step rating from 0.5 to 10 are required")
	}
	path := "/" + string(input.Target.MediaType) + "/" + strconv.FormatInt(input.Target.MediaID, 10) + "/rating"
	return c.writeStatus(ctx, http.MethodPost, path, query, map[string]float64{"value": input.Value}, "set_rating", options...)
}

func (c *Client) DeleteRating(ctx context.Context, target MediaTarget, options ...socialhub.CallOption) (*StatusResponse, error) {
	query, err := c.ratingQuery("delete_rating")
	if err != nil {
		return nil, err
	}
	if !validTarget(target) {
		return nil, invalidArgument("delete_rating", "movie or TV target is invalid")
	}
	path := "/" + string(target.MediaType) + "/" + strconv.FormatInt(target.MediaID, 10) + "/rating"
	return c.writeStatus(ctx, http.MethodDelete, path, query, nil, "delete_rating", options...)
}

func (c *Client) writeStatus(ctx context.Context, method, path string, query url.Values, input any, operation string, options ...socialhub.CallOption) (*StatusResponse, error) {
	var response StatusResponse
	if err := c.requestJSON(ctx, method, path, query, input, &response, options...); err != nil {
		return nil, err
	}
	if err := validateStatus(operation, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

func validTarget(target MediaTarget) bool {
	return target.MediaID > 0 && validMediaType(target.MediaType, false, false)
}
