package github

import (
	"bytes"
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

const maxErrorRawBytes = 64 << 10

// ErrorEnvelope is GitHub's common REST failure shape. Errors remains raw
// because GitHub documents both strings and structured validation objects.
type ErrorEnvelope struct {
	Message          string          `json:"message"`
	DocumentationURL string          `json:"documentation_url"`
	Status           json.RawMessage `json:"status"`
	Errors           json.RawMessage `json:"errors"`
}

// APIError augments socialhub.Error with GitHub's provider envelope and headers.
type APIError struct {
	Hub      *socialhub.Error
	Provider ErrorEnvelope
	Meta     ResponseMeta
	Raw      json.RawMessage
}

func (value *APIError) Error() string {
	if value == nil || value.Hub == nil {
		return "socialhub: github: platform_error"
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

func newHTTPErrorDecoder(clock socialhub.Clock, accessToken string) func(int, http.Header, []byte) error {
	return func(status int, header http.Header, body []byte) error {
		meta := responseMeta(header, clock)
		var provider ErrorEnvelope
		decoded := json.Unmarshal(body, &provider) == nil
		platformCode := firstValidationCode(provider.Errors)
		if platformCode == "" {
			platformCode = "http_" + strconv.Itoa(status)
		}
		message := provider.Message
		if message == "" && !decoded {
			message = string(bytes.TrimSpace(body))
		}
		if message == "" {
			message = http.StatusText(status)
		}
		code, class := classifyHTTPError(status, header, message)
		retryAfter := time.Duration(0)
		if code == socialhub.CodeRateLimited {
			retryAfter = meta.RetryAfterDuration
			if retryAfter == 0 && strings.TrimSpace(meta.RateLimitRemaining) == "0" {
				retryAfter = meta.RateLimitResetAfter
			}
			if retryAfter == 0 {
				retryAfter = time.Minute
			}
		}
		hub := &socialhub.Error{
			Code: code, Class: class, Platform: platformName, Product: productName,
			HTTPStatus: status, PlatformCode: boundedMessage(platformCode, 256),
			PlatformMessage: boundedMessage(redactSensitive(redactExact(message, accessToken)), 1024),
			RequestID:       meta.RequestID, RetryAfter: retryAfter,
		}
		if code == socialhub.CodeUnauthenticated || code == socialhub.CodePermissionDenied {
			hub.ApprovalURL = tokenSettingsURL
		}
		provider.Message = boundedMessage(redactSensitive(redactExact(provider.Message, accessToken)), 1024)
		provider.DocumentationURL = boundedMessage(provider.DocumentationURL, 4096)
		provider.Status = boundedRaw(provider.Status)
		provider.Errors = sanitizeProviderBody(provider.Errors, accessToken)
		return &APIError{Hub: hub, Provider: provider, Meta: meta, Raw: sanitizeProviderBody(body, accessToken)}
	}
}

func classifyHTTPError(status int, header http.Header, message string) (socialhub.ErrorCode, socialhub.ErrorClass) {
	if status == http.StatusTooManyRequests || status == http.StatusForbidden && isRateLimitFailure(header, message) {
		return socialhub.CodeRateLimited, socialhub.ClassRetryable
	}
	switch status {
	case http.StatusBadRequest, http.StatusMethodNotAllowed, http.StatusNotAcceptable,
		http.StatusRequestEntityTooLarge, http.StatusUnsupportedMediaType, http.StatusUnprocessableEntity:
		return socialhub.CodeInvalidArgument, socialhub.ClassPermanent
	case http.StatusUnauthorized:
		return socialhub.CodeUnauthenticated, socialhub.ClassUserAction
	case http.StatusForbidden, http.StatusUnavailableForLegalReasons:
		return socialhub.CodePermissionDenied, socialhub.ClassUserAction
	case http.StatusNotFound, http.StatusGone:
		return socialhub.CodeNotFound, socialhub.ClassPermanent
	case http.StatusMovedPermanently, http.StatusConflict:
		return socialhub.CodeConflict, socialhub.ClassPermanent
	case http.StatusRequestTimeout, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable
	default:
		if status >= 500 {
			return socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable
		}
		return socialhub.CodePlatformError, socialhub.ClassPermanent
	}
}

func isRateLimitFailure(header http.Header, message string) bool {
	if strings.TrimSpace(header.Get("X-RateLimit-Remaining")) == "0" || strings.TrimSpace(header.Get("Retry-After")) != "" {
		return true
	}
	normalized := strings.ToLower(message)
	return strings.Contains(normalized, "rate limit") || strings.Contains(normalized, "secondary limit") ||
		strings.Contains(normalized, "abuse detection")
}

func parseRetryAfter(value string, now time.Time) time.Duration {
	value = strings.TrimSpace(value)
	if seconds, err := strconv.ParseInt(value, 10, 64); err == nil && seconds >= 0 && seconds <= int64((48*time.Hour)/time.Second) {
		return time.Duration(seconds) * time.Second
	}
	when, err := http.ParseTime(value)
	if err != nil {
		return 0
	}
	return boundedDelay(when.Sub(now))
}

func boundedDelay(value time.Duration) time.Duration {
	if value <= 0 || value > 48*time.Hour {
		return 0
	}
	return value
}

func firstValidationCode(raw json.RawMessage) string {
	var values []struct {
		Code json.RawMessage `json:"code"`
	}
	if json.Unmarshal(raw, &values) != nil || len(values) == 0 {
		return ""
	}
	var code string
	if json.Unmarshal(values[0].Code, &code) == nil {
		return code
	}
	var number json.Number
	if json.Unmarshal(values[0].Code, &number) == nil {
		return number.String()
	}
	return ""
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

func platformContractError(operation, message string) error {
	return &socialhub.Error{
		Code: socialhub.CodePlatformError, Class: socialhub.ClassPermanent,
		Platform: platformName, Product: productName, Op: operation,
		PlatformMessage: boundedMessage(message, 1024),
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

func boundedMessage(value string, maximum int) string {
	if utf8.RuneCountInString(value) <= maximum {
		return value
	}
	return string([]rune(value)[:maximum])
}

func boundedRaw(value []byte) json.RawMessage {
	trimmed := bytes.TrimSpace(value)
	if len(trimmed) == 0 {
		return json.RawMessage("null")
	}
	if len(trimmed) <= maxErrorRawBytes && json.Valid(trimmed) {
		return append(json.RawMessage(nil), trimmed...)
	}
	if len(trimmed) > maxErrorRawBytes {
		return json.RawMessage(`{"truncated":true}`)
	}
	encoded, _ := json.Marshal(string(bytes.ToValidUTF8(trimmed, []byte("?"))))
	if len(encoded) > maxErrorRawBytes {
		return json.RawMessage(`{"truncated":true}`)
	}
	return encoded
}

func redactExact(value, secret string) string {
	if secret == "" {
		return value
	}
	return strings.ReplaceAll(value, secret, "[REDACTED]")
}

func redactSensitive(value string) string {
	for _, key := range []string{"access_token", "authorization", "bearer", "client_secret", "password", "private_key"} {
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
			for valueEnd < len(value) && !strings.ContainsRune(" \t\r\n,;&}\"'<", rune(value[valueEnd])) {
				valueEnd++
			}
			value = value[:valueStart] + "[REDACTED]" + value[valueEnd:]
			cursor = valueStart + len("[REDACTED]")
		}
	}
	return value
}

func sanitizeProviderBody(body []byte, secret string) json.RawMessage {
	return boundedRaw([]byte(redactSensitive(redactExact(string(body), secret))))
}

func sanitizeCause(err error) error {
	var urlError *url.Error
	if errors.As(err, &urlError) && urlError.Err != nil {
		return urlError.Err
	}
	return err
}
