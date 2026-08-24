package googledatamanager

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

type GoogleErrorDetail struct {
	Type   string `json:"@type,omitempty"`
	Reason string `json:"reason,omitempty"`
	Domain string `json:"domain,omitempty"`
}

type GoogleError struct {
	Code    int                 `json:"code"`
	Message string              `json:"message"`
	Status  string              `json:"status,omitempty"`
	Details []GoogleErrorDetail `json:"details,omitempty"`
}

type APIError struct {
	Hub    *socialhub.Error
	Google GoogleError
}

func (err *APIError) Error() string {
	if err == nil || err.Hub == nil {
		return "socialhub: google-data-manager: platform_error"
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

func newHTTPErrorDecoder(clock socialhub.Clock) func(int, http.Header, []byte) error {
	return func(status int, header http.Header, body []byte) error {
		return decodeHTTPErrorAt(status, header, body, clock.Now())
	}
}

func decodeHTTPError(status int, header http.Header, body []byte) error {
	return decodeHTTPErrorAt(status, header, body, time.Now())
}

func decodeHTTPErrorAt(status int, header http.Header, body []byte, now time.Time) error {
	var envelope struct {
		Error GoogleError `json:"error"`
	}
	_ = json.Unmarshal(body, &envelope)
	raw := envelope.Error
	reason := ""
	for index := range raw.Details {
		if reason == "" {
			reason = raw.Details[index].Reason
		}
	}
	code, class := classifyError(status, raw.Status, reason)
	statusCode := canonicalGoogleStatus(raw.Status)
	reasonCode := canonicalGoogleReason(reason)
	google := GoogleError{Code: raw.Code, Message: "Google Data Manager request failed", Status: statusCode}
	if reasonCode != "" {
		google.Details = []GoogleErrorDetail{{Reason: reasonCode}}
	}
	return &APIError{Hub: &socialhub.Error{
		Code: code, Class: class, Platform: platformName, Product: productName, HTTPStatus: status,
		PlatformCode: firstNonEmpty(reasonCode, statusCode), PlatformMessage: google.Message,
		RequestID:  boundedMessage(firstNonEmpty(header.Get("x-goog-request-id"), header.Get("x-google-request-id"), header.Get("x-request-id")), 256),
		RetryAfter: parseRetryAfterAt(header.Get("Retry-After"), now),
	}, Google: google}
}

func canonicalGoogleReason(value string) string {
	normalized := strings.NewReplacer("_", "", "-", "", ".", "").Replace(strings.ToLower(strings.TrimSpace(value)))
	switch normalized {
	case "ratelimitexceeded":
		return "RATE_LIMIT_EXCEEDED"
	case "quotaexceeded":
		return "QUOTA_EXCEEDED"
	case "exceededquota":
		return "EXCEEDED_QUOTA"
	case "resourceexhausted":
		return "RESOURCE_EXHAUSTED"
	case "dailylimitexceeded":
		return "DAILY_LIMIT_EXCEEDED"
	case "autherror":
		return "AUTH_ERROR"
	case "invalidcredentials":
		return "INVALID_CREDENTIALS"
	case "unauthenticated":
		return "UNAUTHENTICATED"
	case "insufficientpermissions":
		return "INSUFFICIENT_PERMISSIONS"
	case "permissiondenied":
		return "PERMISSION_DENIED"
	case "forbidden":
		return "FORBIDDEN"
	case "notfound":
		return "NOT_FOUND"
	case "backenderror":
		return "BACKEND_ERROR"
	case "internalerror":
		return "INTERNAL_ERROR"
	case "unavailable":
		return "UNAVAILABLE"
	case "invalid":
		return "INVALID"
	case "invalidargument":
		return "INVALID_ARGUMENT"
	case "failedprecondition":
		return "FAILED_PRECONDITION"
	case "outofrange":
		return "OUT_OF_RANGE"
	default:
		return ""
	}
}

func canonicalGoogleStatus(value string) string {
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case "INVALID_ARGUMENT", "FAILED_PRECONDITION", "OUT_OF_RANGE", "RESOURCE_EXHAUSTED",
		"UNAUTHENTICATED", "PERMISSION_DENIED", "NOT_FOUND", "ALREADY_EXISTS", "ABORTED",
		"UNAVAILABLE", "DEADLINE_EXCEEDED", "INTERNAL":
		return strings.ToUpper(strings.TrimSpace(value))
	default:
		return ""
	}
}

func classifyError(status int, platformStatus, reason string) (socialhub.ErrorCode, socialhub.ErrorClass) {
	normalized := strings.NewReplacer("_", "", "-", "", ".", "").Replace(strings.ToLower(strings.TrimSpace(reason)))
	switch normalized {
	case "ratelimitexceeded", "quotaexceeded", "exceededquota", "resourceexhausted":
		return socialhub.CodeRateLimited, socialhub.ClassRetryable
	case "dailylimitexceeded":
		return socialhub.CodeRateLimited, socialhub.ClassUserAction
	case "autherror", "invalidcredentials", "unauthenticated":
		return socialhub.CodeUnauthenticated, socialhub.ClassUserAction
	case "insufficientpermissions", "permissiondenied", "forbidden":
		return socialhub.CodePermissionDenied, socialhub.ClassUserAction
	case "notfound":
		return socialhub.CodeNotFound, socialhub.ClassPermanent
	case "backenderror", "internalerror", "unavailable":
		return socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable
	case "invalid", "invalidargument", "failedprecondition", "outofrange":
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
	return &socialhub.Error{Code: code, Class: class, Platform: platformName, Product: productName, Op: operation, Cause: sanitizeCause(cause)}
}

func authenticationError(operation, message string, cause error, secrets ...string) error {
	return &socialhub.Error{
		Code: socialhub.CodeUnauthenticated, Class: socialhub.ClassUserAction,
		Platform: platformName, Product: productName, Op: operation,
		PlatformMessage: boundedMessage(message, 1024), Cause: sanitizeCredentialCause(cause, secrets...), ApprovalURL: approvalURL,
	}
}

func invalidArgument(operation, message string) error {
	return &socialhub.Error{Code: socialhub.CodeInvalidArgument, Class: socialhub.ClassPermanent, Platform: platformName, Product: productName, Op: operation, PlatformMessage: boundedMessage(message, 1024)}
}

func platformContractError(operation, message string) error {
	return &socialhub.Error{Code: socialhub.CodePlatformError, Class: socialhub.ClassPermanent, Platform: platformName, Product: productName, Op: operation, PlatformMessage: boundedMessage(message, 1024)}
}

func withOperation(err error, operation string) error {
	if err == nil {
		return nil
	}
	var hub *socialhub.Error
	if errors.As(err, &hub) {
		hub.Op = operation
	}
	return err
}

func parseRetryAfter(value string) time.Duration {
	return parseRetryAfterAt(value, time.Now())
}

func parseRetryAfterAt(value string, now time.Time) time.Duration {
	value = strings.TrimSpace(value)
	seconds, err := strconv.ParseFloat(value, 64)
	if err == nil && seconds >= 0 && seconds <= float64((24*time.Hour)/time.Second) {
		return time.Duration(seconds * float64(time.Second))
	}
	when, err := http.ParseTime(value)
	if err != nil {
		return 0
	}
	delay := when.Sub(now)
	if delay < 0 || delay > 24*time.Hour {
		return 0
	}
	return delay
}

func boundedMessage(value string, maximum int) string {
	if !utf8.ValidString(value) {
		return ""
	}
	if utf8.RuneCountInString(value) <= maximum {
		return value
	}
	return string([]rune(value)[:maximum])
}

func redactExact(value string, secrets ...string) string {
	for _, secret := range secrets {
		if secret != "" {
			value = strings.ReplaceAll(value, secret, "[REDACTED]")
		}
	}
	return value
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
	for _, key := range []string{"access_token", "refresh_token", "client_secret", "assertion", "authorization", "bearer", "private_key"} {
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

func sanitizeCause(err error) error {
	if err == nil {
		return nil
	}
	var urlError *url.Error
	if errors.As(err, &urlError) && urlError.Err != nil {
		return urlError.Err
	}
	return err
}

func sanitizeCredentialCause(err error, secrets ...string) error {
	if err == nil {
		return nil
	}
	clean := sanitizeCause(err)
	return errors.New(boundedMessage(redactExact(redactSensitive(clean.Error()), secrets...), 1024))
}
