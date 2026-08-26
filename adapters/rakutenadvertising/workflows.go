package rakutenadvertising

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"social-hub/pkg/socialhub"
)

func (client *Client) SearchAdvertisers(
	ctx context.Context,
	input SearchAdvertisersRequest,
	options ...socialhub.CallOption,
) (AdvertisersResponse, error) {
	const operation = "search_advertisers"
	if !validSearchAdvertisers(input) {
		return AdvertisersResponse{}, invalidArgument(operation, "page, limit, shipping country, or network is invalid")
	}
	query := make(url.Values)
	if input.Page > 0 {
		query.Set("page", strconv.Itoa(input.Page))
	}
	if input.Limit > 0 {
		query.Set("limit", strconv.Itoa(input.Limit))
	}
	setOptionalQuery(query, "ships_to", input.ShipsTo)
	if input.DeepLinks != nil {
		query.Set("deep_links", strconv.FormatBool(*input.DeepLinks))
	}
	if input.Network > 0 {
		query.Set("network", strconv.Itoa(input.Network))
	}
	var output AdvertisersResponse
	metadata, raw, err := client.getJSON(ctx, operation, "/v2/advertisers", query, &output, options...)
	output.Meta, output.Raw = metadata, raw
	if err != nil {
		return output, err
	}
	if err := validateAdvertisersResponse(operation, output); err != nil {
		output.Raw = client.errorRaw(output.Raw)
		return output, err
	}
	return output, nil
}

func (client *Client) ListPartnerships(
	ctx context.Context,
	input ListPartnershipsRequest,
	options ...socialhub.CallOption,
) (PartnershipsResponse, error) {
	const operation = "list_partnerships"
	if !validListPartnerships(input) {
		return PartnershipsResponse{}, invalidArgument(operation, "status, network, date range, sort, or pagination is invalid")
	}
	query := make(url.Values)
	setOptionalQuery(query, "partner_status", string(input.PartnerStatus))
	if input.Network > 0 {
		query.Set("network", strconv.Itoa(input.Network))
	}
	setOptionalQuery(query, "advertiser_status", string(input.AdvertiserStatus))
	setOptionalQuery(query, "category", input.Category)
	setOptionalQuery(query, "status_update_range", string(input.StatusUpdateRange))
	setOptionalQuery(query, "approve_date_range", string(input.ApproveDateRange))
	setOptionalQuery(query, "apply_date_range", string(input.ApplyDateRange))
	setOptionalQuery(query, "sort_by", string(input.SortBy))
	setOptionalQuery(query, "order_by", string(input.OrderBy))
	if input.Limit > 0 {
		query.Set("limit", strconv.Itoa(input.Limit))
	}
	if input.Page > 0 {
		query.Set("page", strconv.Itoa(input.Page))
	}
	var output PartnershipsResponse
	metadata, raw, err := client.getJSON(ctx, operation, "/v1/partnerships", query, &output, options...)
	output.Meta, output.Raw = metadata, raw
	if err != nil {
		return output, err
	}
	if err := validatePartnershipsResponse(operation, output); err != nil {
		output.Raw = client.errorRaw(output.Raw)
		return output, err
	}
	return output, nil
}

func (client *Client) SearchProducts(
	ctx context.Context,
	input SearchProductsRequest,
	options ...socialhub.CallOption,
) (ProductSearchResponse, error) {
	const operation = "search_products"
	if !validSearchProducts(input) {
		return ProductSearchResponse{}, invalidArgument(operation, "search terms, language, pagination, advertiser, or sort is invalid")
	}
	query := make(url.Values)
	setOptionalQuery(query, "keyword", input.Keyword)
	setOptionalQuery(query, "exact", input.Exact)
	setOptionalQuery(query, "one", input.One)
	setOptionalQuery(query, "none", input.None)
	setOptionalQuery(query, "cat", input.Category)
	setOptionalQuery(query, "language", string(input.Language))
	if input.Max > 0 {
		query.Set("max", strconv.Itoa(input.Max))
	}
	if input.PageNumber > 0 {
		query.Set("pagenumber", strconv.Itoa(input.PageNumber))
	}
	if input.AdvertiserID > 0 {
		query.Set("mid", strconv.FormatInt(input.AdvertiserID, 10))
	}
	for _, sort := range input.Sort {
		query.Add("sort", string(sort.Field))
		query.Add("sorttype", string(sort.Direction))
	}
	var output ProductSearchResponse
	metadata, raw, err := client.getXML(ctx, operation, "/productsearch/1.0", query, &output, options...)
	output.Meta, output.Raw = metadata, raw
	if err != nil {
		return output, err
	}
	if err := validateProductSearchResponse(operation, output); err != nil {
		output.Raw = client.errorRaw(output.Raw)
		return output, err
	}
	return output, nil
}

func (client *Client) CreateDeepLink(
	ctx context.Context,
	input CreateDeepLinkRequest,
	options ...socialhub.CallOption,
) (DeepLinkResponse, error) {
	const operation = "create_deep_link"
	if !validCreateDeepLink(input) {
		return DeepLinkResponse{}, invalidArgument(operation, "advertiser ID, destination URL, or u1 value is invalid")
	}
	var output DeepLinkResponse
	metadata, raw, err := client.postJSON(ctx, operation, "/v1/links/deep_links", input, &output, options...)
	output.Meta, output.Raw = metadata, raw
	if err != nil {
		return output, withMutationOutcome(operation, metadata.RequestID, err)
	}
	if provider := decodeProviderError(raw); provider.Code != "" || provider.Message != "" {
		output.Raw = client.errorRaw(output.Raw)
		return output, withOperation(client.decodeResponseError(http.StatusOK, nil, raw), operation)
	}
	if err := validateDeepLinkResponse(operation, output, input.AdvertiserID); err != nil {
		output.Raw = client.errorRaw(output.Raw)
		return output, withMutationOutcome(operation, metadata.RequestID, withHTTPStatus(err, http.StatusOK))
	}
	return output, nil
}

func (client *Client) ListTransactions(
	ctx context.Context,
	input ListTransactionsRequest,
	options ...socialhub.CallOption,
) (TransactionsResponse, error) {
	const operation = "list_transactions"
	if !validListTransactions(input, client.clock.Now()) {
		return TransactionsResponse{}, invalidArgument(operation, "date pair, 30-day window, history, pagination, currency, or transaction type is invalid")
	}
	query := make(url.Values)
	setOptionalAPITime(query, "process_date_start", input.ProcessDateStart)
	setOptionalAPITime(query, "process_date_end", input.ProcessDateEnd)
	setOptionalAPITime(query, "transaction_date_start", input.TransactionDateStart)
	setOptionalAPITime(query, "transaction_date_end", input.TransactionDateEnd)
	if input.Limit > 0 {
		query.Set("limit", strconv.Itoa(input.Limit))
	}
	if input.Page > 0 {
		query.Set("page", strconv.Itoa(input.Page))
	}
	setOptionalQuery(query, "currency", string(input.Currency))
	setOptionalQuery(query, "type", string(input.Type))
	var output TransactionsResponse
	metadata, raw, err := client.getJSON(ctx, operation, "/events/1.0/transactions", query, &output, options...)
	output.Meta, output.Raw = metadata, raw
	if err != nil {
		return output, err
	}
	if err := validateTransactionsResponse(operation, output, client.publisherID); err != nil {
		output.Raw = client.errorRaw(output.Raw)
		return output, err
	}
	return output, nil
}

func setOptionalQuery(query url.Values, key, value string) {
	if value != "" {
		query.Set(key, value)
	}
}

func setOptionalAPITime(query url.Values, key string, value time.Time) {
	if !value.IsZero() {
		query.Set(key, value.UTC().Format("2006-01-02 15:04:05"))
	}
}

var _ PublisherWorkflow = (*Client)(nil)
