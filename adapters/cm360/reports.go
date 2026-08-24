package cm360

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"social-hub/pkg/socialhub"
)

const (
	maxDownloadChunkBytes int64 = 8 << 20
	maxErrorResponseBytes int64 = 1 << 20
)

func (client *Client) QueryReportData(ctx context.Context, input ReportDataQueryRequest, options ...socialhub.CallOption) (ReportDataResponse, error) {
	const operation = "report_data_query"
	prepared, err := prepareReportDataQuery(input, client.advertiserID)
	if err != nil {
		return ReportDataResponse{}, err
	}
	var response ReportDataResponse
	path := client.profilePath() + "/reportdata/query"
	if err := client.postJSON(ctx, operation, path, nil, prepared, &response, reportingScope, options...); err != nil {
		return ReportDataResponse{}, err
	}
	if err := validateReportDataResponse(response, prepared.MaxResults); err != nil {
		return ReportDataResponse{}, withOperation(err, operation)
	}
	return response, nil
}

func (client *Client) GetReport(ctx context.Context, reportID string, options ...socialhub.CallOption) (Report, error) {
	const operation = "report_get"
	if !validID(reportID) {
		return Report{}, invalidArgument(operation, "report ID must be a positive string-encoded integer")
	}
	var report Report
	path := client.profilePath() + "/reports/" + reportID
	if err := client.getJSON(ctx, operation, path, nil, &report, reportingScope, options...); err != nil {
		return Report{}, err
	}
	if !validReport(report) || report.ID != reportID {
		return Report{}, platformContractError(operation, "CM360 returned an invalid or different report")
	}
	return report, nil
}

func (client *Client) ListReports(ctx context.Context, input ReportListRequest, options ...socialhub.CallOption) (Page[Report], error) {
	const operation = "report_list"
	if !validListBase(input.MaxResults, input.PageToken, "", input.SortOrder) || !validReportScope(input.Scope) {
		return Page[Report]{}, invalidArgument(operation, "report scope, pagination, or sorting are invalid")
	}
	query := make(url.Values)
	setListBase(query, input.MaxResults, input.PageToken, "", input.SortOrder)
	scope := input.Scope
	if scope == "" {
		scope = ReportScopeMine
	}
	query.Set("scope", string(scope))
	var response listReportsResponse
	if err := client.getJSON(ctx, operation, client.profilePath()+"/reports", query, &response, reportingScope, options...); err != nil {
		return Page[Report]{}, err
	}
	seen := make(map[string]struct{}, len(response.Items))
	for _, report := range response.Items {
		if !validReport(report) {
			return Page[Report]{}, platformContractError(operation, "CM360 returned an invalid report")
		}
		if _, exists := seen[report.ID]; exists {
			return Page[Report]{}, platformContractError(operation, "CM360 returned duplicate reports")
		}
		seen[report.ID] = struct{}{}
	}
	if !validPageToken(response.NextPageToken) {
		return Page[Report]{}, platformContractError(operation, "CM360 returned an invalid page token")
	}
	return Page[Report]{Items: response.Items, NextPageToken: response.NextPageToken}, nil
}

func (client *Client) RunReport(ctx context.Context, reportID string, synchronous bool, options ...socialhub.CallOption) (ReportFile, error) {
	const operation = "report_run"
	if !validID(reportID) {
		return ReportFile{}, invalidArgument(operation, "report ID must be a positive string-encoded integer")
	}
	query := url.Values{"synchronous": {strconv.FormatBool(synchronous)}}
	var file ReportFile
	path := client.profilePath() + "/reports/" + reportID + "/run"
	if err := client.postJSON(ctx, operation, path, query, nil, &file, reportingScope, options...); err != nil {
		return ReportFile{}, err
	}
	if !validReportFile(file) || file.ReportID != reportID {
		return ReportFile{}, platformContractError(operation, "CM360 returned an invalid report file")
	}
	return file, nil
}

func (client *Client) GetReportFile(ctx context.Context, reportID, fileID string, options ...socialhub.CallOption) (ReportFile, error) {
	const operation = "report_file_get"
	if !validID(reportID) || !validID(fileID) {
		return ReportFile{}, invalidArgument(operation, "report and file IDs must be positive string-encoded integers")
	}
	var file ReportFile
	path := client.profilePath() + "/reports/" + reportID + "/files/" + fileID
	if err := client.getJSON(ctx, operation, path, nil, &file, reportingScope, options...); err != nil {
		return ReportFile{}, err
	}
	if !validReportFile(file) || file.ID != fileID || file.ReportID != reportID {
		return ReportFile{}, platformContractError(operation, "CM360 returned an invalid or different report file")
	}
	return file, nil
}

func (client *Client) ListReportFiles(ctx context.Context, reportID string, input ReportFileListRequest, options ...socialhub.CallOption) (Page[ReportFile], error) {
	const operation = "report_file_list"
	if !validID(reportID) || !validListBase(input.MaxResults, input.PageToken, "", input.SortOrder) {
		return Page[ReportFile]{}, invalidArgument(operation, "report ID, pagination, or sorting are invalid")
	}
	query := make(url.Values)
	setListBase(query, input.MaxResults, input.PageToken, "", input.SortOrder)
	var response listReportFilesResponse
	path := client.profilePath() + "/reports/" + reportID + "/files"
	if err := client.getJSON(ctx, operation, path, query, &response, reportingScope, options...); err != nil {
		return Page[ReportFile]{}, err
	}
	seen := make(map[string]struct{}, len(response.Items))
	for _, file := range response.Items {
		if !validReportFile(file) || file.ReportID != reportID {
			return Page[ReportFile]{}, platformContractError(operation, "CM360 returned an invalid report file")
		}
		if _, exists := seen[file.ID]; exists {
			return Page[ReportFile]{}, platformContractError(operation, "CM360 returned duplicate report files")
		}
		seen[file.ID] = struct{}{}
	}
	if !validPageToken(response.NextPageToken) {
		return Page[ReportFile]{}, platformContractError(operation, "CM360 returned an invalid page token")
	}
	return Page[ReportFile]{Items: response.Items, NextPageToken: response.NextPageToken}, nil
}

func (client *Client) DownloadReportFileRange(
	ctx context.Context,
	reportID, fileID string,
	byteRange ByteRange,
	output io.Writer,
	options ...socialhub.CallOption,
) (DownloadResult, error) {
	const operation = "report_file_download"
	if !validID(reportID) || !validID(fileID) || !validByteRange(byteRange) || output == nil {
		return DownloadResult{}, invalidArgument(operation, "report ID, file ID, byte range, and output are required")
	}
	if err := client.requireScope(operation, reportingScope); err != nil {
		return DownloadResult{}, err
	}
	query := url.Values{"alt": {"media"}}
	path := "/reports/" + reportID + "/files/" + fileID
	request, err := client.api.NewRequest(ctx, http.MethodGet, path, query, nil, options...)
	if err != nil {
		return DownloadResult{}, withOperation(err, operation)
	}
	request.Header.Set("Accept", "application/octet-stream")
	request.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", byteRange.Start, byteRange.End))
	response, err := client.httpClient.Do(request)
	if err != nil {
		return DownloadResult{}, platformError(operation, socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK && response.StatusCode != http.StatusPartialContent {
		body, readErr := io.ReadAll(io.LimitReader(response.Body, maxErrorResponseBytes+1))
		if readErr != nil {
			return DownloadResult{}, platformError(operation, socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, readErr)
		}
		if int64(len(body)) > maxErrorResponseBytes {
			return DownloadResult{}, platformContractError(operation, "CM360 error response exceeded size limit")
		}
		return DownloadResult{}, withOperation(decodeHTTPError(response.StatusCode, response.Header, body), operation)
	}
	maximum := byteRange.End - byteRange.Start + 1
	if response.ContentLength > maximum {
		return DownloadResult{}, platformContractError(operation, "CM360 returned more bytes than requested")
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, maximum+1))
	if err != nil {
		return DownloadResult{}, platformError(operation, socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
	}
	if int64(len(body)) > maximum {
		return DownloadResult{}, platformContractError(operation, "CM360 returned more bytes than requested")
	}
	complete := response.StatusCode == http.StatusOK
	contentRange := response.Header.Get("Content-Range")
	if response.StatusCode == http.StatusOK {
		if byteRange.Start != 0 {
			return DownloadResult{}, platformContractError(operation, "CM360 ignored a nonzero byte range")
		}
	} else {
		start, end, total, ok := parseContentRange(contentRange)
		if !ok || start != byteRange.Start || end > byteRange.End || int64(len(body)) != end-start+1 {
			return DownloadResult{}, platformContractError(operation, "CM360 returned an invalid content range")
		}
		complete = total >= 0 && end+1 == total
	}
	written, err := output.Write(body)
	if err != nil {
		return DownloadResult{}, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	if written != len(body) {
		return DownloadResult{}, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, io.ErrShortWrite)
	}
	return DownloadResult{BytesWritten: written, ContentRange: contentRange, Complete: complete}, nil
}

func validateReportDataResponse(response ReportDataResponse, maxResults int) error {
	limit := maxResults
	if limit == 0 {
		limit = 100
	}
	if len(response.ColumnHeaders) == 0 || len(response.Rows) > limit || !validPageToken(response.NextPageToken) {
		return platformContractError("report_data_query", "CM360 returned invalid report data metadata")
	}
	seen := make(map[string]struct{}, len(response.ColumnHeaders))
	for _, header := range response.ColumnHeaders {
		if !validCMField(header.Name) || (header.Type != "DIMENSION" && header.Type != "METRIC") {
			return platformContractError("report_data_query", "CM360 returned an invalid report data column")
		}
		if _, exists := seen[header.Name]; exists {
			return platformContractError("report_data_query", "CM360 returned duplicate report data columns")
		}
		seen[header.Name] = struct{}{}
	}
	for _, row := range response.Rows {
		if len(row.Values) != len(response.ColumnHeaders) {
			return platformContractError("report_data_query", "CM360 returned a report row with a mismatched column count")
		}
	}
	if response.TotalRow != nil && len(response.TotalRow.Values) != len(response.ColumnHeaders) {
		return platformContractError("report_data_query", "CM360 returned a total row with a mismatched column count")
	}
	return nil
}

func validReport(report Report) bool {
	if !validID(report.ID) || !validName(report.Name, 512) ||
		(report.OwnerProfileID != "" && !validID(report.OwnerProfileID)) ||
		(report.AccountID != "" && !validID(report.AccountID)) ||
		!validOptionalText(report.FileName, 512) || (report.Format != "" && report.Format != "CSV" && report.Format != "EXCEL") {
		return false
	}
	switch report.Type {
	case "STANDARD", "REACH", "PATH_TO_CONVERSION", "FLOODLIGHT", "CROSS_MEDIA_REACH":
		return true
	default:
		return false
	}
}

func validReportFile(file ReportFile) bool {
	return validID(file.ID) && validID(file.ReportID) && validReportFileStatus(file.Status) &&
		validOptionalText(file.FileName, 512) && (file.Format == "" || file.Format == "CSV" || file.Format == "EXCEL") &&
		(file.DateRange == (DateRange{}) || validQueryDateRange(file.DateRange))
}

func parseContentRange(value string) (start, end, total int64, ok bool) {
	if !strings.HasPrefix(value, "bytes ") {
		return 0, 0, 0, false
	}
	parts := strings.Split(strings.TrimPrefix(value, "bytes "), "/")
	if len(parts) != 2 {
		return 0, 0, 0, false
	}
	rangeParts := strings.Split(parts[0], "-")
	if len(rangeParts) != 2 {
		return 0, 0, 0, false
	}
	start, errStart := strconv.ParseInt(rangeParts[0], 10, 64)
	end, errEnd := strconv.ParseInt(rangeParts[1], 10, 64)
	total = -1
	var errTotal error
	if parts[1] != "*" {
		total, errTotal = strconv.ParseInt(parts[1], 10, 64)
	}
	return start, end, total, errStart == nil && errEnd == nil && errTotal == nil && start >= 0 && end >= start &&
		(total == -1 || total > end)
}
