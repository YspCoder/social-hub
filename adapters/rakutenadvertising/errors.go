package rakutenadvertising

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
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

// ErrOutcomeUnknown means a deep-link request may have reached Rakuten
// Advertising and must be reconciled before it is retried.
var ErrOutcomeUnknown = errors.New("rakutenadvertising: deep-link outcome is unknown")

// ProviderError preserves Rakuten Advertising's error identifier and message.
type ProviderError struct {
	Code    string
	Message string
}

// APIError augments socialhub.Error with the bounded provider response body.
type APIError struct {
	Hub      *socialhub.Error
	Provider ProviderError
	Raw      []byte
}

func (value *APIError) Error() string {
	if value == nil || value.Hub == nil {
		return "socialhub: rakuten-advertising: platform_error"
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
	return providerResponseError("", status, header, provider, body, now, secrets...)
}

func providerResponseError(operation string, status int, header http.Header, provider ProviderError, raw []byte, now time.Time, secrets ...string) error {
	code, class := classifyHTTPError(status, provider.Code)
	message := firstNonEmpty(provider.Message, provider.Code, http.StatusText(status), "Rakuten Advertising rejected the request")
	platformCode := provider.Code
	if platformCode == "" {
		platformCode = "http_" + strconv.Itoa(status)
	}
	hub := &socialhub.Error{
		Code: code, Class: class, Platform: platformName, Product: productName, Op: operation,
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
	return &APIError{Hub: hub, Provider: provider, Raw: boundedRedactedRaw(raw, secrets...)}
}

func classifyHTTPError(status int, platformCode string) (socialhub.ErrorCode, socialhub.ErrorClass) {
	if code, class, found := classifyProviderCode(platformCode); found {
		return code, class
	}
	switch status {
	case http.StatusBadRequest, http.StatusMethodNotAllowed, http.StatusRequestEntityTooLarge,
		http.StatusUnsupportedMediaType, http.StatusUnprocessableEntity:
		return socialhub.CodeInvalidArgument, socialhub.ClassPermanent
	case http.StatusUnauthorized:
		return socialhub.CodeUnauthenticated, socialhub.ClassUserAction
	case http.StatusForbidden:
		// The official Affiliate APIs contract uses 403 for per-minute limits.
		return socialhub.CodeRateLimited, socialhub.ClassRetryable
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

func classifyProviderCode(value string) (socialhub.ErrorCode, socialhub.ErrorClass, bool) {
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case "ACCESS_DENIED", "DEEP_LINKING_NOT_ENABLED":
		return socialhub.CodeApprovalRequired, socialhub.ClassUserAction, true
	case "DEEP_LINK_DENIED", "URL_TEMPLATE_MISMATCH", "ENTITY_DECODE_ERROR", "REQUEST_PARAM_INVALID", "UNEXPECTED_ERROR":
		return socialhub.CodeInvalidArgument, socialhub.ClassPermanent, true
	case "CANNOT_RESOLVE_ADVERTISER":
		return socialhub.CodeNotFound, socialhub.ClassPermanent, true
	case "INVALID_CLIENT", "INVALID_GRANT", "UNAUTHORIZED_CLIENT", "INVALID_TOKEN":
		return socialhub.CodeUnauthenticated, socialhub.ClassUserAction, true
	case "INSUFFICIENT_SCOPE":
		return socialhub.CodePermissionDenied, socialhub.ClassUserAction, true
	case "TEMPORARILY_UNAVAILABLE", "SERVER_ERROR":
		return socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, true
	default:
		return "", "", false
	}
}

func decodeProviderError(body []byte) ProviderError {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		return ProviderError{}
	}
	if trimmed[0] == '{' && json.Valid(trimmed) {
		var payload struct {
			Error            json.RawMessage `json:"error"`
			Code             json.RawMessage `json:"code"`
			Message          string          `json:"message"`
			Description      string          `json:"description"`
			ErrorDescription string          `json:"error_description"`
		}
		if json.Unmarshal(trimmed, &payload) == nil {
			return ProviderError{
				Code:    firstNonEmpty(jsonScalarText(payload.Error), jsonScalarText(payload.Code)),
				Message: firstNonEmpty(payload.Message, payload.ErrorDescription, payload.Description),
			}
		}
	}
	return decodeXMLError(trimmed)
}

func decodeXMLError(body []byte) ProviderError {
	decoder := xml.NewDecoder(bytes.NewReader(body))
	var result ProviderError
	var current string
	for {
		token, err := decoder.Token()
		if err != nil {
			return result
		}
		switch typed := token.(type) {
		case xml.StartElement:
			name := strings.ToLower(typed.Name.Local)
			if name == "error" || name == "code" || name == "message" || name == "description" {
				current = name
			}
		case xml.CharData:
			text := strings.TrimSpace(string(typed))
			if text == "" {
				continue
			}
			if current == "error" || current == "code" {
				result.Code = firstNonEmpty(result.Code, text)
			} else if current == "message" || current == "description" {
				result.Message = firstNonEmpty(result.Message, text)
			}
		case xml.EndElement:
			current = ""
		}
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
	if (trimmed[0] >= '0' && trimmed[0] <= '9') || trimmed[0] == '-' {
		return string(trimmed)
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
		PlatformMessage: "Rakuten Advertising deep-link outcome is unknown; reconcile publisher state before retrying",
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
	return hub.HTTPStatus == 0 && hub.Code == socialhub.CodeTemporarilyUnavailable && hub.Class == socialhub.ClassRetryable
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
	for _, key := range []string{"access_token", "refresh_token", "authorization", "bearer", "token-key", "password", "secret"} {
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

func boundedRedactedRaw(value []byte, secrets ...string) []byte {
	text := redactErrorValue(string(value), secrets...)
	text = boundedMessage(text, maximumErrorBodyRunes)
	return []byte(text)
}

func sanitizeCause(err error) error {
	var urlError *url.Error
	if errors.As(err, &urlError) && urlError.Err != nil {
		return urlError.Err
	}
	return err
}
