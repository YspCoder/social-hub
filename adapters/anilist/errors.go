package anilist

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

type graphQLError struct {
	Message    string              `json:"message"`
	Status     int                 `json:"status"`
	Validation map[string][]string `json:"validation,omitempty"`
	Extensions struct {
		Code string `json:"code"`
	} `json:"extensions,omitempty"`
}

type errorEnvelope struct {
	Errors           []graphQLError `json:"errors"`
	Error            string         `json:"error"`
	ErrorDescription string         `json:"error_description"`
	Message          string         `json:"message"`
}

func decodeHTTPError(status int, header http.Header, body []byte) error {
	var envelope errorEnvelope
	_ = json.Unmarshal(body, &envelope)
	if len(envelope.Errors) > 0 {
		return graphQLPlatformError("graphql", status, header, envelope.Errors[0])
	}
	message := firstNonEmpty(envelope.ErrorDescription, envelope.Message)
	code, class := classifyError(status, envelope.Error, message)
	return &socialhub.Error{
		Code: code, Class: class, Platform: "anilist", Product: productName, Op: "http",
		HTTPStatus: status, PlatformCode: bounded(envelope.Error, 128), PlatformMessage: bounded(message, 512),
		RequestID:  bounded(firstNonEmpty(header.Get("X-Request-ID"), header.Get("X-Correlation-ID"), header.Get("CF-Ray")), 256),
		RetryAfter: parseRetryAfter(header.Get("Retry-After")),
	}
}

func graphQLPlatformError(operation string, httpStatus int, header http.Header, source graphQLError) error {
	status := source.Status
	if status == 0 {
		status = httpStatus
	}
	platformCode := source.Extensions.Code
	if platformCode == "" && status != 0 {
		platformCode = strconv.Itoa(status)
	}
	code, class := classifyError(status, platformCode, source.Message)
	return &socialhub.Error{
		Code: code, Class: class, Platform: "anilist", Product: productName, Op: operation,
		HTTPStatus: httpStatus, PlatformCode: bounded(platformCode, 128), PlatformMessage: bounded(source.Message, 512),
		RequestID:  bounded(firstNonEmpty(header.Get("X-Request-ID"), header.Get("X-Correlation-ID"), header.Get("CF-Ray")), 256),
		RetryAfter: parseRetryAfter(header.Get("Retry-After")),
	}
}

func classifyError(status int, platformCode, message string) (socialhub.ErrorCode, socialhub.ErrorClass) {
	if status == http.StatusTooManyRequests {
		return socialhub.CodeRateLimited, socialhub.ClassRetryable
	}
	switch strings.ToLower(platformCode) {
	case "invalid_grant", "invalid_client", "invalid_token", "unauthenticated":
		return socialhub.CodeUnauthenticated, socialhub.ClassUserAction
	case "access_denied", "forbidden":
		return socialhub.CodePermissionDenied, socialhub.ClassUserAction
	case "server_error", "temporarily_unavailable":
		return socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable
	}
	lowerMessage := strings.ToLower(message)
	if status == http.StatusForbidden && (strings.Contains(lowerMessage, "temporarily disabled") ||
		strings.Contains(lowerMessage, "temporarily unavailable") || strings.Contains(lowerMessage, "maintenance")) {
		return socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable
	}
	switch status {
	case http.StatusBadRequest, http.StatusMethodNotAllowed, http.StatusNotAcceptable, http.StatusUnprocessableEntity:
		return socialhub.CodeInvalidArgument, socialhub.ClassPermanent
	case http.StatusUnauthorized:
		return socialhub.CodeUnauthenticated, socialhub.ClassUserAction
	case http.StatusForbidden:
		return socialhub.CodePermissionDenied, socialhub.ClassUserAction
	case http.StatusNotFound, http.StatusGone:
		return socialhub.CodeNotFound, socialhub.ClassPermanent
	case http.StatusConflict:
		return socialhub.CodeConflict, socialhub.ClassPermanent
	default:
		if status >= 500 {
			return socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable
		}
		return socialhub.CodePlatformError, socialhub.ClassPermanent
	}
}

func platformError(operation string, code socialhub.ErrorCode, class socialhub.ErrorClass, cause error) error {
	return &socialhub.Error{Code: code, Class: class, Platform: "anilist", Product: productName, Op: operation, Cause: cause}
}

func invalidArgument(operation, message string) error {
	return &socialhub.Error{
		Code: socialhub.CodeInvalidArgument, Class: socialhub.ClassPermanent,
		Platform: "anilist", Product: productName, Op: operation, PlatformMessage: message,
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
