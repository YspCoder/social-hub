package tenor

import (
	"context"
	"net/url"
	"strconv"
	"strings"

	"social-hub/pkg/socialhub"
)

func (client *Client) Search(ctx context.Context, input SearchRequest, options ...socialhub.CallOption) (Page, error) {
	const operation = "search"
	if err := validateSearch(input); err != nil {
		return Page{}, err
	}
	query := discoveryQuery(input.DiscoveryOptions)
	query.Set("q", input.Query)
	setOptional(query, "searchfilter", string(input.Content))
	if input.Random != nil {
		query.Set("random", strconv.FormatBool(*input.Random))
	}
	return client.getPage(ctx, operation, "/search", query, options...)
}

func (client *Client) Featured(ctx context.Context, input FeaturedRequest, options ...socialhub.CallOption) (Page, error) {
	const operation = "featured"
	if err := validateFeatured(input); err != nil {
		return Page{}, err
	}
	query := discoveryQuery(input.DiscoveryOptions)
	setOptional(query, "searchfilter", string(input.Content))
	return client.getPage(ctx, operation, "/featured", query, options...)
}

func (client *Client) Categories(ctx context.Context, input CategoriesRequest, options ...socialhub.CallOption) (CategoriesResponse, error) {
	const operation = "categories"
	if err := validateCategories(input); err != nil {
		return CategoriesResponse{}, err
	}
	query := make(url.Values)
	setOptional(query, "type", string(input.Type))
	setOptional(query, "country", input.Country)
	setOptional(query, "locale", input.Locale)
	setOptional(query, "contentfilter", string(input.Safety))
	var envelope categoriesEnvelope
	meta, err := client.getJSON(ctx, operation, "/categories", query, &envelope, options...)
	if err != nil {
		return CategoriesResponse{Meta: meta}, err
	}
	for index := range envelope.Tags {
		category := &envelope.Tags[index]
		if !validOpaque(category.SearchTerm, 2048) || !validOptionalOpaque(category.Name, 2048) || !validProviderURL(category.Image) {
			return CategoriesResponse{Meta: meta}, platformContractError(operation, "Tenor returned an invalid category object")
		}
		category.Path, err = sanitizedCategoryPath(category.Path)
		if err != nil {
			return CategoriesResponse{Meta: meta}, platformContractError(operation, "Tenor returned an invalid category search path")
		}
	}
	return CategoriesResponse{Categories: envelope.Tags, Meta: meta}, nil
}

func (client *Client) Posts(ctx context.Context, input PostsRequest, options ...socialhub.CallOption) (PostsResponse, error) {
	const operation = "posts"
	if err := validatePosts(input); err != nil {
		return PostsResponse{}, err
	}
	query := make(url.Values)
	query.Set("ids", strings.Join(input.IDs, ","))
	setMediaFormats(query, input.MediaFormats)
	var envelope postsEnvelope
	meta, err := client.getJSON(ctx, operation, "/posts", query, &envelope, options...)
	if err != nil {
		return PostsResponse{Meta: meta}, err
	}
	if len(envelope.Results) > MaximumPostIDs || !validProviderPosts(envelope.Results) {
		return PostsResponse{Meta: meta}, platformContractError(operation, "Tenor returned an invalid Posts response")
	}
	return PostsResponse{Posts: envelope.Results, Meta: meta}, nil
}

func (client *Client) getPage(
	ctx context.Context,
	operation string,
	path string,
	query url.Values,
	options ...socialhub.CallOption,
) (Page, error) {
	var envelope pageEnvelope
	meta, err := client.getJSON(ctx, operation, path, query, &envelope, options...)
	if err != nil {
		return Page{Meta: meta}, err
	}
	if len(envelope.Results) > MaximumPageSize || !validNextPosition(envelope.Next) || !validProviderPosts(envelope.Results) {
		return Page{Meta: meta}, platformContractError(operation, "Tenor returned an invalid discovery page")
	}
	return Page{Posts: envelope.Results, NextPosition: envelope.Next, Meta: meta}, nil
}

func discoveryQuery(input DiscoveryOptions) url.Values {
	query := make(url.Values)
	setOptional(query, "country", input.Country)
	setOptional(query, "locale", input.Locale)
	setOptional(query, "contentfilter", string(input.Safety))
	setOptional(query, "ar_range", string(input.AspectRatio))
	setOptional(query, "pos", input.NextPosition)
	setMediaFormats(query, input.MediaFormats)
	if input.Limit > 0 {
		query.Set("limit", strconv.Itoa(input.Limit))
	}
	return query
}

func setMediaFormats(query url.Values, formats []MediaFormatName) {
	if len(formats) == 0 {
		return
	}
	values := make([]string, len(formats))
	for index, format := range formats {
		values[index] = string(format)
	}
	query.Set("media_filter", strings.Join(values, ","))
}

func setOptional(query url.Values, key, value string) {
	if value != "" {
		query.Set(key, value)
	}
}

func validProviderPosts(posts []Post) bool {
	for _, post := range posts {
		if !validProviderPost(post) {
			return false
		}
	}
	return true
}

func sanitizedCategoryPath(value string) (string, error) {
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme != "https" || !strings.EqualFold(parsed.Host, "tenor.googleapis.com") ||
		parsed.User != nil || parsed.Path != "/v2/search" || parsed.Fragment != "" {
		return "", errInvalidCategoryPath
	}
	query := parsed.Query()
	query.Del("key")
	query.Del("client_key")
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}
