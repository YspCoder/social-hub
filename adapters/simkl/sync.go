package simkl

import (
	"context"
	"net/http"
	"net/url"
	"slices"

	"social-hub/pkg/socialhub"
)

func (c *Client) GetActivities(ctx context.Context, options ...socialhub.CallOption) (*Activities, error) {
	if err := c.requireOAuth("sync_activities"); err != nil {
		return nil, err
	}
	var response Activities
	if _, err := requestJSON(ctx, c.userAPI, "sync_activities", http.MethodGet, "/sync/activities", nil, nil, &response, options...); err != nil {
		return nil, err
	}
	return &response, nil
}

func (c *Client) ListAllItems(ctx context.Context, input AllItemsRequest, options ...socialhub.CallOption) (*AllItems, error) {
	if err := c.requireOAuth("sync_all_items"); err != nil {
		return nil, err
	}
	if input.Type == "" {
		input.Type = SyncAll
	}
	if input.Status == "" {
		input.Status = StatusAll
	}
	if !slices.Contains(syncMediaTypes, input.Type) || !validStatus(input.Status, true) ||
		(input.Extended != "" && !slices.Contains(syncExtensions, input.Extended)) ||
		(input.IncludeAllEpisodes != "" && !slices.Contains(includeEpisodes, input.IncludeAllEpisodes)) {
		return nil, invalidArgument("sync_all_items", "type, status, extended mode, or episode inclusion mode is invalid")
	}
	if (input.EpisodeWatchedAt || input.IncludeAllEpisodes == IncludeEpisodesYes || input.IncludeAllEpisodes == IncludeEpisodesOriginal) &&
		input.Extended != SyncFull && input.Extended != SyncFullAnimeSeasons {
		return nil, invalidArgument("sync_all_items", "episode timestamps and episode inclusion require extended=full or full_anime_seasons")
	}
	query := make(url.Values)
	if input.DateFrom != nil {
		if input.DateFrom.IsZero() {
			return nil, invalidArgument("sync_all_items", "date_from must not be zero")
		}
		query.Set("date_from", input.DateFrom.UTC().Format("2006-01-02T15:04:05Z"))
	}
	if input.Extended != "" {
		query.Set("extended", string(input.Extended))
	}
	setYes(query, "next_watch_info", input.NextWatchInfo)
	setYes(query, "episode_tvdb_id", input.EpisodeTVDBID)
	setYes(query, "episode_watched_at", input.EpisodeWatchedAt)
	setYes(query, "memos", input.Memos)
	if input.IncludeAllEpisodes != "" {
		query.Set("include_all_episodes", string(input.IncludeAllEpisodes))
	}
	var response AllItems
	path := "/sync/all-items/" + string(input.Type) + "/" + string(input.Status)
	if _, err := requestJSON(ctx, c.userAPI, "sync_all_items", http.MethodGet, path, query, nil, &response, options...); err != nil {
		return nil, err
	}
	return &response, nil
}

func (c *Client) AddToList(ctx context.Context, input AddToListRequest, options ...socialhub.CallOption) (*ListMutationResult, error) {
	if err := c.requireOAuth("sync_add_to_list"); err != nil {
		return nil, err
	}
	if !validStatus(input.To, false) || len(input.Movies)+len(input.Shows)+len(input.Anime) == 0 {
		return nil, invalidArgument("sync_add_to_list", "a target status and at least one media item are required")
	}
	if len(input.Movies) > 0 && (input.To == StatusWatching || input.To == StatusHold) {
		return nil, invalidArgument("sync_add_to_list", "movies do not support watching or hold status")
	}
	if !validMediaRefs(input.Movies) || !validMediaRefs(input.Shows) || !validMediaRefs(input.Anime) {
		return nil, invalidArgument("sync_add_to_list", "one or more media references are invalid")
	}
	var response ListMutationResult
	if _, err := requestJSON(ctx, c.userAPI, "sync_add_to_list", http.MethodPost, "/sync/add-to-list", nil, input, &response, options...); err != nil {
		return nil, err
	}
	return &response, nil
}

func (c *Client) AddHistory(ctx context.Context, input HistoryMutation, options ...socialhub.CallOption) (*MutationResult, error) {
	return c.historyMutation(ctx, "sync_add_history", "/sync/history", input, false, options...)
}

func (c *Client) RemoveHistory(ctx context.Context, input HistoryMutation, options ...socialhub.CallOption) (*MutationResult, error) {
	return c.historyMutation(ctx, "sync_remove_history", "/sync/history/remove", input, true, options...)
}

func (c *Client) historyMutation(ctx context.Context, operation, path string, input HistoryMutation, removing bool, options ...socialhub.CallOption) (*MutationResult, error) {
	if err := c.requireOAuth(operation); err != nil {
		return nil, err
	}
	if !validHistoryMutation(input, removing) {
		return nil, invalidArgument(operation, "at least one valid movie, show, or anime history item is required")
	}
	var response MutationResult
	if _, err := requestJSON(ctx, c.userAPI, operation, http.MethodPost, path, nil, input, &response, options...); err != nil {
		return nil, err
	}
	return &response, nil
}

func (c *Client) AddRatings(ctx context.Context, input RatingsMutation, options ...socialhub.CallOption) (*MutationResult, error) {
	if err := c.requireOAuth("sync_add_ratings"); err != nil {
		return nil, err
	}
	if !validRatingsMutation(input) {
		return nil, invalidArgument("sync_add_ratings", "at least one valid integer rating from 1 through 10 is required")
	}
	var response MutationResult
	if _, err := requestJSON(ctx, c.userAPI, "sync_add_ratings", http.MethodPost, "/sync/ratings", nil, input, &response, options...); err != nil {
		return nil, err
	}
	return &response, nil
}

func (c *Client) RemoveRatings(ctx context.Context, input RatingRemoval, options ...socialhub.CallOption) (*MutationResult, error) {
	if err := c.requireOAuth("sync_remove_ratings"); err != nil {
		return nil, err
	}
	if len(input.Movies)+len(input.Shows)+len(input.Anime) == 0 ||
		!validMediaRefs(input.Movies) || !validMediaRefs(input.Shows) || !validMediaRefs(input.Anime) {
		return nil, invalidArgument("sync_remove_ratings", "at least one valid media reference is required")
	}
	var response MutationResult
	if _, err := requestJSON(ctx, c.userAPI, "sync_remove_ratings", http.MethodPost, "/sync/ratings/remove", nil, input, &response, options...); err != nil {
		return nil, err
	}
	return &response, nil
}

func validHistoryMutation(input HistoryMutation, removing bool) bool {
	if len(input.Movies)+len(input.Shows)+len(input.Anime) == 0 {
		return false
	}
	for _, item := range input.Movies {
		if !validateHistoryMedia(item, true) || removing && hasHistoryMetadata(item) {
			return false
		}
	}
	for _, items := range [][]HistorySeries{input.Shows, input.Anime} {
		for _, item := range items {
			if !validateHistorySeries(item) || removing && hasHistoryMetadata(item.HistoryMedia) {
				return false
			}
		}
	}
	return true
}

func hasHistoryMetadata(item HistoryMedia) bool {
	return item.WatchedAt != nil || item.Status != "" || item.Rating != 0 || item.Memo != nil
}

func validMediaRefs(items []MediaRef) bool {
	for _, item := range items {
		if !validMediaRef(item) {
			return false
		}
	}
	return true
}

func validRatingsMutation(input RatingsMutation) bool {
	if len(input.Movies)+len(input.Shows)+len(input.Anime) == 0 {
		return false
	}
	for _, items := range [][]MediaRating{input.Movies, input.Shows, input.Anime} {
		for _, item := range items {
			if !validMediaRef(item.MediaRef) || item.Rating < 1 || item.Rating > 10 {
				return false
			}
		}
	}
	return true
}

func setYes(query url.Values, name string, enabled bool) {
	if enabled {
		query.Set(name, "yes")
	}
}
