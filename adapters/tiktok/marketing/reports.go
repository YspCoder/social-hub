package marketing

import (
	"bytes"
	"context"
	"encoding/json"
	"net/url"
	"strconv"

	"social-hub/pkg/socialhub"
)

func (client *Client) GetReport(ctx context.Context, input ReportRequest, options ...socialhub.CallOption) (NumberPage[ReportRow], error) {
	const operation = "report_get"
	if !validReportLevel(input.DataLevel) || len(input.Dimensions) == 0 || len(input.Metrics) == 0 ||
		!validateFields(input.Dimensions, 100) || !validateFields(input.Metrics, 100) ||
		!validReportFilters(input.Filtering) {
		return NumberPage[ReportRow]{}, invalidArgument(operation, "report level, dimensions, metrics, or filters are invalid")
	}
	dates, valid := inclusiveDates(input.StartDate, input.EndDate)
	if !valid || dates > reportMaximumDates(input.Dimensions) {
		return NumberPage[ReportRow]{}, invalidArgument(operation, "date range exceeds the selected report time dimension limit")
	}
	page, pageSize, err := validatePage(input.Page, input.PageSize)
	if err != nil {
		return NumberPage[ReportRow]{}, err
	}
	query := url.Values{
		"advertiser_id": {client.advertiserID}, "service_type": {"AUCTION"}, "report_type": {"BASIC"},
		"data_level": {string(input.DataLevel)}, "start_date": {input.StartDate}, "end_date": {input.EndDate},
		"page": {strconv.Itoa(page)}, "page_size": {strconv.Itoa(pageSize)},
	}
	for key, value := range map[string]any{"dimensions": input.Dimensions, "metrics": input.Metrics} {
		if err := setJSONQuery(query, key, value, operation); err != nil {
			return NumberPage[ReportRow]{}, err
		}
	}
	if len(input.Filtering) > 0 {
		if err := setJSONQuery(query, "filtering", input.Filtering, operation); err != nil {
			return NumberPage[ReportRow]{}, err
		}
	}
	var response apiEnvelope[struct {
		List     []ReportRow `json:"list"`
		PageInfo *pageInfo   `json:"page_info"`
	}]
	header, err := client.getJSON(ctx, operation, "/v1.3/report/integrated/get/", query, &response, options...)
	if err != nil {
		return NumberPage[ReportRow]{}, err
	}
	data, err := requireEnvelope(operation, response, header)
	if err != nil {
		return NumberPage[ReportRow]{}, err
	}
	for _, row := range data.List {
		actual := firstNonEmpty(rawScalar(row.Dimensions["advertiser_id"]), rawScalar(row.Metrics["advertiser_id"]))
		if err := requireAdvertiser(operation, client.advertiserID, actual); err != nil {
			return NumberPage[ReportRow]{}, err
		}
	}
	return numberPage(operation, data.List, data.PageInfo)
}

func rawScalar(value json.RawMessage) string {
	value = bytes.TrimSpace(value)
	if len(value) == 0 || bytes.Equal(value, []byte("null")) {
		return ""
	}
	var text string
	if json.Unmarshal(value, &text) == nil {
		return text
	}
	var number json.Number
	decoder := json.NewDecoder(bytes.NewReader(value))
	decoder.UseNumber()
	if decoder.Decode(&number) == nil {
		return number.String()
	}
	return ""
}
