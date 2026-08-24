package conversions

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

type EventFailure struct {
	Event int
	Codes []string
}

type ErrorDetails struct {
	Status        string
	EventFailures []EventFailure
}

// APIError preserves only structured validation codes. Snap's reason and
// error message strings are omitted because validation responses may echo PII.
type APIError struct {
	Hub     *socialhub.Error
	Details ErrorDetails
}

func (err *APIError) Error() string {
	if err == nil || err.Hub == nil {
		return "socialhub: snapchat: platform_error"
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

type errorEnvelope struct {
	Status    string `json:"status"`
	EventLogs []struct {
		Event  int    `json:"event"`
		Status string `json:"status"`
		Errors struct {
			Codes []string `json:"codes"`
		} `json:"errors"`
	} `json:"event_logs"`
}

func newHTTPErrorDecoder(clock socialhub.Clock, secrets ...string) func(int, http.Header, []byte) error {
	return func(status int, header http.Header, body []byte) error {
		var response errorEnvelope
		_ = json.Unmarshal(body, &response)
		code, class := classifyHTTPError(status)
		failures := make([]EventFailure, 0)
		allCodes := make([]string, 0, 10)
		for _, eventLog := range response.EventLogs {
			codes := make([]string, 0)
			for _, value := range eventLog.Errors.Codes {
				if validErrorCode(value) {
					if len(codes) < 100 {
						codes = append(codes, value)
					}
					if len(allCodes) < 10 {
						allCodes = append(allCodes, value)
					}
				}
			}
			if eventLog.Event > 0 && len(codes) > 0 && len(failures) < MaximumBatchSize {
				failures = append(failures, EventFailure{Event: eventLog.Event, Codes: codes})
			}
		}
		requestID := boundedHeader(firstHeader(header, "X-Snap-Request-ID", "X-Request-ID", "X-Correlation-ID"), 256)
		if containsAny(requestID, secrets...) {
			requestID = ""
		}
		hub := &socialhub.Error{
			Code: code, Class: class, Platform: platformName, Product: productName,
			HTTPStatus: status, PlatformCode: strings.Join(allCodes, ","),
			PlatformMessage: "Snap rejected the Conversions API request", RequestID: requestID,
			RetryAfter: parseRetryAfter(header.Get("Retry-After"), clock.Now()),
		}
		if code == socialhub.CodeUnauthenticated || code == socialhub.CodePermissionDenied {
			hub.ApprovalURL = documentationURL
		}
		return &APIError{Hub: hub, Details: ErrorDetails{Status: safeStatus(response.Status), EventFailures: failures}}
	}
}

func classifyHTTPError(status int) (socialhub.ErrorCode, socialhub.ErrorClass) {
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
		Code: code, Class: class, Platform: platformName, Product: productName,
		Op: operation, Cause: sanitizeCause(cause),
	}
}

func invalidArgument(operation, message string) error {
	return &socialhub.Error{
		Code: socialhub.CodeInvalidArgument, Class: socialhub.ClassPermanent,
		Platform: platformName, Product: productName, Op: operation,
		PlatformMessage: boundedMessage(message, 512),
	}
}

func authenticationError(operation, message string, cause error, secrets ...string) error {
	return &socialhub.Error{
		Code: socialhub.CodeUnauthenticated, Class: socialhub.ClassUserAction,
		Platform: platformName, Product: productName, Op: operation,
		PlatformMessage: boundedMessage(message, 512), ApprovalURL: documentationURL,
		Cause: sanitizeCredentialCause(cause, secrets...),
	}
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

func withRetrySafety(err error, retrySafe bool) error {
	if err == nil || retrySafe {
		return err
	}
	var hub *socialhub.Error
	if errors.As(err, &hub) && hub.Class == socialhub.ClassRetryable {
		hub.Class = socialhub.ClassPermanent
		hub.PlatformMessage = "Snap submission is not safe to retry without a stable event_id on every event"
	}
	return err
}

func validErrorCode(value string) bool {
	if value == "" || len(value) > 64 {
		return false
	}
	for _, character := range value {
		if character < '0' || character > '9' {
			return false
		}
	}
	return true
}

func safeStatus(value string) string {
	if value == "VALID" || value == "INVALID" || value == "SUCCESS" {
		return value
	}
	return ""
}

func firstHeader(header http.Header, names ...string) string {
	for _, name := range names {
		if value := header.Get(name); value != "" {
			return value
		}
	}
	return ""
}

func parseRetryAfter(value string, now time.Time) time.Duration {
	value = boundedHeader(value, 128)
	if value == "" {
		return 0
	}
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
	if !utf8.ValidString(value) || len(value) > maximum || strings.ContainsFunc(value, unicode.IsControl) {
		return ""
	}
	return strings.TrimSpace(value)
}

func containsAny(value string, candidates ...string) bool {
	if value == "" {
		return false
	}
	for _, candidate := range candidates {
		if candidate != "" && strings.Contains(value, candidate) {
			return true
		}
	}
	return false
}

func sanitizeCause(err error) error {
	var urlError *url.Error
	if errors.As(err, &urlError) && urlError.Err != nil {
		return urlError.Err
	}
	return err
}

func sanitizeCredentialCause(cause error, secrets ...string) error {
	if cause == nil {
		return nil
	}
	clean := sanitizeCause(cause)
	message := clean.Error()
	for _, secret := range secrets {
		if secret != "" {
			message = strings.ReplaceAll(message, secret, "[REDACTED]")
		}
	}
	if !utf8.ValidString(message) || len(message) > 1024 || strings.ContainsFunc(message, unicode.IsControl) {
		message = "credential resolution failed"
	}
	return errors.New(message)
}
