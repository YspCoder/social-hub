package moloco

import (
	"net/url"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

func validIdentifier(value string) bool {
	if value == "" || value == "." || value == ".." || len(value) > 256 {
		return false
	}
	for _, character := range value {
		if (character < 'a' || character > 'z') && (character < 'A' || character > 'Z') &&
			(character < '0' || character > '9') && character != '-' && character != '_' && character != '.' {
			return false
		}
	}
	return true
}

func validSecret(value string) bool {
	if value == "" || value != strings.TrimSpace(value) || len(value) > 16_384 || !utf8.ValidString(value) {
		return false
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return false
		}
	}
	return true
}

func validDownloadURL(value string) bool {
	if value == "" || len(value) > 16_384 || !utf8.ValidString(value) {
		return false
	}
	parsed, err := url.Parse(value)
	return err == nil && parsed.Scheme == "https" && parsed.Host != "" && parsed.User == nil && parsed.Fragment == ""
}

func validReport(report Report, adAccountID string) bool {
	if !validIdentifier(report.ID) || report.AdAccountID != adAccountID ||
		report.DateRange.Start == "" || report.DateRange.End == "" || len(report.Dimensions) == 0 {
		return false
	}
	for _, dimension := range report.Dimensions {
		if !validIdentifier(string(dimension)) {
			return false
		}
	}
	for _, metric := range report.OptionalMetrics {
		if !validIdentifier(string(metric)) {
			return false
		}
	}
	start, err := time.Parse(dateLayout, report.DateRange.Start)
	if err != nil {
		return false
	}
	end, err := time.Parse(dateLayout, report.DateRange.End)
	return err == nil && !end.Before(start)
}

func prepareCallOptions(operation string, options []socialhub.CallOption) error {
	resolved, err := socialhub.ResolveCallOptions(options...)
	if err != nil {
		return platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	if resolved.IdempotencyKey != "" {
		return invalidArgument(operation, "read-only Moloco operations do not use idempotency keys")
	}
	if len(resolved.Fields) != 0 {
		return invalidArgument(operation, "the selected Moloco endpoints do not define field selection")
	}
	if resolved.RequestID != "" && !validRequestID(resolved.RequestID) {
		return invalidArgument(operation, "request ID is invalid")
	}
	return nil
}

func validRequestID(value string) bool {
	if value == "" || len(value) > 256 || value != strings.TrimSpace(value) || !utf8.ValidString(value) {
		return false
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return false
		}
	}
	return true
}
