package letterboxd

import (
	"context"
	"net/http"

	"social-hub/pkg/socialhub"
)

func (c *Client) SetLike(ctx context.Context, id string, liked bool, options ...socialhub.CallOption) error {
	return c.setRelationship(ctx, "set_like", "/me/like/", id, struct {
		Liked bool `json:"liked"`
	}{Liked: liked}, options...)
}

func (c *Client) SetRating(ctx context.Context, id string, rating *float64, options ...socialhub.CallOption) error {
	if rating != nil && !validRating(*rating) {
		return invalidArgument("set_rating", "rating must be null or 0.5 through 5.0 in 0.5 increments")
	}
	return c.setRelationship(ctx, "set_rating", "/me/rate/", id, struct {
		Rating *float64 `json:"rating"`
	}{Rating: rating}, options...)
}

func (c *Client) SetWatched(ctx context.Context, id string, watched bool, options ...socialhub.CallOption) error {
	return c.setRelationship(ctx, "set_watched", "/me/watch/", id, struct {
		Watched bool `json:"watched"`
	}{Watched: watched}, options...)
}

func (c *Client) SetWatchlist(ctx context.Context, id string, inWatchlist bool, options ...socialhub.CallOption) error {
	return c.setRelationship(ctx, "set_watchlist", "/me/watchlist/", id, struct {
		InWatchlist bool `json:"inWatchlist"`
	}{InWatchlist: inWatchlist}, options...)
}

func (c *Client) setRelationship(ctx context.Context, operation, prefix, id string, body any, options ...socialhub.CallOption) error {
	if err := c.requireContentModify(operation); err != nil {
		return err
	}
	if !validIdentifier(id) {
		return invalidArgument(operation, "object ID is invalid")
	}
	return c.requestJSON(ctx, http.MethodPatch, prefix+escaped(id), nil, body, nil, options...)
}
