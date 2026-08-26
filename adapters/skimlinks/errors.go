package skimlinks

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"sort"
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

// ProviderError preserves Skimlinks' best-effort error identifier and message.
type ProviderError struct {
	Code    string
	Message string
}

// APIError augments socialhub.Error with a bounded, redacted provider body.
type APIError struct {
	Hub      *socialhub.Error
	Provider ProviderError
	Raw      []byte
}

func (value *APIError) Error() string {
	if value == nil || value.Hub == nil {
		return "socialhub: skimlinks: platform_error"
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
		return decodeHTTPError(status, header, body, clock.Now(), current...)
	}
}

func decodeHTTPError(status int, header http.Header, body []byte, now time.Time, secrets ...string) error {
	provider := decodeProviderError(body)
	code, class := classifyHTTPError(status, provider.Code)
	message := firstNonEmpty(
		provider.Message, provider.Code, strings.TrimSpace(string(body)),
		http.StatusText(status), "Skimlinks rejected the request",
	)
	platformCode := provider.Code
	if platformCode == "" {
		platformCode = "http_" + strconv.Itoa(status)
	}
	hub := &socialhub.Error{
		Code: code, Class: class, Platform: platformName, Product: productName,
		HTTPStatus: status, PlatformCode: boundedMessage(redactErrorValue(platformCode, secrets...), 256),
		PlatformMessage: boundedMessage(redactErrorValue(message, secrets...), 1024),
		RequestID: boundedMessage(redactErrorValue(
			firstHeader(header, "X-Request-ID", "X-Correlation-ID"), secrets...,
		), 256),
		RetryAfter: parseRetryAfter(header.Get("Retry-After"), now),
	}
	if code == socialhub.CodePermissionDenied || code == socialhub.CodeApprovalRequired {
		hub.ApprovalURL = documentationURL
	}
	provider.Code = boundedMessage(redactErrorValue(provider.Code, secrets...), 256)
	provider.Message = boundedMessage(redactErrorValue(provider.Message, secrets...), 1024)
	return &APIError{Hub: hub, Provider: provider, Raw: boundedRedactedRaw(body, secrets...)}
}

func classifyHTTPError(status int, providerCode string) (socialhub.ErrorCode, socialhub.ErrorClass) {
	switch strings.ToLower(strings.TrimSpace(providerCode)) {
	case "invalid_client", "invalid_grant", "unauthorized_client", "invalid_token", "unauthorized":
		return socialhub.CodeUnauthenticated, socialhub.ClassUserAction
	case "insufficient_scope", "forbidden", "permission_denied":
		return socialhub.CodePermissionDenied, socialhub.ClassUserAction
	case "rate_limit_exceeded", "too_many_requests":
		return socialhub.CodeRateLimited, socialhub.ClassRetryable
	}
	switch status {
	case http.StatusBadRequest, http.StatusMethodNotAllowed, http.StatusRequestEntityTooLarge,
		http.StatusNotAcceptable, http.StatusUnsupportedMediaType, http.StatusUnprocessableEntity:
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
	case http.StatusRequestTimeout, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable
	default:
		if status >= 500 {
			return socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable
		}
		return socialhub.CodePlatformError, socialhub.ClassPermanent
	}
}

func decodeProviderError(body []byte) ProviderError {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 || trimmed[0] != '{' || !json.Valid(trimmed) {
		return ProviderError{}
	}
	var payload struct {
		Error            json.RawMessage `json:"error"`
		Code             json.RawMessage `json:"code"`
		Message          string          `json:"message"`
		Detail           string          `json:"detail"`
		Description      string          `json:"description"`
		ErrorDescription string          `json:"error_description"`
	}
	if json.Unmarshal(trimmed, &payload) != nil {
		return ProviderError{}
	}
	return ProviderError{
		Code:    firstNonEmpty(jsonScalarText(payload.Error), jsonScalarText(payload.Code)),
		Message: firstNonEmpty(payload.ErrorDescription, payload.Message, payload.Detail, payload.Description),
	}
}

func jsonScalarText(value json.RawMessage) string {
	trimmed := bytes.TrimSpace(value)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return ""
	}
	if trimmed[0] == '"' {
		var text string
		if json.Unmarshal(trimmed, &text) == nil {
			return text
		}
	}
	if trimmed[0] == '-' || trimmed[0] >= '0' && trimmed[0] <= '9' {
		return string(trimmed)
	}
	return ""
}

func withOperation(err error, operation string) error {
	if err == nil {
		return nil
	}
	var apiError *APIError
	if errors.As(err, &apiError) && apiError.Hub != nil {
		apiError.Hub.Op = operation
		return apiError
	}
	var hubError *socialhub.Error
	if errors.As(err, &hubError) {
		hubError.Op = operation
		hubError.Platform = platformName
		hubError.Product = productName
		return hubError
	}
	return platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, err)
}

func platformError(operation string, code socialhub.ErrorCode, class socialhub.ErrorClass, cause error) error {
	return &socialhub.Error{
		Code: code, Class: class, Op: operation, Platform: platformName, Product: productName,
		Cause: sanitizeCause(cause),
	}
}

func invalidArgument(operation, message string) error {
	return &socialhub.Error{
		Code: socialhub.CodeInvalidArgument, Class: socialhub.ClassPermanent,
		Op: operation, Platform: platformName, Product: productName,
		PlatformMessage: boundedMessage(message, 1024),
	}
}

func platformContractError(operation, message string, statuses ...int) error {
	result := &socialhub.Error{
		Code: socialhub.CodePlatformError, Class: socialhub.ClassPermanent,
		Op: operation, Platform: platformName, Product: productName,
		PlatformMessage: boundedMessage(message, 1024),
	}
	if len(statuses) > 0 {
		result.HTTPStatus = statuses[0]
	}
	return result
}

func withHTTPStatus(err error, status int) error {
	var hub *socialhub.Error
	if errors.As(err, &hub) {
		hub.HTTPStatus = status
	}
	return err
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

func parseRetryAfter(value string, now time.Time) time.Duration {
	value = strings.TrimSpace(value)
	if value != "" && onlyASCIIDigits(value) {
		seconds, err := strconv.ParseUint(value, 10, 64)
		if err != nil || seconds >= uint64(maxRetryAfter/time.Second) {
			return maxRetryAfter
		}
		return time.Duration(seconds) * time.Second
	}
	at, err := http.ParseTime(value)
	if err != nil {
		return 0
	}
	delay := at.Sub(now)
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

func boundedMessage(value string, maximum int) string {
	if utf8.RuneCountInString(value) <= maximum {
		return value
	}
	return string([]rune(value)[:maximum])
}

func boundedRedactedRaw(value []byte, secrets ...string) []byte {
	if len(value) == 0 {
		return nil
	}
	text := boundedMessage(redactErrorValue(string(value), secrets...), maximumErrorBodyRunes)
	return []byte(text)
}

func redactErrorValue(value string, secrets ...string) string {
	ordered := append([]string(nil), secrets...)
	sort.SliceStable(ordered, func(left, right int) bool {
		return len(ordered[left]) > len(ordered[right])
	})
	for _, secret := range ordered {
		if secret != "" {
			value = strings.ReplaceAll(value, secret, "[REDACTED]")
		}
	}
	return redactSensitive(value)
}

func redactSensitive(value string) string {
	for _, key := range []string{"access_token", "authorization", "client_secret", "password", "secret"} {
		cursor := 0
		for cursor < len(value) {
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

func sanitizeCause(err error) error {
	var urlError *url.Error
	if errors.As(err, &urlError) && urlError.Err != nil {
		return urlError.Err
	}
	return err
}
