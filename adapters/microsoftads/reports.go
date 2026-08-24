package microsoftads

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"social-hub/pkg/socialhub"
)

func (client *Client) SubmitCampaignPerformanceReport(ctx context.Context, input CampaignPerformanceReportRequest, options ...socialhub.CallOption) (string, error) {
	const operation = "submit_campaign_performance_report"
	if message := validateReportRequest(input); message != "" {
		return "", invalidArgument(operation, message)
	}
	if err := client.validateAccount(ctx, options...); err != nil {
		return "", err
	}
	for _, campaignID := range input.CampaignIDs {
		if _, err := client.GetCampaign(ctx, campaignID, options...); err != nil {
			return "", err
		}
	}
	format := input.Format
	if format == "" {
		format = ReportFormatCSV
	}
	aggregation := input.Aggregation
	if aggregation == "" {
		aggregation = "Summary"
	}
	scope := reportScope{AccountIDs: []string{client.customerAccountID}}
	if len(input.CampaignIDs) > 0 {
		scope.AccountIDs = nil
		scope.Campaigns = make([]campaignReportScope, len(input.CampaignIDs))
		for index, campaignID := range input.CampaignIDs {
			scope.Campaigns[index] = campaignReportScope{AccountID: client.customerAccountID, CampaignID: campaignID}
		}
	}
	payload := campaignPerformanceReportPayload{
		Type: "CampaignPerformanceReportRequest", ReportName: input.ReportName,
		Format: format, FormatVersion: "2.0", Aggregation: aggregation,
		Columns: append([]string(nil), input.Columns...), Time: input.Time, Scope: scope,
		ExcludeColumnHeaders: input.ExcludeColumnHeaders, ExcludeReportFooter: input.ExcludeReportFooter,
		ExcludeReportHeader: input.ExcludeReportHeader, ReturnOnlyCompleteData: input.ReturnOnlyCompleteData,
	}
	var response struct {
		ReportRequestID string `json:"ReportRequestId"`
	}
	_, err := client.postJSON(ctx, operation, client.reporting, "/GenerateReport/Submit", struct {
		ReportRequest campaignPerformanceReportPayload `json:"ReportRequest"`
	}{ReportRequest: payload}, &response, options...)
	if err != nil {
		return "", err
	}
	if !validOpaque(response.ReportRequestID, 1024) {
		return "", platformContractError(operation, "response did not contain a valid report request ID")
	}
	return response.ReportRequestID, nil
}

func (client *Client) PollReport(ctx context.Context, reportRequestID string, options ...socialhub.CallOption) (ReportRequestStatus, error) {
	const operation = "poll_report"
	if !validOpaque(reportRequestID, 1024) {
		return ReportRequestStatus{}, invalidArgument(operation, "report request ID is required")
	}
	var response struct {
		Status ReportRequestStatus `json:"ReportRequestStatus"`
	}
	_, err := client.postJSON(ctx, operation, client.reporting, "/GenerateReport/Poll", struct {
		ReportRequestID string `json:"ReportRequestId"`
	}{ReportRequestID: reportRequestID}, &response, options...)
	if err != nil {
		return ReportRequestStatus{}, err
	}
	switch response.Status.Status {
	case "Pending", "Success":
	case "Error":
		return ReportRequestStatus{}, platformContractError(operation, "report generation failed")
	default:
		return ReportRequestStatus{}, platformContractError(operation, "response contained an unknown report status")
	}
	if response.Status.Status == "Success" && response.Status.ReportDownloadURL == "" {
		return ReportRequestStatus{}, platformContractError(operation, "successful report response did not contain a download URL")
	}
	return response.Status, nil
}

func (client *Client) DownloadReport(ctx context.Context, rawURL string, destination io.Writer, options ...socialhub.CallOption) (int64, error) {
	const operation = "download_report"
	if destination == nil {
		return 0, invalidArgument(operation, "destination writer is required")
	}
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil || parsed.Fragment != "" ||
		!client.allowedReportOrigin(parsed) {
		return 0, invalidArgument(operation, "report URL must be an allowed HTTPS reporting or Azure Blob URL")
	}
	callOptions, err := socialhub.ResolveCallOptions(options...)
	if err != nil {
		return 0, err
	}
	if callOptions.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, callOptions.Timeout)
		defer cancel()
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return 0, platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	request.Header.Set("Accept", "application/zip, application/octet-stream")
	if callOptions.RequestID != "" {
		request.Header.Set("X-Request-ID", callOptions.RequestID)
	}
	response, err := client.httpClient.Do(request)
	if err != nil {
		return 0, platformError(operation, socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		body, readErr := io.ReadAll(io.LimitReader(response.Body, (1<<20)+1))
		if readErr != nil {
			return 0, platformError(operation, socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, readErr)
		}
		return 0, decodeAPIError(operation, response.StatusCode, response.Header, body)
	}
	if response.ContentLength > client.maxReportBytes {
		return 0, platformContractError(operation, "report exceeded configured size limit")
	}
	limited := &io.LimitedReader{R: response.Body, N: client.maxReportBytes}
	written, err := io.Copy(destination, limited)
	if err != nil {
		return written, platformError(operation, socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
	}
	var extra [1]byte
	count, readErr := response.Body.Read(extra[:])
	if readErr != nil && readErr != io.EOF {
		return written, platformError(operation, socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, readErr)
	}
	if count != 0 {
		return written, platformContractError(operation, "report exceeded configured size limit")
	}
	return written, nil
}

type campaignPerformanceReportPayload struct {
	ExcludeColumnHeaders   bool         `json:"ExcludeColumnHeaders"`
	ExcludeReportFooter    bool         `json:"ExcludeReportFooter"`
	ExcludeReportHeader    bool         `json:"ExcludeReportHeader"`
	Format                 ReportFormat `json:"Format"`
	FormatVersion          string       `json:"FormatVersion"`
	ReportName             string       `json:"ReportName,omitempty"`
	ReturnOnlyCompleteData bool         `json:"ReturnOnlyCompleteData"`
	Type                   string       `json:"Type"`
	Aggregation            string       `json:"Aggregation"`
	Columns                []string     `json:"Columns"`
	Scope                  reportScope  `json:"Scope"`
	Time                   ReportTime   `json:"Time"`
}

type reportScope struct {
	AccountIDs []string              `json:"AccountIds,omitempty"`
	Campaigns  []campaignReportScope `json:"Campaigns,omitempty"`
}

type campaignReportScope struct {
	AccountID  string `json:"AccountId"`
	CampaignID string `json:"CampaignId"`
}

func validateReportRequest(input CampaignPerformanceReportRequest) string {
	if !validOptionalText(input.ReportName, 200) {
		return "report name is invalid"
	}
	if input.Format != "" && input.Format != ReportFormatCSV && input.Format != ReportFormatTSV {
		return "report format must be Csv or Tsv"
	}
	if !validOptionalText(input.Aggregation, 64) || len(input.Columns) == 0 || len(input.Columns) > 300 {
		return "aggregation and columns are invalid"
	}
	for _, column := range input.Columns {
		if !validRequiredText(column, 128) {
			return "report column is invalid"
		}
	}
	if len(input.CampaignIDs) > 300 {
		return "too many campaign IDs"
	}
	for _, campaignID := range input.CampaignIDs {
		if !validNumericID(campaignID) {
			return "campaign ID is invalid"
		}
	}
	predefined := input.Time.PredefinedTime != ""
	custom := input.Time.CustomDateRangeStart != nil || input.Time.CustomDateRangeEnd != nil
	if predefined == custom || (predefined && !validRequiredText(input.Time.PredefinedTime, 64)) ||
		!validOptionalText(input.Time.ReportTimeZone, 128) {
		return "report time must contain either a predefined period or a complete custom range"
	}
	if custom {
		if input.Time.CustomDateRangeStart == nil || input.Time.CustomDateRangeEnd == nil ||
			!validReportDate(*input.Time.CustomDateRangeStart) || !validReportDate(*input.Time.CustomDateRangeEnd) ||
			reportDateValue(*input.Time.CustomDateRangeStart).After(reportDateValue(*input.Time.CustomDateRangeEnd)) {
			return "custom report date range is invalid"
		}
	}
	return ""
}

func validReportDate(value ReportDate) bool {
	if value.Year < 2000 || value.Year > 9999 || value.Month < 1 || value.Month > 12 || value.Day < 1 || value.Day > 31 {
		return false
	}
	date := time.Date(value.Year, time.Month(value.Month), value.Day, 0, 0, 0, 0, time.UTC)
	return date.Year() == value.Year && int(date.Month()) == value.Month && date.Day() == value.Day
}

func reportDateValue(value ReportDate) time.Time {
	return time.Date(value.Year, time.Month(value.Month), value.Day, 0, 0, 0, 0, time.UTC)
}

func (client *Client) allowedReportOrigin(parsed *url.URL) bool {
	if parsed.Scheme != "https" {
		return false
	}
	if client.reportingBaseURL != nil && parsed.Scheme == client.reportingBaseURL.Scheme &&
		strings.EqualFold(parsed.Host, client.reportingBaseURL.Host) {
		return true
	}
	host := strings.ToLower(parsed.Hostname())
	return host == "reporting.api.bingads.microsoft.com" || strings.HasSuffix(host, ".blob.core.windows.net") ||
		strings.HasSuffix(host, ".blob.storage.azure.net")
}
