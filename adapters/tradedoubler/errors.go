package tradedoubler

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

const (
	maximumErrorBodyRunes = 16_384
	maxRetryAfter         = 24 * time.Hour
)

type ErrorEnvelope struct {
	Message    string     `json:"message"`
	StatusCode ExactValue `json:"statuscode"`
}

// APIError augments socialhub.Error with Tradedoubler's structured failure,
// bounded diagnostic body, and any quota headers returned by the gateway.
type APIError struct {
	Hub                *socialhub.Error
	Provider           ErrorEnvelope
	Raw                []byte
	RateLimitLimit     string
	RateLimitRemaining string
	RateLimitReset     string
}

func (value *APIError) Error() string {
	if value == nil || value.Hub == nil {
		return "socialhub: tradedoubler: platform_error"
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

func newHTTPErrorDecoder(clock socialhub.Clock, secrets func() []string) func(int, http.Header, []byte) error {
	return func(status int, header http.Header, body []byte) error {
		var current []string
		if secrets != nil {
			current = secrets()
		}
		var provider ErrorEnvelope
		decoded := json.Unmarshal(body, &provider) == nil
		message := provider.Message
		if message == "" && !decoded {
			message = strings.TrimSpace(string(body))
		}
		if message == "" {
			message = http.StatusText(status)
		}
		platformCode := provider.StatusCode.String()
		if platformCode == "" {
			platformCode = "http_" + strconv.Itoa(status)
		}
		code, class := classifyHTTPError(status, platformCode, message)
		hub := &socialhub.Error{
			Code: code, Class: class, Platform: platformName, Product: productName, HTTPStatus: status,
			PlatformCode:    boundedMessage(redactErrorValue(platformCode, current...), 256),
			PlatformMessage: boundedMessage(redactErrorValue(message, current...), 1024),
			RequestID: boundedMessage(redactErrorValue(
				firstHeader(header, "X-Request-ID", "X-Correlation-ID"), current...,
			), 256),
			RetryAfter: parseRetryAfter(firstHeader(header, "Retry-After"), clock.Now()),
		}
		if code == socialhub.CodePermissionDenied {
			hub.ApprovalURL = documentationURL
		}
		provider.Message = boundedMessage(redactErrorValue(provider.Message, current...), 1024)
		provider.StatusCode = redactExactValue(provider.StatusCode, current...)
		return &APIError{
			Hub: hub, Provider: provider, Raw: boundedRedactedRaw(body, current...),
			RateLimitLimit: boundedMessage(redactErrorValue(
				firstHeader(header, "RateLimit-Limit", "X-RateLimit-Limit"), current...,
			), 64),
			RateLimitRemaining: boundedMessage(redactErrorValue(
				firstHeader(header, "RateLimit-Remaining", "X-RateLimit-Remaining"), current...,
			), 64),
			RateLimitReset: boundedMessage(redactErrorValue(
				firstHeader(header, "RateLimit-Reset", "X-RateLimit-Reset"), current...,
			), 64),
		}
	}
}

func classifyHTTPError(status int, platformCode, message string) (socialhub.ErrorCode, socialhub.ErrorClass) {
	normalizedCode := strings.ToUpper(strings.TrimSpace(platformCode))
	normalizedMessage := strings.ToLower(message)
	switch normalizedCode {
	case "5", "PF_250":
		return socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable
	case "PF_392":
		return socialhub.CodePermissionDenied, socialhub.ClassUserAction
	case "1", "2", "4000", "4001", "PF_300":
		return socialhub.CodeUnauthenticated, socialhub.ClassUserAction
	case "3", "4", "6", "7":
		return socialhub.CodeNotFound, socialhub.ClassPermanent
	case "PF_200", "PF_210", "PF_230", "PF_240", "PF_260", "PF_270", "PF_280", "PF_290", "PF_391", "PF_430":
		return socialhub.CodeInvalidArgument, socialhub.ClassPermanent
	}
	if strings.Contains(normalizedMessage, "token") {
		return socialhub.CodeUnauthenticated, socialhub.ClassUserAction
	}
	switch status {
	case http.StatusBadRequest, http.StatusMethodNotAllowed, http.StatusRequestEntityTooLarge,
		http.StatusNotAcceptable, http.StatusUnsupportedMediaType, http.StatusUnprocessableEntity:
		return socialhub.CodeInvalidArgument, socialhub.ClassPermanent
	case http.StatusUnauthorized:
		return socialhub.CodeUnauthenticated, socialhub.ClassUserAction
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
	if seconds, err := strconv.ParseFloat(value, 64); err == nil && seconds >= 0 {
		if seconds >= float64(maxRetryAfter/time.Second) {
			return maxRetryAfter
		}
		return time.Duration(seconds * float64(time.Second))
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

func boundedMessage(value string, maximum int) string {
	if utf8.RuneCountInString(value) <= maximum {
		return value
	}
	return string([]rune(value)[:maximum])
}

func redactSensitive(value string) string {
	for _, key := range []string{"access_token", "auth_token", "password", "secret", "token"} {
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
			for valueEnd < len(value) && !strings.ContainsRune(" \t\r\n,;&}\"'", rune(value[valueEnd])) {
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

func redactExactValue(value ExactValue, secrets ...string) ExactValue {
	raw := value.Bytes()
	if len(raw) == 0 {
		return ExactValue{}
	}
	sanitized := []byte(boundedMessage(redactErrorValue(string(raw), secrets...), maxExactValueBytes))
	if json.Valid(sanitized) {
		return ExactValue{raw: sanitized}
	}
	encoded, _ := json.Marshal(boundedMessage(redactErrorValue(value.String(), secrets...), maxExactValueBytes))
	return ExactValue{raw: encoded}
}

func boundedRedactedRaw(value []byte, secrets ...string) []byte {
	if len(value) == 0 {
		return nil
	}
	text := boundedMessage(redactErrorValue(string(value), secrets...), maximumErrorBodyRunes)
	return []byte(text)
}

func sanitizeCause(err error) error {
	var urlError *url.Error
	if errors.As(err, &urlError) && urlError.Err != nil {
		return urlError.Err
	}
	return err
}
