package matrix

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

type apiError struct {
	ErrCode      string `json:"errcode"`
	Message      string `json:"error"`
	RetryAfterMS int64  `json:"retry_after_ms"`
	SoftLogout   bool   `json:"soft_logout"`
}

func decodeHTTPError(status int, header http.Header, body []byte) error {
	var response apiError
	_ = json.Unmarshal(body, &response)
	code, class := classifyError(status, response.ErrCode)
	retryAfter := retryAfterHeader(header.Get("Retry-After"))
	if retryAfter == 0 && response.RetryAfterMS > 0 && response.RetryAfterMS <= int64((24*time.Hour)/time.Millisecond) {
		retryAfter = time.Duration(response.RetryAfterMS) * time.Millisecond
	}
	message := response.Message
	if response.SoftLogout {
		message = firstNonEmpty(message, "Matrix session requires soft logout")
	}
	return &socialhub.Error{
		Code: code, Class: class, Platform: "matrix", Product: productName, Op: "http",
		HTTPStatus: status, PlatformCode: boundedMessage(response.ErrCode, 128), PlatformMessage: boundedMessage(message, 512),
		RequestID: boundedMessage(firstNonEmpty(header.Get("X-Request-ID"), header.Get("X-Correlation-ID")), 512), RetryAfter: retryAfter,
	}
}

func classifyError(status int, platformCode string) (socialhub.ErrorCode, socialhub.ErrorClass) {
	switch platformCode {
	case "M_BAD_JSON", "M_NOT_JSON", "M_MISSING_PARAM", "M_INVALID_PARAM", "M_TOO_LARGE", "M_BAD_ALIAS":
		return socialhub.CodeInvalidArgument, socialhub.ClassPermanent
	case "M_MISSING_TOKEN", "M_UNKNOWN_TOKEN", "M_UNAUTHORIZED", "M_USER_DEACTIVATED":
		return socialhub.CodeUnauthenticated, socialhub.ClassUserAction
	case "M_FORBIDDEN", "M_GUEST_ACCESS_FORBIDDEN":
		return socialhub.CodePermissionDenied, socialhub.ClassUserAction
	case "M_NOT_FOUND":
		return socialhub.CodeNotFound, socialhub.ClassPermanent
	case "M_ROOM_IN_USE", "M_USER_IN_USE", "M_EXCLUSIVE", "M_CANNOT_OVERWRITE_MEDIA":
		return socialhub.CodeConflict, socialhub.ClassPermanent
	case "M_LIMIT_EXCEEDED":
		return socialhub.CodeRateLimited, socialhub.ClassRetryable
	case "M_RESOURCE_LIMIT_EXCEEDED":
		return socialhub.CodeRateLimited, socialhub.ClassUserAction
	case "M_NOT_YET_UPLOADED":
		return socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable
	case "M_UNRECOGNIZED", "M_UNSUPPORTED_ROOM_VERSION":
		return socialhub.CodeUnsupported, socialhub.ClassPermanent
	}
	switch status {
	case http.StatusBadRequest, http.StatusUnprocessableEntity, http.StatusRequestEntityTooLarge:
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

func retryAfterHeader(value string) time.Duration {
	seconds, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err != nil || seconds <= 0 || seconds > (24*time.Hour).Seconds() {
		return 0
	}
	return time.Duration(seconds * float64(time.Second))
}

func platformError(operation string, code socialhub.ErrorCode, class socialhub.ErrorClass, cause error) error {
	return &socialhub.Error{Code: code, Class: class, Platform: "matrix", Product: productName, Op: operation, Cause: cause}
}

func invalidArgument(operation, message string) error {
	return &socialhub.Error{Code: socialhub.CodeInvalidArgument, Class: socialhub.ClassPermanent, Platform: "matrix", Product: productName, Op: operation, PlatformMessage: message}
}

func unsupported(operation, message string) error {
	return &socialhub.Error{Code: socialhub.CodeUnsupported, Class: socialhub.ClassPermanent, Platform: "matrix", Product: productName, Op: operation, PlatformMessage: message}
}

func boundedMessage(value string, maximum int) string {
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
