package imgur

import (
	"context"
	"net/http"
	"net/url"
	"strconv"

	"social-hub/pkg/socialhub"
)

// GetUser returns a public Imgur account profile.
func (client *Client) GetUser(ctx context.Context, userID string, options ...socialhub.CallOption) (*socialhub.User, error) {
	username := firstNonEmpty(userID, client.username)
	api := client.public
	if username == "" && client.user != nil {
		username, api = "me", client.user
	}
	if !validIdentifier(username) {
		return nil, invalidArgument("get_user", "an Imgur username is required")
	}
	var account Account
	if err := client.request(ctx, api, http.MethodGet, path("account", username), nil, &account, options...); err != nil {
		return nil, err
	}
	return client.mapAccount(account)
}

// ListAccountImages returns one zero-based page of images owned by an account.
func (client *Client) ListAccountImages(ctx context.Context, username, cursor string, maxResults int, options ...socialhub.CallOption) (socialhub.Page[Image], error) {
	api, err := client.requireUser("list_account_images")
	if err != nil {
		return socialhub.Page[Image]{}, err
	}
	username = firstNonEmpty(username, client.username, "me")
	if !validIdentifier(username) || maxResults < 0 || maxResults > 100 {
		return socialhub.Page[Image]{}, invalidArgument("list_account_images", "a valid username and max results between 0 and 100 are required")
	}
	pageNumber, err := parsePage(cursor)
	if err != nil {
		return socialhub.Page[Image]{}, err
	}
	limit := maxResults
	if limit == 0 {
		limit = 50
	}
	query := url.Values{"perPage": {strconv.Itoa(limit)}}
	var images []Image
	if err := client.request(ctx, api, http.MethodGet, path("account", username, "images", strconv.Itoa(pageNumber)), query, &images, options...); err != nil {
		return socialhub.Page[Image]{}, err
	}
	page := socialhub.Page[Image]{Items: images}
	if len(images) == limit {
		next := strconv.Itoa(pageNumber + 1)
		page.NextCursor, page.HasMore = &next, true
	}
	if pageNumber > 0 {
		previous := strconv.Itoa(pageNumber - 1)
		page.PrevCursor = &previous
	}
	return page, nil
}

// ListPosts maps the configured account's image page to common posts.
func (client *Client) ListPosts(ctx context.Context, input socialhub.ListPostsRequest, options ...socialhub.CallOption) (socialhub.Page[socialhub.Post], error) {
	if input.StartTime != nil || input.EndTime != nil {
		return socialhub.Page[socialhub.Post]{}, unsupported("list_posts", "Imgur account image pages do not support exact time-range filters")
	}
	images, err := client.ListAccountImages(ctx, input.UserID, input.Cursor, input.MaxResults, options...)
	if err != nil {
		return socialhub.Page[socialhub.Post]{}, err
	}
	items := make([]socialhub.Post, 0, len(images.Items))
	for _, image := range images.Items {
		post, err := client.mapImage(image)
		if err != nil {
			return socialhub.Page[socialhub.Post]{}, err
		}
		items = append(items, *post)
	}
	return socialhub.Page[socialhub.Post]{
		Items: items, NextCursor: images.NextCursor, PrevCursor: images.PrevCursor, HasMore: images.HasMore,
	}, nil
}
