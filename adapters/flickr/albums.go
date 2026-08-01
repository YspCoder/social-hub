package flickr

import (
	"context"
	"net/http"
	"net/url"
	"strconv"

	"social-hub/pkg/socialhub"
)

func (c *Client) GetAlbum(ctx context.Context, albumID, ownerID string, options ...socialhub.CallOption) (*Album, error) {
	if ownerID == "" {
		ownerID = c.userID
	}
	if !validResourceID(albumID) || !validResourceID(ownerID) {
		return nil, invalidArgument("get_album", "valid album and owner IDs are required")
	}
	var response albumResponse
	if err := c.call(ctx, http.MethodGet, "flickr.photosets.getInfo", url.Values{"photoset_id": {albumID}, "user_id": {ownerID}}, c.signed != nil, &response, options...); err != nil {
		return nil, err
	}
	if response.Album.ID != albumID {
		return nil, platformError("get_album", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return &response.Album, nil
}

func (c *Client) ListAlbums(ctx context.Context, input AlbumListRequest, options ...socialhub.CallOption) (socialhub.Page[Album], error) {
	userID := input.UserID
	if userID == "" {
		userID = c.userID
	}
	if !validResourceID(userID) {
		return socialhub.Page[Album]{}, invalidArgument("list_albums", "a valid user ID is required")
	}
	values, err := pageValues("list_albums", input.Cursor, input.MaxResults)
	if err != nil {
		return socialhub.Page[Album]{}, err
	}
	values.Set("user_id", userID)
	values.Set("primary_photo_extras", "media,url_m")
	var response albumsResponse
	if err := c.call(ctx, http.MethodGet, "flickr.photosets.getList", values, c.signed != nil, &response, options...); err != nil {
		return socialhub.Page[Album]{}, err
	}
	next, previous, hasMore, err := pageCursors(response.Albums.Page, response.Albums.Pages)
	if err != nil {
		return socialhub.Page[Album]{}, err
	}
	return socialhub.Page[Album]{Items: response.Albums.Albums, NextCursor: next, PrevCursor: previous, HasMore: hasMore}, nil
}

func (c *Client) ListAlbumPhotos(ctx context.Context, input AlbumPhotosRequest, options ...socialhub.CallOption) (socialhub.Page[PhotoSummary], error) {
	ownerID := input.OwnerID
	if ownerID == "" {
		ownerID = c.userID
	}
	if !validResourceID(input.AlbumID) || !validResourceID(ownerID) || input.Privacy < 0 || input.Privacy > 5 || !validAlbumMedia(input.Media) {
		return socialhub.Page[PhotoSummary]{}, invalidArgument("list_album_photos", "album ID, owner ID, privacy, or media filter is invalid")
	}
	values, err := pageValues("list_album_photos", input.Cursor, input.MaxResults)
	if err != nil {
		return socialhub.Page[PhotoSummary]{}, err
	}
	values.Set("photoset_id", input.AlbumID)
	values.Set("user_id", ownerID)
	values.Set("extras", photoListExtras)
	if input.Privacy > 0 {
		values.Set("privacy_filter", strconv.Itoa(input.Privacy))
	}
	if input.Media != "" {
		values.Set("media", input.Media)
	}
	var response albumPhotosResponse
	if err := c.call(ctx, http.MethodGet, "flickr.photosets.getPhotos", values, c.signed != nil, &response, options...); err != nil {
		return socialhub.Page[PhotoSummary]{}, err
	}
	if response.Photos.ID != input.AlbumID {
		return socialhub.Page[PhotoSummary]{}, platformError("list_album_photos", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	next, previous, hasMore, err := pageCursors(response.Photos.Page, response.Photos.Pages)
	if err != nil {
		return socialhub.Page[PhotoSummary]{}, err
	}
	return socialhub.Page[PhotoSummary]{Items: response.Photos.Items, NextCursor: next, PrevCursor: previous, HasMore: hasMore}, nil
}

func (c *Client) CreateAlbum(ctx context.Context, input CreateAlbumRequest, options ...socialhub.CallOption) (*AlbumReference, error) {
	if err := c.requirePermission("create_album", PermissionWrite); err != nil {
		return nil, err
	}
	if !validText(input.Title, 1024) || len(input.Description) > 65_536 || !validResourceID(input.PrimaryPhotoID) {
		return nil, invalidArgument("create_album", "title, description, and primary photo ID are required and must be valid")
	}
	values := url.Values{"title": {input.Title}, "primary_photo_id": {input.PrimaryPhotoID}}
	if input.Description != "" {
		values.Set("description", input.Description)
	}
	var response createAlbumResponse
	if err := c.call(ctx, http.MethodPost, "flickr.photosets.create", values, true, &response, options...); err != nil {
		return nil, err
	}
	if !validResourceID(response.Album.ID) {
		return nil, platformError("create_album", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return &response.Album, nil
}

func (c *Client) AddAlbumPhoto(ctx context.Context, albumID, photoID string, options ...socialhub.CallOption) error {
	if err := c.requirePermission("add_album_photo", PermissionWrite); err != nil {
		return err
	}
	if !validResourceID(albumID) || !validResourceID(photoID) {
		return invalidArgument("add_album_photo", "valid album and photo IDs are required")
	}
	return c.call(ctx, http.MethodPost, "flickr.photosets.addPhoto", url.Values{"photoset_id": {albumID}, "photo_id": {photoID}}, true, nil, options...)
}

func (c *Client) RemoveAlbumPhoto(ctx context.Context, albumID, photoID string, options ...socialhub.CallOption) error {
	if err := c.requirePermission("remove_album_photo", PermissionWrite); err != nil {
		return err
	}
	if !validResourceID(albumID) || !validResourceID(photoID) {
		return invalidArgument("remove_album_photo", "valid album and photo IDs are required")
	}
	return c.call(ctx, http.MethodPost, "flickr.photosets.removePhoto", url.Values{"photoset_id": {albumID}, "photo_id": {photoID}}, true, nil, options...)
}

var _ AlbumWorkflow = (*Client)(nil)
