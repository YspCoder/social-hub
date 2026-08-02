package tmdb

import (
	"context"
	"net/http"
	"net/url"
	"strconv"

	"social-hub/pkg/socialhub"
)

func (c *Client) Search(ctx context.Context, input SearchRequest, options ...socialhub.CallOption) (socialhub.Page[MediaItem], error) {
	page, err := validatePage(input.Cursor)
	if err != nil || !validSearch(input.Query) || !validLocale(input.Language) {
		if err != nil {
			return socialhub.Page[MediaItem]{}, err
		}
		return socialhub.Page[MediaItem]{}, invalidArgument("search", "query or language is invalid")
	}
	query := url.Values{"query": {input.Query}}
	setPageAndLanguage(query, page, input.Language)
	if input.IncludeAdult {
		query.Set("include_adult", "true")
	}
	var response pageEnvelope[MediaItem]
	if err := c.requestJSON(ctx, http.MethodGet, "/search/multi", query, nil, &response, options...); err != nil {
		return socialhub.Page[MediaItem]{}, err
	}
	return pageFromEnvelope(response)
}

func (c *Client) GetMovie(ctx context.Context, id int64, language string, options ...socialhub.CallOption) (*Movie, error) {
	if id <= 0 || !validLocale(language) {
		return nil, invalidArgument("get_movie", "movie ID or language is invalid")
	}
	query := url.Values{}
	if language != "" {
		query.Set("language", language)
	}
	var response Movie
	if err := c.requestJSON(ctx, http.MethodGet, "/movie/"+strconv.FormatInt(id, 10), query, nil, &response, options...); err != nil {
		return nil, err
	}
	return &response, nil
}

func (c *Client) GetTVSeries(ctx context.Context, id int64, language string, options ...socialhub.CallOption) (*TVSeries, error) {
	if id <= 0 || !validLocale(language) {
		return nil, invalidArgument("get_tv", "TV series ID or language is invalid")
	}
	query := url.Values{}
	if language != "" {
		query.Set("language", language)
	}
	var response TVSeries
	if err := c.requestJSON(ctx, http.MethodGet, "/tv/"+strconv.FormatInt(id, 10), query, nil, &response, options...); err != nil {
		return nil, err
	}
	return &response, nil
}

func (c *Client) GetPerson(ctx context.Context, id int64, language string, options ...socialhub.CallOption) (*Person, error) {
	if id <= 0 || !validLocale(language) {
		return nil, invalidArgument("get_person", "person ID or language is invalid")
	}
	query := url.Values{}
	if language != "" {
		query.Set("language", language)
	}
	var response Person
	if err := c.requestJSON(ctx, http.MethodGet, "/person/"+strconv.FormatInt(id, 10), query, nil, &response, options...); err != nil {
		return nil, err
	}
	return &response, nil
}

func (c *Client) Trending(ctx context.Context, input TrendingRequest, options ...socialhub.CallOption) (socialhub.Page[MediaItem], error) {
	page, err := validatePage(input.Cursor)
	if err != nil || !validMediaType(input.MediaType, true, true) || (input.Window != "day" && input.Window != "week") || !validLocale(input.Language) {
		if err != nil {
			return socialhub.Page[MediaItem]{}, err
		}
		return socialhub.Page[MediaItem]{}, invalidArgument("trending", "media type, time window, or language is invalid")
	}
	query := url.Values{}
	setPageAndLanguage(query, page, input.Language)
	var response pageEnvelope[MediaItem]
	path := "/trending/" + string(input.MediaType) + "/" + input.Window
	if err := c.requestJSON(ctx, http.MethodGet, path, query, nil, &response, options...); err != nil {
		return socialhub.Page[MediaItem]{}, err
	}
	return pageFromEnvelope(response)
}

func (c *Client) PopularMovies(ctx context.Context, input PageRequest, options ...socialhub.CallOption) (socialhub.Page[MediaItem], error) {
	return c.popular(ctx, MediaMovie, input, options...)
}

func (c *Client) PopularTV(ctx context.Context, input PageRequest, options ...socialhub.CallOption) (socialhub.Page[MediaItem], error) {
	return c.popular(ctx, MediaTV, input, options...)
}

func (c *Client) popular(ctx context.Context, mediaType MediaType, input PageRequest, options ...socialhub.CallOption) (socialhub.Page[MediaItem], error) {
	page, err := validatePage(input.Cursor)
	if err != nil || !validLocale(input.Language) {
		if err != nil {
			return socialhub.Page[MediaItem]{}, err
		}
		return socialhub.Page[MediaItem]{}, invalidArgument("popular", "language is invalid")
	}
	query := url.Values{}
	setPageAndLanguage(query, page, input.Language)
	var response pageEnvelope[MediaItem]
	if err := c.requestJSON(ctx, http.MethodGet, "/"+string(mediaType)+"/popular", query, nil, &response, options...); err != nil {
		return socialhub.Page[MediaItem]{}, err
	}
	setMediaType(response.Results, mediaType)
	return pageFromEnvelope(response)
}

func (c *Client) GetConfiguration(ctx context.Context, options ...socialhub.CallOption) (*Configuration, error) {
	var response Configuration
	if err := c.requestJSON(ctx, http.MethodGet, "/configuration", nil, nil, &response, options...); err != nil {
		return nil, err
	}
	if !validEndpoint(response.Images.SecureBaseURL) {
		return nil, platformError("configuration", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return &response, nil
}

func setMediaType(items []MediaItem, mediaType MediaType) {
	for index := range items {
		items[index].MediaType = mediaType
	}
}
