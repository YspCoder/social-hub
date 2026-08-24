package unsplash

import (
	"context"
	"net/url"
	"strconv"
	"strings"

	"social-hub/pkg/socialhub"
)

func (client *Client) SearchPhotos(ctx context.Context, input SearchPhotosRequest, options ...socialhub.CallOption) (SearchPhotosResponse, error) {
	const operation = "search_photos"
	if !validSearchRequest(input) {
		return SearchPhotosResponse{}, invalidArgument(operation, "query, pagination, filters, language, or collection IDs are invalid")
	}
	query := pageQuery(input.Page, input.PerPage)
	query.Set("query", input.Query)
	setOptional(query, "order_by", string(input.OrderBy))
	if len(input.Collections) > 0 {
		query.Set("collections", strings.Join(input.Collections, ","))
	}
	setOptional(query, "content_filter", string(input.ContentFilter))
	setOptional(query, "color", string(input.Color))
	setOptional(query, "orientation", string(input.Orientation))
	setOptional(query, "lang", string(input.Language))
	var response SearchPhotosResponse
	meta, _, err := client.getJSON(ctx, operation, "/search/photos", query, &response, options...)
	if err != nil {
		return SearchPhotosResponse{}, err
	}
	if response.Total < 0 || response.TotalPages < 0 || len(response.Results) > effectivePerPage(input.PerPage) ||
		len(response.Results) > response.Total || !client.validPhotos(response.Results) {
		return SearchPhotosResponse{}, platformContractError(operation, "Unsplash returned an invalid photo search response")
	}
	meta = withPagination(meta, input.Page, input.PerPage)
	if meta.NextPage == nil && meta.Page < response.TotalPages {
		next := meta.Page + 1
		meta.NextPage = &next
	}
	response.Meta = meta
	return response, nil
}

func (client *Client) GetPhoto(ctx context.Context, assetSlug string, options ...socialhub.CallOption) (Photo, error) {
	const operation = "get_photo"
	if !validResourceID(assetSlug) {
		return Photo{}, invalidArgument(operation, "photo ID or slug is invalid")
	}
	var response Photo
	meta, _, err := client.getJSON(ctx, operation, "/photos/"+assetSlug, nil, &response, options...)
	if err != nil {
		return Photo{}, err
	}
	if !validatePhoto(response) {
		return Photo{}, platformContractError(operation, "Unsplash returned an invalid photo")
	}
	response.Meta = meta
	return response, nil
}

// TrackDownload records one download-like event using the exact
// photo.links.download_location returned by Unsplash. It validates the JSON
// response but deliberately does not return or fetch the response URL.
func (client *Client) TrackDownload(ctx context.Context, downloadLocation string, options ...socialhub.CallOption) (ResponseMeta, error) {
	path, query, err := parseDownloadLocation(downloadLocation)
	if err != nil {
		return ResponseMeta{}, err
	}
	var response struct {
		URL string `json:"url"`
	}
	meta, _, err := client.getJSON(ctx, "track_download", path, query, &response, options...)
	if err != nil {
		return ResponseMeta{}, err
	}
	if !validHTTPSURL(response.URL) {
		return meta, platformContractError("track_download", "Unsplash returned an invalid tracking response")
	}
	return meta, nil
}

func (client *Client) GetUser(ctx context.Context, username string, options ...socialhub.CallOption) (User, error) {
	const operation = "get_user"
	if !validResourceID(username) {
		return User{}, invalidArgument(operation, "username is invalid")
	}
	var response User
	meta, _, err := client.getJSON(ctx, operation, "/users/"+username, nil, &response, options...)
	if err != nil {
		return User{}, err
	}
	if !validateUser(response) || !strings.EqualFold(response.Username, username) {
		return User{}, platformContractError(operation, "Unsplash returned an absent or mismatched user")
	}
	response.Meta = meta
	return response, nil
}

func (client *Client) ListUserPhotos(ctx context.Context, input ListUserPhotosRequest, options ...socialhub.CallOption) (PhotoPage, error) {
	const operation = "list_user_photos"
	if !validResourceID(input.Username) || !validPage(input.Page, input.PerPage) ||
		!validUserPhotoOrder(input.OrderBy) || !validOrientation(input.Orientation) {
		return PhotoPage{}, invalidArgument(operation, "username, pagination, order, or orientation is invalid")
	}
	query := pageQuery(input.Page, input.PerPage)
	setOptional(query, "order_by", string(input.OrderBy))
	setOptional(query, "orientation", string(input.Orientation))
	return client.photoPage(ctx, operation, "/users/"+input.Username+"/photos", query, input.Page, input.PerPage, options...)
}

func (client *Client) ListUserCollections(ctx context.Context, input ListUserCollectionsRequest, options ...socialhub.CallOption) (CollectionPage, error) {
	const operation = "list_user_collections"
	if !validResourceID(input.Username) || !validPage(input.Page, input.PerPage) {
		return CollectionPage{}, invalidArgument(operation, "username or pagination is invalid")
	}
	return client.collectionPage(ctx, operation, "/users/"+input.Username+"/collections", pageQuery(input.Page, input.PerPage), input.Page, input.PerPage, options...)
}

func (client *Client) ListCollections(ctx context.Context, input PageRequest, options ...socialhub.CallOption) (CollectionPage, error) {
	const operation = "list_collections"
	if !validPage(input.Page, input.PerPage) {
		return CollectionPage{}, invalidArgument(operation, "pagination is invalid")
	}
	return client.collectionPage(ctx, operation, "/collections", pageQuery(input.Page, input.PerPage), input.Page, input.PerPage, options...)
}

func (client *Client) GetCollection(ctx context.Context, collectionID string, options ...socialhub.CallOption) (Collection, error) {
	const operation = "get_collection"
	if !validResourceID(collectionID) {
		return Collection{}, invalidArgument(operation, "collection ID is invalid")
	}
	var response Collection
	meta, _, err := client.getJSON(ctx, operation, "/collections/"+collectionID, nil, &response, options...)
	if err != nil {
		return Collection{}, err
	}
	if !client.validCollection(response) || string(response.ID) != collectionID {
		return Collection{}, platformContractError(operation, "Unsplash returned an absent or mismatched collection")
	}
	response.Meta = meta
	return response, nil
}

func (client *Client) ListCollectionPhotos(ctx context.Context, input ListCollectionPhotosRequest, options ...socialhub.CallOption) (PhotoPage, error) {
	const operation = "list_collection_photos"
	if !validResourceID(input.CollectionID) || !validPage(input.Page, input.PerPage) || !validOrientation(input.Orientation) {
		return PhotoPage{}, invalidArgument(operation, "collection ID, pagination, or orientation is invalid")
	}
	query := pageQuery(input.Page, input.PerPage)
	setOptional(query, "orientation", string(input.Orientation))
	return client.photoPage(ctx, operation, "/collections/"+input.CollectionID+"/photos", query, input.Page, input.PerPage, options...)
}

func (client *Client) photoPage(ctx context.Context, operation, path string, query url.Values, page, perPage int, options ...socialhub.CallOption) (PhotoPage, error) {
	var photos []Photo
	meta, raw, err := client.getJSON(ctx, operation, path, query, &photos, options...)
	if err != nil {
		return PhotoPage{}, err
	}
	if err := decodeProviderArray(raw, &photos); err != nil || len(photos) > effectivePerPage(perPage) || !client.validPhotos(photos) {
		return PhotoPage{}, platformContractError(operation, "Unsplash returned an invalid photo page")
	}
	return PhotoPage{Photos: photos, Meta: withPagination(meta, page, perPage), Raw: raw}, nil
}

func (client *Client) collectionPage(ctx context.Context, operation, path string, query url.Values, page, perPage int, options ...socialhub.CallOption) (CollectionPage, error) {
	var collections []Collection
	meta, raw, err := client.getJSON(ctx, operation, path, query, &collections, options...)
	if err != nil {
		return CollectionPage{}, err
	}
	if err := decodeProviderArray(raw, &collections); err != nil || collections == nil || len(collections) > effectivePerPage(perPage) {
		return CollectionPage{}, platformContractError(operation, "Unsplash returned an invalid collection page")
	}
	seen := make(map[Identifier]struct{}, len(collections))
	for _, collection := range collections {
		if !client.validCollection(collection) {
			return CollectionPage{}, platformContractError(operation, "Unsplash returned an invalid collection page")
		}
		if _, duplicate := seen[collection.ID]; duplicate {
			return CollectionPage{}, platformContractError(operation, "Unsplash returned a duplicate collection ID")
		}
		seen[collection.ID] = struct{}{}
	}
	return CollectionPage{Collections: collections, Meta: withPagination(meta, page, perPage), Raw: raw}, nil
}

func (client *Client) validPhotos(photos []Photo) bool {
	if photos == nil {
		return false
	}
	seen := make(map[string]struct{}, len(photos))
	for _, photo := range photos {
		if !validatePhoto(photo) {
			return false
		}
		if _, duplicate := seen[photo.ID]; duplicate {
			return false
		}
		seen[photo.ID] = struct{}{}
	}
	return true
}

func (client *Client) validCollection(collection Collection) bool {
	if !validResourceID(string(collection.ID)) || collection.TotalPhotos < 0 || collection.User == nil ||
		!validateUser(*collection.User) || !validHTTPSURL(collection.Links.Self) ||
		!validHTTPSURL(collection.Links.HTML) || !validHTTPSURL(collection.Links.Photos) {
		return false
	}
	if collection.Links.Related != "" && !validHTTPSURL(collection.Links.Related) {
		return false
	}
	if collection.CoverPhoto != nil && !validatePhoto(*collection.CoverPhoto) {
		return false
	}
	for _, photo := range collection.PreviewPhotos {
		if !validResourceID(photo.ID) {
			return false
		}
		for _, imageURL := range []string{photo.URLs.Raw, photo.URLs.Full, photo.URLs.Regular, photo.URLs.Small, photo.URLs.Thumb} {
			if !validHTTPSURL(imageURL) {
				return false
			}
		}
	}
	return true
}

func pageQuery(page, perPage int) url.Values {
	query := make(url.Values)
	if page > 0 {
		query.Set("page", strconv.Itoa(page))
	}
	if perPage > 0 {
		query.Set("per_page", strconv.Itoa(perPage))
	}
	return query
}

func setOptional(query url.Values, key, value string) {
	if value != "" {
		query.Set(key, value)
	}
}

func withPagination(meta ResponseMeta, page, perPage int) ResponseMeta {
	meta.Page = effectivePage(page)
	meta.PerPage = effectivePerPage(perPage)
	return meta
}

var _ PhotosWorkflow = (*Client)(nil)
var _ UsersWorkflow = (*Client)(nil)
var _ CollectionsWorkflow = (*Client)(nil)
