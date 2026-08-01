package discord

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
	Code       int     `json:"code"`
	Message    string  `json:"message"`
	RetryAfter float64 `json:"retry_after"`
	Global     bool    `json:"global"`
}

func decodeHTTPError(status int, header http.Header, body []byte) error {
	var response apiError
	_ = json.Unmarshal(body, &response)
	code, class := classifyError(status, response.Code)
	retryAfter := secondsDuration(response.RetryAfter)
	if retryAfter == 0 {
		if value, err := strconv.ParseFloat(header.Get("Retry-After"), 64); err == nil {
			retryAfter = secondsDuration(value)
		}
	}
	platformCode := ""
	if response.Code != 0 {
		platformCode = strconv.Itoa(response.Code)
	}
	return &socialhub.Error{
		Code: code, Class: class, Platform: "discord", Product: "bot-api", Op: "http",
		HTTPStatus: status, PlatformCode: platformCode, PlatformMessage: boundedMessage(response.Message, 512),
		RequestID: firstNonEmpty(header.Get("X-Discord-Trace-Id"), header.Get("CF-Ray")), RetryAfter: retryAfter,
	}
}

func classifyError(status, platformCode int) (socialhub.ErrorCode, socialhub.ErrorClass) {
	switch platformCode {
	case 10001, 10003, 10008, 10013:
		return socialhub.CodeNotFound, socialhub.ClassPermanent
	case 20016, 20028, 20029:
		return socialhub.CodeRateLimited, socialhub.ClassRetryable
	case 40005, 50006, 50035, 50041, 50045, 50046:
		return socialhub.CodeInvalidArgument, socialhub.ClassPermanent
	case 50001, 50013:
		return socialhub.CodePermissionDenied, socialhub.ClassUserAction
	case 50014:
		return socialhub.CodeUnauthenticated, socialhub.ClassUserAction
	case 130000:
		return socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable
	}
	switch status {
	case http.StatusBadRequest, http.StatusUnprocessableEntity:
		return socialhub.CodeInvalidArgument, socialhub.ClassPermanent
	case http.StatusUnauthorized:
		return socialhub.CodeUnauthenticated, socialhub.ClassUserAction
	case http.StatusForbidden:
		return socialhub.CodePermissionDenied, socialhub.ClassUserAction
	case http.StatusNotFound:
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

func secondsDuration(value float64) time.Duration {
	if value <= 0 {
		return 0
	}
	return time.Duration(value * float64(time.Second))
}

func boundedMessage(value string, maximum int) string {
	if utf8.RuneCountInString(value) <= maximum {
		return value
	}
	return string([]rune(value)[:maximum])
}

func wrapError(operation string, code socialhub.ErrorCode, class socialhub.ErrorClass, cause error) error {
	return &socialhub.Error{Code: code, Class: class, Platform: "discord", Product: "bot-api", Op: operation, Cause: cause}
}

func invalidArgument(operation, message string) error {
	return &socialhub.Error{Code: socialhub.CodeInvalidArgument, Class: socialhub.ClassPermanent, Platform: "discord", Product: "bot-api", Op: operation, PlatformMessage: message}
}

func unsupported(operation, message string) error {
	return &socialhub.Error{Code: socialhub.CodeUnsupported, Class: socialhub.ClassPermanent, Platform: "discord", Product: "bot-api", Op: operation, PlatformMessage: message}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
