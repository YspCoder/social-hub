package awinpublisher

import (
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

const maximumTransactionWindow = 31 * 24 * time.Hour

func validEndpoint(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && (parsed.Scheme == "https" || parsed.Scheme == "http") && parsed.Host != "" &&
		parsed.User == nil && parsed.RawQuery == "" && parsed.Fragment == "" && !strings.HasSuffix(value, "/")
}

func validOpaque(value string, maximum int) bool {
	if value == "" || value != strings.TrimSpace(value) || len(value) > maximum || !utf8.ValidString(value) {
		return false
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return false
		}
	}
	return true
}

func validOptionalOpaque(value string, maximum int) bool {
	return value == "" || validOpaque(value, maximum)
}

func validCountryCode(value string) bool {
	return value == "" || len(value) == 2 && value[0] >= 'A' && value[0] <= 'Z' && value[1] >= 'A' && value[1] <= 'Z'
}

func validLocale(value string) bool {
	return len(value) == 5 && value[0] >= 'a' && value[0] <= 'z' && value[1] >= 'a' && value[1] <= 'z' &&
		value[2] == '_' && value[3] >= 'A' && value[3] <= 'Z' && value[4] >= 'A' && value[4] <= 'Z'
}

func validWebURL(value string) bool {
	if value == "" {
		return true
	}
	if !validOpaque(value, 4096) {
		return false
	}
	parsed, err := url.Parse(value)
	return err == nil && (parsed.Scheme == "https" || parsed.Scheme == "http") && parsed.Host != "" && parsed.User == nil
}

func validRequiredWebURL(value string) bool { return value != "" && validWebURL(value) }

func positiveExactID(value ExactValue) (int64, bool) {
	parsed, err := strconv.ParseInt(value.String(), 10, 64)
	return parsed, err == nil && parsed > 0
}

func validExactIdentifier(value ExactValue) bool {
	return value.IsSet() && !value.IsNull() && validOpaque(value.String(), maxExactValueBytes)
}

func validListPrograms(input ListProgramsRequest) bool {
	if !validCountryCode(input.CountryCode) || input.IncludeHidden && input.Relationship != "" {
		return false
	}
	switch input.Relationship {
	case "", RelationshipJoined, RelationshipPending, RelationshipSuspended, RelationshipRejected, RelationshipNotJoined:
		return true
	default:
		return false
	}
}

func validDownloadEnhancedFeed(input DownloadEnhancedFeedRequest) bool {
	return input.AdvertiserID > 0 && validLocale(input.Locale)
}

func validGenerateTrackingLink(input GenerateTrackingLinkRequest) bool {
	if input.AdvertiserID <= 0 || !validWebURL(input.DestinationURL) {
		return false
	}
	for _, value := range []string{
		input.Parameters.Campaign, input.Parameters.ClickRef, input.Parameters.ClickRef2,
		input.Parameters.ClickRef3, input.Parameters.ClickRef4, input.Parameters.ClickRef5, input.Parameters.ClickRef6,
	} {
		if !validOptionalOpaque(value, 255) {
			return false
		}
	}
	return true
}

func validListTransactions(input ListTransactionsRequest) bool {
	if input.StartDate.IsZero() || input.EndDate.IsZero() || input.EndDate.Before(input.StartDate) ||
		input.EndDate.Sub(input.StartDate) > maximumTransactionWindow || !validTimezone(input.Timezone) {
		return false
	}
	switch input.DateType {
	case "", DateTypeTransaction, DateTypeValidation, DateTypeAmendment:
	default:
		return false
	}
	switch input.Status {
	case "", TransactionPending, TransactionApproved, TransactionDeclined, TransactionDeleted:
	default:
		return false
	}
	seen := make(map[int64]struct{}, len(input.AdvertiserIDs))
	for _, identifier := range input.AdvertiserIDs {
		if identifier <= 0 {
			return false
		}
		if _, found := seen[identifier]; found {
			return false
		}
		seen[identifier] = struct{}{}
	}
	return true
}

func validGetAdvertiserPerformance(input GetAdvertiserPerformanceRequest) bool {
	start, startOK := parseDate(input.StartDate)
	end, endOK := parseDate(input.EndDate)
	if !startOK || !endOK || end.Before(start) || !validRegion(input.Region) || !validTimezone(input.Timezone) {
		return false
	}
	return input.DateType == "" || input.DateType == DateTypeTransaction || input.DateType == DateTypeValidation
}

func parseDate(value Date) (time.Time, bool) {
	if len(value) != len("2006-01-02") {
		return time.Time{}, false
	}
	parsed, err := time.Parse("2006-01-02", string(value))
	return parsed, err == nil && Date(parsed.Format("2006-01-02")) == value
}

func validTimezone(value string) bool {
	if value == "" {
		return true
	}
	if !validOpaque(value, 255) {
		return false
	}
	_, err := time.LoadLocation(value)
	return err == nil
}

func validRegion(value Region) bool {
	switch value {
	case RegionAT, RegionAU, RegionBE, RegionBR, RegionBU, RegionCA, RegionCH, RegionDE, RegionDK, RegionES,
		RegionFI, RegionFR, RegionGB, RegionIE, RegionIT, RegionNL, RegionNO, RegionPL, RegionSE, RegionUS:
		return true
	default:
		return false
	}
}

func prepareCallOptions(operation string, options []socialhub.CallOption) (socialhub.CallOptions, error) {
	resolved, err := socialhub.ResolveCallOptions(options...)
	if err != nil {
		return socialhub.CallOptions{}, platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	if resolved.IdempotencyKey != "" {
		return socialhub.CallOptions{}, invalidArgument(operation, "Awin Publisher API 1.0 does not define idempotency keys for these workflows")
	}
	if len(resolved.Fields) > 0 {
		return socialhub.CallOptions{}, invalidArgument(operation, "these Publisher API endpoints do not define field selection")
	}
	if resolved.RequestID != "" && !validOpaque(resolved.RequestID, 256) {
		return socialhub.CallOptions{}, invalidArgument(operation, "request ID is invalid")
	}
	return resolved, nil
}
