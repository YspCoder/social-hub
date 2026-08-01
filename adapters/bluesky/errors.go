package bluesky

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

type xrpcError struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

func decodeHTTPError(status int, header http.Header, body []byte) error {
	var response xrpcError
	_ = json.Unmarshal(body, &response)
	code, class := socialhub.CodePlatformError, socialhub.ClassPermanent
	switch status {
	case http.StatusBadRequest, http.StatusUnprocessableEntity:
		code = socialhub.CodeInvalidArgument
	case http.StatusUnauthorized:
		code, class = socialhub.CodeUnauthenticated, socialhub.ClassUserAction
	case http.StatusForbidden:
		code, class = socialhub.CodePermissionDenied, socialhub.ClassUserAction
	case http.StatusNotFound, http.StatusGone:
		code = socialhub.CodeNotFound
	case http.StatusConflict:
		code = socialhub.CodeConflict
	case http.StatusTooManyRequests:
		code, class = socialhub.CodeRateLimited, socialhub.ClassRetryable
	default:
		if status >= 500 {
			code, class = socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable
		}
	}
	switch response.Error {
	case "RateLimitExceeded":
		code, class = socialhub.CodeRateLimited, socialhub.ClassRetryable
	case "InvalidToken", "ExpiredToken":
		code, class = socialhub.CodeUnauthenticated, socialhub.ClassUserAction
	case "AuthFactorTokenRequired":
		code, class = socialhub.CodeApprovalRequired, socialhub.ClassUserAction
	case "AccountTakedown", "AccountSuspended", "AccountDeactivated":
		code, class = socialhub.CodePermissionDenied, socialhub.ClassUserAction
	case "InvalidSwap":
		code = socialhub.CodeConflict
	case "NotFound", "RecordNotFound":
		code = socialhub.CodeNotFound
	}
	return &socialhub.Error{
		Code: code, Class: class, Platform: "bluesky", Product: productName, HTTPStatus: status,
		PlatformCode: xrpcErrorCode(response.Error), PlatformMessage: boundedMessage(response.Message, 512),
		RequestID: firstHeader(header, "x-request-id", "atproto-request-id", "x-correlation-id"), RetryAfter: retryAfter(header),
	}
}

func xrpcErrorCode(value string) string {
	if value == "" || strings.ContainsAny(value, " \t\r\n:") {
		return ""
	}
	return value
}

func retryAfter(header http.Header) time.Duration {
	if seconds, err := strconv.ParseInt(header.Get("Retry-After"), 10, 64); err == nil && seconds >= 0 {
		return time.Duration(seconds) * time.Second
	}
	if date, err := http.ParseTime(header.Get("Retry-After")); err == nil {
		return positiveDuration(time.Until(date))
	}
	if reset, err := strconv.ParseInt(header.Get("RateLimit-Reset"), 10, 64); err == nil && reset >= 0 {
		return positiveDuration(time.Until(time.Unix(reset, 0)))
	}
	return 0
}

func positiveDuration(value time.Duration) time.Duration {
	if value < 0 {
		return 0
	}
	return value
}

func firstHeader(header http.Header, names ...string) string {
	for _, name := range names {
		if value := header.Get(name); value != "" {
			return value
		}
	}
	return ""
}

func platformError(operation string, code socialhub.ErrorCode, class socialhub.ErrorClass, cause error) error {
	return &socialhub.Error{Code: code, Class: class, Platform: "bluesky", Product: productName, Op: operation, Cause: cause}
}

func invalidArgument(operation, message string) error {
	return &socialhub.Error{
		Code: socialhub.CodeInvalidArgument, Class: socialhub.ClassPermanent, Platform: "bluesky", Product: productName,
		Op: operation, PlatformMessage: message,
	}
}

func unsupported(operation, message string) error {
	return &socialhub.Error{
		Code: socialhub.CodeUnsupported, Class: socialhub.ClassPermanent, Platform: "bluesky", Product: productName,
		Op: operation, PlatformMessage: message,
	}
}

func boundedMessage(value string, maximum int) string {
	if utf8.RuneCountInString(value) <= maximum {
		return value
	}
	return string([]rune(value)[:maximum])
}
