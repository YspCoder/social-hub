package myanimelist

import (
	"context"
	"net/http"
	"strconv"

	"social-hub/pkg/socialhub"
)

func (c *Client) SearchManga(ctx context.Context, input SearchRequest, options ...socialhub.CallOption) (socialhub.Page[Manga], error) {
	if !validQuery(input.Query) || !validPage(input.Cursor, input.Limit) {
		return socialhub.Page[Manga]{}, invalidArgument("search_manga", "query, cursor, or limit is invalid")
	}
	query := pageQuery(input.Cursor, input.Limit)
	query.Set("q", input.Query)
	var response nodePageEnvelope[Manga]
	if err := c.requestJSON(ctx, http.MethodGet, "/manga", query, &response, options...); err != nil {
		return socialhub.Page[Manga]{}, err
	}
	return toPage(nodes(response.Data), response.Paging)
}

func (c *Client) GetManga(ctx context.Context, mangaID int64, options ...socialhub.CallOption) (*Manga, error) {
	if mangaID <= 0 {
		return nil, invalidArgument("get_manga", "manga ID must be positive")
	}
	var response Manga
	if err := c.requestJSON(ctx, http.MethodGet, "/manga/"+strconv.FormatInt(mangaID, 10), nil, &response, options...); err != nil {
		return nil, err
	}
	return &response, nil
}

func (c *Client) ListMangaRanking(ctx context.Context, input MangaRankingRequest, options ...socialhub.CallOption) (socialhub.Page[RankedManga], error) {
	if !validMangaRanking(input.Type) || !validPage(input.Cursor, input.Limit) {
		return socialhub.Page[RankedManga]{}, invalidArgument("manga_ranking", "ranking type, cursor, or limit is invalid")
	}
	query := pageQuery(input.Cursor, input.Limit)
	query.Set("ranking_type", string(input.Type))
	var response pageEnvelope[RankedManga]
	if err := c.requestJSON(ctx, http.MethodGet, "/manga/ranking", query, &response, options...); err != nil {
		return socialhub.Page[RankedManga]{}, err
	}
	return toPage(response.Data, response.Paging)
}
