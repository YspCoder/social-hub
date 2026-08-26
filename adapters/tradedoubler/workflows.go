package tradedoubler

import (
	"context"
	"strconv"

	"social-hub/pkg/socialhub"
)

func (client *Client) SearchProducts(
	ctx context.Context,
	input SearchProductsRequest,
	options ...socialhub.CallOption,
) (ProductsResponse, error) {
	const operation = "search_products"
	if !validSearchProducts(input) {
		return ProductsResponse{}, invalidArgument(operation, "feed IDs, filters, ordering, or pagination are invalid")
	}
	path, err := buildMatrixPath("1.0/products.json",
		matrixParameter{name: "fid", values: int64Strings(input.FeedIDs)},
		optionalMatrixParameter("q", input.Keyword),
		optionalMatrixParameter("currency", input.Currency),
		boolMatrixParameter("sourceproducturl", input.IncludeSourceProductURL),
		optionalMatrixParameter("minPrice", input.MinPrice),
		optionalMatrixParameter("maxPrice", input.MaxPrice),
		optionalMatrixParameter("minUpdateDate", input.MinUpdateDate),
		optionalMatrixParameter("maxUpdateDate", input.MaxUpdateDate),
		matrixParameter{name: "tdCategoryId", values: int64Strings(input.TDCategoryIDs)},
		matrixParameter{name: "brand", values: input.Brands},
		optionalMatrixParameter("language", input.Language),
		optionalMatrixParameter("orderBy", string(input.OrderBy)),
		optionalPositiveIntParameter("page", input.Page),
		optionalPositiveIntParameter("pageSize", input.PageSize),
		optionalPositiveIntParameter("limit", input.Limit),
		boolMatrixParameter("groupOffersByProduct", input.GroupOffersByProduct),
		boolMatrixParameter("priceHistory", input.IncludePriceHistory),
		optionalMatrixParameter("dateOutputFormat", string(input.DateOutputFormat)),
	)
	if err != nil {
		return ProductsResponse{}, invalidArgument(operation, err.Error())
	}
	var output ProductsResponse
	metadata, raw, err := client.getJSON(ctx, operation, path, &output, options...)
	output.Meta, output.Raw = metadata, raw
	if err != nil {
		return output, err
	}
	if err := validateProductsResponse(operation, output, input); err != nil {
		output.Raw = client.errorRaw(raw)
		return output, withHTTPStatus(err, 200)
	}
	return output, nil
}

func (client *Client) ListProductFeeds(
	ctx context.Context,
	input ListProductFeedsRequest,
	options ...socialhub.CallOption,
) (ProductFeedsResponse, error) {
	const operation = "list_product_feeds"
	if !validPositiveIDs(input.ProgramIDs, false) {
		return ProductFeedsResponse{}, invalidArgument(operation, "program IDs must be unique positive integers")
	}
	path, err := buildMatrixPath("1.0/productFeeds.json",
		matrixParameter{name: "programId", values: int64Strings(input.ProgramIDs)},
	)
	if err != nil {
		return ProductFeedsResponse{}, invalidArgument(operation, err.Error())
	}
	var output ProductFeedsResponse
	metadata, raw, err := client.getJSON(ctx, operation, path, &output, options...)
	output.Meta, output.Raw = metadata, raw
	if err != nil {
		return output, err
	}
	if err := validateProductFeedsResponse(operation, output, input); err != nil {
		output.Raw = client.errorRaw(raw)
		return output, withHTTPStatus(err, 200)
	}
	return output, nil
}

func (client *Client) GetUnlimitedFeedLastUpdated(
	ctx context.Context,
	input GetUnlimitedFeedLastUpdatedRequest,
	options ...socialhub.CallOption,
) (UnlimitedFeedLastUpdatedResponse, error) {
	const operation = "get_unlimited_feed_last_updated"
	if input.FeedID <= 0 {
		return UnlimitedFeedLastUpdatedResponse{}, invalidArgument(operation, "feed ID must be a positive integer")
	}
	path, err := buildMatrixPath("1.0/productsUnlimited/lastUpdated.json",
		matrixParameter{name: "fid", values: []string{strconv.FormatInt(input.FeedID, 10)}},
	)
	if err != nil {
		return UnlimitedFeedLastUpdatedResponse{}, invalidArgument(operation, err.Error())
	}
	var output UnlimitedFeedLastUpdatedResponse
	metadata, raw, err := client.getJSON(ctx, operation, path, &output, options...)
	output.Meta, output.Raw = metadata, raw
	if err != nil {
		return output, err
	}
	if err := validateUnlimitedFeedLastUpdatedResponse(operation, output, input.FeedID); err != nil {
		output.Raw = client.errorRaw(raw)
		return output, withHTTPStatus(err, 200)
	}
	return output, nil
}

func optionalMatrixParameter(name, value string) matrixParameter {
	if value == "" {
		return matrixParameter{}
	}
	return matrixParameter{name: name, values: []string{value}}
}

func boolMatrixParameter(name string, value *bool) matrixParameter {
	if value == nil {
		return matrixParameter{}
	}
	return matrixParameter{name: name, values: []string{strconv.FormatBool(*value)}}
}

func optionalPositiveIntParameter(name string, value int) matrixParameter {
	if value <= 0 {
		return matrixParameter{}
	}
	return matrixParameter{name: name, values: []string{strconv.Itoa(value)}}
}

func int64Strings(values []int64) []string {
	encoded := make([]string, len(values))
	for index, value := range values {
		encoded[index] = strconv.FormatInt(value, 10)
	}
	return encoded
}

var _ ProductsWorkflow = (*Client)(nil)
