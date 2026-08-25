package searchads360

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

const maximumPageSize = 10000

func validCustomerID(value string) bool {
	return len(value) == 10 && validNumericID(value, 10)
}

func validNumericID(value string, maximum int) bool {
	if value == "" || len(value) > maximum {
		return false
	}
	nonZero := false
	for index := range value {
		if value[index] < '0' || value[index] > '9' {
			return false
		}
		nonZero = nonZero || value[index] != '0'
	}
	return nonZero
}

func validOpaque(value string, maximum int) bool {
	if value == "" || len(value) > maximum || !utf8.ValidString(value) || strings.TrimSpace(value) != value {
		return false
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return false
		}
	}
	return true
}

func validPageToken(value string) bool {
	return value == "" || validOpaque(value, 16384)
}

func prepareCallOptions(operation string, options []socialhub.CallOption) ([]socialhub.CallOption, error) {
	resolved, err := socialhub.ResolveCallOptions(options...)
	if err != nil {
		return nil, platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	if resolved.RequestID != "" {
		return nil, invalidArgument(operation, "Search Ads 360 does not document caller-supplied request IDs")
	}
	if resolved.IdempotencyKey != "" {
		return nil, invalidArgument(operation, "read-only Search Ads 360 Reporting API methods do not use idempotency keys")
	}
	if len(resolved.Fields) > 0 {
		return nil, invalidArgument(operation, "field selection must be expressed in Search Ads 360 Query Language")
	}
	if resolved.Timeout == 0 {
		return nil, nil
	}
	return []socialhub.CallOption{socialhub.WithCallTimeout(resolved.Timeout)}, nil
}

func validReturnedScopes(scopes []string) bool {
	if len(scopes) > 64 {
		return false
	}
	for _, scope := range scopes {
		if !validOpaque(scope, 2048) {
			return false
		}
	}
	return true
}

func validOptionalUint(value string) bool {
	if value == "" {
		return true
	}
	if len(value) > 20 {
		return false
	}
	for index := range value {
		if value[index] < '0' || value[index] > '9' {
			return false
		}
	}
	return true
}

func validBoundedText(value string, maximum int) bool {
	return value != "" && len(value) <= maximum && utf8.ValidString(value) && !strings.ContainsRune(value, '\x00')
}

func validQuery(value string, requireFrom bool) bool {
	if !validBoundedText(value, 65536) || strings.TrimSpace(value) != value ||
		strings.Contains(value, ";") || strings.Contains(value, "--") ||
		strings.Contains(value, "/*") || strings.Contains(value, "*/") {
		return false
	}
	upper := strings.ToUpper(strings.Join(strings.Fields(value), " "))
	if !strings.HasPrefix(upper, "SELECT ") {
		return false
	}
	return !requireFrom || strings.Contains(upper, " FROM ")
}

func validSummaryRowSetting(value SummaryRowSetting) bool {
	switch value {
	case "", SummaryRowUnspecified, SummaryRowUnknown, SummaryRowNone, SummaryRowWithResults, SummaryRowOnly:
		return true
	default:
		return false
	}
}

func validReportRequest(input SearchRequest) bool {
	return validQuery(input.Query, true) && input.PageSize >= 0 && input.PageSize <= maximumPageSize &&
		validPageToken(input.PageToken) && validSummaryRowSetting(input.SummaryRowSetting)
}

func validFieldSearchRequest(input FieldSearchRequest) bool {
	return validQuery(input.Query, false) && input.PageSize >= 0 && input.PageSize <= maximumPageSize &&
		validPageToken(input.PageToken)
}

func effectivePageSize(value int) int {
	if value == 0 {
		return maximumPageSize
	}
	return value
}

func validSearchResponse(response searchResponse, requestedPageSize int) bool {
	if len(response.Results) > effectivePageSize(requestedPageSize) || !validPageToken(response.NextPageToken) ||
		!validOptionalUint(response.TotalResultsCount) || len(response.FieldMask) > 65536 ||
		!utf8.ValidString(response.FieldMask) || strings.ContainsRune(response.FieldMask, '\x00') {
		return false
	}
	for _, row := range response.Results {
		if len(row) == 0 {
			return false
		}
	}
	return validHeaders(response)
}

func validHeaders(response searchResponse) bool {
	for _, headers := range [][]ResultHeader{
		response.ConversionCustomMetricHeaders, response.ConversionCustomDimensionHeaders,
		response.RawEventConversionMetricHeaders, response.RawEventConversionDimensionHeaders,
	} {
		for _, header := range headers {
			if !validNumericID(header.ID, 20) || !validBoundedText(header.Name, 4096) {
				return false
			}
		}
	}
	for _, header := range response.CustomColumnHeaders {
		if !validNumericID(header.ID, 20) || !validBoundedText(header.Name, 4096) {
			return false
		}
	}
	return true
}

func validCustomerResourceName(value string) bool {
	return strings.HasPrefix(value, "customers/") && validCustomerID(strings.TrimPrefix(value, "customers/"))
}

func validCustomColumnResourceName(customerID, value string) bool {
	prefix := "customers/" + customerID + "/customColumns/"
	return strings.HasPrefix(value, prefix) && validNumericID(strings.TrimPrefix(value, prefix), 20)
}

func validCustomColumn(customerID string, column CustomColumn) bool {
	return validCustomColumnResourceName(customerID, column.ResourceName) &&
		validNumericID(column.ID, 20) && strings.HasSuffix(column.ResourceName, "/"+column.ID) &&
		utf8.ValidString(column.Name) && len(column.Name) <= 4096 &&
		utf8.ValidString(column.Description) && len(column.Description) <= 16384
}

func validFieldName(value string) bool {
	if value == "" || len(value) > 512 || value[0] < 'a' || value[0] > 'z' {
		return false
	}
	for index := 1; index < len(value); index++ {
		character := value[index]
		if character >= 'a' && character <= 'z' || character >= '0' && character <= '9' ||
			character == '_' || character == '.' {
			continue
		}
		return false
	}
	return true
}

func validFieldResourceName(value string) bool {
	return strings.HasPrefix(value, "searchAds360Fields/") && validFieldName(strings.TrimPrefix(value, "searchAds360Fields/"))
}

func validField(field Field) bool {
	return validFieldResourceName(field.ResourceName) && field.Name == strings.TrimPrefix(field.ResourceName, "searchAds360Fields/")
}

func validFieldPage(fields []Field, requestedPageSize int) bool {
	if len(fields) > effectivePageSize(requestedPageSize) {
		return false
	}
	seen := make(map[string]struct{}, len(fields))
	for _, field := range fields {
		if !validField(field) {
			return false
		}
		if _, found := seen[field.ResourceName]; found {
			return false
		}
		seen[field.ResourceName] = struct{}{}
	}
	return true
}
