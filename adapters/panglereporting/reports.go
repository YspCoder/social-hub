package panglereporting

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"social-hub/pkg/socialhub"
)

const (
	reportPath            = "/union_pangle/open/api/rt/income"
	maximumSignedQueryLen = 32 << 10
	maximumErrorBodyBytes = int64(1 << 20)
)

type reportEnvelope struct {
	Code    json.RawMessage `json:"Code"`
	Message string          `json:"Message"`
	Data    json.RawMessage `json:"Data"`
}

func (client *Client) IncomeReport(ctx context.Context, input ReportRequest, options ...socialhub.CallOption) (Report, error) {
	const operation = "income_report"
	if err := validateReportRequest(input); err != nil {
		return Report{}, err
	}
	callOptions, err := validateCallOptions(options)
	if err != nil {
		return Report{}, err
	}
	query, err := client.reportQuery(input)
	if err != nil {
		return Report{}, err
	}
	envelope, status, header, err := client.executeReport(ctx, query, callOptions)
	if err != nil {
		return Report{}, err
	}
	code := scalarCode(envelope.Code)
	switch code {
	case "100":
	case "PD0004":
		return Report{Date: input.Date, NoData: true}, nil
	case "":
		return Report{}, platformContractError(operation, "Pangle response omitted a valid business code", status)
	default:
		return Report{}, businessError(operation, status, header, code, client.clock.Now())
	}
	if len(envelope.Message) > 4_096 || len(envelope.Data) == 0 || bytes.Equal(bytes.TrimSpace(envelope.Data), []byte("null")) {
		return Report{}, platformContractError(operation, "Pangle returned an invalid success envelope", status)
	}
	var byDate map[string][]IncomeRow
	if err := json.Unmarshal(envelope.Data, &byDate); err != nil || byDate == nil {
		return Report{}, platformContractError(operation, "Pangle returned invalid report data", status)
	}
	if len(byDate) == 0 {
		return Report{Date: input.Date, NoData: true}, nil
	}
	rows, found := byDate[string(input.Date)]
	if !found || len(byDate) != 1 || len(rows) > MaximumReportRows {
		return Report{}, platformContractError(operation, "Pangle returned an unexpected report date or row count", status)
	}
	for _, row := range rows {
		if !validIncomeRow(row, input.Date) {
			return Report{}, platformContractError(operation, "Pangle returned an invalid income row", status)
		}
	}
	return Report{
		Date: input.Date, Rows: rows, NoData: len(rows) == 0,
		MayBeTruncated: len(rows) == MaximumReportRows,
	}, nil
}

func (client *Client) reportQuery(input ReportRequest) (url.Values, error) {
	now := client.clock.Now()
	if now.Unix() <= 0 {
		return nil, invalidArgument("income_report", "clock must return a time after the Unix epoch")
	}
	query := url.Values{
		"user_id":   {client.userID},
		"role_id":   {client.roleID},
		"timestamp": {strconv.FormatInt(now.Unix(), 10)},
		"version":   {apiVersion},
		"date":      {string(input.Date)},
		"sign_type": {"MD5"},
	}
	if input.TimeZone != nil {
		query.Set("time_zone", strconv.Itoa(int(*input.TimeZone)))
	}
	if input.Currency != "" {
		query.Set("currency", string(input.Currency))
	}
	if input.Region != "" {
		query.Set("region", input.Region)
	}
	if len(input.AppIDs) > 0 {
		values := make([]string, len(input.AppIDs))
		for index, id := range input.AppIDs {
			values[index] = string(id)
		}
		query.Set("app_id", strings.Join(values, ","))
	}
	if len(input.Dimensions) > 0 {
		values := make([]string, len(input.Dimensions))
		for index, dimension := range input.Dimensions {
			values[index] = string(dimension)
		}
		query.Set("dimensions", strings.Join(values, ","))
	}
	query.Set("sign", signValues(query, client.securityKey))
	if len(query.Encode()) > maximumSignedQueryLen {
		return nil, invalidArgument("income_report", "signed report query exceeds 32 KiB")
	}
	return query, nil
}

func (client *Client) executeReport(ctx context.Context, query url.Values, options socialhub.CallOptions) (reportEnvelope, int, http.Header, error) {
	requestURL := *client.baseURL
	requestURL.Path = strings.TrimRight(client.baseURL.Path, "/") + reportPath
	requestURL.RawQuery = query.Encode()
	if options.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, options.Timeout)
		defer cancel()
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL.String(), nil)
	if err != nil {
		return reportEnvelope{}, 0, nil, platformError("income_report", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	request.Header.Set("Accept", "application/json")
	response, err := client.httpClient.Do(request)
	if err != nil {
		return reportEnvelope{}, 0, nil, platformError("income_report", socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, sanitizeTransportError(err))
	}
	defer response.Body.Close()
	redactions := []string{client.securityKey, client.userID, client.roleID, query.Get("sign")}
	safeResponseHeader := sanitizedResponseHeaders(response.Header, redactions...)
	limit := client.maxResponseBytes
	if response.StatusCode != http.StatusOK {
		limit = maximumErrorBodyBytes
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, limit+1))
	if err != nil {
		return reportEnvelope{}, response.StatusCode, safeResponseHeader, platformError("income_report", socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
	}
	if int64(len(body)) > limit {
		return reportEnvelope{}, response.StatusCode, safeResponseHeader, platformContractError("income_report", "Pangle response exceeded the configured size limit", response.StatusCode)
	}
	if response.StatusCode != http.StatusOK {
		code := ""
		var envelope reportEnvelope
		if json.Unmarshal(body, &envelope) == nil {
			code = scalarCode(envelope.Code)
		}
		return reportEnvelope{}, response.StatusCode, safeResponseHeader, httpStatusError(response.StatusCode, safeResponseHeader, code, client.clock.Now())
	}
	mediaType, _, err := mime.ParseMediaType(response.Header.Get("Content-Type"))
	if err != nil || !strings.EqualFold(mediaType, "application/json") {
		return reportEnvelope{}, response.StatusCode, safeResponseHeader, platformContractError("income_report", "Pangle returned an unexpected content type", response.StatusCode)
	}
	var envelope reportEnvelope
	if err := json.Unmarshal(body, &envelope); err != nil {
		return reportEnvelope{}, response.StatusCode, safeResponseHeader, platformError("income_report", socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	return envelope, response.StatusCode, safeResponseHeader, nil
}

func scalarCode(raw json.RawMessage) string {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return ""
	}
	if trimmed[0] == '"' {
		var value string
		if json.Unmarshal(trimmed, &value) != nil || !validBusinessCode(value) {
			return ""
		}
		return value
	}
	for _, character := range trimmed {
		if character < '0' || character > '9' {
			return ""
		}
	}
	if len(trimmed) > 20 {
		return ""
	}
	return string(trimmed)
}

func validBusinessCode(value string) bool {
	if value == "PD0004" {
		return true
	}
	if value == "" || len(value) > 20 {
		return false
	}
	for index := range value {
		if value[index] < '0' || value[index] > '9' {
			return false
		}
	}
	return true
}
