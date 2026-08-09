package oceanengine

import (
	"context"
	"net/http"
	"net/url"
	"strconv"

	"social-hub/pkg/socialhub"
)

func (client *Client) GetCustomReport(ctx context.Context, input CustomReportRequest, options ...socialhub.CallOption) (CustomReportPage, error) {
	page, pageSize, err := validatePage(input.Page, input.PageSize, 1000)
	if err != nil {
		return CustomReportPage{}, err
	}
	if len(input.Dimensions) == 0 || len(input.Metrics) == 0 || !validateFields(input.Dimensions) || !validateFields(input.Metrics) ||
		!validTimeRange(input.StartTime, input.EndTime) ||
		input.DataTopic != "" && !validEnum(string(input.DataTopic)) || !validateReportFilters(input.Filters) || !validateReportOrder(input.OrderBy) {
		return CustomReportPage{}, invalidArgument("report_custom_get", "dimensions, metrics, time range, filters, or ordering are invalid")
	}
	query := url.Values{
		"advertiser_id": {strconv.FormatInt(client.advertiserID, 10)},
		"start_time":    {input.StartTime}, "end_time": {input.EndTime},
		"page": {strconv.Itoa(page)}, "page_size": {strconv.Itoa(pageSize)},
	}
	for key, value := range map[string]any{
		"dimensions": input.Dimensions, "metrics": input.Metrics,
		"filters": input.Filters, "order_by": input.OrderBy,
	} {
		if err := setJSONQuery(query, key, value, "report_custom_get"); err != nil {
			return CustomReportPage{}, err
		}
	}
	if input.DataTopic != "" {
		query.Set("data_topic", string(input.DataTopic))
	}
	type responseData struct {
		Rows         []ReportRow       `json:"rows"`
		TotalMetrics map[string]string `json:"total_metrics"`
		PageInfo     *pageInfo         `json:"page_info"`
	}
	var response apiEnvelope[responseData]
	if err := client.api.JSON(ctx, http.MethodGet, "/open_api/v3.0/report/custom/get/", query, nil, &response, options...); err != nil {
		return CustomReportPage{}, err
	}
	data, err := requireEnvelope("report_custom_get", response)
	if err != nil {
		return CustomReportPage{}, err
	}
	if err := validatePageInfo("report_custom_get", data.PageInfo); err != nil {
		return CustomReportPage{}, err
	}
	return CustomReportPage{
		NumberPage:   numberPage(data.Rows, data.PageInfo),
		TotalMetrics: data.TotalMetrics,
	}, nil
}

func validateReportFilters(values []ReportFilter) bool {
	if len(values) > 100 {
		return false
	}
	for _, value := range values {
		if !validFieldName(value.Field) || value.Type < 0 || value.Operator != nil && *value.Operator < 0 || len(value.Values) > 1000 {
			return false
		}
		for _, item := range value.Values {
			if len(item) > 4096 || !validOpaque(item, 4096) {
				return false
			}
		}
	}
	return true
}

func validateReportOrder(values []ReportOrder) bool {
	if len(values) > 100 {
		return false
	}
	for _, value := range values {
		if !validFieldName(value.Field) || value.Type != "" && value.Type != "ASC" && value.Type != "DESC" {
			return false
		}
	}
	return true
}
