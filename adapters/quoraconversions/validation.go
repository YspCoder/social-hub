package quoraconversions

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

const maximumEventTextRunes = 4096

var (
	decimalPattern   = regexp.MustCompile(`^-?(?:0|[1-9][0-9]*)(?:\.[0-9]+)?(?:[eE][+-]?[0-9]+)?$`)
	errorCodePattern = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,128}$`)
)

// NormalizeEmail applies Quora's documented pre-hash normalization: trim and
// lowercase, remove a plus suffix, and remove dots from gmail.com local parts.
func NormalizeEmail(value string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(value))
	separator := strings.LastIndexByte(normalized, '@')
	if separator <= 0 || separator == len(normalized)-1 || strings.Contains(normalized[:separator], "@") {
		return "", fmt.Errorf("quoraconversions: invalid email")
	}
	local, domain := normalized[:separator], normalized[separator+1:]
	if plus := strings.IndexByte(local, '+'); plus >= 0 {
		local = local[:plus]
	}
	if domain == "gmail.com" {
		local = strings.ReplaceAll(local, ".", "")
	}
	if local == "" || strings.ContainsFunc(normalized, unicode.IsSpace) || strings.ContainsFunc(normalized, unicode.IsControl) {
		return "", fmt.Errorf("quoraconversions: invalid email")
	}
	return local + "@" + domain, nil
}

// HashEmail returns the lowercase SHA-256 hex digest accepted by Quora.
func HashEmail(value string) (string, error) {
	normalized, err := NormalizeEmail(value)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256([]byte(normalized))
	return hex.EncodeToString(digest[:]), nil
}

func validateEvent(input ConversionEvent) error {
	if !validEventName(input.Conversion.EventName) {
		return fmt.Errorf("conversion.event_name is invalid")
	}
	if input.Conversion.Timestamp != nil && *input.Conversion.Timestamp < 0 {
		return fmt.Errorf("conversion.timestamp must be non-negative Unix microseconds")
	}
	if input.Conversion.Value != "" && !validDecimal(input.Conversion.Value) {
		return fmt.Errorf("conversion.value must be a valid JSON number")
	}
	fields := []struct {
		path  string
		value string
	}{
		{"conversion.event_id", input.Conversion.EventID},
		{"conversion.click_id", input.Conversion.ClickID},
		{"user.email", input.User.Email},
		{"user.phone_number", input.User.PhoneNumber},
		{"user.name", input.User.Name},
		{"user.ip", input.User.IP},
		{"user.country", input.User.Country},
		{"user.region", input.User.Region},
		{"user.city", input.User.City},
		{"user.postal_code", input.User.PostalCode},
		{"user.company_name", input.User.CompanyName},
		{"user.job_title", input.User.JobTitle},
		{"user.date_of_birth", input.User.DateOfBirth},
		{"device.mobile_device_id", input.Device.MobileDeviceID},
		{"device.referer", input.Device.Referer},
		{"device.user_agent", input.Device.UserAgent},
		{"device.language", input.Device.Language},
	}
	for _, field := range fields {
		if !validOptionalText(field.value, maximumEventTextRunes) {
			return fmt.Errorf("%s must be valid UTF-8 without control characters and at most %d characters", field.path, maximumEventTextRunes)
		}
	}
	return nil
}

func validateBatch(events []ConversionEvent) error {
	if len(events) == 0 || len(events) > MaximumBatchSize {
		return fmt.Errorf("events must contain between 1 and %d items", MaximumBatchSize)
	}
	for index := range events {
		if err := validateEvent(events[index]); err != nil {
			return fmt.Errorf("events[%d]: %w", index, err)
		}
	}
	return nil
}

func validEventName(value EventName) bool {
	switch value {
	case EventGeneric, EventAppInstall, EventPurchase, EventGenerateLead, EventCompleteRegistration,
		EventAddPaymentInfo, EventAddToCart, EventAddToWishlist, EventInitiateCheckout, EventSearch:
		return true
	default:
		return false
	}
}

func validDecimal(value Decimal) bool {
	return len(value) > 0 && len(value) <= 128 && decimalPattern.MatchString(string(value))
}

func validWarningCode(value WarningCode) bool {
	switch value {
	case WarningClickIDMissing, WarningClickIDInvalidFormat, WarningEventIDMissing:
		return true
	default:
		return false
	}
}

func validErrorCode(value string) bool {
	return value == "" || errorCodePattern.MatchString(value)
}

func validOpaque(value string, maximum int) bool {
	return value != "" && len(value) <= maximum && strings.TrimSpace(value) == value &&
		utf8.ValidString(value) && !strings.ContainsFunc(value, unicode.IsControl)
}

func validOptionalText(value string, maximum int) bool {
	return value == "" || len(value) <= maximum*4 && utf8.ValidString(value) &&
		utf8.RuneCountInString(value) <= maximum && !strings.ContainsFunc(value, unicode.IsControl)
}

func validResponseText(value string, maximum int) bool {
	return len(value) <= maximum*4 && utf8.ValidString(value) && utf8.RuneCountInString(value) <= maximum
}

func prepareCallOptions(operation string, options []socialhub.CallOption) ([]socialhub.CallOption, error) {
	resolved, err := socialhub.ResolveCallOptions(options...)
	if err != nil {
		return nil, platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	if resolved.RequestID != "" || resolved.IdempotencyKey != "" || len(resolved.Fields) != 0 {
		return nil, invalidArgument(operation, "only per-call timeouts are supported by Quora conversion submission")
	}
	if resolved.Timeout < 0 {
		return nil, invalidArgument(operation, "timeout must not be negative")
	}
	if resolved.Timeout == 0 {
		return nil, nil
	}
	return []socialhub.CallOption{socialhub.WithCallTimeout(resolved.Timeout)}, nil
}
