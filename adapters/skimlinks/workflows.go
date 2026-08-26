package skimlinks

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"social-hub/pkg/socialhub"
)

func (client *Client) ListMerchants(
	ctx context.Context,
	input ListMerchantsRequest,
	options ...socialhub.CallOption,
) (MerchantsResponse, error) {
	const operation = "list_merchants"
	if !validListMerchants(input) {
		return MerchantsResponse{}, invalidArgument(operation, "merchant filters, pagination, alternative vertical, or sort is invalid")
	}
	publisherDomainID := input.PublisherDomainID
	if publisherDomainID == 0 {
		publisherDomainID = client.publisherDomainID
	}
	query := make(url.Values)
	query.Set("publisher_domain_id", strconv.FormatInt(publisherDomainID, 10))
	setOptionalQuery(query, "search", input.Search)
	setOptionalInt64(query, "a_id", input.AdvertiserID)
	setOptionalInt64(query, "merchant_id", input.MerchantID)
	setOptionalInt64(query, "vertical", input.VerticalID)
	setOptionalQuery(query, "country", input.Country)
	if input.FavouritesOnly {
		query.Set("favourite_type", "favourite")
	}
	setOptionalInt(query, "limit", input.Limit)
	if input.Offset > 0 {
		query.Set("offset", strconv.Itoa(input.Offset))
	}
	setOptionalQuery(query, "sort_by", string(input.SortBy))
	setOptionalQuery(query, "sort_dir", string(input.SortDirection))
	setOptionalInt64(query, "alternative_vertical_id", input.AlternativeVerticalID)
	setOptionalQuery(query, "alternative_vertical_taxonomy", input.AlternativeVerticalTaxonomy)
	setOptionalQuery(query, "alternative_vertical_country", input.AlternativeVerticalCountry)
	var output MerchantsResponse
	metadata, raw, err := client.getJSON(
		ctx, client.merchantAPI, operation,
		"/v4/publisher/"+strconv.FormatInt(client.publisherID, 10)+"/merchants",
		query, http.StatusCreated, &output, options...,
	)
	output.Meta, output.Raw = metadata, raw
	if err != nil {
		client.redactMerchantsErrorRaw(&output)
		return output, err
	}
	if err := validateMerchantsResponse(operation, output, input, publisherDomainID); err != nil {
		client.redactMerchantsErrorRaw(&output)
		return output, withHTTPStatus(err, metadata.StatusCode)
	}
	return output, nil
}

func (client *Client) ListDomains(
	ctx context.Context,
	options ...socialhub.CallOption,
) (DomainsResponse, error) {
	const operation = "list_domains"
	var output DomainsResponse
	metadata, raw, err := client.getJSON(
		ctx, client.merchantAPI, operation,
		"/v4/publisher/"+strconv.FormatInt(client.publisherID, 10)+"/domains",
		nil, http.StatusOK, &output, options...,
	)
	output.Meta, output.Raw = metadata, raw
	if err != nil {
		client.redactDomainsErrorRaw(&output)
		return output, err
	}
	if err := validateDomainsResponse(operation, output); err != nil {
		client.redactDomainsErrorRaw(&output)
		return output, withHTTPStatus(err, metadata.StatusCode)
	}
	return output, nil
}

func (client *Client) WrapLink(
	ctx context.Context,
	input WrapLinkRequest,
	options ...socialhub.CallOption,
) (WrappedLink, error) {
	const operation = "wrap_link"
	if !validWrapLink(input) {
		return WrappedLink{}, invalidArgument(operation, "destination URL, source URL, or custom ID is invalid")
	}
	if _, err := prepareCallOptions(operation, options); err != nil {
		return WrappedLink{}, err
	}
	if err := ctx.Err(); err != nil {
		return WrappedLink{}, platformError(operation, socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
	}
	wrapped, err := url.Parse(client.linkBaseURL)
	if err != nil {
		return WrappedLink{}, platformContractError(operation, "configured Link Wrapper endpoint is invalid")
	}
	query := make(url.Values)
	query.Set("id", client.siteID)
	query.Set("url", input.DestinationURL)
	setOptionalQuery(query, "xcust", input.CustomID)
	setOptionalQuery(query, "sref", input.SourceURL)
	wrapped.RawQuery = query.Encode()
	return WrappedLink{URL: wrapped.String()}, nil
}

func (client *Client) ListCommissions(
	ctx context.Context,
	input ListCommissionsRequest,
	options ...socialhub.CallOption,
) (CommissionsResponse, error) {
	const operation = "list_commissions"
	if !validListCommissions(input) {
		return CommissionsResponse{}, invalidArgument(operation, "date selection, filters, pagination, status, type, or sort is invalid")
	}
	query := make(url.Values)
	setOptionalInt(query, "limit", input.Limit)
	if input.Offset > 0 {
		query.Set("offset", strconv.Itoa(input.Offset))
	}
	setOptionalTime(query, "start_date", input.StartDate)
	setOptionalTime(query, "end_date", input.EndDate)
	setOptionalTime(query, "updated_since", input.UpdatedSince)
	setOptionalQuery(query, "custom_id", input.CustomID)
	setOptionalInt64(query, "merchant_id", input.MerchantID)
	setOptionalInt64(query, "a_id", input.AdvertiserID)
	setOptionalInt64(query, "domain_id", input.DomainID)
	setOptionalQuery(query, "sort_dir", string(input.SortDirection))
	setOptionalQuery(query, "sort_by", string(input.SortBy))
	setOptionalQuery(query, "commission_id", input.CommissionID)
	setOptionalQuery(query, "status", string(input.Status))
	setOptionalQuery(query, "commission_type", string(input.CommissionType))
	var output CommissionsResponse
	metadata, raw, err := client.getJSON(
		ctx, client.reportingAPI, operation,
		"/publisher/"+strconv.FormatInt(client.publisherID, 10)+"/commission-report",
		query, http.StatusOK, &output, options...,
	)
	output.Meta, output.Raw = metadata, raw
	if err != nil {
		client.redactCommissionsErrorRaw(&output)
		return output, err
	}
	if err := validateCommissionsResponse(operation, output, input, client.publisherID); err != nil {
		client.redactCommissionsErrorRaw(&output)
		return output, withHTTPStatus(err, metadata.StatusCode)
	}
	return output, nil
}

func (client *Client) GetPerformanceReport(
	ctx context.Context,
	input PerformanceReportRequest,
	options ...socialhub.CallOption,
) (PerformanceReportResponse, error) {
	const operation = "get_performance_report"
	if !validPerformanceReport(input) {
		return PerformanceReportResponse{}, invalidArgument(operation, "report grouping, date range, filters, pagination, currency, timezone, or sort is invalid")
	}
	query := make(url.Values)
	query.Set("report_by", string(input.ReportBy))
	query.Set("start_date", string(input.StartDate))
	query.Set("end_date", string(input.EndDate))
	setOptionalInt(query, "limit", input.Limit)
	if input.Offset > 0 {
		query.Set("offset", strconv.Itoa(input.Offset))
	}
	setOptionalQuery(query, "sort_by", string(input.SortBy))
	setOptionalQuery(query, "sort_dir", string(input.SortDirection))
	setOptionalQuery(query, "time_period", string(input.TimePeriod))
	setOptionalQuery(query, "currency", input.Currency)
	setOptionalInt64(query, "a_id", input.AdvertiserID)
	setOptionalInt64(query, "domain_id", input.DomainID)
	setOptionalQuery(query, "page_search", input.PageSearch)
	setOptionalQuery(query, "link_search", input.LinkSearch)
	setOptionalQuery(query, "merchant_search", input.MerchantSearch)
	if len(input.UserCountries) > 0 {
		countries := make([]string, len(input.UserCountries))
		for index, country := range input.UserCountries {
			countries[index] = strings.ToLower(country)
		}
		query.Set("user_ip_countries", strings.Join(countries, ","))
	}
	setOptionalQuery(query, "payment_type", string(input.PaymentType))
	setOptionalQuery(query, "timezone", input.Timezone)
	var output PerformanceReportResponse
	metadata, raw, err := client.getJSON(
		ctx, client.reportingAPI, operation,
		"/publisher/"+strconv.FormatInt(client.publisherID, 10)+"/reports",
		query, http.StatusOK, &output, options...,
	)
	output.Meta, output.Raw = metadata, raw
	if err != nil {
		client.redactPerformanceErrorRaw(&output)
		return output, err
	}
	if err := validatePerformanceReportResponse(operation, output, input); err != nil {
		client.redactPerformanceErrorRaw(&output)
		return output, withHTTPStatus(err, metadata.StatusCode)
	}
	return output, nil
}

func setOptionalQuery(query url.Values, key, value string) {
	if value != "" {
		query.Set(key, value)
	}
}

func setOptionalInt64(query url.Values, key string, value int64) {
	if value > 0 {
		query.Set(key, strconv.FormatInt(value, 10))
	}
}

func setOptionalInt(query url.Values, key string, value int) {
	if value > 0 {
		query.Set(key, strconv.Itoa(value))
	}
}

func setOptionalTime(query url.Values, key string, value time.Time) {
	if !value.IsZero() {
		query.Set(key, value.UTC().Format(time.RFC3339Nano))
	}
}

func (client *Client) redactMerchantsErrorRaw(response *MerchantsResponse) {
	response.Raw = client.errorRaw(response.Raw)
	for index := range response.Merchants {
		response.Merchants[index].Raw = client.errorRaw(response.Merchants[index].Raw)
	}
}

func (client *Client) redactDomainsErrorRaw(response *DomainsResponse) {
	response.Raw = client.errorRaw(response.Raw)
	for index := range response.Domains {
		response.Domains[index].Raw = client.errorRaw(response.Domains[index].Raw)
	}
}

func (client *Client) redactCommissionsErrorRaw(response *CommissionsResponse) {
	response.Raw = client.errorRaw(response.Raw)
	for index := range response.Commissions {
		response.Commissions[index].Raw = client.errorRaw(response.Commissions[index].Raw)
	}
}

func (client *Client) redactPerformanceErrorRaw(response *PerformanceReportResponse) {
	response.Raw = client.errorRaw(response.Raw)
	for index := range response.Reports {
		response.Reports[index].Raw = client.errorRaw(response.Reports[index].Raw)
	}
	if response.Totals != nil {
		response.Totals.Raw = client.errorRaw(response.Totals.Raw)
	}
}

var _ PublisherWorkflow = (*Client)(nil)
