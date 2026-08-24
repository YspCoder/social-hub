package panglereporting

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

func businessError(operation string, status int, header http.Header, platformCode string, now time.Time) error {
	code, class, message := classifyBusinessCode(platformCode)
	result := &socialhub.Error{
		Code: code, Class: class, Platform: platformName, Product: productName, Op: operation,
		HTTPStatus: status, PlatformCode: platformCode, PlatformMessage: message,
		RequestID:  firstBoundedHeader(header, 256, "X-Request-ID", "X-Tt-Logid"),
		RetryAfter: parseRetryAfter(header.Get("Retry-After"), now),
	}
	setApprovalURL(result)
	return result
}

func classifyBusinessCode(platformCode string) (socialhub.ErrorCode, socialhub.ErrorClass, string) {
	switch platformCode {
	case "101":
		return socialhub.CodeUnauthenticated, socialhub.ClassUserAction, "Pangle rejected the Reporting API signature"
	case "102":
		return socialhub.CodeUnauthenticated, socialhub.ClassUserAction, "Pangle rejected the configured publisher account"
	case "103":
		return socialhub.CodeInvalidArgument, socialhub.ClassPermanent, "Pangle rejected the report date"
	case "106":
		return socialhub.CodeRateLimited, socialhub.ClassRetryable, "Pangle Reporting API quota was exceeded"
	case "114":
		return socialhub.CodeInvalidArgument, socialhub.ClassPermanent, "Pangle rejected a report parameter"
	case "133":
		return socialhub.CodeInvalidArgument, socialhub.ClassPermanent, "Pangle rejected the report region"
	default:
		return socialhub.CodePlatformError, socialhub.ClassPermanent, "Pangle rejected the publisher report request"
	}
}

func httpStatusError(status int, header http.Header, platformCode string, now time.Time) error {
	code, class := classifyHTTPStatus(status)
	result := &socialhub.Error{
		Code: code, Class: class, Platform: platformName, Product: productName, Op: "income_report",
		HTTPStatus: status, PlatformCode: platformCode, PlatformMessage: "Pangle rejected the publisher report request",
		RequestID:  firstBoundedHeader(header, 256, "X-Request-ID", "X-Tt-Logid"),
		RetryAfter: parseRetryAfter(header.Get("Retry-After"), now),
	}
	setApprovalURL(result)
	return result
}

func setApprovalURL(err *socialhub.Error) {
	if err != nil && (err.Code == socialhub.CodeUnauthenticated || err.Code == socialhub.CodePermissionDenied ||
		err.Code == socialhub.CodeApprovalRequired) {
		err.ApprovalURL = documentationURL
	}
}

func classifyHTTPStatus(status int) (socialhub.ErrorCode, socialhub.ErrorClass) {
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
		Op: operation, Cause: sanitizeTransportError(cause),
	}
}

func authenticationError(operation string, cause error) error {
	var safeCause error
	if cause != nil {
		safeCause = errors.New("credential resolution failed")
	}
	return &socialhub.Error{
		Code: socialhub.CodeUnauthenticated, Class: socialhub.ClassUserAction,
		Platform: platformName, Product: productName, Op: operation,
		PlatformMessage: "Pangle Security Key resolution failed", ApprovalURL: documentationURL,
		Cause: safeCause,
	}
}

func invalidArgument(operation, message string) error {
	return &socialhub.Error{
		Code: socialhub.CodeInvalidArgument, Class: socialhub.ClassPermanent,
		Platform: platformName, Product: productName, Op: operation, PlatformMessage: message,
	}
}

func platformContractError(operation, message string, status int) error {
	return &socialhub.Error{
		Code: socialhub.CodePlatformError, Class: socialhub.ClassPermanent,
		Platform: platformName, Product: productName, Op: operation,
		HTTPStatus: status, PlatformMessage: message,
	}
}

func sanitizeTransportError(err error) error {
	var urlError *url.Error
	if errors.As(err, &urlError) && urlError.Err != nil {
		return urlError.Err
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

func safeHeader(value string, maximum int, redactions ...string) string {
	value = boundedHeader(value, maximum)
	if value == "" {
		return ""
	}
	for _, redaction := range redactions {
		if redaction != "" && strings.Contains(value, redaction) {
			return ""
		}
	}
	return value
}

func sanitizedResponseHeaders(header http.Header, redactions ...string) http.Header {
	result := make(http.Header)
	for _, name := range []string{"X-Request-ID", "X-Tt-Logid"} {
		if value := safeHeader(header.Get(name), 256, redactions...); value != "" {
			result.Set(name, value)
		}
	}
	if value := boundedHeader(header.Get("Retry-After"), 128); value != "" {
		result.Set("Retry-After", value)
	}
	return result
}
