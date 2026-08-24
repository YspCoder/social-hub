package ximalaya

import (
	"context"
	"net/url"
	"strconv"

	"social-hub/pkg/socialhub"
)

func (client *Client) ListCategories(ctx context.Context, options ...socialhub.CallOption) (CategoryList, error) {
	const operation = "list_categories"
	var categories []Category
	meta, raw, err := client.getJSON(ctx, operation, "/categories/list", nil, '[', &categories, options...)
	if err != nil {
		return CategoryList{}, err
	}
	if categories == nil {
		return CategoryList{}, platformContractError(operation, "Ximalaya returned a null category list")
	}
	seen := make(map[ID]struct{}, len(categories))
	for _, category := range categories {
		if !validID(category.ID) || category.Kind != "category" || category.Name == "" {
			return CategoryList{}, platformContractError(operation, "Ximalaya returned an invalid category")
		}
		if _, exists := seen[category.ID]; exists {
			return CategoryList{}, platformContractError(operation, "Ximalaya returned a duplicate category ID")
		}
		seen[category.ID] = struct{}{}
	}
	return CategoryList{Categories: categories, Meta: meta, Raw: raw}, nil
}

func (client *Client) ListAlbums(
	ctx context.Context,
	input ListAlbumsRequest,
	options ...socialhub.CallOption,
) (AlbumPage, error) {
	const operation = "list_albums"
	if err := validateListAlbums(input); err != nil {
		return AlbumPage{}, err
	}
	query := url.Values{
		"category_id":    {strconv.FormatInt(int64(input.CategoryID), 10)},
		"calc_dimension": {strconv.Itoa(int(input.Dimension))},
	}
	if input.TagName != "" {
		query.Set("tag_name", input.TagName)
	}
	setPageQuery(query, input.Page, input.Count)
	if input.ContainsPaid != nil {
		query.Set("contains_paid", strconv.FormatBool(*input.ContainsPaid))
	}
	var output AlbumPage
	meta, raw, err := client.getJSON(ctx, operation, "/v2/albums/list", query, '{', &output, options...)
	if err != nil {
		return AlbumPage{}, err
	}
	if err := validateAlbumPage(operation, output, input.Page, input.Count, 50); err != nil {
		return AlbumPage{}, err
	}
	output.Meta, output.Raw = meta, raw
	return output, nil
}

func (client *Client) BrowseAlbum(
	ctx context.Context,
	input BrowseAlbumRequest,
	options ...socialhub.CallOption,
) (AlbumTracksPage, error) {
	const operation = "browse_album"
	if err := validateBrowseAlbum(input); err != nil {
		return AlbumTracksPage{}, err
	}
	query := url.Values{"album_id": {strconv.FormatInt(int64(input.AlbumID), 10)}}
	if input.Sort != "" {
		query.Set("sort", string(input.Sort))
	}
	setPageQuery(query, input.Page, input.Count)
	var output AlbumTracksPage
	meta, raw, err := client.getJSON(ctx, operation, "/albums/browse", query, '{', &output, options...)
	if err != nil {
		return AlbumTracksPage{}, err
	}
	if output.AlbumID != input.AlbumID || output.Tracks == nil ||
		!validPageResponse(output.TotalPages, output.TotalCount, output.CurrentPage, input.Page) ||
		!validPageItems(output.TotalCount, len(output.Tracks), input.Count, 200) {
		return AlbumTracksPage{}, platformContractError(operation, "Ximalaya returned an inconsistent album tracks page")
	}
	if err := validateTracks(operation, output.Tracks); err != nil {
		return AlbumTracksPage{}, err
	}
	output.Meta, output.Raw = meta, raw
	return output, nil
}

func (client *Client) SearchAlbums(
	ctx context.Context,
	input SearchAlbumsRequest,
	options ...socialhub.CallOption,
) (AlbumPage, error) {
	const operation = "search_albums"
	if err := validateSearchAlbums(input); err != nil {
		return AlbumPage{}, err
	}
	query := make(url.Values)
	setIDQuery(query, "id", input.ID)
	setTextQuery(query, "title", input.Title)
	setIDQuery(query, "uid", input.AnnouncerID)
	setTextQuery(query, "nickname", input.Nickname)
	setTextQuery(query, "tags", input.Tags)
	setBooleanIntegerQuery(query, "is_paid", input.Paid)
	if input.PriceType != 0 {
		query.Set("price_type", strconv.Itoa(input.PriceType))
	}
	setIDQuery(query, "category_id", input.CategoryID)
	setTextQuery(query, "category_name", input.CategoryName)
	if input.SortBy != "" {
		query.Set("sort_by", string(input.SortBy))
	}
	if input.Descending != nil {
		query.Set("desc", strconv.FormatBool(*input.Descending))
	}
	setPageQuery(query, input.Page, input.Count)
	var output AlbumPage
	meta, raw, err := client.getJSON(ctx, operation, "/v2/search/albums", query, '{', &output, options...)
	if err != nil {
		return AlbumPage{}, err
	}
	if err := validateAlbumPage(operation, output, input.Page, input.Count, 50); err != nil {
		return AlbumPage{}, err
	}
	output.Meta, output.Raw = meta, raw
	return output, nil
}

func (client *Client) SearchTracks(
	ctx context.Context,
	input SearchTracksRequest,
	options ...socialhub.CallOption,
) (TrackPage, error) {
	const operation = "search_tracks"
	if err := validateSearchTracks(input); err != nil {
		return TrackPage{}, err
	}
	query := make(url.Values)
	setIDQuery(query, "id", input.ID)
	setTextQuery(query, "title", input.Title)
	setIDQuery(query, "album_id", input.AlbumID)
	setTextQuery(query, "album_title", input.AlbumTitle)
	setIDQuery(query, "uid", input.AnnouncerID)
	setTextQuery(query, "nickname", input.Nickname)
	setTextQuery(query, "tags", input.Tags)
	setBooleanIntegerQuery(query, "is_paid", input.Paid)
	setIDQuery(query, "category_id", input.CategoryID)
	setTextQuery(query, "category_name", input.CategoryName)
	if input.SortBy != "" {
		query.Set("sort_by", string(input.SortBy))
	}
	if input.Descending != nil {
		query.Set("desc", strconv.FormatBool(*input.Descending))
	}
	setPageQuery(query, input.Page, input.Count)
	var output TrackPage
	meta, raw, err := client.getJSON(ctx, operation, "/v2/search/tracks", query, '{', &output, options...)
	if err != nil {
		return TrackPage{}, err
	}
	if output.Tracks == nil || !validPageResponse(output.TotalPages, output.TotalCount, output.CurrentPage, input.Page) ||
		!validPageItems(output.TotalCount, len(output.Tracks), input.Count, 50) {
		return TrackPage{}, platformContractError(operation, "Ximalaya returned an inconsistent track search page")
	}
	if err := validateTracks(operation, output.Tracks); err != nil {
		return TrackPage{}, err
	}
	output.Meta, output.Raw = meta, raw
	return output, nil
}

func setPageQuery(query url.Values, page, count int) {
	if page != 0 {
		query.Set("page", strconv.Itoa(page))
	}
	if count != 0 {
		query.Set("count", strconv.Itoa(count))
	}
}

func setIDQuery(query url.Values, key string, value ID) {
	if value != 0 {
		query.Set(key, strconv.FormatInt(int64(value), 10))
	}
}

func setTextQuery(query url.Values, key, value string) {
	if value != "" {
		query.Set(key, value)
	}
}

func setBooleanIntegerQuery(query url.Values, key string, value *bool) {
	if value == nil {
		return
	}
	if *value {
		query.Set(key, "1")
		return
	}
	query.Set(key, "0")
}

func validateAlbumPage(operation string, output AlbumPage, requestedPage, requestedCount, maximumCount int) error {
	if output.Albums == nil || !validPageResponse(output.TotalPages, output.TotalCount, output.CurrentPage, requestedPage) ||
		!validPageItems(output.TotalCount, len(output.Albums), requestedCount, maximumCount) {
		return platformContractError(operation, "Ximalaya returned an inconsistent album page")
	}
	seen := make(map[ID]struct{}, len(output.Albums))
	for _, album := range output.Albums {
		if !validID(album.ID) || (album.Kind != "album" && album.Kind != "paid_album") {
			return platformContractError(operation, "Ximalaya returned an invalid album")
		}
		if _, exists := seen[album.ID]; exists {
			return platformContractError(operation, "Ximalaya returned a duplicate album ID")
		}
		seen[album.ID] = struct{}{}
	}
	return nil
}

func validateTracks(operation string, tracks []Track) error {
	seen := make(map[ID]struct{}, len(tracks))
	for _, track := range tracks {
		if !validID(track.ID) || (track.Kind != "track" && track.Kind != "paid_track") {
			return platformContractError(operation, "Ximalaya returned an invalid track")
		}
		if _, exists := seen[track.ID]; exists {
			return platformContractError(operation, "Ximalaya returned a duplicate track ID")
		}
		seen[track.ID] = struct{}{}
	}
	return nil
}

func validPageResponse(totalPages int, totalCount int64, currentPage, requestedPage int) bool {
	if totalPages < 0 || totalCount < 0 || currentPage < 1 {
		return false
	}
	if totalCount > 0 && totalPages < 1 {
		return false
	}
	return currentPage == effectivePage(requestedPage)
}

func validPageItems(totalCount int64, itemCount, requestedCount, maximumCount int) bool {
	return itemCount <= effectiveCount(requestedCount, 20) && itemCount <= maximumCount && int64(itemCount) <= totalCount
}
