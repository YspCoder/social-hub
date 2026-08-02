package mixcloud

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"social-hub/pkg/socialhub"
)

func (client *Client) SearchCloudcasts(ctx context.Context, input SearchRequest, options ...socialhub.CallOption) (*CloudcastPage, error) {
	query, err := searchQuery(input, "cloudcast")
	if err != nil {
		return nil, err
	}
	var response CloudcastPage
	if err := client.request(ctx, http.MethodGet, "/search/", query, nil, "", &response, options...); err != nil {
		return nil, err
	}
	for _, item := range response.Data {
		if _, err := client.mapCloudcast(item); err != nil {
			return nil, err
		}
	}
	if _, _, err := pageCursors(response.Paging, client.apiBaseURL); err != nil {
		return nil, platformError("search_cloudcasts", socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	response.Paging, err = sanitizedPaging(response.Paging, client.apiBaseURL)
	if err != nil {
		return nil, platformError("search_cloudcasts", socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	return &response, nil
}

func (client *Client) SearchUsers(ctx context.Context, input SearchRequest, options ...socialhub.CallOption) (*UserPage, error) {
	query, err := searchQuery(input, "user")
	if err != nil {
		return nil, err
	}
	var response UserPage
	if err := client.request(ctx, http.MethodGet, "/search/", query, nil, "", &response, options...); err != nil {
		return nil, err
	}
	for _, item := range response.Data {
		if _, err := client.mapUser(item); err != nil {
			return nil, err
		}
	}
	if _, _, err := pageCursors(response.Paging, client.apiBaseURL); err != nil {
		return nil, platformError("search_users", socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	response.Paging, err = sanitizedPaging(response.Paging, client.apiBaseURL)
	if err != nil {
		return nil, platformError("search_users", socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	return &response, nil
}

func (client *Client) SearchTags(ctx context.Context, input SearchRequest, options ...socialhub.CallOption) (*TagPage, error) {
	query, err := searchQuery(input, "tag")
	if err != nil {
		return nil, err
	}
	var response TagPage
	if err := client.request(ctx, http.MethodGet, "/search/", query, nil, "", &response, options...); err != nil {
		return nil, err
	}
	for _, item := range response.Data {
		if !validTag(item) {
			return nil, platformError("search_tags", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
		}
	}
	if _, _, err := pageCursors(response.Paging, client.apiBaseURL); err != nil {
		return nil, platformError("search_tags", socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	response.Paging, err = sanitizedPaging(response.Paging, client.apiBaseURL)
	if err != nil {
		return nil, platformError("search_tags", socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	return &response, nil
}

func searchQuery(input SearchRequest, resourceType string) (url.Values, error) {
	if !validText(input.Query, false, 512) {
		return nil, invalidArgument("search_"+resourceType, "search query is invalid")
	}
	offset, ok := parseOffset(input.Cursor)
	if !ok || input.MaxResults < 0 || input.MaxResults > 100 {
		return nil, invalidArgument("search_"+resourceType, "cursor or max_results is invalid")
	}
	limit := input.MaxResults
	if limit == 0 {
		limit = 20
	}
	return url.Values{
		"q": {input.Query}, "type": {resourceType}, "limit": {strconv.Itoa(limit)}, "offset": {strconv.Itoa(offset)},
	}, nil
}

func validTag(tag Tag) bool {
	return validText(tag.Name, false, 512) && strings.HasPrefix(tag.Key, "/genres/") && strings.HasSuffix(tag.Key, "/") && len(tag.Key) <= 1024
}
