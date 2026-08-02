package myanimelist

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"social-hub/pkg/socialhub"
)

func (c *Client) GetMe(ctx context.Context, options ...socialhub.CallOption) (*User, error) {
	if err := c.requireUser("get_me"); err != nil {
		return nil, err
	}
	var response User
	if err := c.requestJSON(ctx, http.MethodGet, "/users/@me", nil, &response, options...); err != nil {
		return nil, err
	}
	return &response, nil
}

func (c *Client) ListAnimeList(ctx context.Context, input AnimeListRequest, options ...socialhub.CallOption) (socialhub.Page[AnimeListEntry], error) {
	if !validUsername(input.Username) || !validAnimeListState(input.Status) ||
		!validAnimeListSort(input.Sort) || !validPage(input.Cursor, input.Limit) {
		return socialhub.Page[AnimeListEntry]{}, invalidArgument("anime_list", "username, status, sort, cursor, or limit is invalid")
	}
	if input.Username == "@me" {
		if err := c.requireUser("anime_list"); err != nil {
			return socialhub.Page[AnimeListEntry]{}, err
		}
	}
	query := pageQuery(input.Cursor, input.Limit)
	if input.Status != "" {
		query.Set("status", string(input.Status))
	}
	if input.Sort != "" {
		query.Set("sort", string(input.Sort))
	}
	var response pageEnvelope[AnimeListEntry]
	path := "/users/" + escaped(input.Username) + "/animelist"
	if err := c.requestJSON(ctx, http.MethodGet, path, query, &response, options...); err != nil {
		return socialhub.Page[AnimeListEntry]{}, err
	}
	return toPage(response.Data, response.Paging)
}

func (c *Client) ListMangaList(ctx context.Context, input MangaListRequest, options ...socialhub.CallOption) (socialhub.Page[MangaListEntry], error) {
	if !validUsername(input.Username) || !validMangaListState(input.Status) ||
		!validMangaListSort(input.Sort) || !validPage(input.Cursor, input.Limit) {
		return socialhub.Page[MangaListEntry]{}, invalidArgument("manga_list", "username, status, sort, cursor, or limit is invalid")
	}
	if input.Username == "@me" {
		if err := c.requireUser("manga_list"); err != nil {
			return socialhub.Page[MangaListEntry]{}, err
		}
	}
	query := pageQuery(input.Cursor, input.Limit)
	if input.Status != "" {
		query.Set("status", string(input.Status))
	}
	if input.Sort != "" {
		query.Set("sort", string(input.Sort))
	}
	var response pageEnvelope[MangaListEntry]
	path := "/users/" + escaped(input.Username) + "/mangalist"
	if err := c.requestJSON(ctx, http.MethodGet, path, query, &response, options...); err != nil {
		return socialhub.Page[MangaListEntry]{}, err
	}
	return toPage(response.Data, response.Paging)
}

func (c *Client) UpdateAnimeListStatus(ctx context.Context, input UpdateAnimeListStatusRequest, options ...socialhub.CallOption) (*AnimeListStatus, error) {
	if err := c.requireWrite("update_anime_list"); err != nil {
		return nil, err
	}
	if input.AnimeID <= 0 || (input.Status != nil && !validAnimeListState(*input.Status)) ||
		!validScore(input.Score) || !validPriority(input.Priority) || !validRepeatValue(input.RewatchValue) ||
		!validCount(input.NumWatchedEpisodes) || !validCount(input.NumTimesRewatched) ||
		!validTags(input.Tags) || !validComment(input.Comments) {
		return nil, invalidArgument("update_anime_list", "anime list update is invalid")
	}
	values := url.Values{}
	if input.Status != nil {
		values.Set("status", string(*input.Status))
	}
	setBool(values, "is_rewatching", input.IsRewatching)
	setInt(values, "score", input.Score)
	setInt(values, "num_watched_episodes", input.NumWatchedEpisodes)
	setInt(values, "priority", input.Priority)
	setInt(values, "num_times_rewatched", input.NumTimesRewatched)
	setInt(values, "rewatch_value", input.RewatchValue)
	setTagsAndComments(values, input.Tags, input.Comments)
	if len(values) == 0 {
		return nil, invalidArgument("update_anime_list", "at least one list field is required")
	}
	var response AnimeListStatus
	path := "/anime/" + strconv.FormatInt(input.AnimeID, 10) + "/my_list_status"
	if err := c.requestForm(ctx, http.MethodPatch, path, values, &response, options...); err != nil {
		return nil, err
	}
	return &response, nil
}

func (c *Client) UpdateMangaListStatus(ctx context.Context, input UpdateMangaListStatusRequest, options ...socialhub.CallOption) (*MangaListStatus, error) {
	if err := c.requireWrite("update_manga_list"); err != nil {
		return nil, err
	}
	if input.MangaID <= 0 || (input.Status != nil && !validMangaListState(*input.Status)) ||
		!validScore(input.Score) || !validPriority(input.Priority) || !validRepeatValue(input.RereadValue) ||
		!validCount(input.NumVolumesRead) || !validCount(input.NumChaptersRead) || !validCount(input.NumTimesReread) ||
		!validTags(input.Tags) || !validComment(input.Comments) {
		return nil, invalidArgument("update_manga_list", "manga list update is invalid")
	}
	values := url.Values{}
	if input.Status != nil {
		values.Set("status", string(*input.Status))
	}
	setBool(values, "is_rereading", input.IsRereading)
	setInt(values, "score", input.Score)
	setInt(values, "num_volumes_read", input.NumVolumesRead)
	setInt(values, "num_chapters_read", input.NumChaptersRead)
	setInt(values, "priority", input.Priority)
	setInt(values, "num_times_reread", input.NumTimesReread)
	setInt(values, "reread_value", input.RereadValue)
	setTagsAndComments(values, input.Tags, input.Comments)
	if len(values) == 0 {
		return nil, invalidArgument("update_manga_list", "at least one list field is required")
	}
	var response MangaListStatus
	path := "/manga/" + strconv.FormatInt(input.MangaID, 10) + "/my_list_status"
	if err := c.requestForm(ctx, http.MethodPatch, path, values, &response, options...); err != nil {
		return nil, err
	}
	return &response, nil
}

func (c *Client) DeleteAnimeListStatus(ctx context.Context, animeID int64, options ...socialhub.CallOption) error {
	if err := c.requireWrite("delete_anime_list"); err != nil {
		return err
	}
	if animeID <= 0 {
		return invalidArgument("delete_anime_list", "anime ID must be positive")
	}
	path := "/anime/" + strconv.FormatInt(animeID, 10) + "/my_list_status"
	return c.requestForm(ctx, http.MethodDelete, path, nil, nil, options...)
}

func (c *Client) DeleteMangaListStatus(ctx context.Context, mangaID int64, options ...socialhub.CallOption) error {
	if err := c.requireWrite("delete_manga_list"); err != nil {
		return err
	}
	if mangaID <= 0 {
		return invalidArgument("delete_manga_list", "manga ID must be positive")
	}
	path := "/manga/" + strconv.FormatInt(mangaID, 10) + "/my_list_status"
	return c.requestForm(ctx, http.MethodDelete, path, nil, nil, options...)
}

func setInt(values url.Values, name string, value *int) {
	if value != nil {
		values.Set(name, strconv.Itoa(*value))
	}
}

func setBool(values url.Values, name string, value *bool) {
	if value != nil {
		values.Set(name, strconv.FormatBool(*value))
	}
}

func setTagsAndComments(values url.Values, tags []string, comments *string) {
	if tags != nil {
		values.Set("tags", strings.Join(tags, ","))
	}
	if comments != nil {
		values.Set("comments", *comments)
	}
}
