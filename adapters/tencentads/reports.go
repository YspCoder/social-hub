package tencentads

import (
	"context"
	"net/http"
	"net/url"
	"strconv"

	"social-hub/pkg/socialhub"
)

func (client *Client) GetReport(ctx context.Context, input ReportRequest, options ...socialhub.CallOption) (NumberPage[ReportRow], error) {
	const operation = "report_get"
	if input.Granularity != ReportDaily && input.Granularity != ReportHourly || !validEnum(string(input.Level)) {
		return NumberPage[ReportRow]{}, invalidArgument(operation, "granularity and an uppercase report level are required")
	}
	if !validDate(input.DateRange.StartDate) || !validDate(input.DateRange.EndDate) || input.DateRange.StartDate > input.DateRange.EndDate {
		return NumberPage[ReportRow]{}, invalidArgument(operation, "date range must contain ordered YYYY-MM-DD values")
	}
	page, pageSize, err := validateList(input.Fields, input.Filtering, input.Page, input.PageSize)
	if err != nil {
		return NumberPage[ReportRow]{}, err
	}
	for _, field := range input.GroupBy {
		if !validFieldName(field) {
			return NumberPage[ReportRow]{}, invalidArgument(operation, "group_by fields must be lowercase API identifiers")
		}
	}
	for _, order := range input.OrderBy {
		if !validFieldName(order.SortField) || !validEnum(order.SortType) {
			return NumberPage[ReportRow]{}, invalidArgument(operation, "order_by requires a field and uppercase sort type")
		}
	}
	if input.TimeLine != "" && !validOpaque(input.TimeLine, 4096) {
		return NumberPage[ReportRow]{}, invalidArgument(operation, "time_line is invalid")
	}
	fields := input.Fields
	if len(fields) == 0 {
		fields = []string{"account_id", "date", "view_count", "valid_click_count", "cost"}
	}
	fields = appendRequiredFields(fields, "account_id", "date")
	if input.Granularity == ReportHourly {
		fields = appendRequiredFields(fields, "hour")
	}
	query := url.Values{
		"account_id": {strconv.FormatInt(client.advertiserID, 10)}, "level": {string(input.Level)},
		"page": {strconv.Itoa(page)}, "page_size": {strconv.Itoa(pageSize)},
	}
	for key, value := range map[string]any{
		"date_range": input.DateRange, "fields": fields,
	} {
		if err := setJSONQuery(query, key, value, operation); err != nil {
			return NumberPage[ReportRow]{}, err
		}
	}
	if len(input.Filtering) > 0 {
		if err := setJSONQuery(query, "filtering", input.Filtering, operation); err != nil {
			return NumberPage[ReportRow]{}, err
		}
	}
	if len(input.GroupBy) > 0 {
		if err := setJSONQuery(query, "group_by", input.GroupBy, operation); err != nil {
			return NumberPage[ReportRow]{}, err
		}
	}
	if len(input.OrderBy) > 0 {
		if err := setJSONQuery(query, "order_by", input.OrderBy, operation); err != nil {
			return NumberPage[ReportRow]{}, err
		}
	}
	if input.TimeLine != "" {
		query.Set("time_line", input.TimeLine)
	}
	path := "/daily_reports/get"
	if input.Granularity == ReportHourly {
		path = "/hourly_reports/get"
	}
	var response apiEnvelope[struct {
		List     []ReportRow `json:"list"`
		PageInfo *pageInfo   `json:"page_info"`
	}]
	header, err := client.requestJSON(ctx, operation, http.MethodGet, path, query, nil, &response, options...)
	if err != nil {
		return NumberPage[ReportRow]{}, err
	}
	data, err := requireEnvelope(operation, response, header)
	if err != nil {
		return NumberPage[ReportRow]{}, err
	}
	if err := validatePageInfo(operation, data.PageInfo); err != nil {
		return NumberPage[ReportRow]{}, err
	}
	for _, row := range data.List {
		if row.AccountID != client.advertiserID {
			return NumberPage[ReportRow]{}, platformContractError(operation, "Tencent Ads report row omitted or mismatched the configured advertiser")
		}
	}
	return numberPage(data.List, data.PageInfo), nil
}
