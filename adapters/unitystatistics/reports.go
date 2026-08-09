package unitystatistics

import (
	"compress/gzip"
	"compress/zlib"
	"context"
	"encoding/csv"
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
	errInvalidEOF     = errors.New("invalid CSV EOF marker")
)

func (client *Client) DownloadAcquisitionsReport(
	ctx context.Context,
	input AcquisitionsReportRequest,
	output io.Writer,
	download DownloadOptions,
	options ...socialhub.CallOption,
) (ReportResult, error) {
	query, format, err := prepareAcquisitionsRequest(input)
	if err != nil {
		return ReportResult{}, err
	}
	return client.downloadReport(ctx, "acquisitions_report", client.reportPath("acquisitions"), query, format, input.EOFMarker, output, download, options...)
}

func (client *Client) DownloadSKANReport(
	ctx context.Context,
	input SKANReportRequest,
	output io.Writer,
	download DownloadOptions,
	options ...socialhub.CallOption,
) (ReportResult, error) {
	query, format, err := prepareSKANRequest(input)
	if err != nil {
		return ReportResult{}, err
	}
	return client.downloadReport(ctx, "skan_report", client.reportPath("skan"), query, format, input.EOFMarker, output, download, options...)
}

func (client *Client) downloadReport(
	ctx context.Context,
	operation, path string,
	query url.Values,
	format ReportFormat,
	verifyEOF bool,
	output io.Writer,
	download DownloadOptions,
	options ...socialhub.CallOption,
) (ReportResult, error) {
	compression, ok := normalizedCompression(download.Compression)
	if output == nil || download.MaxBytes < 0 || !ok {
		return ReportResult{}, invalidArgument(operation, "output, a nonnegative maximum size, and a supported compression are required")
	}
	maximum := download.MaxBytes
	if maximum == 0 {
		maximum = DefaultMaxReportBytes
	}
	request, err := client.api.NewRequest(ctx, http.MethodGet, path, query, nil, options...)
	if err != nil {
		return ReportResult{}, withOperation(err, operation)
	}
	request.Header.Set("Accept", expectedMediaType(format))
	request.Header.Set("Accept-Encoding", string(compression))
	response, err := client.httpClient.Do(request)
	if err != nil {
		return ReportResult{}, platformError(operation, socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
	}
	defer response.Body.Close()

	result := ReportResult{
		StatusCode: response.StatusCode, Format: format,
		ContentType: response.Header.Get("Content-Type"), ContentEncoding: normalizedContentEncoding(response.Header.Get("Content-Encoding")),
		RateLimitPolicy: boundedMessage(headerValue(response.Header, "RateLimit-Policy"), 512),
		RateLimit:       boundedMessage(headerValue(response.Header, "RateLimit"), 512),
		UnityRateLimit:  boundedMessage(headerValue(response.Header, "Unity-RateLimit"), 1024),
	}
	source, closeDecoder, err := decodedResponseBody(response.Body, result.ContentEncoding, compression)
	if err != nil {
		return ReportResult{}, platformContractError(operation, err.Error())
	}
	if closeDecoder != nil {
		defer closeDecoder.Close()
	}

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		body, readErr := io.ReadAll(io.LimitReader(source, maxErrorResponseBytes+1))
		if readErr != nil {
			return ReportResult{}, platformError(operation, socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, readErr)
		}
		if int64(len(body)) > maxErrorResponseBytes {
			return ReportResult{}, platformContractError(operation, "Unity report error response exceeded size limit")
		}
		return ReportResult{}, withOperation(decodeHTTPError(response.StatusCode, response.Header, body), operation)
	}
	if response.StatusCode == http.StatusNoContent {
		var extra [1]byte
		read, readErr := source.Read(extra[:])
		if readErr != nil && !errors.Is(readErr, io.EOF) {
			return ReportResult{}, platformError(operation, socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, readErr)
		}
		if read != 0 {
			return ReportResult{}, platformContractError(operation, "Unity returned a body with HTTP 204")
		}
		result.NoData = true
		return result, nil
	}
	if response.StatusCode != http.StatusOK {
		return ReportResult{}, platformContractError(operation, "Unity returned an unexpected successful report status")
	}
	mediaType, _, err := mime.ParseMediaType(result.ContentType)
	if err != nil || !strings.EqualFold(mediaType, expectedMediaType(format)) {
		return ReportResult{}, platformContractError(operation, "Unity returned an unexpected report content type")
	}

	if verifyEOF {
		written, rows, copyErr := copyCSVWithEOF(output, source, maximum)
		result.BytesWritten, result.DataRows = written, rows
		if copyErr != nil {
			return ReportResult{}, reportCopyError(operation, copyErr)
		}
		result.EOFVerified = true
		return result, nil
	}
	written, copyErr := copyBounded(output, source, maximum)
	result.BytesWritten = written
	if copyErr != nil {
		return ReportResult{}, reportCopyError(operation, copyErr)
	}
	return result, nil
}

func expectedMediaType(format ReportFormat) string {
	if format == FormatJSON {
		return "application/json"
	}
	return "text/csv"
}

func normalizedContentEncoding(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "identity" {
		return ""
	}
	return value
}

func decodedResponseBody(body io.Reader, encoding string, requested Compression) (io.Reader, io.Closer, error) {
	if encoding == "" {
		return body, nil, nil
	}
	if encoding != string(requested) {
		return nil, nil, fmt.Errorf("Unity returned unexpected content encoding %q", encoding)
	}
	switch encoding {
	case string(CompressionGzip):
		reader, err := gzip.NewReader(body)
		if err != nil {
			return nil, nil, errors.New("Unity returned an invalid gzip stream")
		}
		return reader, reader, nil
	case string(CompressionDeflate):
		reader, err := zlib.NewReader(body)
		if err != nil {
			return nil, nil, errors.New("Unity returned an invalid deflate stream")
		}
		return reader, reader, nil
	default:
		return nil, nil, fmt.Errorf("Unity returned unsupported content encoding %q", encoding)
	}
}

func copyBounded(output io.Writer, source io.Reader, maximum int64) (int64, error) {
	sink := &boundedOutput{output: output, maximum: maximum}
	_, err := io.Copy(sink, source)
	if sink.err != nil {
		return sink.written, sink.err
	}
	return sink.written, err
}

func copyCSVWithEOF(output io.Writer, source io.Reader, maximum int64) (int64, int64, error) {
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
	if len(header) < 2 {
		return sink.written, 0, errInvalidCSV
	}
	var rows int64
	marker := false
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
		if len(record) > 0 && record[0] == "#__EOF__" {
			if marker || !validEOFRecord(record, len(header), rows) {
				return sink.written, rows, errInvalidEOF
			}
			marker = true
			continue
		}
		if marker {
			return sink.written, rows, errInvalidEOF
		}
		if len(record) != len(header) {
			return sink.written, rows, errInvalidCSV
		}
		rows++
	}
	if sink.err != nil {
		return sink.written, rows, sink.err
	}
	if !marker {
		return sink.written, rows, errInvalidEOF
	}
	return sink.written, rows, nil
}

func validEOFRecord(record []string, columns int, rows int64) bool {
	if len(record) != columns || len(record) < 2 || !strings.HasPrefix(record[1], "rows=") {
		return false
	}
	count, err := strconv.ParseInt(strings.TrimPrefix(record[1], "rows="), 10, 64)
	if err != nil || count != rows {
		return false
	}
	for _, value := range record[2:] {
		if value != "" {
			return false
		}
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
		return platformContractError(operation, "Unity report exceeded the configured download size limit")
	}
	if errors.Is(err, errInvalidCSV) || errors.Is(err, errInvalidEOF) {
		return platformContractError(operation, err.Error())
	}
	var outputErr *outputError
	if errors.As(err, &outputErr) {
		return platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, outputErr.err)
	}
	return platformError(operation, socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
}
