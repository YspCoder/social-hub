package panglereporting

import (
	"math"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

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

func validNumericID(value string) bool {
	if value == "" || len(value) > 20 {
		return false
	}
	nonzero := false
	for index := range value {
		if value[index] < '0' || value[index] > '9' {
			return false
		}
		nonzero = nonzero || value[index] != '0'
	}
	return nonzero
}

func validDate(value Date) bool {
	if len(value) != len("2006-01-02") {
		return false
	}
	_, err := time.Parse("2006-01-02", string(value))
	return err == nil
}

func validateReportRequest(input ReportRequest) error {
	if !validDate(input.Date) {
		return invalidArgument("income_report", "date must use yyyy-MM-dd")
	}
	if input.TimeZone != nil && *input.TimeZone != TimeZoneUTC && *input.TimeZone != TimeZoneUTC8 {
		return invalidArgument("income_report", "time zone must be UTC+0 or UTC+8")
	}
	if input.Currency != "" && input.Currency != CurrencyUSD && input.Currency != CurrencyCNY {
		return invalidArgument("income_report", "currency must be usd or cny")
	}
	if input.Region != "" && !validRequestRegion(input.Region) {
		return invalidArgument("income_report", "region must be a lowercase two-letter ISO 3166-1 code")
	}
	if !validAppIDs(input.AppIDs) || !validDimensions(input.Dimensions) {
		return invalidArgument("income_report", "app IDs or dimensions are invalid")
	}
	return nil
}

func validRequestRegion(value string) bool {
	return len(value) == 2 && value[0] >= 'a' && value[0] <= 'z' && value[1] >= 'a' && value[1] <= 'z'
}

func validResponseRegion(value string) bool {
	if value == "" {
		return true
	}
	return len(value) == 2 && isASCIILetter(value[0]) && isASCIILetter(value[1])
}

func isASCIILetter(value byte) bool {
	return value >= 'a' && value <= 'z' || value >= 'A' && value <= 'Z'
}

func validAppIDs(values []ID) bool {
	if len(values) > 500 {
		return false
	}
	seen := make(map[ID]struct{}, len(values))
	for _, value := range values {
		if !validNumericID(string(value)) {
			return false
		}
		if _, exists := seen[value]; exists {
			return false
		}
		seen[value] = struct{}{}
	}
	return true
}

func validDimensions(values []Dimension) bool {
	if len(values) > 5 {
		return false
	}
	seen := make(map[Dimension]struct{}, len(values))
	for _, value := range values {
		switch value {
		case DimensionUserID, DimensionSiteID, DimensionAdSlotType, DimensionRegion, DimensionIsBidding:
		default:
			return false
		}
		if _, exists := seen[value]; exists {
			return false
		}
		seen[value] = struct{}{}
	}
	return true
}

func validateCallOptions(options []socialhub.CallOption) (socialhub.CallOptions, error) {
	resolved, err := socialhub.ResolveCallOptions(options...)
	if err != nil {
		return socialhub.CallOptions{}, platformError("income_report", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	if resolved.RequestID != "" {
		return socialhub.CallOptions{}, invalidArgument("income_report", "Pangle Reporting API does not define caller request IDs")
	}
	if resolved.IdempotencyKey != "" || len(resolved.Fields) > 0 {
		return socialhub.CallOptions{}, invalidArgument("income_report", "idempotency keys and field selection are not supported")
	}
	return resolved, nil
}

func validIncomeRow(row IncomeRow, requested Date) bool {
	if row.Date != "" && row.Date != requested || row.TimeZone != TimeZoneUTC && row.TimeZone != TimeZoneUTC8 ||
		row.Currency != CurrencyUSD && row.Currency != CurrencyCNY || !validResponseRegion(row.Region) {
		return false
	}
	for _, id := range []ID{row.UserID, row.SiteID, row.AppID, row.AdSlotID} {
		if id != "" && !validNumericID(string(id)) {
			return false
		}
	}
	if !validAdSlotType(row.AdSlotType) || row.UseMediation < 0 || row.UseMediation > 1 ||
		row.BiddingType < 0 || row.BiddingType > 2 || row.AppCodeType < 0 || row.AppCodeType > 1_000 {
		return false
	}
	if row.Requests < 0 || row.Returned < 0 || row.Impressions < 0 || row.Clicks < 0 ||
		row.AdRequests < 0 || row.Responses < 0 {
		return false
	}
	for _, decimal := range []Decimal{row.FillRate, row.ClickRate, row.Revenue, row.ECPM, row.AdFillRate, row.AdImpressionRate} {
		if decimal != "" && !validNonnegativeDecimal(decimal) {
			return false
		}
	}
	return validResponseText(row.AppName, 1_024) && validResponseText(row.PackageName, 1_024) &&
		validResponseText(row.MediaName, 1_024) && validResponseText(row.CodeName, 1_024) && validResponseText(row.OS, 128)
}

func validAdSlotType(value int) bool {
	switch value {
	case 0, 1, 2, 3, 5, 6:
		return true
	default:
		return false
	}
}

func validNonnegativeDecimal(value Decimal) bool {
	text := string(value)
	if text == "" || len(text) > 128 || strings.TrimSpace(text) != text || strings.HasPrefix(text, "-") {
		return false
	}
	parsed, err := strconv.ParseFloat(text, 64)
	return err == nil && !math.IsNaN(parsed) && !math.IsInf(parsed, 0) && parsed >= 0
}

func validResponseText(value string, maximum int) bool {
	return utf8.ValidString(value) && len(value) <= maximum && !strings.ContainsRune(value, '\x00')
}
