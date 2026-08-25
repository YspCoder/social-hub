package yandexdirect

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"time"

	"social-hub/pkg/socialhub"
)

const maxReportErrorBytes int64 = 1 << 20

func (client *Client) GenerateReport(
	ctx context.Context,
	definition ReportDefinition,
	output io.Writer,
	reportOptions ReportOptions,
	options ...socialhub.CallOption,
) (ReportResult, error) {
	const operation = "report_generate"
	if reportOptions.ProcessingMode == "" {
		reportOptions.ProcessingMode = ProcessingAuto
	}
	if output == nil || !validReport(definition, reportOptions) {
		return ReportResult{}, invalidArgument(operation, "report definition, processing mode, output, or size limit is invalid")
	}
	if err := client.requireAccess(operation); err != nil {
		return ReportResult{}, err
	}
	callOptions, err := prepareCallOptions(operation, options)
	if err != nil {
		return ReportResult{}, err
	}
	encoded, err := json.Marshal(struct {
		Params ReportDefinition `json:"params"`
	}{Params: definition})
	if err != nil {
		return ReportResult{}, platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	if len(encoded) > maxRequestBytes {
		return ReportResult{}, invalidArgument(operation, "report request JSON exceeds 1 MiB")
	}
	if callOptions.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, callOptions.Timeout)
		defer cancel()
	}
	request, err := client.reportsAPI.NewRequest(ctx, http.MethodPost, "reports", nil, bytes.NewReader(encoded))
	if err != nil {
		return ReportResult{}, withOperation(err, operation)
	}
	request.Header.Set("Accept", "text/tab-separated-values, text/plain, application/octet-stream")
	request.Header.Set("Accept-Encoding", "identity")
	request.Header.Set("Content-Type", "application/json; charset=utf-8")
	request.Header.Set("processingMode", string(reportOptions.ProcessingMode))
	if reportOptions.SkipReportHeader {
		request.Header.Set("skipReportHeader", "true")
	}
	if reportOptions.SkipColumnHeader {
		request.Header.Set("skipColumnHeader", "true")
	}
	if reportOptions.SkipReportSummary {
		request.Header.Set("skipReportSummary", "true")
	}
	client.applyHeaders(request)
	response, err := client.httpClient.Do(request)
	if err != nil {
		return ReportResult{}, platformError(operation, socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
	}
	defer response.Body.Close()
	result := ReportResult{
		HTTPStatus: response.StatusCode,
		RequestID:  responseRequestID(client.requestIDValues, response.Header.Get("RequestId")),
		RetryAfter: retryDelay(response.Header, client.clock.Now()), ReportsInQueue: reportQueueCount(response.Header),
		ContentType: boundedOpaque(response.Header.Get("Content-Type"), 512),
	}
	switch response.StatusCode {
	case http.StatusCreated:
		result.Status = ReportQueued
		if err := consumeQueuedReportResponse(response.Body, response.StatusCode, response.Header, client.clock.Now(), client.requestIDValues); err != nil {
			return result, err
		}
		return result, nil
	case http.StatusAccepted:
		result.Status = ReportProcessing
		if err := consumeQueuedReportResponse(response.Body, response.StatusCode, response.Header, client.clock.Now(), client.requestIDValues); err != nil {
			return result, err
		}
		return result, nil
	case http.StatusOK:
		result.Status = ReportReady
	default:
		body, readErr := io.ReadAll(io.LimitReader(response.Body, maxReportErrorBytes+1))
		if readErr != nil {
			return result, platformError(operation, socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, readErr)
		}
		if int64(len(body)) > maxReportErrorBytes {
			return result, platformContractError(operation, "Yandex report error response exceeded 1 MiB")
		}
		return result, withOperation(decodeHTTPError(response.StatusCode, response.Header, body, client.clock.Now(), client.requestIDValues...), operation)
	}
	if encoding := strings.ToLower(strings.TrimSpace(response.Header.Get("Content-Encoding"))); encoding != "" && encoding != "identity" {
		return result, platformContractError(operation, "Yandex returned an unexpected report content encoding")
	}
	mediaType, _, mediaTypeErr := mime.ParseMediaType(response.Header.Get("Content-Type"))
	if mediaTypeErr != nil || !validReportMediaType(mediaType) {
		return result, platformContractError(operation, "Yandex returned an unexpected report content type")
	}
	maximum := reportOptions.MaxBytes
	if maximum == 0 {
		maximum = DefaultMaxReportBytes
	}
	if response.ContentLength > maximum {
		return result, platformContractError(operation, "Yandex report exceeds the configured size limit")
	}
	written, err := copyBounded(output, response.Body, maximum)
	result.BytesWritten = written
	if err != nil {
		if errors.Is(err, errReportTooLarge) {
			return result, platformContractError(operation, "Yandex report exceeds the configured size limit")
		}
		return result, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	return result, nil
}

func consumeQueuedReportResponse(reader io.Reader, status int, header http.Header, now time.Time, requestIDValues []string) error {
	body, err := io.ReadAll(io.LimitReader(reader, maxReportErrorBytes+1))
	if err != nil {
		return platformError("report_generate", socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
	}
	if int64(len(body)) > maxReportErrorBytes {
		return platformContractError("report_generate", "Yandex queued report response exceeded 1 MiB")
	}
	if len(bytes.TrimSpace(body)) == 0 {
		return nil
	}
	var envelope errorEnvelope
	if json.Unmarshal(body, &envelope) == nil && envelope.Error != nil {
		return apiErrorValue("report_generate", status, header, *envelope.Error, now, requestIDValues...)
	}
	// Yandex documents the status and retry headers for 201/202 but does not
	// guarantee an empty entity body. A bounded non-error body is informational.
	return nil
}

func reportQueueCount(header http.Header) int {
	value, err := strconv.Atoi(boundedOpaque(header.Get("reportsInQueue"), 32))
	if err != nil || value < 0 || value > 5 {
		return 0
	}
	return value
}

var errReportTooLarge = errors.New("report exceeded configured size limit")

func copyBounded(output io.Writer, source io.Reader, maximum int64) (int64, error) {
	if maximum <= 0 {
		return 0, errReportTooLarge
	}
	written, err := io.Copy(output, io.LimitReader(source, maximum))
	if err != nil {
		return written, err
	}
	var extra [1]byte
	read, err := source.Read(extra[:])
	if read > 0 {
		return written, errReportTooLarge
	}
	if err != nil && !errors.Is(err, io.EOF) {
		return written, err
	}
	return written, nil
}
