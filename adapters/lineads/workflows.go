package lineads

import (
	"context"
	"net/url"
	"strconv"

	"social-hub/pkg/socialhub"
)

func (client *Client) ListAdAccounts(
	ctx context.Context,
	input ListAdAccountsRequest,
	options ...socialhub.CallOption,
) (AdAccountsResponse, error) {
	const operation = "list_ad_accounts"
	if !validListAdAccounts(input) {
		return AdAccountsResponse{}, invalidArgument(operation, "name, pagination, or sort is invalid")
	}
	query := make(url.Values)
	setOptionalBool(query, "includeLinked", input.IncludeLinked)
	setOptionalBool(query, "includeRemoved", input.IncludeRemoved)
	setOptionalQuery(query, "name", input.Name)
	setPagination(query, input.Page, input.Size)
	setSort(query, input.Sort)
	var output AdAccountsResponse
	metadata, err := client.getJSON(ctx, operation, "v3/groups/"+client.groupID+"/adaccounts", query, &output, options...)
	if err != nil {
		return AdAccountsResponse{}, err
	}
	output.Meta = metadata
	return output, nil
}

func (client *Client) ListCampaigns(
	ctx context.Context,
	input ListCampaignsRequest,
	options ...socialhub.CallOption,
) (CampaignsResponse, error) {
	const operation = "list_campaigns"
	if !client.canReadReportingResources() {
		return CampaignsResponse{}, approvalRequired(operation)
	}
	if !validListCampaigns(input) {
		return CampaignsResponse{}, invalidArgument(operation, "ad account, IDs, pagination, or sort is invalid")
	}
	query := make(url.Values)
	setIDs(query, input.IDs)
	setOptionalBool(query, "includeRemoved", input.IncludeRemoved)
	setPagination(query, input.Page, input.Size)
	setSort(query, input.Sort)
	var output CampaignsResponse
	metadata, err := client.getJSON(ctx, operation, "v3/adaccounts/"+input.AdAccountID+"/campaigns", query, &output, options...)
	if err != nil {
		return CampaignsResponse{}, err
	}
	output.Meta = metadata
	return output, nil
}

func (client *Client) ListPerformanceReports(
	ctx context.Context,
	input ListPerformanceReportsRequest,
	options ...socialhub.CallOption,
) (PerformanceReportsResponse, error) {
	const operation = "list_performance_reports"
	if !client.canReadReportingResources() {
		return PerformanceReportsResponse{}, approvalRequired(operation)
	}
	if !validListPerformanceReports(input) {
		return PerformanceReportsResponse{}, invalidArgument(operation, "ad account, IDs, pagination, or sort is invalid")
	}
	query := make(url.Values)
	setIDs(query, input.IDs)
	setPagination(query, input.Page, input.Size)
	setSort(query, input.Sort)
	var output PerformanceReportsResponse
	metadata, err := client.getJSON(ctx, operation, "v3/adaccounts/"+input.AdAccountID+"/pfreports", query, &output, options...)
	if err != nil {
		return PerformanceReportsResponse{}, err
	}
	output.Meta = metadata
	return output, nil
}

func (client *Client) GetOnlineReport(
	ctx context.Context,
	input GetOnlineReportRequest,
	options ...socialhub.CallOption,
) (OnlineReportResponse, error) {
	const operation = "get_online_report"
	if !client.canReadReportingResources() {
		return OnlineReportResponse{}, approvalRequired(operation)
	}
	if !validGetOnlineReport(input) {
		return OnlineReportResponse{}, invalidArgument(operation, "ad account, report level, filters, dates, or pagination is invalid")
	}
	query := make(url.Values)
	setPositiveInt64(query, "adgroupId", input.AdGroupID)
	setPositiveInt64(query, "campaignId", input.CampaignID)
	setOptionalBool(query, "includeRemoved", input.IncludeRemoved)
	setPositiveInt64(query, "lpId", input.LandingPageID)
	if input.Page > 0 {
		query.Set("page", strconv.Itoa(input.Page))
	}
	setOptionalQuery(query, "searchKey", input.SearchKey)
	setOptionalQuery(query, "since", string(input.Since))
	if input.Size > 0 {
		query.Set("size", strconv.Itoa(input.Size))
	}
	setOptionalQuery(query, "until", string(input.Until))
	path := "v3/adaccounts/" + input.AdAccountID + "/reports/online/" + string(input.Level)
	var output OnlineReportResponse
	metadata, err := client.getJSON(ctx, operation, path, query, &output, options...)
	if err != nil {
		return OnlineReportResponse{}, err
	}
	output.Meta = metadata
	return output, nil
}

func (client *Client) canReadReportingResources() bool {
	return client.partnerType == PartnerCertifiedAdTechGeneral || client.partnerType == PartnerReportingGeneral
}

func setOptionalQuery(query url.Values, key, value string) {
	if value != "" {
		query.Set(key, value)
	}
}

func setOptionalBool(query url.Values, key string, value *bool) {
	if value != nil {
		query.Set(key, strconv.FormatBool(*value))
	}
}

func setPagination(query url.Values, page, size int) {
	if page > 0 {
		query.Set("page", strconv.Itoa(page))
	}
	if size > 0 {
		query.Set("size", strconv.Itoa(size))
	}
}

func setSort(query url.Values, value Sort) {
	if value.Field == "" {
		return
	}
	sort := value.Field
	if value.Direction != "" {
		sort += "," + string(value.Direction)
	}
	query.Set("sort", sort)
}

func setIDs(query url.Values, values []int64) {
	for _, value := range values {
		query.Add("ids", strconv.FormatInt(value, 10))
	}
}

func setPositiveInt64(query url.Values, key string, value int64) {
	if value > 0 {
		query.Set(key, strconv.FormatInt(value, 10))
	}
}

var _ ManagementWorkflow = (*Client)(nil)
