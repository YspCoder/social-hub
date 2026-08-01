package twitch

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

type errorResponse struct {
	Error   string `json:"error"`
	Status  int    `json:"status"`
	Message string `json:"message"`
}

func decodeHTTPError(status int, header http.Header, body []byte) error {
	var response errorResponse
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
	platformCode := response.Error
	if response.Status != 0 {
		platformCode = firstNonEmpty(platformCode, strconv.Itoa(response.Status))
	}
	return &socialhub.Error{
		Code: code, Class: class, Platform: "twitch", Product: productName, HTTPStatus: status,
		PlatformCode: boundedMessage(platformCode, 64), PlatformMessage: boundedMessage(response.Message, 512),
		RequestID: firstNonEmpty(header.Get("Twitch-Trace-Id"), header.Get("X-Request-Id")), RetryAfter: twitchRetryAfter(header),
	}
}

func platformError(operation string, code socialhub.ErrorCode, class socialhub.ErrorClass, cause error) error {
	return &socialhub.Error{Code: code, Class: class, Platform: "twitch", Product: productName, Op: operation, Cause: cause}
}

func invalidArgument(operation, message string) error {
	return &socialhub.Error{
		Code: socialhub.CodeInvalidArgument, Class: socialhub.ClassPermanent, Platform: "twitch", Product: productName,
		Op: operation, PlatformMessage: message,
	}
}

func unsupported(operation, message string) error {
	return &socialhub.Error{
		Code: socialhub.CodeUnsupported, Class: socialhub.ClassPermanent, Platform: "twitch", Product: productName,
		Op: operation, PlatformMessage: message,
	}
}

func twitchRetryAfter(header http.Header) time.Duration {
	if seconds, err := strconv.ParseFloat(header.Get("Retry-After"), 64); err == nil && seconds >= 0 {
		return time.Duration(seconds * float64(time.Second))
	}
	reset, err := strconv.ParseInt(header.Get("Ratelimit-Reset"), 10, 64)
	if err != nil || reset <= 0 {
		return 0
	}
	delay := time.Until(time.Unix(reset, 0))
	if delay < 0 {
		return 0
	}
	return delay
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
