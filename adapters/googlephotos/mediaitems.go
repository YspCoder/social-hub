package googlephotos

import (
	"context"
	"net/http"
	"net/url"

	"social-hub/pkg/socialhub"
)

// ListMediaItems returns media items created by the same OAuth client app.
func (client *Client) ListMediaItems(
	ctx context.Context,
	input ListMediaItemsRequest,
	options ...socialhub.CallOption,
) (*Page[MediaItem], error) {
	const operation = "list_media_items"
	if !validPage(input.Page, 100) {
		return nil, invalidArgument(operation, "page size must be between 0 and 100 and page token must be opaque and bounded")
	}
	query := make(url.Values)
	setPageQuery(query, input.Page)
	meta, raw, err := client.doJSON(ctx, operation, http.MethodGet, "/v1/mediaItems", query, nil, nil, options...)
	if err != nil {
		return nil, err
	}
	return decodePage(operation, "mediaItems", 100, meta, raw, validMediaItem, "media item")
}

// GetMediaItem returns one app-created media item by persistent provider ID.
func (client *Client) GetMediaItem(ctx context.Context, mediaItemID string, options ...socialhub.CallOption) (*MediaItem, ResponseMeta, error) {
	const operation = "get_media_item"
	if !validResourceID(mediaItemID) {
		return nil, ResponseMeta{}, invalidArgument(operation, "media item ID must be a safe bounded path segment")
	}
	var mediaItem MediaItem
	meta, _, err := client.doJSON(ctx, operation, http.MethodGet, "/v1/mediaItems/"+mediaItemID, nil, nil, &mediaItem, options...)
	if err != nil {
		return nil, meta, err
	}
	if !validMediaItem(mediaItem) || mediaItem.ID != mediaItemID {
		return nil, meta, platformContractError(operation, "Google Photos returned an absent or mismatched media item ID")
	}
	return &mediaItem, meta, nil
}

// SearchMediaItems searches app-created media by album or typed filters.
func (client *Client) SearchMediaItems(
	ctx context.Context,
	input SearchMediaItemsRequest,
	options ...socialhub.CallOption,
) (*Page[MediaItem], error) {
	const operation = "search_media_items"
	if !validSearchRequest(input) {
		return nil, invalidArgument(operation, "album, filters, order, page size, or page token violates the Google Photos search contract")
	}
	body := struct {
		AlbumID   string         `json:"albumId,omitempty"`
		Filters   *SearchFilters `json:"filters,omitempty"`
		OrderBy   SearchOrder    `json:"orderBy,omitempty"`
		PageSize  int            `json:"pageSize,omitempty"`
		PageToken string         `json:"pageToken,omitempty"`
	}{
		AlbumID: input.AlbumID, Filters: input.Filters, OrderBy: input.OrderBy,
		PageSize: input.Page.PageSize, PageToken: input.Page.PageToken,
	}
	meta, raw, err := client.doJSON(ctx, operation, http.MethodPost, "/v1/mediaItems:search", nil, body, nil, options...)
	if err != nil {
		return nil, err
	}
	return decodePage(operation, "mediaItems", 100, meta, raw, validMediaItem, "media item")
}

var _ ReadWorkflow = (*Client)(nil)
