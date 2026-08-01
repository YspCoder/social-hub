package imgur

import (
	"context"
	"net/http"
	"net/url"

	"social-hub/pkg/socialhub"
)

// GetImage returns public metadata for one image.
func (client *Client) GetImage(ctx context.Context, imageID string, options ...socialhub.CallOption) (*Image, error) {
	if !validIdentifier(imageID) {
		return nil, invalidArgument("get_image", "a valid Imgur image ID is required")
	}
	var image Image
	if err := client.request(ctx, client.public, http.MethodGet, path("image", imageID), nil, &image, options...); err != nil {
		return nil, err
	}
	if image.ID != imageID || !validHTTPURL(image.Link) {
		return nil, platformError("get_image", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return &image, nil
}

// GetPost maps one Imgur image to a common post.
func (client *Client) GetPost(ctx context.Context, postID string, options ...socialhub.CallOption) (*socialhub.Post, error) {
	image, err := client.GetImage(ctx, postID, options...)
	if err != nil {
		return nil, err
	}
	return client.mapImage(*image)
}

// UpdateImage updates image metadata. Anonymous callers must pass a deletehash.
func (client *Client) UpdateImage(ctx context.Context, imageReference string, input ImageUpdateRequest, options ...socialhub.CallOption) error {
	if !validIdentifier(imageReference) || input.Title == nil && input.Description == nil {
		return invalidArgument("update_image", "an image ID/deletehash and at least one metadata field are required")
	}
	if input.Title != nil && !validText(*input.Title, false) || input.Description != nil && !validText(*input.Description, false) {
		return invalidArgument("update_image", "image metadata contains invalid text")
	}
	values := url.Values{}
	if input.Title != nil {
		values.Set("title", *input.Title)
	}
	if input.Description != nil {
		values.Set("description", *input.Description)
	}
	return client.basic(ctx, client.active(), http.MethodPost, path("image", imageReference), values, options...)
}

// DeleteImage deletes an image. Anonymous callers must pass a deletehash.
func (client *Client) DeleteImage(ctx context.Context, imageReference string, options ...socialhub.CallOption) error {
	if !validIdentifier(imageReference) {
		return invalidArgument("delete_image", "a valid image ID or deletehash is required")
	}
	return client.basic(ctx, client.active(), http.MethodDelete, path("image", imageReference), nil, options...)
}

// ToggleFavorite invokes Imgur's toggle-only favorite endpoint.
func (client *Client) ToggleFavorite(ctx context.Context, imageID string, options ...socialhub.CallOption) (string, error) {
	api, err := client.requireUser("toggle_favorite")
	if err != nil {
		return "", err
	}
	if !validIdentifier(imageID) {
		return "", invalidArgument("toggle_favorite", "a valid image ID is required")
	}
	var state string
	if err := client.request(ctx, api, http.MethodPost, path("image", imageID, "favorite"), nil, &state, options...); err != nil {
		return "", err
	}
	if state != "favorited" && state != "unfavorited" {
		return "", platformError("toggle_favorite", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return state, nil
}

var _ ImageWorkflow = (*Client)(nil)
