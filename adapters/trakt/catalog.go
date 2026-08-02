package trakt

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"social-hub/pkg/socialhub"
)

func (c *Client) Search(ctx context.Context, input SearchRequest, options ...socialhub.CallOption) (socialhub.Page[SearchResult], error) {
	page, err := validatePage(input.Cursor, input.MaxResults)
	if err != nil {
		return socialhub.Page[SearchResult]{}, err
	}
	if !validText(input.Query, maxTextLength) || len(input.Types) == 0 || !validExtended(input.Extended) {
		return socialhub.Page[SearchResult]{}, invalidArgument("search", "query, media types, or extended mode is invalid")
	}
	types := make([]string, 0, len(input.Types))
	seen := make(map[MediaType]struct{}, len(input.Types))
	for _, mediaType := range input.Types {
		if !validSearchType(mediaType) {
			return socialhub.Page[SearchResult]{}, invalidArgument("search", "unsupported media type")
		}
		if _, exists := seen[mediaType]; !exists {
			seen[mediaType] = struct{}{}
			types = append(types, string(mediaType))
		}
	}
	query := url.Values{"query": {input.Query}}
	setPage(query, page, input.MaxResults)
	setExtended(query, input.Extended)
	if len(input.Fields) > 0 {
		for _, field := range input.Fields {
			if !validIdentifier(field, 64) {
				return socialhub.Page[SearchResult]{}, invalidArgument("search", "search field is invalid")
			}
		}
		query.Set("fields", strings.Join(input.Fields, ","))
	}
	var response []SearchResult
	metadata, err := c.requestJSON(ctx, http.MethodGet, "/search/"+strings.Join(types, ","), query, nil, &response, options...)
	if err != nil {
		return socialhub.Page[SearchResult]{}, err
	}
	return pageFromMetadata(response, page, metadata), nil
}

func (c *Client) GetMovie(ctx context.Context, id, extended string, options ...socialhub.CallOption) (*Movie, error) {
	if !validIdentifier(id, maxIdentifierLength) || !validExtended(extended) {
		return nil, invalidArgument("get_movie", "movie ID or extended mode is invalid")
	}
	query := url.Values{}
	setExtended(query, extended)
	var response Movie
	if _, err := c.requestJSON(ctx, http.MethodGet, "/movies/"+escaped(id), query, nil, &response, options...); err != nil {
		return nil, err
	}
	return &response, nil
}

func (c *Client) GetShow(ctx context.Context, id, extended string, options ...socialhub.CallOption) (*Show, error) {
	if !validIdentifier(id, maxIdentifierLength) || !validExtended(extended) {
		return nil, invalidArgument("get_show", "show ID or extended mode is invalid")
	}
	query := url.Values{}
	setExtended(query, extended)
	var response Show
	if _, err := c.requestJSON(ctx, http.MethodGet, "/shows/"+escaped(id), query, nil, &response, options...); err != nil {
		return nil, err
	}
	return &response, nil
}

func (c *Client) GetEpisode(ctx context.Context, showID string, season, episode int, extended string, options ...socialhub.CallOption) (*Episode, error) {
	if !validIdentifier(showID, maxIdentifierLength) || season < 0 || episode < 1 || !validExtended(extended) {
		return nil, invalidArgument("get_episode", "show ID, season, episode, or extended mode is invalid")
	}
	query := url.Values{}
	setExtended(query, extended)
	path := "/shows/" + escaped(showID) + "/seasons/" + strconv.Itoa(season) + "/episodes/" + strconv.Itoa(episode)
	var response Episode
	if _, err := c.requestJSON(ctx, http.MethodGet, path, query, nil, &response, options...); err != nil {
		return nil, err
	}
	return &response, nil
}

func (c *Client) TrendingMovies(ctx context.Context, input PageRequest, options ...socialhub.CallOption) (socialhub.Page[MovieTrend], error) {
	return listCatalog[MovieTrend](ctx, c, "/movies/trending", input, options...)
}

func (c *Client) PopularMovies(ctx context.Context, input PageRequest, options ...socialhub.CallOption) (socialhub.Page[Movie], error) {
	return listCatalog[Movie](ctx, c, "/movies/popular", input, options...)
}

func (c *Client) TrendingShows(ctx context.Context, input PageRequest, options ...socialhub.CallOption) (socialhub.Page[ShowTrend], error) {
	return listCatalog[ShowTrend](ctx, c, "/shows/trending", input, options...)
}

func (c *Client) PopularShows(ctx context.Context, input PageRequest, options ...socialhub.CallOption) (socialhub.Page[Show], error) {
	return listCatalog[Show](ctx, c, "/shows/popular", input, options...)
}

func listCatalog[T any](ctx context.Context, client *Client, path string, input PageRequest, options ...socialhub.CallOption) (socialhub.Page[T], error) {
	page, err := validatePage(input.Cursor, input.MaxResults)
	if err != nil || !validExtended(input.Extended) {
		if err != nil {
			return socialhub.Page[T]{}, err
		}
		return socialhub.Page[T]{}, invalidArgument(path, "extended mode is invalid")
	}
	query := url.Values{}
	setPage(query, page, input.MaxResults)
	setExtended(query, input.Extended)
	var response []T
	metadata, err := client.requestJSON(ctx, http.MethodGet, path, query, nil, &response, options...)
	if err != nil {
		return socialhub.Page[T]{}, err
	}
	return pageFromMetadata(response, page, metadata), nil
}

func validSearchType(value MediaType) bool {
	switch value {
	case MediaMovie, MediaShow, MediaEpisode, MediaPerson, MediaList:
		return true
	default:
		return false
	}
}

func validExtended(value string) bool {
	return value == "" || value == "min" || value == "full"
}

func setExtended(query url.Values, value string) {
	if value != "" {
		query.Set("extended", value)
	}
}
