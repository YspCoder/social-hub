package kitsu

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

type jsonAPIError struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	Code   string `json:"code"`
	Title  string `json:"title"`
	Detail string `json:"detail"`
	Source struct {
		Pointer   string `json:"pointer"`
		Parameter string `json:"parameter"`
	} `json:"source"`
}

type errorEnvelope struct {
	Errors           []jsonAPIError `json:"errors"`
	Error            string         `json:"error"`
	ErrorDescription string         `json:"error_description"`
	Message          string         `json:"message"`
}

func decodeHTTPError(status int, header http.Header, body []byte) error {
	var envelope errorEnvelope
	_ = json.Unmarshal(body, &envelope)
	var source jsonAPIError
	if len(envelope.Errors) > 0 {
		source = envelope.Errors[0]
	}
	platformCode := firstNonEmpty(source.Code, source.Status, envelope.Error)
	message := firstNonEmpty(source.Detail, source.Title, envelope.ErrorDescription, envelope.Message)
	code, class := classifyError(status, platformCode)
	return &socialhub.Error{
		Code: code, Class: class, Platform: "kitsu", Product: productName, Op: "http",
		HTTPStatus: status, PlatformCode: bounded(platformCode, 128), PlatformMessage: bounded(message, 512),
		RequestID:  bounded(firstNonEmpty(header.Get("X-Request-ID"), header.Get("X-Correlation-ID"), header.Get("CF-Ray"), source.ID), 256),
		RetryAfter: parseRetryAfter(header.Get("Retry-After")),
	}
}

func classifyError(status int, platformCode string) (socialhub.ErrorCode, socialhub.ErrorClass) {
	if status == http.StatusTooManyRequests {
		return socialhub.CodeRateLimited, socialhub.ClassRetryable
	}
	switch strings.ToLower(platformCode) {
	case "invalid_grant", "invalid_client", "invalid_token", "unauthorized":
		return socialhub.CodeUnauthenticated, socialhub.ClassUserAction
	case "access_denied", "forbidden":
		return socialhub.CodePermissionDenied, socialhub.ClassUserAction
	case "temporarily_unavailable", "server_error":
		return socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable
	}
	switch status {
	case http.StatusBadRequest, http.StatusMethodNotAllowed, http.StatusNotAcceptable, http.StatusUnsupportedMediaType, http.StatusUnprocessableEntity:
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
	return &socialhub.Error{Code: code, Class: class, Platform: "kitsu", Product: productName, Op: operation, Cause: cause}
}

func invalidArgument(operation, message string) error {
	return &socialhub.Error{
		Code: socialhub.CodeInvalidArgument, Class: socialhub.ClassPermanent,
		Platform: "kitsu", Product: productName, Op: operation, PlatformMessage: message,
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
