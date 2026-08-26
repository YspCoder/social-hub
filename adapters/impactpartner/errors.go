package impactpartner

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

const maxRetryAfter = 24 * time.Hour

// ErrOutcomeUnknown means a tracking-link request may have reached impact.com
// and must be reconciled before it is retried.
var ErrOutcomeUnknown = errors.New("impactpartner: tracking-link outcome is unknown")

type FieldError struct {
	Field   string `json:"Field"`
	Message string `json:"Message"`
}

type ErrorEnvelope struct {
	Status  string       `json:"Status"`
	Message string       `json:"Message"`
	Errors  []FieldError `json:"Errors"`
}

type TrackingError struct {
	Timestamp ExactValue `json:"timestamp"`
	Status    string     `json:"status"`
	Error     string     `json:"error"`
	Message   string     `json:"message"`
}

// APIError augments socialhub.Error with impact.com's structured failure and
// current hourly quota headers.
type APIError struct {
	Hub                    *socialhub.Error
	Provider               ErrorEnvelope
	Tracking               TrackingError
	RateLimitLimitHour     string
	RateLimitRemainingHour string
	RateLimitReset         string
}

func (value *APIError) Error() string {
	if value == nil || value.Hub == nil {
		return "socialhub: impact: platform_error"
	}
	return value.Hub.Error()
}

func (value *APIError) Unwrap() error {
	if value == nil {
		return nil
	}
	return value.Hub
}

func (value *APIError) Retryable() bool {
	return value != nil && value.Hub != nil && value.Hub.Retryable()
}

func newHTTPErrorDecoder(clock socialhub.Clock, secrets ...string) func(int, http.Header, []byte) error {
	return func(status int, header http.Header, body []byte) error {
		var provider ErrorEnvelope
		var tracking TrackingError
		providerDecoded := json.Unmarshal(body, &provider) == nil
		trackingDecoded := json.Unmarshal(body, &tracking) == nil
		message := firstNonEmpty(provider.Message, tracking.Message)
		if message == "" && providerDecoded && len(provider.Errors) > 0 {
			message = provider.Errors[0].Message
		}
		if message == "" && !providerDecoded && !trackingDecoded {
			message = strings.TrimSpace(string(body))
		}
		if message == "" {
			message = http.StatusText(status)
		}
		platformCode := firstNonEmpty(provider.Status, tracking.Status, tracking.Error, "http_"+strconv.Itoa(status))
		code, class := classifyHTTPError(status)
		hub := &socialhub.Error{
			Code: code, Class: class, Platform: platformName, Product: productName, HTTPStatus: status,
			PlatformCode:    boundedMessage(redactErrorValue(platformCode, secrets...), 256),
			PlatformMessage: boundedMessage(redactErrorValue(message, secrets...), 1024),
			RequestID:       boundedMessage(redactErrorValue(firstHeader(header, "X-Request-ID", "X-Correlation-ID"), secrets...), 256),
			RetryAfter:      parseRetryAfter(firstHeader(header, "Retry-After"), clock.Now()),
		}
		if code == socialhub.CodeApprovalRequired || code == socialhub.CodePermissionDenied {
			hub.ApprovalURL = documentationURL
		}
		provider.Status = boundedMessage(redactErrorValue(provider.Status, secrets...), 256)
		provider.Message = boundedMessage(redactErrorValue(provider.Message, secrets...), 1024)
		for index := range provider.Errors {
			provider.Errors[index].Field = boundedMessage(redactErrorValue(provider.Errors[index].Field, secrets...), 256)
			provider.Errors[index].Message = boundedMessage(redactErrorValue(provider.Errors[index].Message, secrets...), 1024)
		}
		tracking.Status = boundedMessage(redactErrorValue(tracking.Status, secrets...), 256)
		tracking.Error = boundedMessage(redactErrorValue(tracking.Error, secrets...), 256)
		tracking.Message = boundedMessage(redactErrorValue(tracking.Message, secrets...), 1024)
		return &APIError{
			Hub: hub, Provider: provider, Tracking: tracking,
			RateLimitLimitHour:     boundedMessage(redactErrorValue(firstHeader(header, "X-RateLimit-Limit-hour"), secrets...), 64),
			RateLimitRemainingHour: boundedMessage(redactErrorValue(firstHeader(header, "X-RateLimit-Remaining-hour"), secrets...), 64),
			RateLimitReset:         boundedMessage(redactErrorValue(firstHeader(header, "RateLimit-Reset"), secrets...), 64),
		}
	}
}

func classifyHTTPError(status int) (socialhub.ErrorCode, socialhub.ErrorClass) {
	switch status {
	case http.StatusBadRequest, http.StatusMethodNotAllowed, http.StatusRequestEntityTooLarge,
		http.StatusUnsupportedMediaType, http.StatusUnprocessableEntity:
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
	case http.StatusRequestTimeout, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable
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
		PlatformMessage: boundedMessage(message, 1024),
	}
}

func platformContractError(operation, message string, statuses ...int) error {
	result := &socialhub.Error{
		Code: socialhub.CodePlatformError, Class: socialhub.ClassPermanent,
		Platform: platformName, Product: productName, Op: operation,
		PlatformMessage: boundedMessage(message, 1024),
	}
	if len(statuses) > 0 {
		result.HTTPStatus = statuses[0]
	}
	return result
}

func outcomeUnknownError(operation string, cause error, requestID string) error {
	if requestID == "" {
		var hub *socialhub.Error
		if errors.As(cause, &hub) {
			requestID = hub.RequestID
		}
	}
	return &socialhub.Error{
		Code: socialhub.CodeConflict, Class: socialhub.ClassUserAction,
		Platform: platformName, Product: productName, Op: operation,
		PlatformMessage: "impact.com tracking-link outcome is unknown; reconcile account state before retrying",
		RequestID:       boundedMessage(redactSensitive(requestID), 256),
		Cause:           errors.Join(ErrOutcomeUnknown, sanitizeCause(cause)),
	}
}

func withMutationOutcome(operation, requestID string, err error) error {
	if err != nil && ambiguousMutationError(err) {
		return outcomeUnknownError(operation, err, requestID)
	}
	return err
}

func ambiguousMutationError(err error) bool {
	var hub *socialhub.Error
	if !errors.As(err, &hub) {
		return true
	}
	if hub.HTTPStatus == http.StatusRequestTimeout || hub.HTTPStatus >= 500 ||
		(hub.HTTPStatus >= 200 && hub.HTTPStatus < 300) {
		return true
	}
	return hub.Code == socialhub.CodeTemporarilyUnavailable && hub.Class == socialhub.ClassRetryable
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

func withHTTPStatus(err error, status int) error {
	var hub *socialhub.Error
	if errors.As(err, &hub) {
		hub.HTTPStatus = status
	}
	return err
}

func parseRetryAfter(value string, now time.Time) time.Duration {
	value = strings.TrimSpace(value)
	if value != "" && onlyASCIIDigits(value) {
		seconds, err := strconv.ParseUint(value, 10, 64)
		if err != nil || seconds >= uint64(maxRetryAfter/time.Second) {
			return maxRetryAfter
		}
		return time.Duration(seconds) * time.Second
	}
	when, err := http.ParseTime(value)
	if err != nil {
		return 0
	}
	delay := when.Sub(now)
	if delay < 0 {
		return 0
	}
	if delay > maxRetryAfter {
		return maxRetryAfter
	}
	return delay
}

func onlyASCIIDigits(value string) bool {
	for _, character := range value {
		if character < '0' || character > '9' {
			return false
		}
	}
	return value != ""
}

func firstHeader(header http.Header, names ...string) string {
	for _, name := range names {
		if value := strings.TrimSpace(header.Get(name)); value != "" {
			return value
		}
	}
	return ""
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func boundedMessage(value string, maximum int) string {
	if utf8.RuneCountInString(value) <= maximum {
		return value
	}
	return string([]rune(value)[:maximum])
}

func redactSensitive(value string) string {
	for _, key := range []string{"authorization", "authtoken", "auth_token", "access_token", "password", "secret"} {
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
			for valueEnd < len(value) && !strings.ContainsRune("\r\n,;&}\"'", rune(value[valueEnd])) {
				valueEnd++
			}
			value = value[:valueStart] + "[REDACTED]" + value[valueEnd:]
			cursor = valueStart + len("[REDACTED]")
		}
	}
	return value
}

func redactErrorValue(value string, secrets ...string) string {
	for _, secret := range secrets {
		if secret != "" {
			value = strings.ReplaceAll(value, secret, "[REDACTED]")
		}
	}
	return redactSensitive(value)
}

func sanitizeCause(err error) error {
	var urlError *url.Error
	if errors.As(err, &urlError) && urlError.Err != nil {
		return urlError.Err
	}
	return err
}
