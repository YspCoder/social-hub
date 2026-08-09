package baiduads

import (
	"context"
	"net/url"
	"time"

	"social-hub/pkg/socialhub"
)

func (client *Client) GetReportData(ctx context.Context, input ReportRequest, options ...socialhub.CallOption) (*ReportData, error) {
	const operation = "report_data_get"
	body, err := reportBody(operation, input, false)
	if err != nil {
		return nil, err
	}
	var response apiEnvelope[ReportData]
	header, err := client.requestJSON(ctx, operation, "/json/sms/service/OpenApiReportService/getReportData", body, &response, options...)
	if err != nil {
		return nil, err
	}
	data, err := requireEnvelope(operation, response, header)
	if err != nil {
		return nil, err
	}
	if data.RowCount < 0 || data.TotalRowCount < data.RowCount || len(data.Rows) != data.RowCount {
		return nil, platformContractError(operation, "Baidu Ads returned inconsistent report row counts")
	}
	return data, nil
}

func (client *Client) CreateReportTask(ctx context.Context, input ReportRequest, options ...socialhub.CallOption) (*ReportTask, error) {
	const operation = "report_task_create"
	body, err := reportBody(operation, input, true)
	if err != nil {
		return nil, err
	}
	var response apiEnvelope[[]ReportTask]
	header, err := client.requestJSON(ctx, operation, "/json/sms/service/OpenApiReportService/createReportTask", body, &response, options...)
	if err != nil {
		return nil, err
	}
	values, err := requireEnvelope(operation, response, header)
	if err != nil {
		return nil, err
	}
	task, err := oneResult(operation, *values)
	if err != nil {
		return nil, err
	}
	if !validOpaque(task.TaskID, 256) {
		return nil, platformContractError(operation, "Baidu Ads returned an invalid report task ID")
	}
	return task, nil
}

func (client *Client) GetReportTask(ctx context.Context, taskID string, options ...socialhub.CallOption) (*ReportTask, error) {
	const operation = "report_task_get"
	if !validOpaque(taskID, 256) {
		return nil, invalidArgument(operation, "report task ID is required")
	}
	var response apiEnvelope[[]ReportTask]
	header, err := client.requestJSON(ctx, operation, "/json/sms/service/OpenApiReportService/getTaskStatus", map[string]any{
		"taskId": taskID,
	}, &response, options...)
	if err != nil {
		return nil, err
	}
	values, err := requireEnvelope(operation, response, header)
	if err != nil {
		return nil, err
	}
	task, err := oneResult(operation, *values)
	if err != nil {
		return nil, err
	}
	if task.TaskID != taskID || !validReportTaskStatus(task.Status) {
		return nil, platformContractError(operation, "Baidu Ads returned an invalid report task")
	}
	if task.Status == "SUCCESS" && !validReportFileURL(task.FileURL) {
		return nil, platformContractError(operation, "Baidu Ads completed a report task without a valid file URL")
	}
	return task, nil
}

func reportBody(operation string, input ReportRequest, asynchronous bool) (map[string]any, error) {
	if input.ReportType <= 0 || !validDate(input.StartDate) || !validDate(input.EndDate) ||
		!validReportTimeUnit(input.TimeUnit) || len(input.Columns) == 0 || len(input.Columns) > 256 ||
		len(input.Sorts) > 2 || len(input.Filters) > 5 || input.StartRow < 0 || !asynchronous && input.RowCount <= 0 {
		return nil, invalidArgument(operation, "report type, dates, time unit, columns, sorting, filters, or pagination are invalid")
	}
	start, _ := time.Parse("2006-01-02", input.StartDate)
	end, _ := time.Parse("2006-01-02", input.EndDate)
	if end.Before(start) {
		return nil, invalidArgument(operation, "report end date cannot precede start date")
	}
	if err := validateFields(operation, input.Columns, 256); err != nil {
		return nil, err
	}
	if err := validateIDs(operation, input.UserIDs, 5000, true); err != nil {
		return nil, err
	}
	columns := make(map[string]struct{}, len(input.Columns))
	for _, column := range input.Columns {
		columns[column] = struct{}{}
	}
	for _, sort := range input.Sorts {
		if _, found := columns[sort.Column]; !found || (sort.Rule != "ASC" && sort.Rule != "DESC") {
			return nil, invalidArgument(operation, "report sorts must reference requested columns and use ASC or DESC")
		}
	}
	for _, filter := range input.Filters {
		if !validFieldName(filter.Column) || !validFilterOperator(filter.Operator) || len(filter.Values) == 0 || len(filter.Values) > 500 {
			return nil, invalidArgument(operation, "report filter is invalid")
		}
		for _, value := range filter.Values {
			if !validOpaque(value, 2048) {
				return nil, invalidArgument(operation, "report filter values must be bounded strings")
			}
		}
		if filter.Operator != "IN" && filter.Operator != "NOT_IN" && len(filter.Values) != 1 {
			return nil, invalidArgument(operation, "report comparison filters accept exactly one value")
		}
	}
	body := map[string]any{
		"reportType": input.ReportType, "startDate": input.StartDate, "endDate": input.EndDate,
		"timeUnit": input.TimeUnit, "columns": append([]string(nil), input.Columns...), "needSum": input.NeedSum,
	}
	if len(input.UserIDs) > 0 {
		body["userIds"] = append([]int64(nil), input.UserIDs...)
	}
	if len(input.Sorts) > 0 {
		body["sorts"] = append([]ReportSort(nil), input.Sorts...)
	}
	if len(input.Filters) > 0 {
		body["filters"] = append([]ReportFilter(nil), input.Filters...)
	}
	if !asynchronous {
		body["startRow"], body["rowCount"] = input.StartRow, input.RowCount
	}
	return body, nil
}

func validReportTimeUnit(value ReportTimeUnit) bool {
	switch value {
	case ReportTimeHour, ReportTimeDay, ReportTimeWeek, ReportTimeMonth, ReportTimeSummary:
		return true
	default:
		return false
	}
}

func validFilterOperator(value string) bool {
	switch value {
	case "GT", "GTE", "LT", "LTE", "EQ", "NOT_EQ", "IN", "NOT_IN":
		return true
	default:
		return false
	}
}

func validReportTaskStatus(value string) bool {
	switch value {
	case "SUBMITTED", "RUNNING", "SUCCESS", "FAIL":
		return true
	default:
		return false
	}
}

func validReportFileURL(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && (parsed.Scheme == "https" || parsed.Scheme == "http") && parsed.Host != "" && parsed.User == nil
}

var _ ReportWorkflow = (*Client)(nil)
