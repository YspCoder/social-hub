package simkl

import (
	"context"
	"net/http"

	"social-hub/pkg/socialhub"
)

func (c *Client) Start(ctx context.Context, input ScrobbleRequest, options ...socialhub.CallOption) (*ScrobbleResult, error) {
	return c.scrobble(ctx, "scrobble_start", "/scrobble/start", input, options...)
}

func (c *Client) Pause(ctx context.Context, input ScrobbleRequest, options ...socialhub.CallOption) (*ScrobbleResult, error) {
	return c.scrobble(ctx, "scrobble_pause", "/scrobble/pause", input, options...)
}

func (c *Client) Stop(ctx context.Context, input ScrobbleRequest, options ...socialhub.CallOption) (*ScrobbleResult, error) {
	return c.scrobble(ctx, "scrobble_stop", "/scrobble/stop", input, options...)
}

func (c *Client) Checkin(ctx context.Context, input ScrobbleRequest, options ...socialhub.CallOption) (*ScrobbleResult, error) {
	return c.scrobble(ctx, "scrobble_checkin", "/scrobble/checkin", input, options...)
}

func (c *Client) scrobble(ctx context.Context, operation, path string, input ScrobbleRequest, options ...socialhub.CallOption) (*ScrobbleResult, error) {
	if err := c.requireOAuth(operation); err != nil {
		return nil, err
	}
	if !validScrobble(input) {
		return nil, invalidArgument(operation, "progress and exactly one movie or one show/anime episode are required")
	}
	var response ScrobbleResult
	if _, err := requestJSON(ctx, c.userAPI, operation, http.MethodPost, path, nil, input, &response, options...); err != nil {
		return nil, err
	}
	return &response, nil
}

func validScrobble(input ScrobbleRequest) bool {
	if !validProgress(input.Progress) {
		return false
	}
	mediaCount := 0
	for _, media := range []*MediaRef{input.Movie, input.Show, input.Anime} {
		if media != nil {
			mediaCount++
			if !validMediaRef(*media) {
				return false
			}
		}
	}
	if mediaCount != 1 {
		return false
	}
	if input.Movie != nil {
		return input.Episode == nil
	}
	if input.Episode == nil {
		return false
	}
	episode := input.Episode
	if !zeroIDs(episode.IDs) {
		return episode.Season == 0 && episode.Number == 0 && validEpisodeIDs(episode.IDs)
	}
	return episode.Season >= 0 && episode.Number >= 1
}
