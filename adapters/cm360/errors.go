package cm360

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

type GoogleErrorItem struct {
	Domain  string `json:"domain,omitempty"`
	Reason  string `json:"reason,omitempty"`
	Message string `json:"message,omitempty"`
}

type GoogleError struct {
	Code    int               `json:"code"`
	Message string            `json:"message"`
	Status  string            `json:"status,omitempty"`
	Errors  []GoogleErrorItem `json:"errors,omitempty"`
}

// APIError augments socialhub.Error with the sanitized Google error envelope.
type APIError struct {
	Hub    *socialhub.Error
	Google GoogleError
}

func (err *APIError) Error() string {
	if err == nil || err.Hub == nil {
		return "socialhub: google-cm360: platform_error"
	}
	return err.Hub.Error()
}

func (err *APIError) Unwrap() error {
	if err == nil {
		return nil
	}
	return err.Hub
}

func (err *APIError) Retryable() bool {
	return err != nil && err.Hub != nil && err.Hub.Retryable()
}

func decodeHTTPError(status int, header http.Header, body []byte) error {
	var envelope struct {
		Error GoogleError `json:"error"`
	}
	_ = json.Unmarshal(body, &envelope)
	google := envelope.Error
	google.Status = boundedMessage(redactSensitive(google.Status), 256)
	google.Message = boundedMessage(redactSensitive(google.Message), 1024)
	if len(google.Errors) > 32 {
		google.Errors = append([]GoogleErrorItem(nil), google.Errors[:32]...)
	}
	reason := ""
	for index := range google.Errors {
		google.Errors[index].Domain = boundedMessage(redactSensitive(google.Errors[index].Domain), 256)
		google.Errors[index].Reason = boundedMessage(redactSensitive(google.Errors[index].Reason), 256)
		google.Errors[index].Message = boundedMessage(redactSensitive(google.Errors[index].Message), 512)
		if reason == "" {
			reason = google.Errors[index].Reason
		}
	}
	code, class := classifyError(status, google.Status, reason)
	platformCode := firstNonEmpty(reason, google.Status)
	return &APIError{
		Hub: &socialhub.Error{
			Code: code, Class: class, Platform: platformName, Product: productName, HTTPStatus: status,
			PlatformCode: boundedMessage(platformCode, 256), PlatformMessage: google.Message,
			RequestID: boundedMessage(firstNonEmpty(
				header.Get("x-goog-request-id"), header.Get("x-google-request-id"), header.Get("x-request-id"),
			), 256),
			RetryAfter: parseRetryAfter(header.Get("Retry-After")),
		},
		Google: google,
	}
}

func classifyError(status int, platformStatus, reason string) (socialhub.ErrorCode, socialhub.ErrorClass) {
	switch strings.ToLower(strings.TrimSpace(reason)) {
	case "userratelimitexceeded", "ratelimitexceeded", "quotaexceeded":
		return socialhub.CodeRateLimited, socialhub.ClassRetryable
	case "dailylimitexceeded":
		return socialhub.CodeRateLimited, socialhub.ClassUserAction
	case "autherror", "invalidcredentials", "required":
		return socialhub.CodeUnauthenticated, socialhub.ClassUserAction
	case "insufficientpermissions", "forbidden":
		return socialhub.CodePermissionDenied, socialhub.ClassUserAction
	case "notfound":
		return socialhub.CodeNotFound, socialhub.ClassPermanent
	case "backenderror", "internalerror":
		return socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable
	case "invalid", "invalidparameter", "badrequest":
		return socialhub.CodeInvalidArgument, socialhub.ClassPermanent
	}
	switch strings.ToUpper(strings.TrimSpace(platformStatus)) {
	case "INVALID_ARGUMENT", "FAILED_PRECONDITION", "OUT_OF_RANGE":
		return socialhub.CodeInvalidArgument, socialhub.ClassPermanent
	case "RESOURCE_EXHAUSTED":
		return socialhub.CodeRateLimited, socialhub.ClassRetryable
	case "UNAUTHENTICATED":
		return socialhub.CodeUnauthenticated, socialhub.ClassUserAction
	case "PERMISSION_DENIED":
		return socialhub.CodePermissionDenied, socialhub.ClassUserAction
	case "NOT_FOUND":
		return socialhub.CodeNotFound, socialhub.ClassPermanent
	case "ALREADY_EXISTS", "ABORTED":
		return socialhub.CodeConflict, socialhub.ClassPermanent
	case "UNAVAILABLE", "DEADLINE_EXCEEDED", "INTERNAL":
		return socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable
	}
	switch status {
	case http.StatusBadRequest, http.StatusUnprocessableEntity, http.StatusRequestEntityTooLarge:
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
	return &socialhub.Error{
		Code: code, Class: class, Platform: platformName, Product: productName, Op: operation,
		Cause: sanitizeCause(cause),
	}
}

func invalidArgument(operation, message string) error {
	return &socialhub.Error{
		Code: socialhub.CodeInvalidArgument, Class: socialhub.ClassPermanent,
		Platform: platformName, Product: productName, Op: operation, PlatformMessage: message,
	}
}

func ownershipError(operation, resource string) error {
	return &socialhub.Error{
		Code: socialhub.CodePermissionDenied, Class: socialhub.ClassUserAction,
		Platform: platformName, Product: productName, Op: operation,
		PlatformMessage: resource + " is not owned by the configured advertiser",
	}
}

func platformContractError(operation, message string) error {
	return &socialhub.Error{
		Code: socialhub.CodePlatformError, Class: socialhub.ClassPermanent,
		Platform: platformName, Product: productName, Op: operation, PlatformMessage: message,
	}
}

func parseRetryAfter(value string) time.Duration {
	value = strings.TrimSpace(value)
	seconds, err := strconv.ParseFloat(value, 64)
	if err == nil && seconds >= 0 && seconds <= float64((24*time.Hour)/time.Second) {
		return time.Duration(seconds * float64(time.Second))
	}
	when, err := http.ParseTime(value)
	if err != nil {
		return 0
	}
	delay := time.Until(when)
	if delay < 0 || delay > 24*time.Hour {
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
