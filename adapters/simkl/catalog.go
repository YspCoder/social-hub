package simkl

import (
	"context"
	"net/http"
	"net/url"
	"slices"
	"strconv"

	"social-hub/pkg/socialhub"
)

func (c *Client) Search(ctx context.Context, input SearchRequest, options ...socialhub.CallOption) (socialhub.Page[SearchResult], error) {
	if err := c.requireClientID("catalog_search"); err != nil {
		return socialhub.Page[SearchResult]{}, err
	}
	page, err := validateSearchPage(input.Cursor, input.Limit)
	if err != nil {
		return socialhub.Page[SearchResult]{}, err
	}
	if !validOpaque(input.Query, maxQueryLength) || !slices.Contains(mediaTypes, input.Type) ||
		(input.Extended != "" && !slices.Contains(searchExtensions, input.Extended)) {
		return socialhub.Page[SearchResult]{}, invalidArgument("catalog_search", "query, media type, or extended mode is invalid")
	}
	query := url.Values{"q": {input.Query}}
	if page > 0 {
		query.Set("page", strconv.Itoa(page))
	}
	if input.Limit > 0 {
		query.Set("limit", strconv.Itoa(input.Limit))
	}
	if input.Extended != "" {
		query.Set("extended", string(input.Extended))
	}
	var response []SearchResult
	metadata, err := requestJSON(ctx, c.api, "catalog_search", http.MethodGet, "/search/"+string(input.Type), query, nil, &response, options...)
	if err != nil {
		return socialhub.Page[SearchResult]{}, err
	}
	return pageFromMetadata(response, page, metadata), nil
}

func (c *Client) GetMovie(ctx context.Context, id int64, options ...socialhub.CallOption) (*MovieDetail, error) {
	var response MovieDetail
	if err := c.catalogDetail(ctx, "catalog_movie", "/movies/", id, &response, options...); err != nil {
		return nil, err
	}
	return &response, nil
}

func (c *Client) GetTV(ctx context.Context, id int64, options ...socialhub.CallOption) (*TVDetail, error) {
	var response TVDetail
	if err := c.catalogDetail(ctx, "catalog_tv", "/tv/", id, &response, options...); err != nil {
		return nil, err
	}
	return &response, nil
}

func (c *Client) GetAnime(ctx context.Context, id int64, options ...socialhub.CallOption) (*AnimeDetail, error) {
	var response AnimeDetail
	if err := c.catalogDetail(ctx, "catalog_anime", "/anime/", id, &response, options...); err != nil {
		return nil, err
	}
	return &response, nil
}

func (c *Client) catalogDetail(ctx context.Context, operation, prefix string, id int64, output any, options ...socialhub.CallOption) error {
	if err := c.requireClientID(operation); err != nil {
		return err
	}
	if id <= 0 {
		return invalidArgument(operation, "a positive Simkl ID is required")
	}
	_, err := requestJSON(ctx, c.api, operation, http.MethodGet, prefix+strconv.FormatInt(id, 10), nil, nil, output, options...)
	return err
}

func (c *Client) ListTrending(ctx context.Context, input TrendingRequest, options ...socialhub.CallOption) ([]TrendingItem, error) {
	if !slices.Contains(mediaTypes, input.Type) || !slices.Contains(trendingPeriods, input.Period) ||
		(input.Limit != 100 && input.Limit != 500) {
		return nil, invalidArgument("trending_list", "media type, period, and limit 100 or 500 are required")
	}
	category := string(input.Type)
	if input.Type == MediaMovie {
		category = "movies"
	}
	path := "/discover/trending/" + category + "/" + string(input.Period) + "_" + strconv.Itoa(input.Limit) + ".json"
	var response []TrendingItem
	if _, err := requestJSON(ctx, c.cdn, "trending_list", http.MethodGet, path, nil, nil, &response, options...); err != nil {
		return nil, err
	}
	return response, nil
}
