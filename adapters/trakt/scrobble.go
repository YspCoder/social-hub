package trakt

import (
	"context"
	"net/http"

	"social-hub/pkg/socialhub"
)

func (c *Client) StartScrobble(ctx context.Context, input ScrobbleRequest, options ...socialhub.CallOption) (*ScrobbleResult, error) {
	return c.scrobble(ctx, "/scrobble/start", input, false, options...)
}

func (c *Client) PauseScrobble(ctx context.Context, input ScrobbleRequest, options ...socialhub.CallOption) (*ScrobbleResult, error) {
	return c.scrobble(ctx, "/scrobble/pause", input, false, options...)
}

func (c *Client) StopScrobble(ctx context.Context, input ScrobbleRequest, options ...socialhub.CallOption) (*ScrobbleResult, error) {
	return c.scrobble(ctx, "/scrobble/stop", input, true, options...)
}

func (c *Client) scrobble(ctx context.Context, path string, input ScrobbleRequest, stopping bool, options ...socialhub.CallOption) (*ScrobbleResult, error) {
	if err := c.requireOAuth(path); err != nil {
		return nil, err
	}
	minimum := 0.0
	if stopping {
		minimum = 1
	}
	if input.Progress < minimum || input.Progress > 100 || (input.Movie == nil) == (input.Episode == nil) {
		return nil, invalidArgument(path, "progress and exactly one movie or episode are required")
	}
	if input.Movie != nil && (!validText(input.Movie.Title, maxTextLength) || input.Movie.Year <= 1800 || !validIDs(input.Movie.IDs, MediaMovie)) {
		return nil, invalidArgument(path, "movie title, year, and identifiers are required")
	}
	if input.Episode != nil && !validEpisodeRef(*input.Episode) {
		return nil, invalidArgument(path, "episode identifiers are required")
	}
	var response ScrobbleResult
	if _, err := c.requestJSON(ctx, http.MethodPost, path, nil, input, &response, options...); err != nil {
		return nil, err
	}
	return &response, nil
}
