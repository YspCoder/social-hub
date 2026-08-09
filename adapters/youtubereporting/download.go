package youtubereporting

import (
	"compress/gzip"
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"

	"social-hub/pkg/socialhub"
)

const maxErrorResponseBytes int64 = 1 << 20

func (client *Client) DownloadReport(
	ctx context.Context,
	jobID, reportID string,
	output io.Writer,
	download DownloadOptions,
	options ...socialhub.CallOption,
) (DownloadResult, error) {
	const operation = "report_download"
	if !validJobID(jobID) || !validReportID(reportID) || output == nil || download.MaxBytes < 0 {
		return DownloadResult{}, invalidArgument(operation, "job ID, report ID, output, and a nonnegative maximum size are required")
	}
	if err := client.requireReportingScope(operation); err != nil {
		return DownloadResult{}, err
	}
	report, err := client.GetReport(ctx, jobID, reportID, options...)
	if err != nil {
		return DownloadResult{}, withOperation(err, operation)
	}
	downloadPath, query, err := client.validDownloadURL(report.DownloadURL)
	if err != nil {
		return DownloadResult{}, platformContractError(operation, "YouTube returned an unsafe report download URL")
	}
	request, err := client.api.NewRequest(ctx, http.MethodGet, downloadPath, query, nil, options...)
	if err != nil {
		return DownloadResult{}, withOperation(err, operation)
	}
	request.Header.Set("Accept", "text/csv, application/octet-stream")
	if download.Gzip {
		request.Header.Set("Accept-Encoding", "gzip")
	} else {
		request.Header.Set("Accept-Encoding", "identity")
	}
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
			return DownloadResult{}, platformContractError(operation, "YouTube download error response exceeded size limit")
		}
		return DownloadResult{}, withOperation(decodeHTTPError(response.StatusCode, response.Header, body), operation)
	}
	if response.StatusCode != http.StatusOK {
		return DownloadResult{}, platformContractError(operation, "YouTube returned an unexpected successful download status")
	}

	maximum := download.MaxBytes
	if maximum == 0 {
		maximum = DefaultMaxDownloadBytes
	}
	encoding := strings.ToLower(strings.TrimSpace(response.Header.Get("Content-Encoding")))
	var source io.Reader = response.Body
	if encoding != "" && encoding != "identity" {
		if !download.Gzip || encoding != "gzip" {
			return DownloadResult{}, platformContractError(operation, "YouTube returned an unexpected content encoding")
		}
		compressed, err := gzip.NewReader(response.Body)
		if err != nil {
			return DownloadResult{}, platformContractError(operation, "YouTube returned an invalid gzip stream")
		}
		defer compressed.Close()
		source = compressed
	}
	written, err := copyBounded(output, source, maximum)
	if err != nil {
		if errors.Is(err, errDownloadTooLarge) {
			return DownloadResult{}, platformContractError(operation, "YouTube report exceeded the configured download size limit")
		}
		return DownloadResult{}, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	return DownloadResult{
		Report: report, BytesWritten: written, ContentType: response.Header.Get("Content-Type"),
		ContentEncoding: encoding, ETag: response.Header.Get("ETag"), LastModified: response.Header.Get("Last-Modified"),
	}, nil
}

var errDownloadTooLarge = errors.New("download exceeded size limit")
var errUnsafeDownloadURL = errors.New("unsafe download URL")

func copyBounded(output io.Writer, source io.Reader, maximum int64) (int64, error) {
	written, err := io.Copy(output, io.LimitReader(source, maximum))
	if err != nil {
		return written, err
	}
	var extra [1]byte
	read, err := source.Read(extra[:])
	if read > 0 {
		return written, errDownloadTooLarge
	}
	if err != nil && !errors.Is(err, io.EOF) {
		return written, err
	}
	return written, nil
}

func (client *Client) validDownloadURL(value string) (string, url.Values, error) {
	parsed, err := url.Parse(value)
	if err != nil || parsed.User != nil || parsed.Fragment != "" || parsed.RawPath != "" || client.baseURL == nil ||
		parsed.Scheme != client.baseURL.Scheme || !strings.EqualFold(parsed.Host, client.baseURL.Host) {
		return "", nil, errUnsafeDownloadURL
	}
	basePath := strings.TrimRight(client.baseURL.Path, "/")
	prefix := basePath + "/v1/media/"
	if !strings.HasPrefix(parsed.Path, prefix) || parsed.Path == prefix || path.Clean(parsed.Path) != parsed.Path || strings.Contains(parsed.Path, "\\") {
		return "", nil, errUnsafeDownloadURL
	}
	query := parsed.Query()
	if len(query) != 1 || len(query["alt"]) != 1 || query.Get("alt") != "media" {
		return "", nil, errUnsafeDownloadURL
	}
	return strings.TrimPrefix(parsed.Path, basePath), query, nil
}
