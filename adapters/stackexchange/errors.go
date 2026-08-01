package stackexchange

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
	ErrorID      int    `json:"error_id"`
	ErrorName    string `json:"error_name"`
	ErrorMessage string `json:"error_message"`
	Backoff      int    `json:"backoff"`
	Error        struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}

func decodeHTTPError(status int, header http.Header, body []byte) error {
	var response errorEnvelope
	_ = json.Unmarshal(body, &response)
	name := firstNonEmpty(response.ErrorName, response.Error.Type)
	message := firstNonEmpty(response.ErrorMessage, response.Error.Message)
	err := stackError("", status, response.ErrorID, name, message)
	retryAfter := parseRetryAfter(header.Get("Retry-After"))
	if response.Backoff > 0 {
		retryAfter = time.Duration(response.Backoff) * time.Second
	}
	err.RetryAfter = retryAfter
	return err
}

func wrapperError(operation string, id int, name, message string, backoff int) error {
	err := stackError(operation, http.StatusOK, id, name, message)
	if backoff > 0 {
		err.RetryAfter = time.Duration(backoff) * time.Second
	}
	return err
}

func stackError(operation string, status, id int, name, message string) *socialhub.Error {
	code, class := classifyError(status, name)
	platformCode := strings.TrimSpace(name)
	if platformCode == "" && id != 0 {
		platformCode = strconv.Itoa(id)
	}
	return &socialhub.Error{
		Code: code, Class: class, Platform: "stackexchange", Product: productName, Op: operation,
		HTTPStatus: status, PlatformCode: platformCode, PlatformMessage: boundedMessage(message, 512),
	}
}

func classifyError(status int, name string) (socialhub.ErrorCode, socialhub.ErrorClass) {
	switch name {
	case "bad_parameter", "invalid_parameter", "no_site", "write_failed", "invalid_request", "invalid_scope", "unsupported_response_type":
		return socialhub.CodeInvalidArgument, socialhub.ClassPermanent
	case "key_required", "key_too_old", "access_token_required", "access_token_expired", "access_token_compromised", "invalid_access_token":
		return socialhub.CodeUnauthenticated, socialhub.ClassUserAction
	case "access_denied", "unauthorized_client":
		return socialhub.CodePermissionDenied, socialhub.ClassUserAction
	case "duplicate_request":
		return socialhub.CodeConflict, socialhub.ClassPermanent
	case "throttle_violation", "too_many_ips":
		return socialhub.CodeRateLimited, socialhub.ClassRetryable
	case "internal_error", "temporarily_unavailable", "server_error":
		return socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable
	case "no_method":
		return socialhub.CodeNotFound, socialhub.ClassPermanent
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

func platformError(operation string, code socialhub.ErrorCode, class socialhub.ErrorClass, cause error) error {
	return &socialhub.Error{Code: code, Class: class, Platform: "stackexchange", Product: productName, Op: operation, Cause: cause}
}

func invalidArgument(operation, message string) error {
	return &socialhub.Error{
		Code: socialhub.CodeInvalidArgument, Class: socialhub.ClassPermanent, Platform: "stackexchange", Product: productName,
		Op: operation, PlatformMessage: message,
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
