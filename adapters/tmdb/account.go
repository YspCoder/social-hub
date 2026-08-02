package tmdb

import (
	"context"
	"net/http"
	"strconv"

	"social-hub/pkg/socialhub"
)

func (c *Client) GetAccount(ctx context.Context, options ...socialhub.CallOption) (*Account, error) {
	query, err := c.accountQuery("get_account")
	if err != nil {
		return nil, err
	}
	var response Account
	path := "/account/" + strconv.FormatInt(c.tmdbAccountID, 10)
	if err := c.requestJSON(ctx, http.MethodGet, path, query, nil, &response, options...); err != nil {
		return nil, err
	}
	return &response, nil
}

func (c *Client) ListLibrary(ctx context.Context, input LibraryRequest, options ...socialhub.CallOption) (socialhub.Page[MediaItem], error) {
	query, err := c.accountQuery("list_library")
	if err != nil {
		return socialhub.Page[MediaItem]{}, err
	}
	page, err := validatePage(input.Cursor)
	if err != nil || !validLibraryRequest(input) {
		if err != nil {
			return socialhub.Page[MediaItem]{}, err
		}
		return socialhub.Page[MediaItem]{}, invalidArgument("list_library", "library kind, media type, language, or sort is invalid")
	}
	setPageAndLanguage(query, page, input.Language)
	if input.Sort != "" {
		query.Set("sort_by", input.Sort)
	}
	path := "/account/" + strconv.FormatInt(c.tmdbAccountID, 10) + "/" + string(input.Kind) + "/" + mediaPlural(input.MediaType)
	var response pageEnvelope[MediaItem]
	if err := c.requestJSON(ctx, http.MethodGet, path, query, nil, &response, options...); err != nil {
		return socialhub.Page[MediaItem]{}, err
	}
	setMediaType(response.Results, input.MediaType)
	return pageFromEnvelope(response)
}

func validLibraryRequest(input LibraryRequest) bool {
	if input.Kind != LibraryFavorites && input.Kind != LibraryWatchlist && input.Kind != LibraryRated {
		return false
	}
	return validMediaType(input.MediaType, false, false) && validLocale(input.Language) && validSort(input.Sort)
}

func mediaPlural(mediaType MediaType) string {
	if mediaType == MediaMovie {
		return "movies"
	}
	return "tv"
}
