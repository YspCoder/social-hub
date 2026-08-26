package tripadvisor

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

const maxErrorRawBytes = 64 << 10

// ProviderCode accepts the numeric error code in the published schema and
// string codes returned by compatible provider gateways.
type ProviderCode string

func (value *ProviderCode) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if bytes.Equal(trimmed, []byte("null")) {
		*value = ""
		return nil
	}
	var decoded string
	if len(trimmed) > 0 && trimmed[0] == '"' {
		if err := json.Unmarshal(trimmed, &decoded); err != nil {
			return err
		}
	} else {
		if _, err := strconv.ParseInt(string(trimmed), 10, 64); err != nil {
			return fmt.Errorf("tripadvisor: provider error code must be an integer or string")
		}
		decoded = string(trimmed)
	}
	if decoded != "" && !validOpaque(decoded, 256) {
		return fmt.Errorf("tripadvisor: provider error code is invalid")
	}
	*value = ProviderCode(decoded)
	return nil
}

type ProviderError struct {
	Message string       `json:"message"`
	Type    string       `json:"type"`
	Code    ProviderCode `json:"code"`
}

// APIError augments socialhub.Error with Tripadvisor's sanitized provider
// envelope and response metadata.
type APIError struct {
	Hub      *socialhub.Error
	Provider ProviderError
	Meta     ResponseMeta
	Raw      json.RawMessage
}

func (value *APIError) Error() string {
	if value == nil || value.Hub == nil {
		return "socialhub: tripadvisor: platform_error"
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

func newHTTPErrorDecoder(clock socialhub.Clock, apiKey string) func(int, http.Header, []byte) error {
	return func(status int, header http.Header, body []byte) error {
		meta := responseMeta(header, clock.Now(), apiKey)
		sanitized := sanitizeProviderBody(body, apiKey)
		var envelope struct {
			Error ProviderError `json:"error"`
		}
		decoded := json.Unmarshal(sanitized, &envelope) == nil
		provider := envelope.Error
		if provider.Message == "" && !decoded {
			if err := json.Unmarshal(sanitized, &provider.Message); err != nil {
				provider.Message = "Tripadvisor returned an invalid error response"
			}
		}
		return newProviderAPIError("", status, meta, provider, sanitized)
	}
}

func newProviderAPIError(operation string, status int, meta ResponseMeta, provider ProviderError, raw json.RawMessage) error {
	provider.Message = boundedMessage(redactSensitive(provider.Message), 1024)
	provider.Type = boundedMessage(redactSensitive(provider.Type), 256)
	provider.Code = ProviderCode(boundedMessage(redactSensitive(string(provider.Code)), 256))
	code, class := classifyProviderError(status, provider)
	platformCode := firstNonEmpty(string(provider.Code), provider.Type)
	if platformCode == "" {
		platformCode = "http_" + strconv.Itoa(status)
	}
	message := firstNonEmpty(provider.Message, http.StatusText(status), "Tripadvisor rejected the request")
	return &APIError{
		Hub: &socialhub.Error{
			Code: code, Class: class, Platform: platformName, Product: productName, Op: operation,
			HTTPStatus: status, PlatformCode: boundedMessage(platformCode, 256),
			PlatformMessage: boundedMessage(message, 1024), RequestID: meta.RequestID, RetryAfter: meta.RetryAfter,
		},
		Provider: provider,
		Meta:     meta,
		Raw:      boundedErrorRaw(raw),
	}
}

func classifyProviderError(status int, provider ProviderError) (socialhub.ErrorCode, socialhub.ErrorClass) {
	if status < 200 || status >= 300 {
		return classifyHTTPError(status)
	}
	if numeric, err := strconv.Atoi(string(provider.Code)); err == nil && numeric >= 300 && numeric <= 599 {
		return classifyHTTPError(numeric)
	}
	normalized := strings.ToUpper(strings.NewReplacer("-", "_", " ", "_").Replace(provider.Type + "_" + string(provider.Code)))
	switch {
	case strings.Contains(normalized, "UNAUTHENTICATED"), strings.Contains(normalized, "UNAUTHORIZED"), strings.Contains(normalized, "AUTHENTICATION"):
		return socialhub.CodeUnauthenticated, socialhub.ClassUserAction
	case strings.Contains(normalized, "FORBIDDEN"), strings.Contains(normalized, "PERMISSION"):
		return socialhub.CodePermissionDenied, socialhub.ClassUserAction
	case strings.Contains(normalized, "NOT_FOUND"):
		return socialhub.CodeNotFound, socialhub.ClassPermanent
	case strings.Contains(normalized, "RATE"), strings.Contains(normalized, "LIMIT"), strings.Contains(normalized, "THROTTL"):
		return socialhub.CodeRateLimited, socialhub.ClassRetryable
	case strings.Contains(normalized, "INVALID"), strings.Contains(normalized, "VALIDATION"):
		return socialhub.CodeInvalidArgument, socialhub.ClassPermanent
	default:
		return socialhub.CodePlatformError, socialhub.ClassPermanent
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
		if status >= 300 && status < 400 {
			return socialhub.CodeConflict, socialhub.ClassPermanent
		}
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

func parseRetryAfter(value string, now time.Time) time.Duration {
	value = strings.TrimSpace(value)
	if seconds, err := strconv.ParseFloat(value, 64); err == nil && seconds >= 0 && seconds <= float64((48*time.Hour)/time.Second) {
		return time.Duration(seconds * float64(time.Second))
	}
	when, err := http.ParseTime(value)
	if err != nil {
		return 0
	}
	delay := when.Sub(now)
	if delay < 0 || delay > 48*time.Hour {
		return 0
	}
	return delay
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

func boundedErrorRaw(value []byte) json.RawMessage {
	if len(value) > maxErrorRawBytes {
		return json.RawMessage(`{"truncated":true}`)
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
	for _, key := range []string{"access_token", "api_key", "apikey", "authorization", "client_secret", "password", "token", "key"} {
		cursor := 0
		for cursor < len(value) {
			lower := strings.ToLower(value)
			start := strings.Index(lower[cursor:], key)
			if start < 0 {
				break
			}
			start += cursor
			if start > 0 && (unicode.IsLetter(rune(value[start-1])) || unicode.IsDigit(rune(value[start-1])) || value[start-1] == '_') {
				cursor = start + len(key)
				continue
			}
			valueStart := start + len(key)
			for valueStart < len(value) && strings.ContainsRune(" \t:=\"'?&", rune(value[valueStart])) {
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
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		return nil
	}
	if json.Valid(trimmed) {
		decoder := json.NewDecoder(bytes.NewReader(trimmed))
		decoder.UseNumber()
		var value any
		if decoder.Decode(&value) == nil {
			value = sanitizeProviderValue(value, secret)
			if encoded, err := json.Marshal(value); err == nil {
				return encoded
			}
		}
	}
	encoded, _ := json.Marshal(redactSensitive(redactExact(string(trimmed), secret)))
	return encoded
}

func sanitizeProviderValue(value any, secret string) any {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			switch strings.ToLower(key) {
			case "key", "api_key", "apikey", "authorization", "access_token", "client_secret", "token", "password":
				typed[key] = "[REDACTED]"
			default:
				typed[key] = sanitizeProviderValue(child, secret)
			}
		}
		return typed
	case []any:
		for index := range typed {
			typed[index] = sanitizeProviderValue(typed[index], secret)
		}
		return typed
	case string:
		return redactProviderString(typed, secret)
	default:
		return value
	}
}

func redactProviderString(value, secret string) string {
	value = redactExact(value, secret)
	if secret != "" {
		value = strings.ReplaceAll(value, url.QueryEscape(secret), "[REDACTED]")
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.RawQuery == "" {
		return value
	}
	query, err := url.ParseQuery(parsed.RawQuery)
	if err != nil {
		return value
	}
	changed := false
	for key := range query {
		switch strings.ToLower(key) {
		case "key", "api_key", "apikey", "authorization", "access_token", "client_secret", "token", "password":
			query.Set(key, "[REDACTED]")
			changed = true
		}
	}
	if changed {
		parsed.RawQuery = query.Encode()
		return parsed.String()
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
