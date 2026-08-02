package myanimelist

import (
	"context"
	"net/http"
	"net/url"
	"strconv"

	"social-hub/pkg/socialhub"
)

func (c *Client) SearchAnime(ctx context.Context, input SearchRequest, options ...socialhub.CallOption) (socialhub.Page[Anime], error) {
	if !validQuery(input.Query) || !validPage(input.Cursor, input.Limit) {
		return socialhub.Page[Anime]{}, invalidArgument("search_anime", "query, cursor, or limit is invalid")
	}
	query := pageQuery(input.Cursor, input.Limit)
	query.Set("q", input.Query)
	var response nodePageEnvelope[Anime]
	if err := c.requestJSON(ctx, http.MethodGet, "/anime", query, &response, options...); err != nil {
		return socialhub.Page[Anime]{}, err
	}
	return toPage(nodes(response.Data), response.Paging)
}

func (c *Client) GetAnime(ctx context.Context, animeID int64, options ...socialhub.CallOption) (*Anime, error) {
	if animeID <= 0 {
		return nil, invalidArgument("get_anime", "anime ID must be positive")
	}
	var response Anime
	if err := c.requestJSON(ctx, http.MethodGet, "/anime/"+strconv.FormatInt(animeID, 10), nil, &response, options...); err != nil {
		return nil, err
	}
	return &response, nil
}

func (c *Client) ListAnimeRanking(ctx context.Context, input AnimeRankingRequest, options ...socialhub.CallOption) (socialhub.Page[RankedAnime], error) {
	if !validAnimeRanking(input.Type) || !validPage(input.Cursor, input.Limit) {
		return socialhub.Page[RankedAnime]{}, invalidArgument("anime_ranking", "ranking type, cursor, or limit is invalid")
	}
	query := pageQuery(input.Cursor, input.Limit)
	query.Set("ranking_type", string(input.Type))
	var response pageEnvelope[RankedAnime]
	if err := c.requestJSON(ctx, http.MethodGet, "/anime/ranking", query, &response, options...); err != nil {
		return socialhub.Page[RankedAnime]{}, err
	}
	return toPage(response.Data, response.Paging)
}

func (c *Client) ListSeasonalAnime(ctx context.Context, input SeasonalAnimeRequest, options ...socialhub.CallOption) (socialhub.Page[Anime], error) {
	if input.Year < 1900 || input.Year > 3000 || !validSeason(input.Season) ||
		!validSeasonalSort(input.Sort) || !validPage(input.Cursor, input.Limit) {
		return socialhub.Page[Anime]{}, invalidArgument("seasonal_anime", "year, season, sort, cursor, or limit is invalid")
	}
	query := pageQuery(input.Cursor, input.Limit)
	if input.Sort != "" {
		query.Set("sort", string(input.Sort))
	}
	path := "/anime/season/" + strconv.Itoa(input.Year) + "/" + url.PathEscape(string(input.Season))
	var response nodePageEnvelope[Anime]
	if err := c.requestJSON(ctx, http.MethodGet, path, query, &response, options...); err != nil {
		return socialhub.Page[Anime]{}, err
	}
	return toPage(nodes(response.Data), response.Paging)
}

func (c *Client) ListAnimeSuggestions(ctx context.Context, input PageRequest, options ...socialhub.CallOption) (socialhub.Page[Anime], error) {
	if err := c.requireUser("anime_suggestions"); err != nil {
		return socialhub.Page[Anime]{}, err
	}
	if !validPage(input.Cursor, input.Limit) {
		return socialhub.Page[Anime]{}, invalidArgument("anime_suggestions", "cursor or limit is invalid")
	}
	var response nodePageEnvelope[Anime]
	if err := c.requestJSON(ctx, http.MethodGet, "/anime/suggestions", pageQuery(input.Cursor, input.Limit), &response, options...); err != nil {
		return socialhub.Page[Anime]{}, err
	}
	return toPage(nodes(response.Data), response.Paging)
}
