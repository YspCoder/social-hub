package ebaybrowse

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

func newHTTPErrorDecoder(clock socialhub.Clock) func(int, http.Header, []byte) error {
	return func(status int, header http.Header, body []byte) error {
		var provider ProviderError
		if json.Unmarshal(body, &provider) == nil && (provider.ErrorID != 0 || provider.Message != "" || provider.LongMessage != "") {
			return ebayErrorValue("http", status, header, provider, clock.Now())
		}
		var envelope struct {
			Errors []ProviderError `json:"errors"`
		}
		if json.Unmarshal(body, &envelope) == nil && len(envelope.Errors) > 0 {
			return ebayErrorValue("http", status, header, envelope.Errors[0], clock.Now())
		}
		code, class := classifyHTTPError(status)
		return &socialhub.Error{
			Code: code, Class: class, Platform: platformName, Product: productName,
			HTTPStatus: status, PlatformMessage: "eBay rejected the request",
			RequestID:  boundedMessage(firstHeader(header, "X-EBAY-C-REQUEST-ID", "X-EBAY-CORRELATION-ID", "X-Request-ID"), 256),
			RetryAfter: parseRetryAfter(header.Get("Retry-After"), clock.Now()),
		}
	}
}

func ebayErrorValue(operation string, status int, header http.Header, provider ProviderError, now time.Time) error {
	code, class := classifyEbayError(status, provider.ErrorID, provider.Category)
	platformCode := providerErrorCode(provider)
	message := firstNonEmpty(provider.LongMessage, provider.Message, "eBay returned an error response")
	result := &socialhub.Error{
		Code: code, Class: class, Platform: platformName, Product: productName, Op: operation,
		HTTPStatus: status, PlatformCode: boundedMessage(platformCode, 256),
		PlatformMessage: boundedMessage(redactSensitive(message), 1024),
		RequestID:       boundedMessage(firstHeader(header, "X-EBAY-C-REQUEST-ID", "X-EBAY-CORRELATION-ID", "X-Request-ID"), 256),
		RetryAfter:      parseRetryAfter(header.Get("Retry-After"), now),
	}
	if code == socialhub.CodeApprovalRequired || code == socialhub.CodePermissionDenied {
		result.RequiredScopes = []string{applicationScope}
		result.ApprovalURL = documentationURL
	}
	return result
}

func providerErrorCode(provider ProviderError) string {
	parts := make([]string, 0, 3)
	if provider.ErrorID != 0 {
		parts = append(parts, strconv.FormatInt(provider.ErrorID, 10))
	}
	if provider.Domain != "" {
		parts = append(parts, provider.Domain)
	}
	if provider.Category != "" {
		parts = append(parts, provider.Category)
	}
	return strings.Join(parts, "/")
}

func classifyEbayError(status int, errorID int64, category string) (socialhub.ErrorCode, socialhub.ErrorClass) {
	if errorID == 11000 || strings.EqualFold(strings.TrimSpace(category), "SYSTEM") {
		return socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable
	}
	return classifyHTTPError(status)
}

func classifyHTTPError(status int) (socialhub.ErrorCode, socialhub.ErrorClass) {
	switch status {
	case http.StatusBadRequest, http.StatusMethodNotAllowed, http.StatusRequestEntityTooLarge,
		http.StatusUnsupportedMediaType, http.StatusUnprocessableEntity:
		return socialhub.CodeInvalidArgument, socialhub.ClassPermanent
	case http.StatusUnauthorized:
		return socialhub.CodeUnauthenticated, socialhub.ClassUserAction
	case http.StatusForbidden:
		return socialhub.CodePermissionDenied, socialhub.ClassUserAction
	case http.StatusNotFound, http.StatusGone:
		return socialhub.CodeNotFound, socialhub.ClassPermanent
	case http.StatusConflict:
		return socialhub.CodeConflict, socialhub.ClassPermanent
	case http.StatusRequestTimeout:
		return socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable
	case http.StatusTooManyRequests:
		return socialhub.CodeRateLimited, socialhub.ClassRetryable
	default:
		if status >= 500 {
			return socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable
		}
		return socialhub.CodePlatformError, socialhub.ClassPermanent
	}
}

func platformError(operation string, code socialhub.ErrorCode, class socialhub.ErrorClass, cause error) error {
	return &socialhub.Error{
		Code: code, Class: class, Platform: platformName, Product: productName, Op: operation,
		Cause: sanitizeCause(cause),
	}
}

func invalidArgument(operation, message string) error {
	return &socialhub.Error{
		Code: socialhub.CodeInvalidArgument, Class: socialhub.ClassPermanent,
		Platform: platformName, Product: productName, Op: operation,
		PlatformMessage: boundedMessage(message, 1024),
	}
}

func platformContractError(operation, message string) error {
	return &socialhub.Error{
		Code: socialhub.CodePlatformError, Class: socialhub.ClassPermanent,
		Platform: platformName, Product: productName, Op: operation,
		PlatformMessage: boundedMessage(message, 1024),
	}
}

func withOperation(err error, operation string) error {
	if err == nil {
		return nil
	}
	var hub *socialhub.Error
	if errors.As(err, &hub) {
		hub.Op = operation
	}
	return err
}

func parseRetryAfter(value string, now time.Time) time.Duration {
	value = strings.TrimSpace(value)
	if seconds, err := strconv.ParseFloat(value, 64); err == nil && seconds >= 0 && seconds <= float64((24*time.Hour)/time.Second) {
		return time.Duration(seconds * float64(time.Second))
	}
	when, err := http.ParseTime(value)
	if err != nil {
		return 0
	}
	delay := when.Sub(now)
	if delay < 0 || delay > 24*time.Hour {
		return 0
	}
	return delay
}

func firstHeader(header http.Header, names ...string) string {
	for _, name := range names {
		if value := header.Get(name); value != "" {
			return value
		}
	}
	return ""
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func boundedMessage(value string, maximum int) string {
	if utf8.RuneCountInString(value) <= maximum {
		return value
	}
	return string([]rune(value)[:maximum])
}

func redactSensitive(value string) string {
	for _, key := range []string{"access_token", "authorization", "client_secret", "cert_id"} {
		cursor := 0
		for {
			start := strings.Index(strings.ToLower(value[cursor:]), key)
			if start < 0 {
				break
			}
			start += cursor
			valueStart := start + len(key)
			for valueStart < len(value) && strings.ContainsRune(" \t:=\"'", rune(value[valueStart])) {
				valueStart++
			}
			if valueStart == start+len(key) {
				cursor = valueStart
				continue
			}
			valueEnd := valueStart
			for valueEnd < len(value) && !strings.ContainsRune(" \t\r\n,;&\"'", rune(value[valueEnd])) {
				valueEnd++
			}
			value = value[:valueStart] + "[REDACTED]" + value[valueEnd:]
			cursor = valueStart + len("[REDACTED]")
		}
	}
	return value
}

func redactExact(value string, secrets ...string) string {
	for _, secret := range secrets {
		if secret != "" {
			value = strings.ReplaceAll(value, secret, "[REDACTED]")
		}
	}
	return value
}

func sanitizeCause(err error) error {
	var urlError *url.Error
	if errors.As(err, &urlError) && urlError.Err != nil {
		return urlError.Err
	}
	return err
}
