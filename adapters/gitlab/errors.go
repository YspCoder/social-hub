package gitlab

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

// ErrorEnvelope covers GitLab's OAuth and REST error shapes. Message remains
// raw because GitLab returns either text or a field-to-errors object.
type ErrorEnvelope struct {
	Error            string          `json:"error"`
	ErrorDescription string          `json:"error_description"`
	Message          json.RawMessage `json:"message"`
}

type APIError struct {
	Hub      *socialhub.Error
	Provider ErrorEnvelope
	Meta     ResponseMeta
	Raw      json.RawMessage
}

func (value *APIError) Error() string {
	if value == nil || value.Hub == nil {
		return "socialhub: gitlab: platform_error"
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

func newHTTPErrorDecoder(clock socialhub.Clock, accessToken, approvalURL string) func(int, http.Header, []byte) error {
	return func(status int, header http.Header, body []byte) error {
		meta := responseMeta(header, clock)
		var provider ErrorEnvelope
		decoded := json.Unmarshal(body, &provider) == nil
		message := firstNonEmpty(provider.ErrorDescription, decodedMessage(provider.Message))
		if message == "" && !decoded {
			message = string(bytes.TrimSpace(body))
		}
		if message == "" {
			message = http.StatusText(status)
		}
		platformCode := redactSensitive(redactExact(firstNonEmpty(provider.Error, "http_"+strconv.Itoa(status)), accessToken))
		code, class := classifyHTTPError(status)
		retryAfter := time.Duration(0)
		if code == socialhub.CodeRateLimited {
			retryAfter = meta.RetryAfterDuration
			if retryAfter == 0 {
				retryAfter = parseHTTPDateDelay(meta.RateLimitResetTime, clock.Now())
			}
			if retryAfter == 0 {
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
			hub.ApprovalURL = approvalURL
		}
		provider.Error = boundedMessage(redactSensitive(redactExact(provider.Error, accessToken)), 256)
		provider.ErrorDescription = boundedMessage(redactSensitive(redactExact(provider.ErrorDescription, accessToken)), 1024)
		provider.Message = sanitizeProviderBody(provider.Message, accessToken)
		return &APIError{Hub: hub, Provider: provider, Meta: meta, Raw: sanitizeProviderBody(body, accessToken)}
	}
}

func classifyHTTPError(status int) (socialhub.ErrorCode, socialhub.ErrorClass) {
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
	case http.StatusMovedPermanently, http.StatusFound, http.StatusTemporaryRedirect, http.StatusPermanentRedirect, http.StatusConflict:
		return socialhub.CodeConflict, socialhub.ClassPermanent
	case http.StatusTooManyRequests:
		return socialhub.CodeRateLimited, socialhub.ClassRetryable
	case http.StatusRequestTimeout, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable
	default:
		if status >= 500 {
			return socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable
		}
		return socialhub.CodePlatformError, socialhub.ClassPermanent
	}
}

func decodedMessage(raw json.RawMessage) string {
	var message string
	if json.Unmarshal(raw, &message) == nil {
		return message
	}
	if len(bytes.TrimSpace(raw)) > 0 && json.Valid(raw) {
		return string(bytes.TrimSpace(raw))
	}
	return ""
}

func parseRetryAfter(value string, now time.Time) time.Duration {
	value = strings.TrimSpace(value)
	if seconds, err := strconv.ParseInt(value, 10, 64); err == nil && seconds >= 0 && seconds <= int64((7*24*time.Hour)/time.Second) {
		return time.Duration(seconds) * time.Second
	}
	return parseHTTPDateDelay(value, now)
}

func parseHTTPDateDelay(value string, now time.Time) time.Duration {
	when, err := http.ParseTime(strings.TrimSpace(value))
	if err != nil {
		return 0
	}
	return boundedDelay(when.Sub(now))
}

func boundedDelay(value time.Duration) time.Duration {
	if value <= 0 || value > 7*24*time.Hour {
		return 0
	}
	return value
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

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
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

func boundedRaw(value []byte) json.RawMessage {
	if len(value) > maxErrorRawBytes {
		value = value[:maxErrorRawBytes]
	}
	return append(json.RawMessage(nil), value...)
}

func redactExact(value, secret string) string {
	if secret == "" {
		return value
	}
	return strings.ReplaceAll(value, secret, "[REDACTED]")
}

func redactSensitive(value string) string {
	for _, key := range []string{"access_token", "authorization", "bearer", "private-token", "private_token", "refresh_token", "client_secret", "password"} {
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
