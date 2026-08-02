package lastfm

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

type apiErrorEnvelope struct {
	Error   *int   `json:"error"`
	Message string `json:"message"`
}

func decodeHTTPError(status int, header http.Header, body []byte) error {
	var envelope apiErrorEnvelope
	_ = json.Unmarshal(body, &envelope)
	if envelope.Error != nil {
		return lastFMError("http", status, *envelope.Error, envelope.Message, header)
	}
	code, class := classifyHTTPError(status)
	return &socialhub.Error{
		Code: code, Class: class, Platform: "lastfm", Product: productName, Op: "http",
		HTTPStatus: status, RequestID: bounded(firstNonEmpty(header.Get("X-Request-ID"), header.Get("X-Correlation-ID")), 256),
		RetryAfter: parseRetryAfter(header.Get("Retry-After")),
	}
}

func lastFMError(operation string, status, platformCode int, message string, header http.Header) error {
	code, class := classifyAPIError(platformCode)
	return &socialhub.Error{
		Code: code, Class: class, Platform: "lastfm", Product: productName, Op: operation,
		HTTPStatus: status, PlatformCode: strconv.Itoa(platformCode), PlatformMessage: bounded(message, 512),
		RequestID:  bounded(firstNonEmpty(header.Get("X-Request-ID"), header.Get("X-Correlation-ID")), 256),
		RetryAfter: parseRetryAfter(header.Get("Retry-After")),
	}
}

func classifyAPIError(code int) (socialhub.ErrorCode, socialhub.ErrorClass) {
	switch code {
	case 4, 9, 10, 13, 15:
		return socialhub.CodeUnauthenticated, socialhub.ClassUserAction
	case 6:
		return socialhub.CodeInvalidArgument, socialhub.ClassPermanent
	case 7:
		return socialhub.CodeNotFound, socialhub.ClassPermanent
	case 14, 26:
		return socialhub.CodePermissionDenied, socialhub.ClassUserAction
	case 11, 16:
		return socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable
	case 29:
		return socialhub.CodeRateLimited, socialhub.ClassRetryable
	default:
		return socialhub.CodePlatformError, socialhub.ClassPermanent
	}
}

func classifyHTTPError(status int) (socialhub.ErrorCode, socialhub.ErrorClass) {
	switch status {
	case http.StatusBadRequest, http.StatusMethodNotAllowed, http.StatusUnprocessableEntity:
		return socialhub.CodeInvalidArgument, socialhub.ClassPermanent
	case http.StatusUnauthorized:
		return socialhub.CodeUnauthenticated, socialhub.ClassUserAction
	case http.StatusForbidden:
		return socialhub.CodePermissionDenied, socialhub.ClassUserAction
	case http.StatusNotFound:
		return socialhub.CodeNotFound, socialhub.ClassPermanent
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
	return &socialhub.Error{Code: code, Class: class, Platform: "lastfm", Product: productName, Op: operation, Cause: cause}
}

func invalidArgument(operation, message string) error {
	return &socialhub.Error{
		Code: socialhub.CodeInvalidArgument, Class: socialhub.ClassPermanent,
		Platform: "lastfm", Product: productName, Op: operation, PlatformMessage: message,
	}
}

func parseRetryAfter(value string) time.Duration {
	seconds, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil || seconds < 0 || seconds > int64((24*time.Hour)/time.Second) {
		return 0
	}
	return time.Duration(seconds) * time.Second
}

func bounded(value string, maximum int) string {
	if utf8.RuneCountInString(value) <= maximum {
		return value
	}
	return string([]rune(value)[:maximum])
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
