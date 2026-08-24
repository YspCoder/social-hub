package applovinreporting

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

type ReportingErrorDetails struct {
	Message string
	TraceID string
}

// APIError augments socialhub.Error with sanitized AppLovin response details.
type APIError struct {
	Hub           *socialhub.Error
	Reporting     ReportingErrorDetails
	notWebAccount bool
}

func (err *APIError) Error() string {
	if err == nil || err.Hub == nil {
		return "socialhub: applovin: platform_error"
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
	message := "AppLovin Growth Reporting API request failed"
	traceID := boundedHeader(headerValue(header, "x-trace-id"), 256)
	code, class := classifyError(status)
	hub := &socialhub.Error{
		Code: code, Class: class, Platform: platformName, Product: productName,
		HTTPStatus: status, PlatformMessage: message, RequestID: traceID,
		RetryAfter: parseRetryAfterAt(headerValue(header, "Retry-After"), now),
	}
	return &APIError{
		Hub: hub, Reporting: ReportingErrorDetails{Message: message, TraceID: traceID},
		notWebAccount: isNotWebAccountResponse(body),
	}
}

func providerResponseMessage(body []byte) string {
	if len(body) == 0 || len(body) > int(maxErrorResponseBytes) {
		return ""
	}
	var envelope struct {
		Message string `json:"message"`
		Error   any    `json:"error"`
		Detail  string `json:"detail"`
	}
	if json.Unmarshal(body, &envelope) == nil {
		message := envelope.Message
		if message == "" {
			switch typed := envelope.Error.(type) {
			case string:
				message = typed
			case map[string]any:
				if value, ok := typed["message"].(string); ok {
					message = value
				}
			}
		}
		if message == "" {
			message = envelope.Detail
		}
		if message != "" {
			return message
		}
	}
	return string(body)
}

func isNotWebAccountResponse(body []byte) bool {
	return strings.Contains(strings.ToLower(providerResponseMessage(body)), "not a web account")
}

func classifyError(status int) (socialhub.ErrorCode, socialhub.ErrorClass) {
	switch status {
	case http.StatusBadRequest, http.StatusRequestEntityTooLarge, http.StatusUnprocessableEntity:
		return socialhub.CodeInvalidArgument, socialhub.ClassPermanent
	case http.StatusUnauthorized:
		return socialhub.CodeUnauthenticated, socialhub.ClassUserAction
	case http.StatusPaymentRequired:
		return socialhub.CodeApprovalRequired, socialhub.ClassUserAction
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
		Platform: platformName, Product: productName, Op: operation,
		PlatformMessage: boundedMessage(message, 512),
	}
}

func authenticationError(operation, message string) error {
	return &socialhub.Error{
		Code: socialhub.CodeUnauthenticated, Class: socialhub.ClassUserAction,
		Platform: platformName, Product: productName, Op: operation,
		PlatformMessage: message, ApprovalURL: documentationURL,
	}
}

func businessError(operation string, status int) error {
	if status <= 0 {
		status = http.StatusInternalServerError
	}
	message := "AppLovin Growth Reporting API returned an error response"
	code, class := classifyError(status)
	hub := &socialhub.Error{
		Code: code, Class: class, Platform: platformName, Product: productName, Op: operation,
		HTTPStatus: status, PlatformCode: strconv.Itoa(status), PlatformMessage: message,
	}
	return &APIError{Hub: hub, Reporting: ReportingErrorDetails{Message: message}}
}

func platformContractError(operation, message string) error {
	return &socialhub.Error{
		Code: socialhub.CodePlatformError, Class: socialhub.ClassPermanent,
		Platform: platformName, Product: productName, Op: operation,
		PlatformMessage: boundedMessage(message, 512),
	}
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

func headerValue(header http.Header, name string) string {
	for key, values := range header {
		if strings.EqualFold(key, name) && len(values) > 0 {
			return values[0]
		}
	}
	return ""
}

func parseRetryAfter(value string) time.Duration {
	return parseRetryAfterAt(value, time.Now())
}

func parseRetryAfterAt(value string, now time.Time) time.Duration {
	value = strings.TrimSpace(value)
	if seconds, err := strconv.ParseFloat(value, 64); err == nil && seconds >= 0 && seconds <= float64((24*time.Hour)/time.Second) {
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

func boundedHeader(value string, maximum int) string {
	value = strings.TrimSpace(value)
	if !utf8.ValidString(value) || len(value) > maximum || strings.ContainsFunc(value, unicode.IsControl) {
		return ""
	}
	return value
}

func sanitizeCause(err error) error {
	var urlError *url.Error
	if errors.As(err, &urlError) && urlError.Err != nil {
		return urlError.Err
	}
	return err
}
