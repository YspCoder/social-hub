package ironsourcereporting

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

// APIError augments the common error with bounded ironSource quota metadata.
type APIError struct {
	Hub                *socialhub.Error
	RateLimitLimit     string
	RateLimitRemaining string
	RateLimitReset     string
}

func (err *APIError) Error() string {
	if err == nil || err.Hub == nil {
		return "socialhub: ironsource: platform_error"
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
	var payload struct {
		Code json.RawMessage `json:"code"`
	}
	_ = json.Unmarshal(body, &payload)
	code, class := classifyHTTPError(status)
	platformCode := scalarCode(payload.Code)
	hub := &socialhub.Error{
		Code: code, Class: class, Platform: platformName, Product: productName, HTTPStatus: status,
		PlatformCode: platformCode, PlatformMessage: "ironSource rejected the advertiser reporting request",
		RequestID:  firstBoundedHeader(header, 256, "X-Request-ID", "X-Correlation-ID"),
		RetryAfter: parseRetryAfterAt(header.Get("Retry-After"), now),
	}
	return &APIError{
		Hub:                hub,
		RateLimitLimit:     firstBoundedHeader(header, 128, "X-RateLimit-Limit", "RateLimit-Limit"),
		RateLimitRemaining: firstBoundedHeader(header, 128, "X-RateLimit-Remaining", "RateLimit-Remaining"),
		RateLimitReset:     firstBoundedHeader(header, 128, "X-RateLimit-Reset", "RateLimit-Reset"),
	}
}

func classifyHTTPError(status int) (socialhub.ErrorCode, socialhub.ErrorClass) {
	switch status {
	case http.StatusBadRequest, http.StatusRequestEntityTooLarge, http.StatusUnsupportedMediaType, http.StatusUnprocessableEntity:
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
		Code: code, Class: class, Platform: platformName, Product: productName,
		Op: operation, Cause: sanitizeCause(cause),
	}
}

func invalidArgument(operation, message string) error {
	return &socialhub.Error{
		Code: socialhub.CodeInvalidArgument, Class: socialhub.ClassPermanent,
		Platform: platformName, Product: productName, Op: operation, PlatformMessage: message,
	}
}

func authenticationError(operation, message string, cause error, secrets ...string) error {
	return &socialhub.Error{
		Code: socialhub.CodeUnauthenticated, Class: socialhub.ClassUserAction,
		Platform: platformName, Product: productName, Op: operation,
		PlatformMessage: message, Cause: sanitizeCredentialCause(cause, secrets...), ApprovalURL: documentationURL,
	}
}

func platformContractError(operation, message string) error {
	return &socialhub.Error{
		Code: socialhub.CodePlatformError, Class: socialhub.ClassPermanent,
		Platform: platformName, Product: productName, Op: operation, PlatformMessage: message,
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

func scalarCode(raw json.RawMessage) string {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" || len(trimmed) > 130 {
		return ""
	}
	if strings.HasPrefix(trimmed, "\"") {
		var value string
		if json.Unmarshal(raw, &value) == nil && validPlatformCode(value) {
			return value
		}
		return ""
	}
	var number json.Number
	if json.Unmarshal(raw, &number) == nil {
		value := number.String()
		if len(value) <= 128 {
			return value
		}
	}
	return ""
}

func validPlatformCode(value string) bool {
	if !validOpaque(value, 128) {
		return false
	}
	for _, character := range value {
		if !unicode.IsLetter(character) && !unicode.IsNumber(character) && !strings.ContainsRune("._:-", character) {
			return false
		}
	}
	return true
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

func firstBoundedHeader(header http.Header, maximum int, names ...string) string {
	for _, name := range names {
		if value := boundedHeader(header.Get(name), maximum); value != "" {
			return value
		}
	}
	return ""
}

func boundedHeader(value string, maximum int) string {
	value = strings.TrimSpace(value)
	if !utf8.ValidString(value) || len(value) > maximum || strings.ContainsFunc(value, unicode.IsControl) {
		return ""
	}
	return value
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

func redactExact(value string, secrets ...string) string {
	for _, secret := range secrets {
		if secret != "" {
			value = strings.ReplaceAll(value, secret, "[REDACTED]")
		}
	}
	return value
}

func sanitizeCredentialCause(err error, secrets ...string) error {
	if err == nil {
		return nil
	}
	clean := sanitizeCause(err)
	message := boundedMessage(redactSensitive(redactExact(clean.Error(), secrets...)), 1024)
	if message == "" || strings.ContainsFunc(message, unicode.IsControl) {
		return nil
	}
	return errors.New(message)
}

func redactSensitive(value string) string {
	markers := []string{"authorization", "access_token", "refresh_token", "secretkey", "secret_key", "bearer", "token"}
	for _, marker := range markers {
		for cursor := 0; cursor < len(value); {
			index := strings.Index(strings.ToLower(value[cursor:]), marker)
			if index < 0 {
				break
			}
			index += cursor
			start := index + len(marker)
			for start < len(value) && strings.ContainsRune(" \t:=\"'", rune(value[start])) {
				start++
			}
			if start == index+len(marker) {
				cursor = start
				continue
			}
			end := start
			for end < len(value) && !strings.ContainsRune("\r\n,;}&\"'", rune(value[end])) &&
				(marker == "authorization" || !strings.ContainsRune(" \t", rune(value[end]))) {
				end++
			}
			value = value[:start] + "[REDACTED]" + value[end:]
			cursor = start + len("[REDACTED]")
		}
	}
	return value
}
