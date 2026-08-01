package flickr

import (
	"context"
	"net/http"
	"net/url"
	"strconv"

	"social-hub/pkg/socialhub"
)

const photoListExtras = "description,date_upload,last_update,owner_name,tags,views,media,o_dims,url_m,url_c,url_l,url_o"

func (c *Client) GetUser(ctx context.Context, userID string, options ...socialhub.CallOption) (*socialhub.User, error) {
	if !validResourceID(userID) {
		return nil, invalidArgument("get_user", "a valid Flickr NSID is required")
	}
	var response personResponse
	if err := c.call(ctx, http.MethodGet, "flickr.people.getInfo", url.Values{"user_id": {userID}}, c.signed != nil, &response, options...); err != nil {
		return nil, err
	}
	if response.Person.NSID != userID {
		return nil, platformError("get_user", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return c.mapPerson(response.Person)
}

func (c *Client) GetPhoto(ctx context.Context, photoID string, options ...socialhub.CallOption) (*Photo, error) {
	if !validResourceID(photoID) {
		return nil, invalidArgument("get_photo", "a valid photo ID is required")
	}
	var response photoResponse
	if err := c.call(ctx, http.MethodGet, "flickr.photos.getInfo", url.Values{"photo_id": {photoID}}, c.signed != nil, &response, options...); err != nil {
		return nil, err
	}
	if response.Photo.ID != photoID {
		return nil, platformError("get_photo", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return &response.Photo, nil
}

func (c *Client) GetPost(ctx context.Context, postID string, options ...socialhub.CallOption) (*socialhub.Post, error) {
	photo, err := c.GetPhoto(ctx, postID, options...)
	if err != nil {
		return nil, err
	}
	return c.mapPhoto(*photo)
}

func (c *Client) ListPhotos(ctx context.Context, input PhotoListRequest, options ...socialhub.CallOption) (socialhub.Page[PhotoSummary], error) {
	if err := validatePhotoList(input); err != nil {
		return socialhub.Page[PhotoSummary]{}, err
	}
	userID := input.UserID
	if userID == "" {
		userID = c.userID
	}
	if !validResourceID(userID) {
		return socialhub.Page[PhotoSummary]{}, invalidArgument("list_photos", "a user ID is required in the request or account settings")
	}
	values, err := pageValues("list_photos", input.Cursor, input.MaxResults)
	if err != nil {
		return socialhub.Page[PhotoSummary]{}, err
	}
	values.Set("user_id", userID)
	values.Set("extras", photoListExtras)
	if input.StartTime != nil {
		values.Set("min_upload_date", strconv.FormatInt(input.StartTime.Unix(), 10))
	}
	if input.EndTime != nil {
		values.Set("max_upload_date", strconv.FormatInt(input.EndTime.Unix(), 10))
	}
	if input.SafeSearch > 0 {
		values.Set("safe_search", strconv.Itoa(input.SafeSearch))
	}
	if input.Privacy > 0 {
		values.Set("privacy_filter", strconv.Itoa(input.Privacy))
	}
	method, authenticated := "flickr.people.getPublicPhotos", false
	if c.signed != nil {
		method, authenticated = "flickr.people.getPhotos", true
	}
	var response photosResponse
	if err := c.call(ctx, http.MethodGet, method, values, authenticated, &response, options...); err != nil {
		return socialhub.Page[PhotoSummary]{}, err
	}
	next, previous, hasMore, err := pageCursors(response.Photos.Page, response.Photos.Pages)
	if err != nil {
		return socialhub.Page[PhotoSummary]{}, err
	}
	return socialhub.Page[PhotoSummary]{Items: response.Photos.Photos, NextCursor: next, PrevCursor: previous, HasMore: hasMore}, nil
}

func (c *Client) ListPosts(ctx context.Context, input socialhub.ListPostsRequest, options ...socialhub.CallOption) (socialhub.Page[socialhub.Post], error) {
	photos, err := c.ListPhotos(ctx, PhotoListRequest{UserID: input.UserID, Cursor: input.Cursor, MaxResults: input.MaxResults, StartTime: input.StartTime, EndTime: input.EndTime}, options...)
	if err != nil {
		return socialhub.Page[socialhub.Post]{}, err
	}
	items := make([]socialhub.Post, 0, len(photos.Items))
	for _, photo := range photos.Items {
		post, err := c.mapPhotoSummary(photo)
		if err != nil {
			return socialhub.Page[socialhub.Post]{}, err
		}
		items = append(items, *post)
	}
	return socialhub.Page[socialhub.Post]{Items: items, NextCursor: photos.NextCursor, PrevCursor: photos.PrevCursor, HasMore: photos.HasMore}, nil
}

func (c *Client) UpdatePhoto(ctx context.Context, photoID string, input UpdatePhotoRequest, options ...socialhub.CallOption) error {
	if err := c.requirePermission("update_photo", PermissionWrite); err != nil {
		return err
	}
	if !validResourceID(photoID) {
		return invalidArgument("update_photo", "a valid photo ID is required")
	}
	if err := validateUpdatePhoto(input); err != nil {
		return err
	}
	values := url.Values{"photo_id": {photoID}}
	if input.Title != nil {
		values.Set("title", *input.Title)
	}
	if input.Description != nil {
		values.Set("description", *input.Description)
	}
	return c.call(ctx, http.MethodPost, "flickr.photos.setMeta", values, true, nil, options...)
}

func (c *Client) DeletePhoto(ctx context.Context, photoID string, options ...socialhub.CallOption) error {
	if err := c.requirePermission("delete_photo", PermissionDelete); err != nil {
		return err
	}
	if !validResourceID(photoID) {
		return invalidArgument("delete_photo", "a valid photo ID is required")
	}
	return c.call(ctx, http.MethodPost, "flickr.photos.delete", url.Values{"photo_id": {photoID}}, true, nil, options...)
}

var _ PhotoWorkflow = (*Client)(nil)
