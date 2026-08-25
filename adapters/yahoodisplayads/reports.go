package yahoodisplayads

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strings"

	"social-hub/pkg/socialhub"
)

const reportServicePath = "ReportDefinitionService"
const maxReportErrorBytes int64 = 1 << 20

type reportSelectorRequest struct {
	AccountID         int64             `json:"accountId"`
	ReportJobIDs      []int64           `json:"reportJobIds,omitempty"`
	ReportJobStatuses []ReportJobStatus `json:"reportJobStatuses,omitempty"`
	StartIndex        int32             `json:"startIndex,omitempty"`
	NumberResults     int32             `json:"numberResults,omitempty"`
}

type reportOperation struct {
	AccountID int64              `json:"accountId"`
	Operand   []ReportDefinition `json:"operand"`
}

type reportDownloadSelector struct {
	AccountID   int64 `json:"accountId"`
	ReportJobID int64 `json:"reportJobId"`
}

func (client *Client) CreateReport(ctx context.Context, input ReportDefinitionAdd, options ...socialhub.CallOption) (*ReportDefinition, MutationResult[ReportDefinition], error) {
	const operation = "report_create"
	if !validReportDefinitionAdd(input) {
		return nil, MutationResult[ReportDefinition]{}, invalidArgument(operation, "report name, fields, date range, or format is invalid")
	}
	prepared, err := prepareCallOptions(operation, options)
	if err != nil {
		return nil, MutationResult[ReportDefinition]{}, err
	}
	format := input.Format
	if format == "" {
		format = ReportCSV
	}
	skipHeader, skipSummary, includeDeleted := "FALSE", "FALSE", "TRUE"
	if input.SkipHeader {
		skipHeader = "TRUE"
	}
	if input.SkipSummary {
		skipSummary = "TRUE"
	}
	if input.ExcludeDeleted {
		includeDeleted = "FALSE"
	}
	operand := ReportDefinition{
		ReportName: input.Name, Fields: append([]string(nil), input.Fields...),
		ReportDateRangeType: input.DateRangeType, ReportDownloadFormat: format,
		ReportCompressType: "NONE", ReportDownloadEncode: "UTF8", ReportLanguage: "EN",
		ReportIncludeDeleted:   includeDeleted,
		ReportSkipColumnHeader: skipHeader, ReportSkipReportSummary: skipSummary,
	}
	if input.DateRange != nil {
		copy := *input.DateRange
		operand.DateRange = &copy
	}
	result, err := postMutation(ctx, client, operation, reportServicePath+"/add",
		reportOperation{AccountID: client.advertiserAccountID, Operand: []ReportDefinition{operand}},
		1, reportEntity, func(value *ReportDefinition) error {
			return client.validateReportMutation(operation, value)
		}, prepared...)
	if err != nil {
		return nil, result, err
	}
	report, err := client.GetReport(ctx, result.Items[0].Value.ReportJobID, prepared...)
	if err != nil {
		return nil, result, withOperation(err, operation)
	}
	result.Items[0].Value = report
	return report, result, nil
}

func (client *Client) ListReports(ctx context.Context, input ReportSelector, options ...socialhub.CallOption) (Page[ReportDefinition], error) {
	const operation = "report_list"
	if !validReportSelector(input) {
		return Page[ReportDefinition]{}, invalidArgument(operation, "report IDs, statuses, or pagination are invalid")
	}
	request := reportSelectorRequest{
		AccountID: client.advertiserAccountID, ReportJobIDs: input.ReportJobIDs,
		ReportJobStatuses: input.ReportJobStatuses, StartIndex: input.StartIndex, NumberResults: input.NumberResults,
	}
	return postPage(ctx, client, operation, reportServicePath+"/get", request, input.PageRequest,
		MaximumReportPageSize, reportEntity, func(value *ReportDefinition) error {
			return client.validateReport(operation, value, 0)
		}, options...)
}

func (client *Client) GetReport(ctx context.Context, id int64, options ...socialhub.CallOption) (*ReportDefinition, error) {
	const operation = "report_get"
	if id <= 0 {
		return nil, invalidArgument(operation, "report job ID must be positive")
	}
	page, err := client.ListReports(ctx, ReportSelector{
		ReportJobIDs: []int64{id}, PageRequest: PageRequest{StartIndex: 1, NumberResults: 1},
	}, options...)
	if err != nil {
		return nil, withOperation(err, operation)
	}
	if len(page.Items) == 0 {
		return nil, notFound(operation, "report job was not returned")
	}
	if len(page.Items) != 1 || page.Items[0].ReportJobID != id {
		return nil, platformContractError(operation, "LINE Yahoo returned a different report job")
	}
	return &page.Items[0], nil
}

func (client *Client) DeleteReports(ctx context.Context, ids []int64, options ...socialhub.CallOption) (MutationResult[ReportDefinition], error) {
	const operation = "report_delete"
	if !validIDs(ids, MaximumReportMutationBatch, false) {
		return MutationResult[ReportDefinition]{}, invalidArgument(operation, "1-30 unique report job IDs are required")
	}
	operands := make([]ReportDefinition, 0, len(ids))
	for _, id := range ids {
		operands = append(operands, ReportDefinition{ReportJobID: id})
	}
	return postMutation(ctx, client, operation, reportServicePath+"/remove",
		reportOperation{AccountID: client.advertiserAccountID, Operand: operands}, len(operands),
		reportEntity, func(value *ReportDefinition) error {
			return client.validateReportMutation(operation, value)
		}, options...)
}

func (client *Client) DownloadReport(ctx context.Context, id int64, output io.Writer, maximum int64, options ...socialhub.CallOption) (DownloadResult, error) {
	const operation = "report_download"
	if id <= 0 || output == nil || maximum < 0 {
		return DownloadResult{}, invalidArgument(operation, "report job ID, output, and a nonnegative maximum size are required")
	}
	if err := client.requireAccess(operation); err != nil {
		return DownloadResult{}, err
	}
	callOptions, err := resolveCallOptions(operation, options)
	if err != nil {
		return DownloadResult{}, err
	}
	prepared := preparedCallOptions(callOptions)
	report, err := client.GetReport(ctx, id, prepared...)
	if err != nil {
		return DownloadResult{}, withOperation(err, operation)
	}
	if report.ReportJobStatus != ReportCompleted {
		message := "report job is not completed"
		if report.ReportJobStatus == ReportFailed || report.ReportJobStatus == ReportCanceled {
			message = "report job failed or was canceled; inspect the report definition in LINE Yahoo Ads"
		}
		return DownloadResult{}, &socialhub.Error{
			Code: socialhub.CodeConflict, Class: socialhub.ClassUserAction,
			Platform: platformName, Product: productName, Op: operation, PlatformMessage: message,
		}
	}
	encoded, err := json.Marshal(reportDownloadSelector{AccountID: client.advertiserAccountID, ReportJobID: id})
	if err != nil || len(encoded) > maxRequestBytes {
		return DownloadResult{}, platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	requestContext := ctx
	if callOptions.Timeout > 0 {
		var cancel context.CancelFunc
		requestContext, cancel = context.WithTimeout(ctx, callOptions.Timeout)
		defer cancel()
	}
	request, err := client.api.NewRequest(requestContext, http.MethodPost, reportServicePath+"/download", nil, bytes.NewReader(encoded))
	if err != nil {
		return DownloadResult{}, withOperation(err, operation)
	}
	request.Header.Set("Accept", "application/octet-stream")
	request.Header.Set("Accept-Encoding", "identity")
	request.Header.Set("Content-Type", "application/json; charset=utf-8")
	response, err := client.httpClient.Do(request)
	if err != nil {
		return DownloadResult{}, platformError(operation, socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
	}
	defer response.Body.Close()
	result := DownloadResult{
		RID:        client.requestIDs.safe(response.Header.Get("x-z-rid")),
		HTTPStatus: response.StatusCode, ContentType: boundedOpaque(response.Header.Get("Content-Type"), 256),
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		body, readErr := io.ReadAll(io.LimitReader(response.Body, maxReportErrorBytes+1))
		if readErr != nil {
			return result, platformError(operation, socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, readErr)
		}
		if int64(len(body)) > maxReportErrorBytes {
			return result, platformContractError(operation, "report error response exceeded 1 MiB")
		}
		return result, withOperation(client.decodeError(response.StatusCode, response.Header, body), operation)
	}
	if response.StatusCode != http.StatusOK {
		return result, platformContractError(operation, "LINE Yahoo returned an unexpected successful download status")
	}
	if encoding := strings.ToLower(strings.TrimSpace(response.Header.Get("Content-Encoding"))); encoding != "" && encoding != "identity" {
		return result, platformContractError(operation, "LINE Yahoo returned an unexpected report content encoding")
	}
	mediaType, _, err := mime.ParseMediaType(response.Header.Get("Content-Type"))
	if err != nil {
		return result, platformContractError(operation, "LINE Yahoo returned an invalid report content type")
	}
	if mediaType == "application/json" || mediaType == "application/problem+json" {
		body, readErr := io.ReadAll(io.LimitReader(response.Body, maxReportErrorBytes+1))
		if readErr != nil {
			return result, platformError(operation, socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, readErr)
		}
		if int64(len(body)) > maxReportErrorBytes {
			return result, platformContractError(operation, "report JSON response exceeded 1 MiB")
		}
		var envelope errorEnvelope
		if json.Unmarshal(body, &envelope) == nil && len(envelope.Errors) > 0 && validErrorItems(envelope.Errors) {
			return result, client.apiErrorValue(operation, response.StatusCode, response.Header, envelope.RID, envelope.Errors)
		}
		return result, platformContractError(operation, "LINE Yahoo returned JSON instead of report bytes")
	}
	if !validReportMediaType(mediaType) {
		return result, platformContractError(operation, "LINE Yahoo returned an unexpected report content type")
	}
	if maximum == 0 {
		maximum = DefaultMaxReportBytes
	}
	if response.ContentLength > maximum {
		return result, platformContractError(operation, "report exceeds the configured size limit")
	}
	written, err := copyBounded(output, response.Body, maximum)
	result.BytesWritten = written
	if err != nil {
		if errors.Is(err, errReportTooLarge) {
			return result, platformContractError(operation, "report exceeds the configured size limit")
		}
		return result, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	return result, nil
}

func (client *Client) validateReport(operation string, value *ReportDefinition, expectedID int64) error {
	if value == nil || value.AccountID != client.advertiserAccountID || value.ReportJobID <= 0 ||
		value.ReportName != "" && !validText(value.ReportName, 255) ||
		(value.ReportJobStatus != ReportWaiting && value.ReportJobStatus != ReportInProgress &&
			value.ReportJobStatus != ReportCompleted && value.ReportJobStatus != ReportCanceled &&
			value.ReportJobStatus != ReportFailed && value.ReportJobStatus != ReportUnknown) {
		return platformContractError(operation, "LINE Yahoo returned an invalid report definition")
	}
	if expectedID > 0 && value.ReportJobID != expectedID {
		return platformContractError(operation, "report job ID did not match the request")
	}
	return nil
}

func (client *Client) validateReportMutation(operation string, value *ReportDefinition) error {
	if value == nil || value.ReportJobID <= 0 || value.AccountID != 0 && value.AccountID != client.advertiserAccountID {
		return platformContractError(operation, "LINE Yahoo returned an invalid report mutation value")
	}
	return nil
}

func validReportMediaType(value string) bool {
	return strings.EqualFold(strings.TrimSpace(value), "application/octet-stream")
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

var _ ReportWorkflow = (*Client)(nil)
