package sovrncommerce

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"social-hub/pkg/socialhub"
)

func (client *Client) BuildAffiliateLink(
	ctx context.Context,
	input BuildAffiliateLinkRequest,
	options ...socialhub.CallOption,
) (AffiliateLink, error) {
	const operation = "build_affiliate_link"
	callOptions, err := prepareCallOptions(operation, options)
	if err != nil {
		return AffiliateLink{}, err
	}
	if callOptions.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, callOptions.Timeout)
		defer cancel()
	}
	if !validBuildAffiliateLink(input) {
		return AffiliateLink{}, invalidArgument(operation, "destination, fallback URL, bid floor, CUID, or UTM parameter is invalid")
	}
	if err := ctx.Err(); err != nil {
		return AffiliateLink{}, platformError(operation, socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
	}
	link := &url.URL{Scheme: "https", Host: "sovrn.co"}
	query := link.Query()
	query.Set("key", client.apiKey)
	query.Set("u", input.DestinationURL)
	setOptionalQuery(query, "cuid", input.CUID)
	setOptionalQuery(query, "utm_source", input.UTMSource)
	setOptionalQuery(query, "utm_medium", input.UTMMedium)
	setOptionalQuery(query, "utm_campaign", input.UTMCampaign)
	setOptionalQuery(query, "utm_term", input.UTMTerm)
	setOptionalQuery(query, "utm_content", input.UTMContent)
	if input.BidFloor != nil {
		query.Set("bf", strconv.FormatFloat(*input.BidFloor, 'f', -1, 64))
	}
	setOptionalQuery(query, "fbu", input.FallbackURL)
	link.RawQuery = query.Encode()
	encoded := link.String()
	if len(encoded) > maximumRequestBytes {
		return AffiliateLink{}, invalidArgument(operation, "affiliate link exceeds the adapter's 1 MiB safety limit")
	}
	return AffiliateLink{URL: encoded}, nil
}

func (client *Client) ListTransactions(
	ctx context.Context,
	input ListTransactionsRequest,
	options ...socialhub.CallOption,
) (TransactionsResponse, error) {
	const operation = "list_transactions"
	if !validListTransactions(input) {
		return TransactionsResponse{}, invalidArgument(operation, "at least one date is required and filters must use positive IDs and a documented program type")
	}
	query := make(url.Values)
	setOptionalDate(query, "clickDate", input.ClickDate)
	setOptionalDate(query, "commissionDate", input.CommissionDate)
	setOptionalDate(query, "updateDate", input.UpdateDate)
	setOptionalIDs(query, "campaignIds", input.CampaignIDs)
	setOptionalIDs(query, "merchantGroupIds", input.MerchantGroupIDs)
	setOptionalQuery(query, "programType", string(input.ProgramType))
	var output TransactionsResponse
	metadata, err := client.getJSON(ctx, client.reportsAPI, operation, "/reports/transactions", query, &output, options...)
	if err != nil {
		return TransactionsResponse{}, err
	}
	if output.Transactions == nil {
		return TransactionsResponse{}, platformContractError(operation, "Sovrn transaction response omitted transactions", http.StatusOK)
	}
	output.Meta = metadata
	return output, nil
}

func (client *Client) GetMerchantPerformance(
	ctx context.Context,
	input GetMerchantPerformanceRequest,
	options ...socialhub.CallOption,
) (MerchantPerformanceResponse, error) {
	const operation = "get_merchant_performance"
	if !validMerchantPerformance(input) {
		return MerchantPerformanceResponse{}, invalidArgument(operation, "exclusive date window or merchant performance filters are invalid")
	}
	query := url.Values{
		"clickDateStart": {calendarDate(input.ClickDateStart).Format("2006-01-02")},
		"clickDateEnd":   {calendarDate(input.ClickDateEnd).Format("2006-01-02")},
	}
	setOptionalIDs(query, "campaignIds", input.CampaignIDs)
	setOptionalStrings(query, "subIds", input.SubIDs)
	setOptionalIDs(query, "merchantGroupIds", input.MerchantGroupIDs)
	setOptionalStrings(query, "cuids", input.CUIDs)
	setUTMFilters(query, "page", input.PageUTM)
	setUTMFilters(query, "link", input.LinkUTM)
	setOptionalQuery(query, "programType", string(input.ProgramType))
	setOptionalQuery(query, "sovrnProduct", string(input.SovrnProduct))
	setOptionalQuery(query, "deviceType", string(input.DeviceType))
	setOptionalQuery(query, "country", input.Country)
	var output MerchantPerformanceResponse
	metadata, err := client.getJSON(ctx, client.reportsAPI, operation, "/reports/merchants", query, &output, options...)
	if err != nil {
		return MerchantPerformanceResponse{}, err
	}
	if output.Data == nil {
		return MerchantPerformanceResponse{}, platformContractError(operation, "Sovrn merchant response omitted data", http.StatusOK)
	}
	output.Meta = metadata
	return output, nil
}

func (client *Client) ListApprovedMerchants(
	ctx context.Context,
	input ListApprovedMerchantsRequest,
	options ...socialhub.CallOption,
) (ApprovedMerchantsResponse, error) {
	const operation = "list_approved_merchants"
	if !validListApprovedMerchants(input) {
		return ApprovedMerchantsResponse{}, invalidArgument(operation, "campaign, pagination, or merchant filters are invalid")
	}
	page, pageSize := input.Page, input.PageSize
	if page == 0 {
		page = 1
	}
	if pageSize == 0 {
		pageSize = 1000
	}
	type filter struct {
		Type   string `json:"type"`
		Values any    `json:"values"`
	}
	filters := make([]filter, 0, 6)
	if len(input.Names) > 0 {
		filters = append(filters, filter{Type: "NAME", Values: input.Names})
	}
	if len(input.GroupIDs) > 0 {
		filters = append(filters, filter{Type: "GROUP_ID", Values: input.GroupIDs})
	}
	if len(input.Categories) > 0 {
		filters = append(filters, filter{Type: "CATEGORY", Values: input.Categories})
	}
	if len(input.Geos) > 0 {
		filters = append(filters, filter{Type: "GEO", Values: input.Geos})
	}
	if len(input.ProgramTypes) > 0 {
		filters = append(filters, filter{Type: "PROGRAM_TYPE", Values: input.ProgramTypes})
	}
	if len(input.Domains) > 0 {
		filters = append(filters, filter{Type: "DOMAIN", Values: input.Domains})
	}
	payload := struct {
		Filters  []filter `json:"filters,omitempty"`
		Page     int      `json:"page"`
		PageSize int      `json:"pageSize"`
	}{Filters: filters, Page: page, PageSize: pageSize}
	query := url.Values{"campaignId": {strconv.FormatInt(input.CampaignID, 10)}}
	var output ApprovedMerchantsResponse
	metadata, err := client.postJSON(ctx, client.merchantRatesAPI, operation, "/summaries", query, payload, &output, options...)
	if err != nil {
		return ApprovedMerchantsResponse{}, err
	}
	if output.Results == nil || output.Page <= 0 || output.PerPage <= 0 || output.TotalItems < 0 {
		return ApprovedMerchantsResponse{}, platformContractError(operation, "Sovrn approved-merchant response omitted required pagination fields", http.StatusOK)
	}
	output.Meta = metadata
	return output, nil
}

func setOptionalQuery(query url.Values, key, value string) {
	if value != "" {
		query.Set(key, value)
	}
}

func setOptionalDate(query url.Values, key string, value time.Time) {
	if !value.IsZero() {
		query.Set(key, calendarDate(value).Format("2006-01-02"))
	}
}

func setOptionalIDs(query url.Values, key string, values []int64) {
	if len(values) == 0 {
		return
	}
	encoded := make([]string, len(values))
	for index, value := range values {
		encoded[index] = strconv.FormatInt(value, 10)
	}
	query.Set(key, strings.Join(encoded, ","))
}

func setOptionalStrings(query url.Values, key string, values []string) {
	if len(values) > 0 {
		query.Set(key, strings.Join(values, ","))
	}
}

func setUTMFilters(query url.Values, prefix string, filters UTMFilters) {
	setOptionalStrings(query, prefix+"UtmSource", filters.Source)
	setOptionalStrings(query, prefix+"UtmMedium", filters.Medium)
	setOptionalStrings(query, prefix+"UtmCampaign", filters.Campaign)
	setOptionalStrings(query, prefix+"UtmTerm", filters.Term)
	setOptionalStrings(query, prefix+"UtmContent", filters.Content)
}

var _ CommerceWorkflow = (*Client)(nil)
