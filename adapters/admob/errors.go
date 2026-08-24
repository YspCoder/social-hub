package admob

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

type GoogleErrorDetail struct {
	Type     string          `json:"@type,omitempty"`
	Reason   string          `json:"reason,omitempty"`
	Domain   string          `json:"domain,omitempty"`
	Metadata json.RawMessage `json:"metadata,omitempty"`
}

type GoogleError struct {
	Code    int                 `json:"code,omitempty"`
	Message string              `json:"message,omitempty"`
	Status  string              `json:"status,omitempty"`
	Details []GoogleErrorDetail `json:"details,omitempty"`
}

type APIError struct {
	Hub    *socialhub.Error
	Google GoogleError
}

func (err *APIError) Error() string {
	if err == nil || err.Hub == nil {
		return "admob: API error"
	}
	return err.Hub.Error()
}

func (err *APIError) Unwrap() error {
	if err == nil {
		return nil
	}
	return err.Hub
}

func (err *APIError) Retryable() bool { return err != nil && err.Hub != nil && err.Hub.Retryable() }

func decodeHTTPError(status int, header http.Header, body []byte) error {
	var envelope struct {
		Error GoogleError `json:"error"`
	}
	decodeErr := json.Unmarshal(body, &envelope)
	googleError := envelope.Error
	reason := ""
	for _, detail := range googleError.Details {
		if detail.Reason != "" {
			reason = detail.Reason
			break
		}
	}
	code, class := classifyError(status, googleError.Status, reason)
	message := googleError.Message
	if decodeErr != nil {
		message = http.StatusText(status)
	}
	hub := &socialhub.Error{
		Code: code, Class: class, Platform: platformName, Product: productName,
		HTTPStatus: status, PlatformCode: boundedMessage(firstNonEmpty(reason, googleError.Status), 256),
		PlatformMessage: boundedMessage(redactSensitive(message), 512),
		RequestID:       boundedMessage(firstNonEmpty(header.Get("x-goog-request-id"), header.Get("x-google-request-id"), header.Get("x-request-id")), 256),
		RetryAfter:      parseRetryAfter(header.Get("Retry-After")),
	}
	return &APIError{Hub: hub, Google: GoogleError{
		Code: googleError.Code, Message: boundedMessage(redactSensitive(googleError.Message), 512),
		Status: boundedMessage(googleError.Status, 256), Details: sanitizeDetails(googleError.Details),
	}}
}

func sanitizeDetails(values []GoogleErrorDetail) []GoogleErrorDetail {
	result := make([]GoogleErrorDetail, 0, len(values))
	for _, value := range values {
		result = append(result, GoogleErrorDetail{
			Type: boundedMessage(value.Type, 256), Reason: boundedMessage(value.Reason, 256),
			Domain: boundedMessage(value.Domain, 256),
		})
	}
	return result
}

func classifyError(status int, platformStatus, reason string) (socialhub.ErrorCode, socialhub.ErrorClass) {
	code, class := socialhub.CodePlatformError, socialhub.ClassPermanent
	switch status {
	case http.StatusBadRequest:
		code = socialhub.CodeInvalidArgument
	case http.StatusUnauthorized:
		code, class = socialhub.CodeUnauthenticated, socialhub.ClassUserAction
	case http.StatusForbidden:
		code, class = socialhub.CodePermissionDenied, socialhub.ClassUserAction
	case http.StatusNotFound:
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
	switch strings.ToUpper(firstNonEmpty(reason, platformStatus)) {
	case "UNAUTHENTICATED", "INVALID_CREDENTIALS":
		code, class = socialhub.CodeUnauthenticated, socialhub.ClassUserAction
	case "PERMISSION_DENIED", "ACCESS_TOKEN_SCOPE_INSUFFICIENT":
		code, class = socialhub.CodePermissionDenied, socialhub.ClassUserAction
	case "RESOURCE_EXHAUSTED", "RATE_LIMIT_EXCEEDED", "QUOTA_EXCEEDED":
		code, class = socialhub.CodeRateLimited, socialhub.ClassRetryable
	case "INVALID_ARGUMENT", "FAILED_PRECONDITION":
		code = socialhub.CodeInvalidArgument
	case "NOT_FOUND":
		code = socialhub.CodeNotFound
	case "ALREADY_EXISTS", "ABORTED":
		code = socialhub.CodeConflict
	case "UNAVAILABLE", "INTERNAL", "DEADLINE_EXCEEDED":
		code, class = socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable
	}
	return code, class
}

func platformError(operation string, code socialhub.ErrorCode, class socialhub.ErrorClass, cause error) error {
	return &socialhub.Error{Code: code, Class: class, Platform: platformName, Product: productName, Op: operation, Cause: cause}
}

func invalidArgument(operation, message string) error {
	return &socialhub.Error{Code: socialhub.CodeInvalidArgument, Class: socialhub.ClassPermanent, Platform: platformName, Product: productName, Op: operation, PlatformMessage: message}
}

func ownershipError(operation, resource string) error {
	return &socialhub.Error{Code: socialhub.CodePermissionDenied, Class: socialhub.ClassUserAction, Platform: platformName, Product: productName, Op: operation, PlatformMessage: "AdMob returned a " + resource + " outside the configured publisher account"}
}

func platformContractError(operation, message string) error {
	return &socialhub.Error{Code: socialhub.CodePlatformError, Class: socialhub.ClassPermanent, Platform: platformName, Product: productName, Op: operation, PlatformMessage: message}
}

func parseRetryAfter(value string) time.Duration {
	if value == "" {
		return 0
	}
	if seconds, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64); err == nil && seconds >= 0 {
		return time.Duration(seconds) * time.Second
	}
	if instant, err := http.ParseTime(value); err == nil {
		if delay := time.Until(instant); delay > 0 {
			return delay
		}
	}
	return 0
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

func redactSensitive(value string) string {
	for _, key := range []string{"access_token", "refresh_token", "client_secret", "authorization", "bearer"} {
		cursor := 0
		for {
			start := strings.Index(strings.ToLower(value[cursor:]), key)
			if start < 0 {
				break
			}
			start += cursor
			valueStart := start + len(key)
			for valueStart < len(value) && strings.ContainsRune(" \t:=\"'", rune(value[valueStart])) {
				valueStart++
			}
			if valueStart == start+len(key) {
				cursor = valueStart
				continue
			}
			valueEnd := valueStart
			for valueEnd < len(value) && !strings.ContainsRune(" \t\r\n,;&\"'", rune(value[valueEnd])) {
				valueEnd++
			}
			value = value[:valueStart] + "[REDACTED]" + value[valueEnd:]
			cursor = valueStart + len("[REDACTED]")
		}
	}
	return value
}

var _ error = (*APIError)(nil)
var _ interface{ Retryable() bool } = (*APIError)(nil)
