package imgur

import (
	"context"
	"net/http"
	"net/url"

	"social-hub/pkg/socialhub"
)

// GetAlbum returns one public album.
func (client *Client) GetAlbum(ctx context.Context, albumID string, options ...socialhub.CallOption) (*Album, error) {
	if !validIdentifier(albumID) {
		return nil, invalidArgument("get_album", "a valid album ID is required")
	}
	var album Album
	if err := client.request(ctx, client.public, http.MethodGet, path("album", albumID), nil, &album, options...); err != nil {
		return nil, err
	}
	if album.ID != albumID {
		return nil, platformError("get_album", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return &album, nil
}

// ListAlbumImages returns the complete, unpaged image list for one album.
func (client *Client) ListAlbumImages(ctx context.Context, albumID string, options ...socialhub.CallOption) ([]Image, error) {
	if !validIdentifier(albumID) {
		return nil, invalidArgument("list_album_images", "a valid album ID is required")
	}
	var images []Image
	if err := client.request(ctx, client.public, http.MethodGet, path("album", albumID, "images"), nil, &images, options...); err != nil {
		return nil, err
	}
	return images, nil
}

// CreateAlbum creates an account album when Bearer auth exists, otherwise an anonymous album.
func (client *Client) CreateAlbum(ctx context.Context, input AlbumRequest, options ...socialhub.CallOption) (*AlbumReference, error) {
	values, err := albumValues("create_album", input, false)
	if err != nil {
		return nil, err
	}
	var reference AlbumReference
	if err := client.form(ctx, client.active(), http.MethodPost, path("album"), values, &reference, options...); err != nil {
		return nil, err
	}
	if !validIdentifier(reference.ID) || reference.DeleteHash != "" && !validIdentifier(reference.DeleteHash) {
		return nil, platformError("create_album", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return &reference, nil
}

// UpdateAlbum updates an account album by ID or an anonymous album by deletehash.
func (client *Client) UpdateAlbum(ctx context.Context, albumReference string, input AlbumRequest, options ...socialhub.CallOption) error {
	if !validIdentifier(albumReference) {
		return invalidArgument("update_album", "a valid album ID or deletehash is required")
	}
	values, err := albumValues("update_album", input, true)
	if err != nil {
		return err
	}
	return client.basic(ctx, client.active(), http.MethodPut, path("album", albumReference), values, options...)
}

// DeleteAlbum deletes an account album by ID or an anonymous album by deletehash.
func (client *Client) DeleteAlbum(ctx context.Context, albumReference string, options ...socialhub.CallOption) error {
	if !validIdentifier(albumReference) {
		return invalidArgument("delete_album", "a valid album ID or deletehash is required")
	}
	return client.basic(ctx, client.active(), http.MethodDelete, path("album", albumReference), nil, options...)
}

func albumValues(operation string, input AlbumRequest, requireChange bool) (url.Values, error) {
	if requireChange && len(input.ImageIDs) == 0 && len(input.DeleteHashes) == 0 && input.Title == nil && input.Description == nil && input.Cover == nil {
		return nil, invalidArgument(operation, "at least one album field is required")
	}
	values := url.Values{}
	for _, identifier := range input.ImageIDs {
		if !validIdentifier(identifier) {
			return nil, invalidArgument(operation, "album image IDs must be valid identifiers")
		}
		values.Add("ids[]", identifier)
	}
	for _, deleteHash := range input.DeleteHashes {
		if !validIdentifier(deleteHash) {
			return nil, invalidArgument(operation, "album image deletehashes must be valid identifiers")
		}
		values.Add("deletehashes[]", deleteHash)
	}
	if input.Title != nil {
		if !validText(*input.Title, false) {
			return nil, invalidArgument(operation, "album title contains invalid text")
		}
		values.Set("title", *input.Title)
	}
	if input.Description != nil {
		if !validText(*input.Description, false) {
			return nil, invalidArgument(operation, "album description contains invalid text")
		}
		values.Set("description", *input.Description)
	}
	if input.Cover != nil {
		if !validIdentifier(*input.Cover) {
			return nil, invalidArgument(operation, "album cover must be a valid image ID")
		}
		values.Set("cover", *input.Cover)
	}
	return values, nil
}

var _ AlbumWorkflow = (*Client)(nil)
