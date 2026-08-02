package letterboxd

import (
	"context"
	"net/http"
	"net/url"
	"strconv"

	"social-hub/pkg/socialhub"
)

func (c *Client) Search(ctx context.Context, input SearchRequest, options ...socialhub.CallOption) (socialhub.Page[SearchItem], error) {
	if err := c.requireToken("search"); err != nil {
		return socialhub.Page[SearchItem]{}, err
	}
	if !validSearch(input.Input) || !validPage(input.Cursor, input.PerPage) ||
		(input.Method != "" && !containsString(allowedSearchMethods, input.Method)) ||
		!validUniqueValues(input.IncludeTypes, allowedSearchTypes) {
		return socialhub.Page[SearchItem]{}, invalidArgument("search", "input, search method, result types, or pagination is invalid")
	}
	query := pageQuery(input.Cursor, input.PerPage)
	query.Set("input", input.Input)
	if input.Method != "" {
		query.Set("searchMethod", input.Method)
	}
	for _, resultType := range input.IncludeTypes {
		query.Add("include", resultType)
	}
	var response pageEnvelope[SearchItem]
	if err := c.requestJSON(ctx, http.MethodGet, "/search", query, nil, &response, options...); err != nil {
		return socialhub.Page[SearchItem]{}, err
	}
	return toPage(response.Items, response.Next), nil
}

// GetFilm uses Letterboxd's documented legacy endpoint. The endpoint is
// deprecated upstream in favor of /production, whose public contract is not
// yet documented completely enough for this adapter.
func (c *Client) GetFilm(ctx context.Context, id string, options ...socialhub.CallOption) (*Film, error) {
	if err := c.requireToken("get_film"); err != nil {
		return nil, err
	}
	if !validIdentifier(id) {
		return nil, invalidArgument("get_film", "film ID is invalid")
	}
	var response Film
	if err := c.requestJSON(ctx, http.MethodGet, "/film/"+escaped(id), nil, nil, &response, options...); err != nil {
		return nil, err
	}
	return &response, nil
}

func (c *Client) ListFilms(ctx context.Context, input FilmListRequest, options ...socialhub.CallOption) (socialhub.Page[FilmSummary], error) {
	if err := c.requireToken("list_films"); err != nil {
		return socialhub.Page[FilmSummary]{}, err
	}
	query, err := filmListQuery("list_films", input)
	if err != nil {
		return socialhub.Page[FilmSummary]{}, err
	}
	var response pageEnvelope[FilmSummary]
	if err := c.requestJSON(ctx, http.MethodGet, "/films", query, nil, &response, options...); err != nil {
		return socialhub.Page[FilmSummary]{}, err
	}
	return toPage(response.Items, response.Next), nil
}

func filmListQuery(operation string, input FilmListRequest) (url.Values, error) {
	if !validPage(input.Cursor, input.PerPage) || len(input.FilmIDs) > 100 || !validOptionalIdentifier(input.Genre) ||
		!validQueryValue(input.Country, 16) || !validQueryValue(input.Language, 32) || !validDecade(input.Decade) ||
		!validYear(input.Year) || !validQueryValue(input.Sort, 128) {
		return nil, invalidArgument(operation, "film filters or pagination are invalid")
	}
	query := pageQuery(input.Cursor, input.PerPage)
	for _, id := range input.FilmIDs {
		if !validIdentifier(id) {
			return nil, invalidArgument(operation, "film ID is invalid")
		}
		query.Add("filmId", id)
	}
	setOptional(query, "genre", input.Genre)
	setOptional(query, "country", input.Country)
	setOptional(query, "language", input.Language)
	setOptional(query, "sort", input.Sort)
	if input.Decade != 0 {
		query.Set("decade", strconv.Itoa(input.Decade))
	}
	if input.Year != 0 {
		query.Set("year", strconv.Itoa(input.Year))
	}
	return query, nil
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func validOptionalIdentifier(value string) bool { return value == "" || validIdentifier(value) }

func setOptional(values url.Values, key, value string) {
	if value != "" {
		values.Set(key, value)
	}
}
