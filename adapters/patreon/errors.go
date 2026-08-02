package patreon

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
	ID                string `json:"id"`
	Status            string `json:"status"`
	Code              string `json:"code"`
	CodeName          string `json:"code_name"`
	Title             string `json:"title"`
	Detail            string `json:"detail"`
	RetryAfterSeconds int64  `json:"retry_after_seconds"`
}

func decodeHTTPError(status int, header http.Header, body []byte) error {
	var envelope struct {
		Errors           []apiError `json:"errors"`
		Error            string     `json:"error"`
		ErrorDescription string     `json:"error_description"`
	}
	_ = json.Unmarshal(body, &envelope)
	var response apiError
	if len(envelope.Errors) > 0 {
		response = envelope.Errors[0]
	}
	code, class := classifyError(status)
	retryAfter := parseRetryAfter(header.Get("Retry-After"))
	if retryAfter == 0 && response.RetryAfterSeconds > 0 && response.RetryAfterSeconds <= int64((24*time.Hour)/time.Second) {
		retryAfter = time.Duration(response.RetryAfterSeconds) * time.Second
	}
	return &socialhub.Error{
		Code: code, Class: class, Platform: "patreon", Product: productName, HTTPStatus: status,
		PlatformCode:    boundedMessage(firstNonEmpty(response.CodeName, response.Code, response.Status, envelope.Error), 256),
		PlatformMessage: boundedMessage(firstNonEmpty(response.Detail, response.Title, envelope.ErrorDescription), 512),
		RequestID:       boundedMessage(firstNonEmpty(response.ID, header.Get("X-Request-Id"), header.Get("X-Correlation-Id")), 256),
		RetryAfter:      retryAfter,
	}
}

func classifyError(status int) (socialhub.ErrorCode, socialhub.ErrorClass) {
	switch status {
	case http.StatusBadRequest, http.StatusMethodNotAllowed, http.StatusNotAcceptable, http.StatusUnprocessableEntity, http.StatusRequestEntityTooLarge:
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
	return &socialhub.Error{Code: code, Class: class, Platform: "patreon", Product: productName, Op: operation, Cause: cause}
}

func invalidArgument(operation, message string) error {
	return &socialhub.Error{
		Code: socialhub.CodeInvalidArgument, Class: socialhub.ClassPermanent, Platform: "patreon", Product: productName,
		Op: operation, PlatformMessage: message,
	}
}

func unsupported(operation, message string) error {
	return &socialhub.Error{
		Code: socialhub.CodeUnsupported, Class: socialhub.ClassPermanent, Platform: "patreon", Product: productName,
		Op: operation, PlatformMessage: message,
	}
}

func parseRetryAfter(value string) time.Duration {
	seconds, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err != nil || seconds < 0 || seconds > float64((24*time.Hour)/time.Second) {
		return 0
	}
	return time.Duration(seconds * float64(time.Second))
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
