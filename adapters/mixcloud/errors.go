package mixcloud

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
	Error struct {
		Message    string `json:"message"`
		Type       string `json:"type"`
		RetryAfter int64  `json:"retry_after"`
	} `json:"error"`
	Details map[string][]string `json:"details"`
}

func decodeHTTPError(status int, header http.Header, body []byte) error {
	var response apiErrorEnvelope
	_ = json.Unmarshal(body, &response)
	code, class := classifyError(status, response.Error.Type)
	retryAfter := parseRetryAfter(header.Get("Retry-After"))
	if retryAfter == 0 && response.Error.RetryAfter > 0 && response.Error.RetryAfter <= int64((24*time.Hour)/time.Second) {
		retryAfter = time.Duration(response.Error.RetryAfter) * time.Second
	}
	return &socialhub.Error{
		Code: code, Class: class, Platform: "mixcloud", Product: productName, Op: "http",
		HTTPStatus: status, PlatformCode: boundedMessage(response.Error.Type, 128),
		PlatformMessage: boundedMessage(response.Error.Message, 512),
		RequestID:       boundedMessage(firstNonEmpty(header.Get("X-Request-Id"), header.Get("X-Correlation-Id")), 256),
		RetryAfter:      retryAfter,
	}
}

func classifyError(status int, platformType string) (socialhub.ErrorCode, socialhub.ErrorClass) {
	switch strings.TrimSpace(platformType) {
	case "RateLimitException":
		return socialhub.CodeRateLimited, socialhub.ClassRetryable
	case "ResourceNotFoundException":
		return socialhub.CodeNotFound, socialhub.ClassPermanent
	case "InvalidTokenException", "OAuthException":
		return socialhub.CodeUnauthenticated, socialhub.ClassUserAction
	}
	switch status {
	case http.StatusBadRequest, http.StatusMethodNotAllowed, http.StatusUnprocessableEntity, http.StatusRequestEntityTooLarge:
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
	return &socialhub.Error{Code: code, Class: class, Platform: "mixcloud", Product: productName, Op: operation, Cause: cause}
}

func invalidArgument(operation, message string) error {
	return &socialhub.Error{
		Code: socialhub.CodeInvalidArgument, Class: socialhub.ClassPermanent,
		Platform: "mixcloud", Product: productName, Op: operation, PlatformMessage: message,
	}
}

func unsupported(operation, message string) error {
	return &socialhub.Error{
		Code: socialhub.CodeUnsupported, Class: socialhub.ClassPermanent,
		Platform: "mixcloud", Product: productName, Op: operation, PlatformMessage: message,
	}
}

func approvalRequired(operation, message string) error {
	return &socialhub.Error{
		Code: socialhub.CodeApprovalRequired, Class: socialhub.ClassUserAction,
		Platform: "mixcloud", Product: productName, Op: operation, PlatformMessage: message,
		ApprovalURL: documentationURL,
	}
}

func parseRetryAfter(value string) time.Duration {
	seconds, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil || seconds < 0 || seconds > int64((24*time.Hour)/time.Second) {
		return 0
	}
	return time.Duration(seconds) * time.Second
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
