package kochava

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

// Kochava error bodies may echo the App GUID, device identifiers, or strict
// authentication inputs, so only status and bounded response headers survive.
func newHTTPErrorDecoder(clock socialhub.Clock) func(int, http.Header, []byte) error {
	return func(status int, header http.Header, body []byte) error {
		return decodeHTTPErrorAt(status, header, body, clock.Now())
	}
}

func decodeHTTPError(status int, header http.Header, body []byte) error {
	return decodeHTTPErrorAt(status, header, body, time.Now())
}

func decodeHTTPErrorAt(status int, header http.Header, _ []byte, now time.Time) error {
	code, class := classifyError(status)
	return &socialhub.Error{
		Code: code, Class: class, Platform: platformName, Product: productName,
		HTTPStatus: status, PlatformMessage: "Kochava rejected the S2S measurement payload",
		RequestID:  firstBoundedHeader(header, 256, "X-Request-ID", "X-Correlation-ID"),
		RetryAfter: parseRetryAfterAt(header.Get("Retry-After"), now),
	}
}

func classifyError(status int) (socialhub.ErrorCode, socialhub.ErrorClass) {
	switch status {
	case http.StatusBadRequest, http.StatusRequestEntityTooLarge, http.StatusUnprocessableEntity:
		return socialhub.CodeInvalidArgument, socialhub.ClassPermanent
	case http.StatusUnauthorized:
		return socialhub.CodeUnauthenticated, socialhub.ClassUserAction
	case http.StatusForbidden:
		return socialhub.CodePermissionDenied, socialhub.ClassUserAction
	case http.StatusNotFound, http.StatusGone:
		return socialhub.CodeNotFound, socialhub.ClassPermanent
	case http.StatusConflict:
		return socialhub.CodeConflict, socialhub.ClassPermanent
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
		Code: code, Class: class, Platform: platformName, Product: productName,
		Op: operation, Cause: cause,
	}
}

func invalidArgument(operation, message string) error {
	return &socialhub.Error{
		Code: socialhub.CodeInvalidArgument, Class: socialhub.ClassPermanent,
		Platform: platformName, Product: productName, Op: operation, PlatformMessage: message,
	}
}

func authenticationError(operation, message string) error {
	return &socialhub.Error{
		Code: socialhub.CodeUnauthenticated, Class: socialhub.ClassUserAction,
		Platform: platformName, Product: productName, Op: operation,
		PlatformMessage: message, ApprovalURL: documentationURL,
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

func parseRetryAfter(value string) time.Duration {
	return parseRetryAfterAt(value, time.Now())
}

func parseRetryAfterAt(value string, now time.Time) time.Duration {
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

func firstBoundedHeader(header http.Header, maximum int, names ...string) string {
	for _, name := range names {
		if value := boundedHeader(header.Get(name), maximum); value != "" {
			return value
		}
	}
	return ""
}

func boundedHeader(value string, maximum int) string {
	value = strings.TrimSpace(value)
	if !utf8.ValidString(value) || len(value) > maximum || strings.ContainsFunc(value, unicode.IsControl) {
		return ""
	}
	return value
}
