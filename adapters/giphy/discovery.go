package giphy

import (
	"context"
	"net/url"
	"strconv"
	"strings"

	"social-hub/pkg/socialhub"
)

// Search returns one GIPHY GIF or Sticker search page.
func (client *Client) Search(ctx context.Context, input SearchRequest, options ...socialhub.CallOption) (Page[GIF], error) {
	if !validContent(input.Content) || !validText(input.Query, true, 50) || input.Limit < 0 || input.Limit > 50 || input.Offset < 0 || input.Offset > 4999 || !validLanguage(input.Language) || len(input.ChannelIDs) > 5 {
		return Page[GIF]{}, invalidArgument("search", "content, query, limit, offset, language, or channel IDs are invalid")
	}
	query, err := commonValues("search", input.CommonOptions)
	if err != nil {
		return Page[GIF]{}, err
	}
	query.Set("q", input.Query)
	setPage(query, input.Limit, input.Offset)
	if input.Language != "" {
		query.Set("lang", input.Language)
	}
	if len(input.ChannelIDs) > 0 {
		channels := make([]string, len(input.ChannelIDs))
		for index, channelID := range input.ChannelIDs {
			if channelID <= 0 {
				return Page[GIF]{}, invalidArgument("search", "channel IDs must be positive")
			}
			channels[index] = strconv.FormatInt(channelID, 10)
		}
		query.Set("channel_ids", strings.Join(channels, ","))
	}
	return client.gifPage(ctx, "search", "/"+string(input.Content)+"/search", query, options...)
}

// Trending returns one current GIF or Sticker trending page.
func (client *Client) Trending(ctx context.Context, input TrendingRequest, options ...socialhub.CallOption) (Page[GIF], error) {
	if !validContent(input.Content) || input.Limit < 0 || input.Limit > 50 || input.Offset < 0 || input.Offset > 499 {
		return Page[GIF]{}, invalidArgument("trending", "content, limit, or offset is invalid")
	}
	query, err := commonValues("trending", input.CommonOptions)
	if err != nil {
		return Page[GIF]{}, err
	}
	setPage(query, input.Limit, input.Offset)
	return client.gifPage(ctx, "trending", "/"+string(input.Content)+"/trending", query, options...)
}

// Translate returns the single GIF or Sticker best matching a phrase.
func (client *Client) Translate(ctx context.Context, input TranslateRequest, options ...socialhub.CallOption) (*GIF, error) {
	if !validContent(input.Content) || !validText(input.Query, true, 50) {
		return nil, invalidArgument("translate", "content and a query of at most 50 characters are required")
	}
	query, err := commonValues("translate", input.CommonOptions)
	if err != nil {
		return nil, err
	}
	query.Set("s", input.Query)
	return client.singleGIF(ctx, "translate", "/"+string(input.Content)+"/translate", query, "", options...)
}

// Random returns one random GIF or Sticker, optionally filtered by tag.
func (client *Client) Random(ctx context.Context, input RandomRequest, options ...socialhub.CallOption) (*GIF, error) {
	if !validContent(input.Content) || !validText(input.Tag, false, 50) {
		return nil, invalidArgument("random", "content or tag is invalid")
	}
	query, err := commonValues("random", input.CommonOptions)
	if err != nil {
		return nil, err
	}
	if input.Tag != "" {
		query.Set("tag", input.Tag)
	}
	return client.singleGIF(ctx, "random", "/"+string(input.Content)+"/random", query, "", options...)
}

// Get returns one GIF by ID.
func (client *Client) Get(ctx context.Context, input GetRequest, options ...socialhub.CallOption) (*GIF, error) {
	if !validPathSegment(input.ID) {
		return nil, invalidArgument("get", "a valid GIF ID is required")
	}
	query, err := commonValues("get", input.CommonOptions)
	if err != nil {
		return nil, err
	}
	return client.singleGIF(ctx, "get", "/gifs/"+input.ID, query, input.ID, options...)
}

// GetMany returns up to 100 GIFs by ID.
func (client *Client) GetMany(ctx context.Context, input GetManyRequest, options ...socialhub.CallOption) (Page[GIF], error) {
	if len(input.IDs) == 0 || len(input.IDs) > 100 {
		return Page[GIF]{}, invalidArgument("get_many", "between 1 and 100 GIF IDs are required")
	}
	requested := make(map[string]struct{}, len(input.IDs))
	for _, identifier := range input.IDs {
		if !validPathSegment(identifier) {
			return Page[GIF]{}, invalidArgument("get_many", "GIF IDs contain an invalid value")
		}
		requested[identifier] = struct{}{}
	}
	if len(requested) != len(input.IDs) {
		return Page[GIF]{}, invalidArgument("get_many", "GIF IDs must be unique")
	}
	query, err := commonValues("get_many", input.CommonOptions)
	if err != nil {
		return Page[GIF]{}, err
	}
	query.Set("ids", strings.Join(input.IDs, ","))
	page, err := client.gifPage(ctx, "get_many", "/gifs", query, options...)
	if err != nil {
		return Page[GIF]{}, err
	}
	for _, gif := range page.Items {
		if _, ok := requested[gif.ID]; !ok {
			return Page[GIF]{}, platformError("get_many", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
		}
	}
	return page, nil
}

// RandomID returns a non-personal identifier suitable for customer_id.
func (client *Client) RandomID(ctx context.Context, options ...socialhub.CallOption) (string, error) {
	var response singleEnvelope[struct {
		RandomID string `json:"random_id"`
	}]
	if err := client.get(ctx, "/randomid", nil, &response, options...); err != nil {
		return "", err
	}
	if err := checkMeta("random_id", response.Meta); err != nil {
		return "", err
	}
	if !validOpaque(response.Data.RandomID, 512) {
		return "", platformError("random_id", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return response.Data.RandomID, nil
}

// Categories returns GIPHY's GIF category catalog.
func (client *Client) Categories(ctx context.Context, customerID string, options ...socialhub.CallOption) (Page[Category], error) {
	common := CommonOptions{CustomerID: customerID}
	query, err := commonValues("categories", common)
	if err != nil {
		return Page[Category]{}, err
	}
	var response listEnvelope[Category]
	if err := client.get(ctx, "/gifs/categories", query, &response, options...); err != nil {
		return Page[Category]{}, err
	}
	if err := checkMeta("categories", response.Meta); err != nil {
		return Page[Category]{}, err
	}
	if response.Data == nil {
		return Page[Category]{}, platformError("categories", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	if err := validatePagination("categories", response.Pagination, len(response.Data)); err != nil {
		return Page[Category]{}, err
	}
	return Page[Category]{Items: response.Data, Pagination: response.Pagination, ResponseID: response.Meta.ResponseID}, nil
}

// Autocomplete returns terms completing a partial search query.
func (client *Client) Autocomplete(ctx context.Context, input TermRequest, options ...socialhub.CallOption) ([]Term, error) {
	if !validText(input.Query, true, 50) || input.Limit < 0 || input.Limit > 50 || input.Offset < 0 || input.Offset > 4999 {
		return nil, invalidArgument("autocomplete", "query, limit, or offset is invalid")
	}
	query, err := commonValues("autocomplete", CommonOptions{CustomerID: input.CustomerID})
	if err != nil {
		return nil, err
	}
	query.Set("q", input.Query)
	setPage(query, input.Limit, input.Offset)
	return client.terms(ctx, "autocomplete", "/gifs/search/tags", query, options...)
}

// Related returns search terms related to one tag.
func (client *Client) Related(ctx context.Context, term, customerID string, options ...socialhub.CallOption) ([]Term, error) {
	if !validPathSegment(term) || !validText(term, true, 50) {
		return nil, invalidArgument("related", "a valid related-search term is required")
	}
	query, err := commonValues("related", CommonOptions{CustomerID: customerID})
	if err != nil {
		return nil, err
	}
	return client.terms(ctx, "related", "/tags/related/"+term, query, options...)
}

// TrendingSearches returns current popular GIPHY search terms.
func (client *Client) TrendingSearches(ctx context.Context, customerID string, options ...socialhub.CallOption) ([]string, error) {
	query, err := commonValues("trending_searches", CommonOptions{CustomerID: customerID})
	if err != nil {
		return nil, err
	}
	var response singleEnvelope[[]string]
	if err := client.get(ctx, "/trending/searches", query, &response, options...); err != nil {
		return nil, err
	}
	if err := checkMeta("trending_searches", response.Meta); err != nil {
		return nil, err
	}
	if response.Data == nil {
		return nil, platformError("trending_searches", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	for _, term := range response.Data {
		if !validText(term, true, 200) {
			return nil, platformError("trending_searches", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
		}
	}
	return response.Data, nil
}

func (client *Client) singleGIF(ctx context.Context, operation, path string, query url.Values, expectedID string, options ...socialhub.CallOption) (*GIF, error) {
	var response singleEnvelope[GIF]
	if err := client.get(ctx, path, query, &response, options...); err != nil {
		return nil, err
	}
	if err := checkMeta(operation, response.Meta); err != nil {
		return nil, err
	}
	if err := validateGIF(operation, response.Data); err != nil {
		return nil, err
	}
	if expectedID != "" && response.Data.ID != expectedID {
		return nil, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return &response.Data, nil
}

func (client *Client) gifPage(ctx context.Context, operation, path string, query url.Values, options ...socialhub.CallOption) (Page[GIF], error) {
	var response listEnvelope[GIF]
	if err := client.get(ctx, path, query, &response, options...); err != nil {
		return Page[GIF]{}, err
	}
	if err := checkMeta(operation, response.Meta); err != nil {
		return Page[GIF]{}, err
	}
	if response.Data == nil {
		return Page[GIF]{}, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	if err := validatePagination(operation, response.Pagination, len(response.Data)); err != nil {
		return Page[GIF]{}, err
	}
	for _, gif := range response.Data {
		if err := validateGIF(operation, gif); err != nil {
			return Page[GIF]{}, err
		}
	}
	return Page[GIF]{Items: response.Data, Pagination: response.Pagination, ResponseID: response.Meta.ResponseID}, nil
}

func (client *Client) terms(ctx context.Context, operation, path string, query url.Values, options ...socialhub.CallOption) ([]Term, error) {
	var response singleEnvelope[[]Term]
	if err := client.get(ctx, path, query, &response, options...); err != nil {
		return nil, err
	}
	if err := checkMeta(operation, response.Meta); err != nil {
		return nil, err
	}
	if response.Data == nil {
		return nil, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	for _, term := range response.Data {
		if !validText(term.Name, true, 200) {
			return nil, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
		}
	}
	return response.Data, nil
}

func commonValues(operation string, input CommonOptions) (url.Values, error) {
	if input.CustomerID != "" && !validOpaque(input.CustomerID, 512) || !validRating(input.Rating) || !validCountry(input.CountryCode) || !validRegion(input.Region) || !validBundle(input.Bundle) {
		return nil, invalidArgument(operation, "customer ID, rating, country, region, or bundle is invalid")
	}
	query := url.Values{}
	if input.CustomerID != "" {
		query.Set("customer_id", input.CustomerID)
	}
	if input.Rating != "" {
		query.Set("rating", string(input.Rating))
	}
	if input.CountryCode != "" {
		query.Set("country_code", input.CountryCode)
	}
	if input.Region != "" {
		query.Set("region", input.Region)
	}
	if input.Bundle != "" {
		query.Set("bundle", input.Bundle)
	}
	if input.RemoveLowContrast != nil {
		query.Set("remove_low_contrast", strconv.FormatBool(*input.RemoveLowContrast))
	}
	return query, nil
}

func setPage(query url.Values, limit, offset int) {
	if limit > 0 {
		query.Set("limit", strconv.Itoa(limit))
	}
	if offset > 0 {
		query.Set("offset", strconv.Itoa(offset))
	}
}

func validatePagination(operation string, pagination Pagination, items int) error {
	if pagination.Offset < 0 || pagination.TotalCount < 0 || pagination.Count < 0 || pagination.Count != items {
		return platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return nil
}

func validateGIF(operation string, gif GIF) error {
	if !validPathSegment(gif.ID) || !validHTTPURL(gif.URL) || gif.Type == "" {
		return platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return nil
}

var _ DiscoveryWorkflow = (*Client)(nil)
