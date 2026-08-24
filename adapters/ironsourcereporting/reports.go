package ironsourcereporting

import (
	"bytes"
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
	"unicode/utf8"

	"github.com/tomnomnom/linkheader"

	"social-hub/pkg/socialhub"
)

const (
	advertiserReportPath        = "/advertisers/v4/reports"
	costReportPath              = "/advertisers/v4/reports/cost"
	skanReportPath              = "/advertisers/v4/reports/skan"
	skanCVReportPath            = "/advertisers/v4/reports/skan/cv"
	maxErrorResponseBytes int64 = 1 << 20
)

var (
	errReportTooLarge = errors.New("report exceeded configured size limit")
	errInvalidCSV     = errors.New("invalid CSV report")
)

type preparedReport struct {
	path  string
	query url.Values
}

func (client *Client) AdvertiserReport(ctx context.Context, input AdvertiserReportRequest, options ...socialhub.CallOption) (ReportPage, error) {
	prepared, err := prepareAdvertiserReport(input)
	if err != nil {
		return ReportPage{}, err
	}
	return client.queryReport(ctx, "advertiser_report", prepared, options...)
}

func (client *Client) CostReport(ctx context.Context, input CostReportRequest, options ...socialhub.CallOption) (ReportPage, error) {
	prepared, err := prepareCostReport(input)
	if err != nil {
		return ReportPage{}, err
	}
	return client.queryReport(ctx, "cost_report", prepared, options...)
}

func (client *Client) SKANReport(ctx context.Context, input SKANReportRequest, options ...socialhub.CallOption) (ReportPage, error) {
	prepared, err := prepareSKANReport(input)
	if err != nil {
		return ReportPage{}, err
	}
	return client.queryReport(ctx, "skan_report", prepared, options...)
}

func (client *Client) SKANConversionValues(ctx context.Context, input SKANConversionValueRequest, options ...socialhub.CallOption) (ConversionValuePage, error) {
	const operation = "skan_conversion_values"
	prepared, err := prepareSKANConversionValues(input)
	if err != nil {
		return ConversionValuePage{}, err
	}
	call, err := validateCallOptions(operation, options)
	if err != nil {
		return ConversionValuePage{}, err
	}
	query := cloneValues(prepared.query)
	query.Set("format", "json")
	request, err := client.api.NewRequest(ctx, http.MethodGet, prepared.path, query, nil, resolvedCallOptions(call)...)
	if err != nil {
		return ConversionValuePage{}, withOperation(err, operation)
	}
	request.Header.Set("Accept", "application/json")
	var raw json.RawMessage
	if err := client.api.Do(request, &raw); err != nil {
		return ConversionValuePage{}, withOperation(err, operation)
	}
	return client.decodeConversionValuePage(raw, prepared.path, operation)
}

func (client *Client) DownloadAdvertiserCSV(ctx context.Context, input AdvertiserReportRequest, output io.Writer, download DownloadOptions, options ...socialhub.CallOption) (DownloadResult, error) {
	prepared, err := prepareAdvertiserReport(input)
	if err != nil {
		return DownloadResult{}, err
	}
	return client.downloadCSV(ctx, "advertiser_report_download", prepared, output, download, options...)
}

func (client *Client) DownloadCostCSV(ctx context.Context, input CostReportRequest, output io.Writer, download DownloadOptions, options ...socialhub.CallOption) (DownloadResult, error) {
	prepared, err := prepareCostReport(input)
	if err != nil {
		return DownloadResult{}, err
	}
	return client.downloadCSV(ctx, "cost_report_download", prepared, output, download, options...)
}

func (client *Client) DownloadSKANCSV(ctx context.Context, input SKANReportRequest, output io.Writer, download DownloadOptions, options ...socialhub.CallOption) (DownloadResult, error) {
	prepared, err := prepareSKANReport(input)
	if err != nil {
		return DownloadResult{}, err
	}
	return client.downloadCSV(ctx, "skan_report_download", prepared, output, download, options...)
}

func (client *Client) DownloadSKANConversionValuesCSV(ctx context.Context, input SKANConversionValueRequest, output io.Writer, download DownloadOptions, options ...socialhub.CallOption) (DownloadResult, error) {
	prepared, err := prepareSKANConversionValues(input)
	if err != nil {
		return DownloadResult{}, err
	}
	return client.downloadCSV(ctx, "skan_conversion_values_download", prepared, output, download, options...)
}

func prepareAdvertiserReport(input AdvertiserReportRequest) (preparedReport, error) {
	if err := validateAdvertiserRequest(input); err != nil {
		return preparedReport{}, err
	}
	query := baseReportQuery(input.Start, input.End, input.Count, input.Cursor)
	query.Set("metrics", joinStrings(input.Metrics))
	setJoined(query, "breakdowns", joinStrings(input.Breakdowns))
	applyReportFilters(query, input.Filters.ReportFilters)
	setJoined(query, "creativeId", joinIDs(input.Filters.CreativeIDs))
	setJoined(query, "deviceType", string(input.Filters.DeviceType))
	setJoined(query, "adUnit", string(input.Filters.AdUnit))
	setJoined(query, "excl_campaign_id", joinIDs(input.Filters.ExcludeCampaignIDs))
	setJoined(query, "excl_bundle_id", joinStrings(input.Filters.ExcludeBundleIDs))
	setJoined(query, "excl_creative_id", joinIDs(input.Filters.ExcludeCreativeIDs))
	setJoined(query, "excl_country", joinStrings(input.Filters.ExcludeCountries))
	applyOrdering(query, input.Order, input.Direction)
	if !validQuery(query) {
		return preparedReport{}, invalidArgument("advertiser_report", "encoded query exceeds the 64 KiB client safety bound")
	}
	return preparedReport{path: advertiserReportPath, query: query}, nil
}

func prepareCostReport(input CostReportRequest) (preparedReport, error) {
	if err := validateCostRequest(input); err != nil {
		return preparedReport{}, err
	}
	query := baseReportQuery(input.Start, input.End, input.Count, input.Cursor)
	query.Set("metrics", joinStrings(input.Metrics))
	setJoined(query, "breakdowns", joinStrings(input.Breakdowns))
	applyReportFilters(query, input.Filters)
	applyOrdering(query, input.Order, input.Direction)
	if !validQuery(query) {
		return preparedReport{}, invalidArgument("cost_report", "encoded query exceeds the 64 KiB client safety bound")
	}
	return preparedReport{path: costReportPath, query: query}, nil
}

func prepareSKANReport(input SKANReportRequest) (preparedReport, error) {
	if err := validateSKANRequest(input); err != nil {
		return preparedReport{}, err
	}
	query := baseReportQuery(input.Start, input.End, input.Count, input.Cursor)
	query.Set("metrics", joinStrings(input.Metrics))
	setJoined(query, "breakdowns", joinStrings(input.Breakdowns))
	applySKANFilters(query, input.Filters)
	setJoined(query, "adUnit", string(input.AdUnit))
	applyOrdering(query, input.Order, input.Direction)
	if !validQuery(query) {
		return preparedReport{}, invalidArgument("skan_report", "encoded query exceeds the 64 KiB client safety bound")
	}
	return preparedReport{path: skanReportPath, query: query}, nil
}

func prepareSKANConversionValues(input SKANConversionValueRequest) (preparedReport, error) {
	if err := validateSKANCVRequest(input); err != nil {
		return preparedReport{}, err
	}
	query := baseReportQuery(input.Start, input.End, input.Count, input.Cursor)
	setJoined(query, "breakdowns", joinStrings(input.Breakdowns))
	setJoined(query, "campaignId", joinIDs(input.CampaignIDs))
	setJoined(query, "bundleId", joinStrings(input.BundleIDs))
	applyOrdering(query, input.Order, input.Direction)
	if !validQuery(query) {
		return preparedReport{}, invalidArgument("skan_conversion_values", "encoded query exceeds the 64 KiB client safety bound")
	}
	return preparedReport{path: skanCVReportPath, query: query}, nil
}

func baseReportQuery(start, end Date, count int, cursor string) url.Values {
	query := url.Values{
		"startDate": {string(start)}, "endDate": {string(end)}, "count": {strconv.Itoa(normalizedCount(count))},
	}
	setJoined(query, "cursor", cursor)
	return query
}

func applyReportFilters(query url.Values, filters ReportFilters) {
	setJoined(query, "campaignId", joinIDs(filters.CampaignIDs))
	setJoined(query, "bundleId", joinStrings(filters.BundleIDs))
	setJoined(query, "country", joinStrings(filters.Countries))
	setJoined(query, "os", string(filters.OS))
}

func applySKANFilters(query url.Values, filters SKANFilters) {
	setJoined(query, "campaignId", joinIDs(filters.CampaignIDs))
	setJoined(query, "bundleId", joinStrings(filters.BundleIDs))
	setJoined(query, "country", joinStrings(filters.Countries))
}

func applyOrdering(query url.Values, order Order, direction Direction) {
	setJoined(query, "order", string(order))
	setJoined(query, "direction", string(direction))
}

func setJoined(query url.Values, key, value string) {
	if value != "" {
		query.Set(key, value)
	}
}

func (client *Client) queryReport(ctx context.Context, operation string, prepared preparedReport, options ...socialhub.CallOption) (ReportPage, error) {
	call, err := validateCallOptions(operation, options)
	if err != nil {
		return ReportPage{}, err
	}
	query := cloneValues(prepared.query)
	query.Set("format", "json")
	request, err := client.api.NewRequest(ctx, http.MethodGet, prepared.path, query, nil, resolvedCallOptions(call)...)
	if err != nil {
		return ReportPage{}, withOperation(err, operation)
	}
	request.Header.Set("Accept", "application/json")
	var raw json.RawMessage
	if err := client.api.Do(request, &raw); err != nil {
		return ReportPage{}, withOperation(err, operation)
	}
	return client.decodeReportPage(raw, prepared.path, operation)
}

func (client *Client) decodeReportPage(raw json.RawMessage, path, operation string) (ReportPage, error) {
	data, next, err := decodeEnvelope(raw)
	if err != nil {
		return ReportPage{}, platformContractError(operation, err.Error())
	}
	rows := make([]ReportRow, len(data))
	for index, item := range data {
		if err := json.Unmarshal(item, &rows[index]); err != nil || !validReportRow(rows[index]) {
			return ReportPage{}, platformContractError(operation, "ironSource returned an invalid report row")
		}
	}
	cursor, err := client.cursorFromURL(next, path)
	if err != nil {
		return ReportPage{}, platformContractError(operation, "ironSource returned an invalid or cross-origin pagination link")
	}
	return ReportPage{Rows: rows, NextCursor: cursor, HasMore: cursor != ""}, nil
}

func (client *Client) decodeConversionValuePage(raw json.RawMessage, path, operation string) (ConversionValuePage, error) {
	data, next, err := decodeEnvelope(raw)
	if err != nil {
		return ConversionValuePage{}, platformContractError(operation, err.Error())
	}
	rows := make([]ConversionValueRow, len(data))
	for index, item := range data {
		row, err := decodeConversionValueRow(item)
		if err != nil {
			return ConversionValuePage{}, platformContractError(operation, "ironSource returned an invalid conversion-value row")
		}
		rows[index] = row
	}
	cursor, err := client.cursorFromURL(next, path)
	if err != nil {
		return ConversionValuePage{}, platformContractError(operation, "ironSource returned an invalid or cross-origin pagination link")
	}
	return ConversionValuePage{Rows: rows, NextCursor: cursor, HasMore: cursor != ""}, nil
}

func decodeEnvelope(raw json.RawMessage) ([]json.RawMessage, string, error) {
	var envelope struct {
		Data   json.RawMessage `json:"data"`
		Paging struct {
			Next string `json:"next"`
		} `json:"paging"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, "", errors.New("ironSource returned an invalid report envelope")
	}
	trimmed := bytes.TrimSpace(envelope.Data)
	if len(trimmed) == 0 || trimmed[0] != '[' {
		return nil, "", errors.New("ironSource report envelope omitted the data array")
	}
	var data []json.RawMessage
	if err := json.Unmarshal(trimmed, &data); err != nil {
		return nil, "", errors.New("ironSource report data is invalid")
	}
	return data, envelope.Paging.Next, nil
}

func decodeConversionValueRow(raw json.RawMessage) (ConversionValueRow, error) {
	var object map[string]json.RawMessage
	if err := json.Unmarshal(raw, &object); err != nil || object == nil || len(object) > 129 {
		return ConversionValueRow{}, errInvalidReportRow
	}
	conversionRaw, found := object["conversionValues"]
	if !found {
		return ConversionValueRow{}, errInvalidReportRow
	}
	delete(object, "conversionValues")
	fields := make(ReportRow, len(object))
	for key, value := range object {
		var decoded ReportValue
		if err := json.Unmarshal(value, &decoded); err != nil {
			return ConversionValueRow{}, errInvalidReportRow
		}
		fields[key] = decoded
	}
	if !validReportFields(fields, true) {
		return ConversionValueRow{}, errInvalidReportRow
	}
	var values map[string]json.RawMessage
	if err := json.Unmarshal(conversionRaw, &values); err != nil || values == nil || len(values) > 64 {
		return ConversionValueRow{}, errInvalidReportRow
	}
	conversionValues := make(map[uint8]int64, len(values))
	for key, rawValue := range values {
		parsed, err := strconv.ParseUint(key, 10, 8)
		if err != nil || parsed > 63 || strconv.FormatUint(parsed, 10) != key {
			return ConversionValueRow{}, errInvalidReportRow
		}
		var count int64
		if err := json.Unmarshal(rawValue, &count); err != nil || count < 0 {
			return ConversionValueRow{}, errInvalidReportRow
		}
		conversionValues[uint8(parsed)] = count
	}
	return ConversionValueRow{Fields: fields, ConversionValues: conversionValues}, nil
}

var errInvalidReportRow = errors.New("invalid report row")

func validReportRow(row ReportRow) bool {
	return validReportFields(row, false)
}

func validReportFields(row ReportRow, allowEmpty bool) bool {
	if row == nil || (!allowEmpty && len(row) == 0) || len(row) > 128 {
		return false
	}
	for key, value := range row {
		if !validOpaque(key, 128) || !value.Null && (len(value.Text) > 64<<10 || !utf8.ValidString(value.Text)) {
			return false
		}
	}
	return true
}

func (client *Client) cursorFromURL(value, path string) (string, error) {
	if value == "" {
		return "", nil
	}
	parsed, err := url.Parse(value)
	expectedPath := strings.TrimRight(client.baseURL.Path, "/") + "/" + strings.TrimLeft(path, "/")
	if err != nil || parsed.User != nil || parsed.Fragment != "" || parsed.Scheme != client.baseURL.Scheme ||
		!strings.EqualFold(parsed.Host, client.baseURL.Host) || parsed.Path != expectedPath {
		return "", errInvalidPagination
	}
	values := parsed.Query()["cursor"]
	if len(values) != 1 || !validOpaque(values[0], 8192) {
		return "", errInvalidPagination
	}
	return values[0], nil
}

var errInvalidPagination = errors.New("invalid pagination link")

func (client *Client) downloadCSV(
	ctx context.Context,
	operation string,
	prepared preparedReport,
	output io.Writer,
	download DownloadOptions,
	options ...socialhub.CallOption,
) (DownloadResult, error) {
	call, err := validateCallOptions(operation, options)
	if err != nil {
		return DownloadResult{}, err
	}
	if output == nil || download.MaxBytes < 0 {
		return DownloadResult{}, invalidArgument(operation, "output and a nonnegative maximum size are required")
	}
	maximum := download.MaxBytes
	if maximum == 0 {
		maximum = DefaultMaxCSVReportBytes
	}
	if call.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, call.Timeout)
		defer cancel()
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
	result := DownloadResult{StatusCode: response.StatusCode, ContentType: response.Header.Get("Content-Type")}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		body, readErr := io.ReadAll(io.LimitReader(response.Body, maxErrorResponseBytes+1))
		if readErr != nil {
			return DownloadResult{}, platformError(operation, socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, readErr)
		}
		if int64(len(body)) > maxErrorResponseBytes {
			return DownloadResult{}, platformContractError(operation, "ironSource error response exceeded the size limit")
		}
		return DownloadResult{}, withOperation(decodeHTTPErrorAt(response.StatusCode, response.Header, body, client.clock.Now()), operation)
	}
	if response.StatusCode == http.StatusNoContent {
		var extra [1]byte
		read, readErr := response.Body.Read(extra[:])
		if readErr != nil && !errors.Is(readErr, io.EOF) {
			return DownloadResult{}, platformError(operation, socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, readErr)
		}
		if read != 0 {
			return DownloadResult{}, platformContractError(operation, "ironSource returned a body with HTTP 204")
		}
		result.NoData = true
		return result, nil
	}
	if response.StatusCode != http.StatusOK {
		return DownloadResult{}, platformContractError(operation, "ironSource returned an unexpected successful CSV status")
	}
	if encoding := strings.ToLower(strings.TrimSpace(response.Header.Get("Content-Encoding"))); encoding != "" && encoding != "identity" {
		return DownloadResult{}, platformContractError(operation, "ironSource returned an unexpected CSV content encoding")
	}
	mediaType, _, err := mime.ParseMediaType(result.ContentType)
	if err != nil || !validCSVMediaType(mediaType) {
		return DownloadResult{}, platformContractError(operation, "ironSource returned an unexpected CSV content type")
	}
	cursor, err := client.cursorFromLink(response.Header.Get("Link"), prepared.path)
	if err != nil {
		return DownloadResult{}, platformContractError(operation, "ironSource returned an invalid or cross-origin CSV pagination link")
	}
	result.NextCursor, result.HasMore = cursor, cursor != ""
	written, rows, copyErr := copyCSVValidated(output, response.Body, maximum)
	result.BytesWritten, result.DataRows = written, rows
	if copyErr != nil {
		return result, reportCopyError(operation, copyErr)
	}
	return result, nil
}

func (client *Client) cursorFromLink(value, path string) (string, error) {
	var cursor string
	found := false
	for _, link := range linkheader.Parse(value) {
		if link.Rel != "next" {
			continue
		}
		if found {
			return "", errInvalidPagination
		}
		parsed, err := client.cursorFromURL(link.URL, path)
		if err != nil || parsed == "" {
			return "", errInvalidPagination
		}
		cursor, found = parsed, true
	}
	if !found && strings.Contains(strings.ToLower(value), "rel=\"next\"") {
		return "", errInvalidPagination
	}
	return cursor, nil
}

func validCSVMediaType(value string) bool {
	switch strings.ToLower(value) {
	case "text/csv", "application/csv", "application/octet-stream", "binary/octet-stream":
		return true
	default:
		return false
	}
}

func copyCSVValidated(output io.Writer, source io.Reader, maximum int64) (int64, int64, error) {
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
	if !validCSVHeader(header) {
		return sink.written, 0, errInvalidCSV
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

func validCSVHeader(header []string) bool {
	if len(header) == 0 || len(header) > 128 {
		return false
	}
	seen := make(map[string]struct{}, len(header))
	for _, name := range header {
		if !validOpaque(name, 128) {
			return false
		}
		if _, exists := seen[name]; exists {
			return false
		}
		seen[name] = struct{}{}
	}
	return true
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
		writer.err = err
		return written, err
	}
	if written != len(limited) {
		writer.err = io.ErrShortWrite
		return written, writer.err
	}
	if overflow {
		writer.err = errReportTooLarge
		return written, writer.err
	}
	return written, nil
}

func reportCopyError(operation string, err error) error {
	if errors.Is(err, errReportTooLarge) || errors.Is(err, errInvalidCSV) {
		return platformContractError(operation, err.Error())
	}
	return platformError(operation, socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
}

func cloneValues(input url.Values) url.Values {
	output := make(url.Values, len(input))
	for key, values := range input {
		output[key] = append([]string(nil), values...)
	}
	return output
}
