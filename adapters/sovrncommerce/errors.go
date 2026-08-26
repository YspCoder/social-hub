package sovrncommerce

import (
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

// Sovrn's Commerce APIs do not publish one common error envelope. Error bodies
// are therefore discarded because they may contain credentials or report data.
func newHTTPErrorDecoder(clock socialhub.Clock, secrets ...string) func(int, http.Header, []byte) error {
	return func(status int, header http.Header, _ []byte) error {
		return decodeHTTPError(status, header, clock.Now(), secrets...)
	}
}

func decodeHTTPError(status int, header http.Header, now time.Time, secrets ...string) error {
	code, class := classifyHTTPError(status)
	hub := &socialhub.Error{
		Code: code, Class: class, Platform: platformName, Product: productName,
		HTTPStatus:      status,
		PlatformCode:    "http_" + strconv.Itoa(status),
		PlatformMessage: "Sovrn Commerce rejected the request",
		RequestID:       firstSafeHeader(header, 256, secrets, "X-Request-ID", "X-Correlation-ID"),
		RetryAfter:      parseRetryAfter(header.Get("Retry-After"), now),
	}
	if code == socialhub.CodeUnauthenticated || code == socialhub.CodePermissionDenied || code == socialhub.CodeApprovalRequired {
		hub.ApprovalURL = documentationURL
	}
	return hub
}

func classifyHTTPError(status int) (socialhub.ErrorCode, socialhub.ErrorClass) {
	switch status {
	case http.StatusBadRequest, http.StatusMethodNotAllowed, http.StatusRequestEntityTooLarge,
		http.StatusUnsupportedMediaType, http.StatusUnprocessableEntity:
		return socialhub.CodeInvalidArgument, socialhub.ClassPermanent
	case http.StatusUnauthorized:
		return socialhub.CodeUnauthenticated, socialhub.ClassUserAction
	case http.StatusPaymentRequired:
		return socialhub.CodeApprovalRequired, socialhub.ClassUserAction
	case http.StatusForbidden:
		return socialhub.CodePermissionDenied, socialhub.ClassUserAction
	case http.StatusNotFound, http.StatusGone:
		return socialhub.CodeNotFound, socialhub.ClassPermanent
	case http.StatusConflict:
		return socialhub.CodeConflict, socialhub.ClassPermanent
	case http.StatusRequestTimeout, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
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

func authenticationError(operation, message string, cause error, secrets ...string) error {
	return &socialhub.Error{
		Code: socialhub.CodeUnauthenticated, Class: socialhub.ClassUserAction,
		Platform: platformName, Product: productName, Op: operation,
		PlatformMessage: boundedMessage(message, 512), ApprovalURL: documentationURL,
		Cause: sanitizeCredentialCause(cause, secrets...),
	}
}

func invalidArgument(operation, message string) error {
	return &socialhub.Error{
		Code: socialhub.CodeInvalidArgument, Class: socialhub.ClassPermanent,
		Platform: platformName, Product: productName, Op: operation,
		PlatformMessage: boundedMessage(message, 1024),
	}
}

func platformContractError(operation, message string, status int) error {
	return &socialhub.Error{
		Code: socialhub.CodePlatformError, Class: socialhub.ClassPermanent,
		Platform: platformName, Product: productName, Op: operation,
		HTTPStatus: status, PlatformMessage: boundedMessage(message, 1024),
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
	value = boundedHeader(value, 128)
	if value == "" {
		return 0
	}
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

func firstSafeHeader(header http.Header, maximum int, secrets []string, names ...string) string {
	for _, name := range names {
		if value := safeHeaderValue(header.Get(name), maximum, secrets); value != "" {
			return value
		}
	}
	return ""
}

func safeHeaderValue(value string, maximum int, secrets []string) string {
	value = boundedHeader(value, maximum)
	if value == "" {
		return ""
	}
	for _, secret := range secrets {
		if secret != "" && strings.Contains(value, secret) {
			return ""
		}
	}
	return value
}

func boundedHeader(value string, maximum int) string {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > maximum || !utf8.ValidString(value) || strings.ContainsFunc(value, unicode.IsControl) {
		return ""
	}
	return value
}

func boundedMessage(value string, maximum int) string {
	if maximum <= 0 || !utf8.ValidString(value) || strings.ContainsFunc(value, unicode.IsControl) {
		return ""
	}
	if utf8.RuneCountInString(value) <= maximum {
		return value
	}
	return string([]rune(value)[:maximum])
}

func sanitizeCause(err error) error {
	var urlError *url.Error
	if errors.As(err, &urlError) && urlError.Err != nil {
		return urlError.Err
	}
	return err
}

func sanitizeCredentialCause(cause error, secrets ...string) error {
	if cause == nil {
		return nil
	}
	message := sanitizeCause(cause).Error()
	for _, secret := range secrets {
		if secret != "" {
			message = strings.ReplaceAll(message, secret, "[REDACTED]")
		}
	}
	message = redactCredentialFields(message)
	if !utf8.ValidString(message) || len(message) > 1024 || strings.ContainsFunc(message, unicode.IsControl) {
		message = "credential resolution failed"
	}
	return errors.New(message)
}

func redactCredentialFields(value string) string {
	for _, marker := range []string{
		"secret_key", "secret key", "api_key", "api key", "authorization",
		"bearer", "token", "password", "secret", "credential",
	} {
		for cursor := 0; cursor < len(value); {
			index := strings.Index(strings.ToLower(value[cursor:]), marker)
			if index < 0 {
				break
			}
			index += cursor
			start := index + len(marker)
			for start < len(value) && strings.ContainsRune(" \t:=\"'", rune(value[start])) {
				start++
			}
			if start == index+len(marker) {
				cursor = start
				continue
			}
			end := start
			for end < len(value) && !strings.ContainsRune(" \t\r\n,;&}\"'<", rune(value[end])) {
				end++
			}
			value = value[:start] + "[REDACTED]" + value[end:]
			cursor = start + len("[REDACTED]")
		}
	}
	return value
}
