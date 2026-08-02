package simkl

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

type errorEnvelope struct {
	Error            string `json:"error"`
	Code             int    `json:"code"`
	Message          string `json:"message"`
	ErrorDescription string `json:"error_description"`
}

func decodeHTTPError(status int, header http.Header, body []byte) error {
	var envelope errorEnvelope
	_ = json.Unmarshal(body, &envelope)
	code, class := classifyHTTPError(status)
	switch envelope.Error {
	case "user_token_failed", "grant_error", "secret_error", "invalid_token", "invalid_grant":
		code, class = socialhub.CodeUnauthenticated, socialhub.ClassUserAction
	case "client_id_failed":
		// This code covers invalid/disabled clients and exhausted quota. Retrying
		// without operator action can extend Simkl's temporary block.
		code, class = socialhub.CodeApprovalRequired, socialhub.ClassUserAction
	case "rate_limit":
		code, class = socialhub.CodeRateLimited, socialhub.ClassRetryable
	case "access_denied", "forbidden":
		code, class = socialhub.CodePermissionDenied, socialhub.ClassUserAction
	}
	return &socialhub.Error{
		Code: code, Class: class, Platform: "simkl", Product: productName, Op: "http",
		HTTPStatus: status, PlatformCode: bounded(envelope.Error, 128),
		PlatformMessage: bounded(firstNonEmpty(envelope.Message, envelope.ErrorDescription), 512),
		RequestID:       bounded(firstNonEmpty(header.Get("X-Request-ID"), header.Get("X-Correlation-ID"), header.Get("CF-Ray")), 256),
		RetryAfter:      parseRetryAfter(header.Get("Retry-After")), ApprovalURL: approvalURL(envelope.Error),
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
	case http.StatusNotFound, http.StatusGone:
		return socialhub.CodeNotFound, socialhub.ClassPermanent
	case http.StatusConflict:
		return socialhub.CodeConflict, socialhub.ClassPermanent
	case http.StatusPreconditionFailed:
		return socialhub.CodeApprovalRequired, socialhub.ClassUserAction
	case http.StatusTooManyRequests:
		return socialhub.CodeRateLimited, socialhub.ClassRetryable
	default:
		if status >= 500 {
			return socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable
		}
		return socialhub.CodePlatformError, socialhub.ClassPermanent
	}
}

func approvalURL(platformCode string) string {
	if platformCode == "client_id_failed" {
		return developerPortalURL
	}
	return ""
}

func platformError(operation string, code socialhub.ErrorCode, class socialhub.ErrorClass, cause error) error {
	return &socialhub.Error{Code: code, Class: class, Platform: "simkl", Product: productName, Op: operation, Cause: cause}
}

func invalidArgument(operation, message string) error {
	return &socialhub.Error{
		Code: socialhub.CodeInvalidArgument, Class: socialhub.ClassPermanent,
		Platform: "simkl", Product: productName, Op: operation, PlatformMessage: message,
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
