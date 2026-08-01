package mastodon

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

type mastodonError struct {
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

func decodeHTTPError(status int, header http.Header, body []byte) error {
	var response mastodonError
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
	return &socialhub.Error{
		Code: code, Class: class, Platform: "mastodon", Product: "mastodon-rest-api", HTTPStatus: status,
		PlatformCode: oauthErrorCode(response.Error), PlatformMessage: boundedMessage(firstNonEmpty(response.ErrorDescription, response.Error), 512),
		RequestID: firstNonEmpty(header.Get("x-request-id"), header.Get("x-correlation-id")), RetryAfter: retryAfter(header),
	}
}

func oauthErrorCode(value string) string {
	if value == "" || strings.ContainsAny(value, " \t\r\n:") {
		return ""
	}
	return value
}

func retryAfter(header http.Header) time.Duration {
	seconds, err := strconv.ParseInt(header.Get("Retry-After"), 10, 64)
	if err == nil && seconds >= 0 {
		return time.Duration(seconds) * time.Second
	}
	reset, err := time.Parse(time.RFC3339, header.Get("X-RateLimit-Reset"))
	if err != nil {
		return 0
	}
	duration := time.Until(reset)
	if duration < 0 {
		return 0
	}
	return duration
}

func platformError(operation string, code socialhub.ErrorCode, class socialhub.ErrorClass, cause error) error {
	return &socialhub.Error{Code: code, Class: class, Platform: "mastodon", Product: "mastodon-rest-api", Op: operation, Cause: cause}
}

func invalidArgument(operation, message string) error {
	return &socialhub.Error{Code: socialhub.CodeInvalidArgument, Class: socialhub.ClassPermanent, Platform: "mastodon", Product: "mastodon-rest-api", Op: operation, PlatformMessage: message}
}

func unsupported(operation, message string) error {
	return &socialhub.Error{Code: socialhub.CodeUnsupported, Class: socialhub.ClassPermanent, Platform: "mastodon", Product: "mastodon-rest-api", Op: operation, PlatformMessage: message}
}

func boundedMessage(value string, maximum int) string {
	if utf8.RuneCountInString(value) <= maximum {
		return value
	}
	return string([]rune(value)[:maximum])
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
