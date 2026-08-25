package naversearchads

import (
	"context"
	"errors"
	"io"
	"mime"
	"net/http"
	"net/url"
	"path"
	"strings"

	"social-hub/pkg/socialhub"
)

const maxDownloadErrorBytes int64 = 1 << 20

func (client *Client) ListStatReports(ctx context.Context, options ...socialhub.CallOption) ([]StatReport, error) {
	const operation = "stat_reports_list"
	var reports []StatReport
	if err := client.getJSON(ctx, operation, "/stat-reports", nil, &reports, options...); err != nil {
		return nil, err
	}
	if len(reports) > 100_000 {
		return nil, platformContractError(operation, "NAVER returned too many Stat Report jobs")
	}
	seen := make(map[int64]struct{}, len(reports))
	for index := range reports {
		if err := validateStatReport(operation, &reports[index], 0); err != nil {
			return nil, err
		}
		if _, exists := seen[reports[index].ID]; exists {
			return nil, platformContractError(operation, "NAVER returned duplicate Stat Report job IDs")
		}
		seen[reports[index].ID] = struct{}{}
	}
	return reports, nil
}

func (client *Client) CreateStatReport(ctx context.Context, input CreateStatReportRequest, options ...socialhub.CallOption) (*StatReport, error) {
	const operation = "stat_report_create"
	if !validStatReportType(input.Type) || !validReportDate(input.Date) {
		return nil, invalidArgument(operation, "Stat Report type or YYYYMMDD date is invalid")
	}
	payload := struct {
		Type StatReportType `json:"reportTp"`
		Date StatReportDate `json:"statDt"`
	}{Type: input.Type, Date: input.Date}
	var report StatReport
	if err := client.writeJSON(ctx, operation, http.MethodPost, "/stat-reports", nil, payload, &report, options...); err != nil {
		return nil, err
	}
	if err := validateStatReport(operation, &report, 0); err != nil {
		return nil, outcomeUnknownError(operation, err)
	}
	if report.Type != input.Type || !reportDateMatches(input.Date, report.Date) {
		return nil, outcomeUnknownError(operation, platformContractError(operation, "created Stat Report did not match the request"))
	}
	return &report, nil
}

func (client *Client) GetStatReport(ctx context.Context, id int64, options ...socialhub.CallOption) (*StatReport, error) {
	const operation = "stat_report_get"
	if id <= 0 {
		return nil, invalidArgument(operation, "Stat Report job ID must be positive")
	}
	var report StatReport
	if err := client.getJSON(ctx, operation, "/stat-reports/"+formatInt64(id), nil, &report, options...); err != nil {
		return nil, err
	}
	if err := validateStatReport(operation, &report, id); err != nil {
		return nil, err
	}
	return &report, nil
}

func (client *Client) DeleteStatReport(ctx context.Context, id int64, options ...socialhub.CallOption) error {
	const operation = "stat_report_delete"
	if id <= 0 {
		return invalidArgument(operation, "Stat Report job ID must be positive")
	}
	prepared, err := prepareCallOptions(operation, options)
	if err != nil {
		return err
	}
	if _, err := client.GetStatReport(ctx, id, prepared...); err != nil {
		return withOperation(err, operation)
	}
	return client.delete(ctx, operation, "/stat-reports/"+formatInt64(id), prepared...)
}

func (client *Client) DeleteAllStatReports(ctx context.Context, options ...socialhub.CallOption) error {
	const operation = "stat_reports_delete_all"
	prepared, err := prepareCallOptions(operation, options)
	if err != nil {
		return err
	}
	if _, err := client.ListStatReports(ctx, prepared...); err != nil {
		return withOperation(err, operation)
	}
	return client.delete(ctx, operation, "/stat-reports", prepared...)
}

func (client *Client) DownloadStatReport(
	ctx context.Context,
	id int64,
	output io.Writer,
	download DownloadOptions,
	options ...socialhub.CallOption,
) (DownloadResult, error) {
	const operation = "stat_report_download"
	if id <= 0 || output == nil || download.MaxBytes < 0 {
		return DownloadResult{}, invalidArgument(operation, "positive job ID, output, and a nonnegative maximum size are required")
	}
	prepared, err := prepareCallOptions(operation, options)
	if err != nil {
		return DownloadResult{}, err
	}
	report, err := client.GetStatReport(ctx, id, prepared...)
	if err != nil {
		return DownloadResult{}, withOperation(err, operation)
	}
	if report.Status != ReportBuilt {
		return DownloadResult{}, invalidArgument(operation, "Stat Report must have BUILT status before download")
	}
	downloadPath, query, err := client.validDownloadURL(report.DownloadURL)
	if err != nil {
		return DownloadResult{}, platformContractError(operation, "NAVER returned an unsafe Stat Report download URL")
	}
	request, err := client.api.NewRequest(ctx, http.MethodGet, downloadPath, query, nil, prepared...)
	if err != nil {
		return DownloadResult{}, withOperation(err, operation)
	}
	request.Header.Set("Accept", "text/tab-separated-values, text/csv, application/octet-stream")
	request.Header.Set("Accept-Encoding", "identity")
	response, err := client.httpClient.Do(request)
	if err != nil {
		return DownloadResult{}, platformError(operation, socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		body, readErr := io.ReadAll(io.LimitReader(response.Body, maxDownloadErrorBytes+1))
		if readErr != nil {
			return DownloadResult{}, platformError(operation, socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, readErr)
		}
		if int64(len(body)) > maxDownloadErrorBytes {
			return DownloadResult{}, platformContractError(operation, "NAVER download error response exceeded the size limit")
		}
		return DownloadResult{}, withOperation(client.decodeError(response.StatusCode, response.Header, body), operation)
	}
	if response.StatusCode != http.StatusOK {
		return DownloadResult{}, platformContractError(operation, "NAVER returned an unexpected successful download status")
	}
	if encoding := strings.ToLower(strings.TrimSpace(response.Header.Get("Content-Encoding"))); encoding != "" && encoding != "identity" {
		return DownloadResult{}, platformContractError(operation, "NAVER returned an unexpected report content encoding")
	}
	mediaType, _, mediaErr := mime.ParseMediaType(response.Header.Get("Content-Type"))
	if mediaErr == nil && !validReportMediaType(mediaType) {
		return DownloadResult{}, platformContractError(operation, "NAVER returned an unexpected report content type")
	}
	maximum := download.MaxBytes
	if maximum == 0 {
		maximum = DefaultMaxReportBytes
	}
	written, copyErr := copyBounded(output, response.Body, maximum)
	if copyErr != nil {
		if errors.Is(copyErr, errReportTooLarge) {
			return DownloadResult{}, platformContractError(operation, "NAVER Stat Report exceeded the configured download size limit")
		}
		return DownloadResult{}, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, copyErr)
	}
	return DownloadResult{
		Report: *report, StatusCode: response.StatusCode, BytesWritten: written,
		ContentType: response.Header.Get("Content-Type"), ETag: boundedText(response.Header.Get("ETag"), 512),
		LastModified: boundedText(response.Header.Get("Last-Modified"), 512),
	}, nil
}

func validateStatReport(operation string, report *StatReport, expectedID int64) error {
	if report == nil || report.ID <= 0 || !validStatReportType(report.Type) || !validReportResponseDate(report.Date) || !validStatReportStatus(report.Status) {
		return platformContractError(operation, "NAVER returned an invalid Stat Report job")
	}
	if expectedID > 0 && report.ID != expectedID {
		return platformContractError(operation, "Stat Report job ID did not match the request")
	}
	if report.Status == ReportBuilt && report.DownloadURL == "" || report.DownloadURL != "" && !validOpaque(report.DownloadURL, 16_384) {
		return platformContractError(operation, "NAVER returned invalid Stat Report download metadata")
	}
	return nil
}

func validStatReportStatus(value StatReportStatus) bool {
	switch value {
	case ReportRegistered, ReportRunning, ReportBuilt, ReportNoData, ReportError, ReportWaiting, ReportAggregating:
		return true
	default:
		return false
	}
}

func (client *Client) validDownloadURL(value string) (string, url.Values, error) {
	parsed, err := url.Parse(value)
	if err != nil || parsed.User != nil || parsed.Fragment != "" || parsed.RawPath != "" || client.baseURL == nil ||
		parsed.Scheme != client.baseURL.Scheme || !strings.EqualFold(parsed.Host, client.baseURL.Host) {
		return "", nil, errUnsafeDownloadURL
	}
	basePath := strings.TrimRight(client.baseURL.Path, "/")
	expectedPath := basePath + "/report-download"
	if parsed.Path != expectedPath || path.Clean(parsed.Path) != parsed.Path || strings.Contains(parsed.Path, "\\") || parsed.RawQuery == "" {
		return "", nil, errUnsafeDownloadURL
	}
	return "/report-download", parsed.Query(), nil
}

func validReportMediaType(value string) bool {
	switch strings.ToLower(value) {
	case "text/tab-separated-values", "text/tsv", "text/csv", "text/plain", "application/octet-stream", "binary/octet-stream":
		return true
	default:
		return false
	}
}

var (
	errReportTooLarge    = errors.New("report exceeded configured size limit")
	errUnsafeDownloadURL = errors.New("unsafe report download URL")
)

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
