package applovinconversion

import (
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

type ConversionErrorDetails struct {
	Message      string
	BatchDropped bool
}

// APIError augments socialhub.Error with fixed Conversion API details.
// Error deliberately omits the response message because it may echo event PII.
type APIError struct {
	Hub        *socialhub.Error
	Conversion ConversionErrorDetails
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
	return func(status int, header http.Header, _ []byte) error {
		message := fixedErrorMessage(status)
		code, class := classifyError(status)
		hub := &socialhub.Error{
			Code: code, Class: class, Platform: platformName, Product: productName,
			HTTPStatus: status, PlatformMessage: message,
			RetryAfter: parseRetryAfter(header.Get("Retry-After"), clock.Now()),
		}
		if code == socialhub.CodeUnauthenticated || code == socialhub.CodePermissionDenied || code == socialhub.CodeApprovalRequired {
			hub.ApprovalURL = documentationURL
		}
		return &APIError{
			Hub:        hub,
			Conversion: ConversionErrorDetails{Message: message, BatchDropped: status == http.StatusBadRequest},
		}
	}
}

func fixedErrorMessage(status int) string {
	switch status {
	case http.StatusBadRequest:
		return "AppLovin dropped the invalid Conversion API batch"
	case http.StatusUnauthorized:
		return "AppLovin rejected the Conversion API credentials"
	default:
		return "AppLovin rejected the Conversion API request"
	}
}

func classifyError(status int) (socialhub.ErrorCode, socialhub.ErrorClass) {
	switch status {
	case http.StatusBadRequest, http.StatusRequestEntityTooLarge, http.StatusUnsupportedMediaType, http.StatusUnprocessableEntity:
		return socialhub.CodeInvalidArgument, socialhub.ClassPermanent
	case http.StatusRequestTimeout:
		return socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable
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

func authenticationError(operation, message string, cause error, secrets ...string) error {
	return &socialhub.Error{
		Code: socialhub.CodeUnauthenticated, Class: socialhub.ClassUserAction,
		Platform: platformName, Product: productName, Op: operation,
		PlatformMessage: message, ApprovalURL: documentationURL,
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
		hub.PlatformMessage = "AppLovin submission is not safe to retry without a stable dedupe_id on every event"
	}
	return err
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
	message := cause.Error()
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
