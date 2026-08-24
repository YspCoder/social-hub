package applovinreporting

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"social-hub/pkg/socialhub"
)

const maxErrorResponseBytes int64 = 1 << 20

var (
	errReportTooLarge = errors.New("report exceeded configured size limit")
	errInvalidCSV     = errors.New("invalid CSV report")
)

type preparedReport struct {
	path    string
	query   url.Values
	columns []string
}

func (client *Client) CampaignReport(ctx context.Context, input CampaignReportRequest, options ...socialhub.CallOption) (CampaignReport, error) {
	const operation = "campaign_report"
	prepared, err := client.prepareCampaignReport(input)
	if err != nil {
		return CampaignReport{}, err
	}
	rows, err := client.queryRows(ctx, operation, prepared, options...)
	if err != nil {
		return CampaignReport{}, err
	}
	result := make([]CampaignRow, len(rows))
	for index, row := range rows {
		result[index] = make(CampaignRow, len(row))
		for column, value := range row {
			result[index][CampaignColumn(column)] = value
		}
	}
	return CampaignReport{Count: len(result), Rows: result}, nil
}

func (client *Client) DownloadCampaignCSV(
	ctx context.Context,
	input CampaignReportRequest,
	output io.Writer,
	download DownloadOptions,
	options ...socialhub.CallOption,
) (DownloadResult, error) {
	const operation = "campaign_report_download"
	prepared, err := client.prepareCampaignReport(input)
	if err != nil {
		return DownloadResult{}, err
	}
	return client.downloadCSV(ctx, operation, prepared, output, download, options...)
}

func (client *Client) AssetReport(ctx context.Context, input AssetReportRequest, options ...socialhub.CallOption) (AssetReport, error) {
	const operation = "asset_report"
	prepared, err := client.prepareAssetReport(input)
	if err != nil {
		return AssetReport{}, err
	}
	rows, err := client.queryRows(ctx, operation, prepared, options...)
	if err != nil {
		return AssetReport{}, err
	}
	result := make([]AssetRow, len(rows))
	for index, row := range rows {
		result[index] = make(AssetRow, len(row))
		for column, value := range row {
			result[index][AssetColumn(column)] = value
		}
	}
	return AssetReport{Count: len(result), Rows: result}, nil
}

func (client *Client) DownloadAssetCSV(
	ctx context.Context,
	input AssetReportRequest,
	output io.Writer,
	download DownloadOptions,
	options ...socialhub.CallOption,
) (DownloadResult, error) {
	const operation = "asset_report_download"
	prepared, err := client.prepareAssetReport(input)
	if err != nil {
		return DownloadResult{}, err
	}
	return client.downloadCSV(ctx, operation, prepared, output, download, options...)
}

func (client *Client) PlayableReport(ctx context.Context, input PlayableReportRequest, options ...socialhub.CallOption) (PlayableReport, error) {
	const operation = "playable_report"
	prepared, err := client.preparePlayableReport(input)
	if err != nil {
		return PlayableReport{}, err
	}
	rows, err := client.queryRows(ctx, operation, prepared, options...)
	if err != nil {
		return PlayableReport{}, err
	}
	result := make([]PlayableRow, len(rows))
	for index, row := range rows {
		result[index] = make(PlayableRow, len(row))
		for column, value := range row {
			result[index][PlayableColumn(column)] = value
		}
	}
	return PlayableReport{Count: len(result), Rows: result}, nil
}

func (client *Client) DownloadPlayableCSV(
	ctx context.Context,
	input PlayableReportRequest,
	output io.Writer,
	download DownloadOptions,
	options ...socialhub.CallOption,
) (DownloadResult, error) {
	const operation = "playable_report_download"
	prepared, err := client.preparePlayableReport(input)
	if err != nil {
		return DownloadResult{}, err
	}
	return client.downloadCSV(ctx, operation, prepared, output, download, options...)
}

func (client *Client) prepareCampaignReport(input CampaignReportRequest) (preparedReport, error) {
	if input.Type == "" {
		input.Type = ReportAdvertiser
	}
	if !validCampaignRequest(input, client.accountType, client.clock.Now()) {
		return preparedReport{}, invalidArgument("campaign_report", "account type, report type, date window, columns, filters, having, sorting, attribution, or pagination is invalid")
	}
	pagination, _ := normalizedPagination(input.Pagination)
	query := url.Values{
		"report_type": {string(input.Type)}, "start": {string(input.Start)}, "end": {string(input.End)},
		"columns": {joinCampaignColumns(input.Columns)}, "offset": {strconv.Itoa(pagination.Offset)}, "limit": {strconv.Itoa(pagination.Limit)},
	}
	applyCampaignFilters(query, input.Filters)
	for _, sort := range input.Sorts {
		query.Set("sort_"+string(sort.Column), string(sort.Order))
	}
	if input.Having != nil {
		combine := input.Having.Combine
		if combine == "" {
			combine = HavingAND
		}
		conditions := make([]string, len(input.Having.Conditions))
		for index, condition := range input.Having.Conditions {
			conditions[index] = string(condition.Column) + " " + string(condition.Operator) + " " + condition.Value
		}
		query.Set("having", strings.Join(conditions, " "+string(combine)+" "))
	}
	for _, filter := range input.CustomPageFilters {
		query.Set(string(filter), "")
	}
	if input.NotZero {
		query.Set("not_zero", "1")
	}
	if input.Attribution == "" || input.Attribution == AttributionCohort {
		query.Set("day_column", "day")
	}
	return preparedReport{path: "/report", query: query, columns: campaignColumnStrings(input.Columns)}, nil
}

func (client *Client) prepareAssetReport(input AssetReportRequest) (preparedReport, error) {
	if !validAssetRequest(input, client.accountType) {
		return preparedReport{}, invalidArgument("asset_report", "account type, time selector, columns, filters, metric filters, sorting, or pagination is invalid")
	}
	pagination, _ := normalizedPagination(input.Pagination)
	path := "/assetReport"
	query := url.Values{
		"columns": {joinAssetColumns(input.Columns)}, "offset": {strconv.Itoa(pagination.Offset)}, "limit": {strconv.Itoa(pagination.Limit)},
	}
	if input.Range != "" {
		query.Set("range", string(input.Range))
	} else {
		path = "/assetAnalyticsReport"
		query.Set("start", string(input.Start))
		query.Set("end", string(input.End))
	}
	for _, filter := range input.Filters {
		key := "filter_" + string(filter.Column)
		if filter.Negate {
			key = "filter_not_" + string(filter.Column)
		}
		query.Set(key, strings.Join(filter.Values, ","))
	}
	for _, filter := range input.Metrics {
		if filter.GreaterThan != "" {
			query.Set("filter_greater_than_"+string(filter.Column), filter.GreaterThan)
		}
		if filter.LessThan != "" {
			query.Set("filter_less_than_"+string(filter.Column), filter.LessThan)
		}
	}
	for _, sort := range input.Sorts {
		query.Set("sort_"+string(sort.Column), string(sort.Order))
	}
	if input.NotZero {
		query.Set("not_zero", "1")
	}
	return preparedReport{path: path, query: query, columns: assetColumnStrings(input.Columns)}, nil
}

func (client *Client) preparePlayableReport(input PlayableReportRequest) (preparedReport, error) {
	if !validPlayableRequest(input, client.accountType) {
		return preparedReport{}, invalidArgument("playable_report", "playable reports require an APP account and valid dates, columns, filters, sorting, attribution, and pagination")
	}
	pagination, _ := normalizedPagination(input.Pagination)
	query := url.Values{
		"start": {string(input.Start)}, "end": {string(input.End)}, "columns": {joinPlayableColumns(input.Columns)},
		"offset": {strconv.Itoa(pagination.Offset)}, "limit": {strconv.Itoa(pagination.Limit)},
	}
	for _, filter := range input.Filters {
		key := "filter_" + string(filter.Column)
		if filter.Negate {
			key = "filter_not_" + string(filter.Column)
		}
		query.Set(key, strings.Join(filter.Values, ","))
	}
	for _, sort := range input.Sorts {
		query.Set("sort_"+string(sort.Column), string(sort.Order))
	}
	if input.Attribution == AttributionRealtime {
		query.Set("day_column", "event_day")
	}
	return preparedReport{path: "/playableMetrics", query: query, columns: playableColumnStrings(input.Columns)}, nil
}

func applyCampaignFilters(query url.Values, filters []CampaignFilter) {
	for _, filter := range filters {
		key := "filter_" + string(filter.Column)
		if filter.Negate {
			key = "filter_not_" + string(filter.Column)
		}
		query.Set(key, strings.Join(filter.Values, ","))
	}
}

func joinCampaignColumns(values []CampaignColumn) string {
	return strings.Join(campaignColumnStrings(values), ",")
}
func joinAssetColumns(values []AssetColumn) string {
	return strings.Join(assetColumnStrings(values), ",")
}
func joinPlayableColumns(values []PlayableColumn) string {
	return strings.Join(playableColumnStrings(values), ",")
}

func campaignColumnStrings(values []CampaignColumn) []string {
	result := make([]string, len(values))
	for index, value := range values {
		result[index] = string(value)
	}
	return result
}

func assetColumnStrings(values []AssetColumn) []string {
	result := make([]string, len(values))
	for index, value := range values {
		result[index] = string(value)
	}
	return result
}

func playableColumnStrings(values []PlayableColumn) []string {
	result := make([]string, len(values))
	for index, value := range values {
		result[index] = string(value)
	}
	return result
}

func (client *Client) queryRows(ctx context.Context, operation string, prepared preparedReport, options ...socialhub.CallOption) ([]map[string]ReportValue, error) {
	callOptions, err := supportedCallOptions(operation, options)
	if err != nil {
		return nil, err
	}
	query := cloneValues(prepared.query)
	query.Set("format", "json")
	request, err := client.api.NewRequest(ctx, http.MethodGet, prepared.path, query, nil, forwardCallOptions(callOptions)...)
	if err != nil {
		return nil, withOperation(err, operation)
	}
	request.Header.Set("Accept", "application/json")
	var raw json.RawMessage
	if err := client.api.Do(request, &raw); err != nil {
		return nil, withOperation(err, operation)
	}
	rows, err := decodeReportRows(raw, operation)
	if err != nil {
		var hub *socialhub.Error
		if errors.As(err, &hub) {
			return nil, err
		}
		return nil, platformContractError(operation, err.Error())
	}
	expected := make(map[string]struct{}, len(prepared.columns))
	for _, column := range prepared.columns {
		expected[column] = struct{}{}
	}
	if !validRawRows(rows, expected) {
		return nil, platformContractError(operation, "AppLovin returned malformed or unexpected report columns")
	}
	return rows, nil
}

func decodeReportRows(raw json.RawMessage, operation string) ([]map[string]ReportValue, error) {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" {
		return nil, errors.New("AppLovin returned an empty JSON report")
	}
	var rows []map[string]ReportValue
	if trimmed[0] == '[' {
		if err := json.Unmarshal(raw, &rows); err != nil {
			return nil, errors.New("AppLovin returned an invalid JSON report array")
		}
		return rows, nil
	}
	if trimmed[0] != '{' {
		return nil, errors.New("AppLovin returned an unexpected JSON report shape")
	}
	var envelope struct {
		Code    int             `json:"code"`
		Count   *int            `json:"count"`
		Results json.RawMessage `json:"results"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, errors.New("AppLovin returned an invalid JSON report object")
	}
	if envelope.Code != 0 && envelope.Code != http.StatusOK {
		return nil, businessError(operation, envelope.Code)
	}
	if len(envelope.Results) == 0 || string(envelope.Results) == "null" {
		return nil, errors.New("AppLovin JSON report object is missing results")
	}
	if err := json.Unmarshal(envelope.Results, &rows); err != nil {
		return nil, errors.New("AppLovin JSON report results are invalid")
	}
	if envelope.Count != nil && *envelope.Count != len(rows) {
		return nil, errors.New("AppLovin JSON report count does not match results")
	}
	return rows, nil
}

func (client *Client) downloadCSV(
	ctx context.Context,
	operation string,
	prepared preparedReport,
	output io.Writer,
	download DownloadOptions,
	options ...socialhub.CallOption,
) (DownloadResult, error) {
	if output == nil || download.MaxBytes < 0 {
		return DownloadResult{}, invalidArgument(operation, "output and a nonnegative maximum size are required")
	}
	callOptions, err := supportedCallOptions(operation, options)
	if err != nil {
		return DownloadResult{}, err
	}
	if callOptions.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, callOptions.Timeout)
		defer cancel()
	}
	maximum := download.MaxBytes
	if maximum == 0 {
		maximum = DefaultMaxCSVReportBytes
	}
	query := cloneValues(prepared.query)
	query.Set("format", "csv")
	request, err := client.api.NewRequest(ctx, http.MethodGet, prepared.path, query, nil)
	if err != nil {
		return DownloadResult{}, withOperation(err, operation)
	}
	request.Header.Set("Accept", "text/csv")
	request.Header.Set("Accept-Encoding", "identity")
	response, err := client.httpClient.Do(request)
	if err != nil {
		return DownloadResult{}, platformError(operation, socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		body, readErr := io.ReadAll(io.LimitReader(response.Body, maxErrorResponseBytes+1))
		if readErr != nil {
			return DownloadResult{}, platformError(operation, socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, readErr)
		}
		if int64(len(body)) > maxErrorResponseBytes {
			return DownloadResult{}, platformContractError(operation, "AppLovin error response exceeded the size limit")
		}
		return DownloadResult{}, withOperation(decodeHTTPErrorAt(response.StatusCode, response.Header, body, client.clock.Now()), operation)
	}
	if response.StatusCode != http.StatusOK {
		return DownloadResult{}, platformContractError(operation, "AppLovin returned an unexpected successful CSV status")
	}
	if encoding := strings.ToLower(strings.TrimSpace(response.Header.Get("Content-Encoding"))); encoding != "" && encoding != "identity" {
		return DownloadResult{}, platformContractError(operation, "AppLovin returned an unexpected CSV content encoding")
	}
	contentType := response.Header.Get("Content-Type")
	mediaType, _, mediaErr := mime.ParseMediaType(contentType)
	if mediaErr == nil && strings.EqualFold(mediaType, "application/json") {
		body, readErr := io.ReadAll(io.LimitReader(response.Body, maxErrorResponseBytes+1))
		if readErr != nil {
			return DownloadResult{}, platformError(operation, socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, readErr)
		}
		if int64(len(body)) > maxErrorResponseBytes {
			return DownloadResult{}, platformContractError(operation, "AppLovin JSON error response exceeded the size limit")
		}
		return DownloadResult{}, decodeEmbeddedCSVError(operation, body)
	}
	if mediaErr != nil || !validCSVMediaType(mediaType) {
		return DownloadResult{}, platformContractError(operation, "AppLovin returned an unexpected CSV content type")
	}
	written, rows, copyErr := copyCSVValidated(output, response.Body, maximum, prepared.columns)
	if copyErr != nil {
		return DownloadResult{}, reportCopyError(operation, copyErr)
	}
	return DownloadResult{
		StatusCode: response.StatusCode, BytesWritten: written, DataRows: rows,
		ContentType: boundedHeader(contentType, 512),
	}, nil
}

func decodeEmbeddedCSVError(operation string, body []byte) error {
	var envelope struct {
		Code   int `json:"code"`
		Status int `json:"status"`
	}
	if json.Unmarshal(body, &envelope) == nil {
		code := envelope.Code
		if code == 0 {
			code = envelope.Status
		}
		if code != 0 && code != http.StatusOK {
			return businessError(operation, code)
		}
	}
	return platformContractError(operation, "AppLovin returned JSON instead of a CSV report")
}

func cloneValues(input url.Values) url.Values {
	output := make(url.Values, len(input))
	for key, values := range input {
		output[key] = append([]string(nil), values...)
	}
	return output
}

func validCSVMediaType(value string) bool {
	switch strings.ToLower(value) {
	case "text/csv", "application/csv", "application/octet-stream", "binary/octet-stream":
		return true
	default:
		return false
	}
}

func copyCSVValidated(output io.Writer, source io.Reader, maximum int64, expected []string) (int64, int64, error) {
	sink := &boundedOutput{output: output, maximum: maximum}
	reader := csv.NewReader(io.TeeReader(source, sink))
	reader.FieldsPerRecord = -1
	header, err := reader.Read()
	if err != nil {
		if sink.err != nil {
			return sink.written, 0, sink.err
		}
		return sink.written, 0, fmt.Errorf("%w: %v", errInvalidCSV, err)
	}
	if len(header) > 0 {
		header[0] = strings.TrimPrefix(header[0], "\ufeff")
	}
	if len(header) != len(expected) {
		return sink.written, 0, errInvalidCSV
	}
	for index := range expected {
		if header[index] != expected[index] {
			return sink.written, 0, errInvalidCSV
		}
	}
	var rows int64
	for {
		record, readErr := reader.Read()
		if errors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil {
			if sink.err != nil {
				return sink.written, rows, sink.err
			}
			return sink.written, rows, fmt.Errorf("%w: %v", errInvalidCSV, readErr)
		}
		if len(record) != len(header) {
			return sink.written, rows, errInvalidCSV
		}
		rows++
	}
	if sink.err != nil {
		return sink.written, rows, sink.err
	}
	return sink.written, rows, nil
}

type boundedOutput struct {
	output  io.Writer
	maximum int64
	written int64
	err     error
}

func (writer *boundedOutput) Write(data []byte) (int, error) {
	if writer.err != nil {
		return 0, writer.err
	}
	remaining := writer.maximum - writer.written
	if remaining <= 0 {
		writer.err = errReportTooLarge
		return 0, writer.err
	}
	limited := data
	overflow := int64(len(data)) > remaining
	if overflow {
		limited = data[:remaining]
	}
	written, err := writer.output.Write(limited)
	writer.written += int64(written)
	if err != nil {
		writer.err = &outputError{err: err}
		return written, writer.err
	}
	if written != len(limited) {
		writer.err = &outputError{err: io.ErrShortWrite}
		return written, writer.err
	}
	if overflow {
		writer.err = errReportTooLarge
		return written, writer.err
	}
	return written, nil
}

type outputError struct{ err error }

func (err *outputError) Error() string { return err.err.Error() }
func (err *outputError) Unwrap() error { return err.err }

func reportCopyError(operation string, err error) error {
	if errors.Is(err, errReportTooLarge) {
		return platformContractError(operation, "AppLovin CSV report exceeded the configured download size limit")
	}
	if errors.Is(err, errInvalidCSV) {
		return platformContractError(operation, "AppLovin returned malformed, truncated, or reordered CSV")
	}
	var outputErr *outputError
	if errors.As(err, &outputErr) {
		return platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, outputErr.err)
	}
	return platformError(operation, socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
}
