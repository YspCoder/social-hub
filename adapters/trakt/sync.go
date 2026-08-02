package trakt

import (
	"context"
	"net/http"

	"social-hub/pkg/socialhub"
)

func (c *Client) AddHistory(ctx context.Context, input HistoryMutation, options ...socialhub.CallOption) (*SyncResult, error) {
	if err := validateHistoryMutation(input, false); err != nil {
		return nil, err
	}
	return c.sync(ctx, "/sync/history", input, options...)
}

func (c *Client) RemoveHistory(ctx context.Context, input HistoryMutation, options ...socialhub.CallOption) (*SyncResult, error) {
	if err := validateHistoryMutation(input, true); err != nil {
		return nil, err
	}
	return c.sync(ctx, "/sync/history/remove", input, options...)
}

func (c *Client) AddWatchlist(ctx context.Context, input MediaMutation, options ...socialhub.CallOption) (*SyncResult, error) {
	if err := validateMediaMutation(input); err != nil {
		return nil, err
	}
	return c.sync(ctx, "/sync/watchlist", input, options...)
}

func (c *Client) RemoveWatchlist(ctx context.Context, input MediaMutation, options ...socialhub.CallOption) (*SyncResult, error) {
	if err := validateMediaMutation(input); err != nil {
		return nil, err
	}
	return c.sync(ctx, "/sync/watchlist/remove", input, options...)
}

func (c *Client) AddRatings(ctx context.Context, input RatingsMutation, options ...socialhub.CallOption) (*SyncResult, error) {
	if err := validateRatingsMutation(input, true); err != nil {
		return nil, err
	}
	return c.sync(ctx, "/sync/ratings", input, options...)
}

func (c *Client) RemoveRatings(ctx context.Context, input RatingsMutation, options ...socialhub.CallOption) (*SyncResult, error) {
	if err := validateRatingsMutation(input, false); err != nil {
		return nil, err
	}
	return c.sync(ctx, "/sync/ratings/remove", withoutRatings(input), options...)
}

func (c *Client) sync(ctx context.Context, path string, input any, options ...socialhub.CallOption) (*SyncResult, error) {
	if err := c.requireOAuth(path); err != nil {
		return nil, err
	}
	var response SyncResult
	if _, err := c.requestJSON(ctx, http.MethodPost, path, nil, input, &response, options...); err != nil {
		return nil, err
	}
	return &response, nil
}

func validateHistoryMutation(input HistoryMutation, allowIDs bool) error {
	if len(input.Movies)+len(input.Shows)+len(input.Seasons)+len(input.Episodes) == 0 && (!allowIDs || len(input.IDs) == 0) {
		return invalidArgument("sync_history", "at least one media item or history ID is required")
	}
	for _, item := range input.Movies {
		if !validMovieRef(item.MovieRef) {
			return invalidArgument("sync_history", "movie reference is invalid")
		}
	}
	for _, item := range input.Shows {
		if !validShowRef(item.ShowRef) {
			return invalidArgument("sync_history", "show reference is invalid")
		}
	}
	for _, item := range input.Seasons {
		if !validSeasonRef(item.SeasonRef) {
			return invalidArgument("sync_history", "season reference is invalid")
		}
	}
	for _, item := range input.Episodes {
		if !validEpisodeRef(item.EpisodeRef) {
			return invalidArgument("sync_history", "episode reference is invalid")
		}
	}
	for _, id := range input.IDs {
		if !allowIDs || id <= 0 {
			return invalidArgument("sync_history", "history IDs are only valid for removal and must be positive")
		}
	}
	return nil
}

func validateMediaMutation(input MediaMutation) error {
	if len(input.Movies)+len(input.Shows)+len(input.Seasons)+len(input.Episodes) == 0 {
		return invalidArgument("sync_media", "at least one media item is required")
	}
	for _, item := range input.Movies {
		if !validMovieRef(item) {
			return invalidArgument("sync_media", "movie reference is invalid")
		}
	}
	for _, item := range input.Shows {
		if !validShowRef(item) {
			return invalidArgument("sync_media", "show reference is invalid")
		}
	}
	for _, item := range input.Seasons {
		if !validSeasonRef(item) {
			return invalidArgument("sync_media", "season reference is invalid")
		}
	}
	for _, item := range input.Episodes {
		if !validEpisodeRef(item) {
			return invalidArgument("sync_media", "episode reference is invalid")
		}
	}
	return nil
}

func validateRatingsMutation(input RatingsMutation, requireRating bool) error {
	if len(input.Movies)+len(input.Shows)+len(input.Seasons)+len(input.Episodes) == 0 {
		return invalidArgument("sync_ratings", "at least one rated item is required")
	}
	for _, item := range input.Movies {
		if !validIDs(item.IDs, MediaMovie) || !validMutationRating(item.Rating, requireRating) {
			return invalidArgument("sync_ratings", "movie rating is invalid")
		}
	}
	for _, item := range input.Shows {
		if !validIDs(item.IDs, MediaShow) || !validMutationRating(item.Rating, requireRating) {
			return invalidArgument("sync_ratings", "show rating is invalid")
		}
	}
	for _, item := range input.Seasons {
		if !validIDs(item.IDs, MediaSeason) || !validMutationRating(item.Rating, requireRating) {
			return invalidArgument("sync_ratings", "season rating is invalid")
		}
	}
	for _, item := range input.Episodes {
		if !validIDs(item.IDs, MediaEpisode) || !validMutationRating(item.Rating, requireRating) {
			return invalidArgument("sync_ratings", "episode rating is invalid")
		}
	}
	return nil
}

func validMutationRating(value int, required bool) bool {
	if required {
		return value >= 1 && value <= 10
	}
	return value >= 0 && value <= 10
}

func withoutRatings(input RatingsMutation) RatingsMutation {
	for index := range input.Movies {
		input.Movies[index].Rating = 0
	}
	for index := range input.Shows {
		input.Shows[index].Rating = 0
	}
	for index := range input.Seasons {
		input.Seasons[index].Rating = 0
	}
	for index := range input.Episodes {
		input.Episodes[index].Rating = 0
	}
	return input
}

func validMovieRef(input MovieRef) bool {
	return validIDs(input.IDs, MediaMovie) || (validText(input.Title, maxTextLength) && input.Year > 1800 && input.Year < 3000)
}

func validShowRef(input ShowRef) bool {
	return validIDs(input.IDs, MediaShow) || (validText(input.Title, maxTextLength) && input.Year > 1800 && input.Year < 3000)
}

func validSeasonRef(input SeasonRef) bool {
	return input.Number >= 0 && validIDs(input.IDs, MediaSeason)
}

func validEpisodeRef(input EpisodeRef) bool {
	return validIDs(input.IDs, MediaEpisode)
}

func validIDs(ids IDs, mediaType MediaType) bool {
	if ids.Trakt > 0 || ids.Slug != "" || ids.IMDB != "" || ids.TMDB > 0 || ids.TVDB > 0 {
		if ids.Trakt < 0 || ids.TMDB < 0 || ids.TVDB < 0 || (ids.Slug != "" && !validIdentifier(ids.Slug, maxIdentifierLength)) ||
			(ids.IMDB != "" && !validIdentifier(ids.IMDB, maxIdentifierLength)) {
			return false
		}
		switch mediaType {
		case MediaMovie:
			return ids.TVDB == 0
		case MediaEpisode:
			return ids.Slug == "" && ids.IMDB == "" && ids.TMDB == 0
		case MediaSeason:
			return ids.Slug == "" && ids.IMDB == ""
		default:
			return true
		}
	}
	return false
}
