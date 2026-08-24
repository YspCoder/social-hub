package applovinmax

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strings"

	"social-hub/pkg/socialhub"
)

const maxErrorResponseBytes int64 = 1 << 20

type reportEnvelope[T any] struct {
	Code    int    `json:"code"`
	Count   int    `json:"count"`
	Results []T    `json:"results"`
	Message string `json:"message,omitempty"`
	Error   string `json:"error,omitempty"`
}

func (client *Client) RevenueReport(ctx context.Context, input RevenueReportRequest, options ...socialhub.CallOption) (RevenueReport, error) {
	prepared, err := prepareRevenueRequest(input, MaximumJSONReportLimit, client.clock.Now())
	if err != nil {
		return RevenueReport{}, err
	}
	var envelope reportEnvelope[RevenueRow]
	if err := client.getReportJSON(ctx, "revenue_report", "/maxReport", prepared, &envelope, options...); err != nil {
		return RevenueReport{}, err
	}
	if err := validateReportEnvelope("revenue_report", envelope.Code, envelope.Count, len(envelope.Results), envelope.Message, envelope.Error); err != nil {
		return RevenueReport{}, err
	}
	if !revenueRowsMatch(envelope.Results, input.Columns) {
		return RevenueReport{}, platformContractError("revenue_report", "AppLovin returned malformed or unexpected revenue columns")
	}
	return RevenueReport{Count: envelope.Count, Rows: envelope.Results}, nil
}

func (client *Client) DownloadRevenueReport(
	ctx context.Context,
	input RevenueReportRequest,
	output io.Writer,
	download DownloadOptions,
	options ...socialhub.CallOption,
) (DownloadResult, error) {
	prepared, err := prepareRevenueRequest(input, MaximumStreamReportLimit, client.clock.Now())
	if err != nil {
		return DownloadResult{}, err
	}
	return client.downloadReport(ctx, "revenue_report_download", "/maxReport", prepared, output, download, options...)
}

func (client *Client) CohortReport(ctx context.Context, input CohortReportRequest, options ...socialhub.CallOption) (CohortReport, error) {
	prepared, err := prepareCohortRequest(input, MaximumJSONReportLimit, client.clock.Now())
	if err != nil {
		return CohortReport{}, err
	}
	var envelope reportEnvelope[CohortRow]
	if err := client.getReportJSON(ctx, "cohort_report", cohortPath(input.Kind), prepared, &envelope, options...); err != nil {
		return CohortReport{}, err
	}
	if err := validateReportEnvelope("cohort_report", envelope.Code, envelope.Count, len(envelope.Results), envelope.Message, envelope.Error); err != nil {
		return CohortReport{}, err
	}
	if !cohortRowsMatch(envelope.Results, input.Columns) {
		return CohortReport{}, platformContractError("cohort_report", "AppLovin returned malformed or unexpected cohort columns")
	}
	return CohortReport{Count: envelope.Count, Rows: envelope.Results}, nil
}

func (client *Client) DownloadCohortReport(
	ctx context.Context,
	input CohortReportRequest,
	output io.Writer,
	download DownloadOptions,
	options ...socialhub.CallOption,
) (DownloadResult, error) {
	prepared, err := prepareCohortRequest(input, MaximumStreamReportLimit, client.clock.Now())
	if err != nil {
		return DownloadResult{}, err
	}
	return client.downloadReport(ctx, "cohort_report_download", cohortPath(input.Kind), prepared, output, download, options...)
}

func (client *Client) RequestUserLevelReport(ctx context.Context, input UserLevelReportRequest, options ...socialhub.CallOption) (UserLevelReport, error) {
	const operation = "user_level_report"
	query, err := prepareUserLevelRequest(input, client.clock.Now())
	if err != nil {
		return UserLevelReport{}, err
	}
	callOptions, err := supportedCallOptions(operation, options)
	if err != nil {
		return UserLevelReport{}, err
	}
	request, err := client.api.NewRequest(ctx, http.MethodGet, "/max/userAdRevenueReport", query, nil, forwardCallOptions(callOptions)...)
	if err != nil {
		return UserLevelReport{}, withOperation(err, operation)
	}
	var report UserLevelReport
	if err := client.api.Do(request, &report); err != nil {
		return UserLevelReport{}, withOperation(err, operation)
	}
	if report.Status != http.StatusOK {
		return UserLevelReport{}, decodeBusinessError(operation, report.Status, "AppLovin user-level report is not available")
	}
	if !client.validDownloadLocation(report.URL) || !client.validDownloadLocation(report.AdRevenueReportURL) ||
		report.FBEstimatedRevenueURL != "" && !client.validDownloadLocation(report.FBEstimatedRevenueURL) {
		return UserLevelReport{}, platformContractError(operation, "AppLovin returned an unsafe user-level report URL")
	}
	return report, nil
}

func (client *Client) DownloadUserLevelReport(
	ctx context.Context,
	report UserLevelReport,
	variant UserLevelReportVariant,
	output io.Writer,
	download DownloadOptions,
	options ...socialhub.CallOption,
) (DownloadResult, error) {
	const operation = "user_level_report_download"
	if report.Status != http.StatusOK {
		return DownloadResult{}, invalidArgument(operation, "user-level report status must be 200")
	}
	var location string
	switch variant {
	case UserLevelWithoutMetaEstimate:
		location = report.URL
	case UserLevelWithMetaEstimate:
		location = report.AdRevenueReportURL
	case UserLevelMetaEstimateOnly:
		location = report.FBEstimatedRevenueURL
	default:
		return DownloadResult{}, invalidArgument(operation, "user-level report variant is invalid")
	}
	if !client.validDownloadLocation(location) {
		return DownloadResult{}, invalidArgument(operation, "user-level report URL is missing or unsafe")
	}
	return client.downloadLocation(ctx, operation, location, output, download, options...)
}

func (client *Client) getReportJSON(
	ctx context.Context,
	operation, path string,
	prepared preparedReportQuery,
	output any,
	options ...socialhub.CallOption,
) error {
	prepared.values = cloneValues(prepared.values)
	prepared.values.Set("format", "json")
	request, err := client.newReportRequest(ctx, operation, path, prepared, options...)
	if err != nil {
		return withOperation(err, operation)
	}
	request.Header.Set("Accept", "application/json")
	if err := client.api.Do(request, output); err != nil {
		return withOperation(err, operation)
	}
	return nil
}

func (client *Client) newReportRequest(ctx context.Context, operation, path string, prepared preparedReportQuery, options ...socialhub.CallOption) (*http.Request, error) {
	callOptions, err := supportedCallOptions(operation, options)
	if err != nil {
		return nil, err
	}
	request, err := client.api.NewRequest(ctx, http.MethodGet, path, prepared.values, nil, forwardCallOptions(callOptions)...)
	if err != nil {
		return nil, err
	}
	if len(prepared.sorts) == 0 {
		return request, nil
	}
	rawQuery := request.URL.Query().Encode()
	for _, sort := range prepared.sorts {
		if rawQuery != "" {
			rawQuery += "&"
		}
		rawQuery += url.QueryEscape("sort_"+sort.column) + "=" + url.QueryEscape(string(sort.order))
	}
	request.URL.RawQuery = rawQuery
	return request, nil
}

func (client *Client) downloadReport(
	ctx context.Context,
	operation, path string,
	prepared preparedReportQuery,
	output io.Writer,
	download DownloadOptions,
	options ...socialhub.CallOption,
) (DownloadResult, error) {
	if output == nil || download.MaxBytes < 0 {
		return DownloadResult{}, invalidArgument(operation, "output and a nonnegative maximum size are required")
	}
	prepared.values = cloneValues(prepared.values)
	prepared.values.Set("format", "csv")
	request, err := client.newReportRequest(ctx, operation, path, prepared, options...)
	if err != nil {
		return DownloadResult{}, withOperation(err, operation)
	}
	request.Header.Set("Accept", "text/csv")
	request.Header.Set("Accept-Encoding", "identity")
	return client.executeDownload(request, operation, output, download)
}

func (client *Client) downloadLocation(
	ctx context.Context,
	operation, location string,
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
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, location, nil)
	if err != nil {
		return DownloadResult{}, invalidArgument(operation, "user-level report URL is invalid")
	}
	request.Header.Set("Accept", "text/csv, application/octet-stream")
	request.Header.Set("Accept-Encoding", "identity")
	return client.executeDownload(request, operation, output, download)
}

func (client *Client) executeDownload(request *http.Request, operation string, output io.Writer, download DownloadOptions) (DownloadResult, error) {
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
		return DownloadResult{}, platformContractError(operation, "AppLovin returned an unexpected successful download status")
	}
	if encoding := strings.ToLower(strings.TrimSpace(response.Header.Get("Content-Encoding"))); encoding != "" && encoding != "identity" {
		return DownloadResult{}, platformContractError(operation, "AppLovin returned an unexpected content encoding")
	}
	mediaType, _, mediaErr := mime.ParseMediaType(response.Header.Get("Content-Type"))
	if mediaErr == nil && strings.EqualFold(mediaType, "application/json") {
		body, readErr := io.ReadAll(io.LimitReader(response.Body, maxErrorResponseBytes+1))
		if readErr != nil {
			return DownloadResult{}, platformError(operation, socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, readErr)
		}
		if int64(len(body)) > maxErrorResponseBytes {
			return DownloadResult{}, platformContractError(operation, "AppLovin JSON report response exceeded the size limit")
		}
		return DownloadResult{}, decodeEmbeddedReportError(operation, body)
	}
	if mediaErr != nil || !validCSVMediaType(mediaType) {
		return DownloadResult{}, platformContractError(operation, "AppLovin returned an unexpected report content type")
	}
	maximum := download.MaxBytes
	if maximum == 0 {
		maximum = DefaultMaxReportBytes
	}
	written, copyErr := copyBounded(output, response.Body, maximum)
	if copyErr != nil {
		if errors.Is(copyErr, errReportTooLarge) {
			return DownloadResult{}, platformContractError(operation, "AppLovin report exceeded the configured download size limit")
		}
		var writerErr *outputError
		if errors.As(copyErr, &writerErr) {
			return DownloadResult{}, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, writerErr.err)
		}
		return DownloadResult{}, platformError(operation, socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, copyErr)
	}
	return DownloadResult{
		StatusCode: response.StatusCode, BytesWritten: written, ContentType: boundedHeader(response.Header.Get("Content-Type"), 512),
		ETag: boundedHeader(response.Header.Get("ETag"), 512), LastModified: boundedHeader(response.Header.Get("Last-Modified"), 512),
	}, nil
}

func validateReportEnvelope(operation string, code, count, rows int, messages ...string) error {
	if code != http.StatusOK {
		return decodeBusinessError(operation, code, firstNonEmpty(messages...))
	}
	if count < 0 || count != rows {
		return platformContractError(operation, "AppLovin returned inconsistent report metadata")
	}
	return nil
}

func (client *Client) validDownloadLocation(value string) bool {
	if value == "" || len(value) > 16_384 {
		return false
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme != "https" || parsed.User != nil || parsed.Fragment != "" || parsed.Host == "" || parsed.Path == "" || parsed.RawQuery == "" {
		return false
	}
	_, allowed := client.downloadOrigins[normalizedOrigin(value)]
	return allowed
}

func validCSVMediaType(value string) bool {
	switch strings.ToLower(value) {
	case "text/csv", "application/csv", "application/octet-stream", "binary/octet-stream":
		return true
	default:
		return false
	}
}

func cloneValues(input url.Values) url.Values {
	output := make(url.Values, len(input))
	for key, values := range input {
		output[key] = append([]string(nil), values...)
	}
	return output
}

var errReportTooLarge = errors.New("report exceeded configured size limit")

type outputError struct{ err error }

func (err *outputError) Error() string { return err.err.Error() }
func (err *outputError) Unwrap() error { return err.err }

func copyBounded(output io.Writer, source io.Reader, maximum int64) (int64, error) {
	if maximum <= 0 {
		return 0, errReportTooLarge
	}
	limited := io.LimitReader(source, maximum+1)
	written, err := io.Copy(&boundedWriter{output: output, maximum: maximum}, limited)
	if err != nil {
		return minInt64(written, maximum), err
	}
	if written > maximum {
		return maximum, errReportTooLarge
	}
	return written, nil
}

type boundedWriter struct {
	output  io.Writer
	maximum int64
	written int64
}

func (writer *boundedWriter) Write(data []byte) (int, error) {
	remaining := writer.maximum - writer.written
	if remaining <= 0 {
		return 0, errReportTooLarge
	}
	overflow := int64(len(data)) > remaining
	if int64(len(data)) > remaining {
		data = data[:remaining]
	}
	written, err := writer.output.Write(data)
	writer.written += int64(written)
	if err != nil {
		return written, &outputError{err: err}
	}
	if written != len(data) {
		return written, &outputError{err: io.ErrShortWrite}
	}
	if overflow {
		return written, errReportTooLarge
	}
	return written, nil
}

func minInt64(left, right int64) int64 {
	if left < right {
		return left
	}
	return right
}

func decodeEmbeddedReportError(operation string, body []byte) error {
	var envelope struct {
		Code    int    `json:"code"`
		Status  int    `json:"status"`
		Message string `json:"message"`
		Error   string `json:"error"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return platformContractError(operation, "AppLovin returned JSON instead of a CSV report")
	}
	code := envelope.Code
	if code == 0 {
		code = envelope.Status
	}
	return decodeBusinessError(operation, code, firstNonEmpty(envelope.Message, envelope.Error))
}
