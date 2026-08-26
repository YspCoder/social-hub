package tidal

import (
	"bytes"
	"encoding/json"
	"errors"
	"mime"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

// ErrorObject is one TIDAL JSON:API error object.
type ErrorObject struct {
	ID     string          `json:"id,omitempty"`
	Status string          `json:"status,omitempty"`
	Code   string          `json:"code,omitempty"`
	Detail string          `json:"detail,omitempty"`
	Source json.RawMessage `json:"source,omitempty"`
	Meta   json.RawMessage `json:"meta,omitempty"`
}

// ErrorDocument is TIDAL's JSON:API error envelope.
type ErrorDocument struct {
	Errors []ErrorObject `json:"errors"`
	Links  *Links        `json:"links,omitempty"`
}

// APIError preserves a sanitized, bounded TIDAL error response.
type APIError struct {
	Hub      *socialhub.Error
	Provider ErrorDocument
	Response ResponseMeta
	Raw      json.RawMessage
}

func (value *APIError) Error() string {
	if value == nil || value.Hub == nil {
		return "socialhub: tidal: platform_error"
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

func decodeHTTPError(operation string, status int, header http.Header, body []byte, token string, clock socialhub.Clock) error {
	response := responseMeta(status, header, token)
	var provider ErrorDocument
	var raw json.RawMessage
	if validErrorContentType(header.Get("Content-Type")) && json.Valid(bytes.TrimSpace(body)) {
		if sanitized, ok := sanitizeJSON(body, token); ok {
			raw = sanitized
			_ = json.Unmarshal(sanitized, &provider)
		}
	}
	providerCode := ""
	if len(provider.Errors) != 0 {
		providerCode = boundedMessage(provider.Errors[0].Code, 256)
	}
	if providerCode == "" {
		providerCode = "http_" + strconv.Itoa(status)
	}
	code, class := classifyHTTPError(status)
	requestID := response.RequestID
	if requestID == "" {
		requestID = response.CloudFrontID
	}
	hub := &socialhub.Error{
		Code: code, Class: class, Platform: platformName, Product: productName, Op: operation,
		HTTPStatus: status, PlatformCode: providerCode, PlatformMessage: "TIDAL rejected the catalog request",
		RequestID: requestID, RetryAfter: parseRetryAfter(response.RetryAfter, clock.Now()),
	}
	if code == socialhub.CodeUnauthenticated || code == socialhub.CodePermissionDenied {
		hub.ApprovalURL = documentationURL
	}
	return &APIError{
		Hub:      hub,
		Provider: provider, Response: response, Raw: raw,
	}
}

func classifyHTTPError(status int) (socialhub.ErrorCode, socialhub.ErrorClass) {
	switch status {
	case http.StatusBadRequest, http.StatusMethodNotAllowed, http.StatusNotAcceptable,
		http.StatusUnsupportedMediaType, http.StatusUnprocessableEntity:
		return socialhub.CodeInvalidArgument, socialhub.ClassPermanent
	case http.StatusUnauthorized:
		return socialhub.CodeUnauthenticated, socialhub.ClassUserAction
	case http.StatusForbidden:
		return socialhub.CodePermissionDenied, socialhub.ClassUserAction
	case http.StatusNotFound:
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

func platformError(operation string, code socialhub.ErrorCode, class socialhub.ErrorClass, cause error) error {
	return &socialhub.Error{
		Code: code, Class: class, Platform: platformName, Product: productName, Op: operation,
		Cause: sanitizeCause(cause),
	}
}

func credentialPlatformError(operation string, code socialhub.ErrorCode, class socialhub.ErrorClass, cause error, secret string) error {
	return &socialhub.Error{
		Code: code, Class: class, Platform: platformName, Product: productName, Op: operation,
		Cause: sanitizeCredentialCause(cause, secret),
	}
}

func invalidArgument(operation, message string) error {
	return &socialhub.Error{
		Code: socialhub.CodeInvalidArgument, Class: socialhub.ClassPermanent,
		Platform: platformName, Product: productName, Op: operation,
		PlatformMessage: boundedMessage(message, 1024),
	}
}

func authenticationError(operation, message string) error {
	return &socialhub.Error{
		Code: socialhub.CodeUnauthenticated, Class: socialhub.ClassUserAction,
		Platform: platformName, Product: productName, Op: operation,
		PlatformMessage: boundedMessage(message, 512), ApprovalURL: documentationURL,
	}
}

func platformContractError(operation, message string) error {
	return &socialhub.Error{
		Code: socialhub.CodePlatformError, Class: socialhub.ClassPermanent,
		Platform: platformName, Product: productName, Op: operation,
		PlatformMessage: boundedMessage(message, 1024),
	}
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

func validErrorContentType(value string) bool {
	mediaType, _, err := mime.ParseMediaType(value)
	return err == nil && (strings.EqualFold(mediaType, "application/vnd.api+json") || strings.EqualFold(mediaType, "application/json"))
}

func sanitizeJSON(body []byte, token string) (json.RawMessage, bool) {
	var value any
	if err := json.Unmarshal(body, &value); err != nil {
		return nil, false
	}
	redactJSONValue(value, token)
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, false
	}
	return json.RawMessage(encoded), true
}

func redactJSONValue(value any, token string) {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			if token != "" && strings.Contains(key, token) {
				delete(typed, key)
				typed["[REDACTED]"] = "[REDACTED]"
				continue
			}
			if sensitiveJSONKey(key) {
				typed[key] = "[REDACTED]"
				continue
			}
			if text, ok := child.(string); ok {
				typed[key] = redactExact(text, token)
				continue
			}
			redactJSONValue(child, token)
		}
	case []any:
		for index, child := range typed {
			if text, ok := child.(string); ok {
				typed[index] = redactExact(text, token)
				continue
			}
			redactJSONValue(child, token)
		}
	}
}

func sensitiveJSONKey(value string) bool {
	normalized := strings.ToLower(strings.ReplaceAll(value, "-", "_"))
	for _, marker := range []string{"authorization", "access_token", "refresh_token", "api_key", "client_secret", "token", "secret", "password"} {
		if normalized == marker || strings.HasSuffix(normalized, "_"+marker) {
			return true
		}
	}
	return false
}

func redactExact(value, secret string) string {
	if secret == "" {
		return value
	}
	return strings.ReplaceAll(value, secret, "[REDACTED]")
}

func boundedMessage(value string, maximum int) string {
	if maximum <= 0 || !utf8.ValidString(value) || strings.ContainsFunc(value, unicode.IsControl) {
		return ""
	}
	if utf8.RuneCountInString(value) <= maximum {
		return value
	}
	return string([]rune(value)[:maximum])
}

func boundedHeader(value string, maximum int) string {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > maximum || !utf8.ValidString(value) || strings.ContainsFunc(value, unicode.IsControl) {
		return ""
	}
	return value
}

func safeHeader(value, secret string, maximum int) string {
	value = boundedHeader(value, maximum)
	if secret != "" && strings.Contains(value, secret) {
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

func sanitizeCredentialCause(err error, secret string) error {
	if err == nil {
		return nil
	}
	message := sanitizeCause(err).Error()
	if secret != "" {
		message = strings.ReplaceAll(message, secret, "[REDACTED]")
	}
	lower := strings.ToLower(message)
	for _, marker := range []string{"authorization", "bearer", "access_token", "access token", "token", "password", "secret", "credential"} {
		if strings.Contains(lower, marker) {
			return errors.New("TIDAL request failed")
		}
	}
	message = boundedMessage(message, 1024)
	if message == "" {
		message = "TIDAL request failed"
	}
	return errors.New(message)
}
