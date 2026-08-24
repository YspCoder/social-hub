package steam

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

const maxErrorRawBytes = 64 << 10

// APIError augments socialhub.Error with a sanitized Steam response body.
type APIError struct {
	Hub  *socialhub.Error
	Meta ResponseMeta
	Raw  json.RawMessage
}

func (value *APIError) Error() string {
	if value == nil || value.Hub == nil {
		return "socialhub: steam: platform_error"
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

func newHTTPErrorDecoder(clock socialhub.Clock, webAPIKey string) func(int, http.Header, []byte) error {
	return func(status int, header http.Header, body []byte) error {
		meta := responseMeta(status, header, clock, webAPIKey)
		raw := sanitizeProviderBody(body, webAPIKey)
		message := providerMessage(raw)
		if message == "" {
			message = http.StatusText(status)
		}
		code, class := classifyHTTPError(status, message)
		hub := &socialhub.Error{
			Code: code, Class: class, Platform: platformName, Product: productName,
			HTTPStatus: status, PlatformCode: "http_" + strconv.Itoa(status),
			PlatformMessage: boundedMessage(message, 1024),
		}
		if code == socialhub.CodeRateLimited {
			hub.RetryAfter = meta.RetryAfterDuration
		}
		if code == socialhub.CodeUnauthenticated {
			hub.ApprovalURL = userKeyURL
		}
		return &APIError{Hub: hub, Meta: meta, Raw: raw}
	}
}

func classifyHTTPError(status int, message string) (socialhub.ErrorCode, socialhub.ErrorClass) {
	if status >= 300 && status < 400 {
		return socialhub.CodeConflict, socialhub.ClassPermanent
	}
	if (status == http.StatusBadRequest || status == http.StatusForbidden) && credentialFailureMessage(message) {
		return socialhub.CodeUnauthenticated, socialhub.ClassUserAction
	}
	switch status {
	case http.StatusBadRequest, http.StatusMethodNotAllowed:
		return socialhub.CodeInvalidArgument, socialhub.ClassPermanent
	case http.StatusUnauthorized:
		return socialhub.CodeUnauthenticated, socialhub.ClassUserAction
	case http.StatusForbidden:
		return socialhub.CodePermissionDenied, socialhub.ClassUserAction
	case http.StatusNotFound:
		return socialhub.CodeNotFound, socialhub.ClassPermanent
	case http.StatusTooManyRequests:
		return socialhub.CodeRateLimited, socialhub.ClassRetryable
	case http.StatusInternalServerError, http.StatusServiceUnavailable:
		return socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable
	default:
		if status >= 500 {
			return socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable
		}
		return socialhub.CodePlatformError, socialhub.ClassPermanent
	}
}

func credentialFailureMessage(message string) bool {
	lower := strings.ToLower(message)
	return strings.Contains(lower, "x-webapi-key") || strings.Contains(lower, "web api key") ||
		strings.Contains(lower, "parameter 'key'") || strings.Contains(lower, "parameter \"key\"") ||
		strings.Contains(lower, "key=")
}

func platformError(operation string, code socialhub.ErrorCode, class socialhub.ErrorClass, cause error) error {
	return &socialhub.Error{
		Code: code, Class: class, Op: operation, Platform: platformName, Product: productName,
		Cause: sanitizeCause(cause),
	}
}

func authenticationError(operation string, cause error, webAPIKey string) error {
	return &socialhub.Error{
		Code: socialhub.CodeUnauthenticated, Class: socialhub.ClassUserAction,
		Op: operation, Platform: platformName, Product: productName,
		PlatformMessage: "Steam user Web API key could not be resolved or is invalid",
		Cause:           sanitizeCredentialCause(cause, webAPIKey), ApprovalURL: userKeyURL,
	}
}

func invalidArgument(operation, message string) error {
	return &socialhub.Error{
		Code: socialhub.CodeInvalidArgument, Class: socialhub.ClassPermanent,
		Op: operation, Platform: platformName, Product: productName,
		PlatformMessage: boundedMessage(message, 1024),
	}
}

func platformContractError(operation, message string) error {
	return &socialhub.Error{
		Code: socialhub.CodePlatformError, Class: socialhub.ClassPermanent,
		Op: operation, Platform: platformName, Product: productName,
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
	if seconds, err := strconv.ParseInt(value, 10, 64); err == nil && seconds >= 0 && seconds <= int64((48*time.Hour)/time.Second) {
		return time.Duration(seconds) * time.Second
	}
	when, err := http.ParseTime(value)
	if err != nil || !when.After(now) {
		return 0
	}
	delay := when.Sub(now)
	if delay > 48*time.Hour {
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

func redactExact(value, secret string) string {
	if secret == "" {
		return value
	}
	return strings.ReplaceAll(value, secret, "[REDACTED]")
}

func redactNamedValues(value string) string {
	for _, name := range []string{"key", "x-webapi-key", "webapi_key", "api_key", "apikey", "access_token", "authorization", "client_secret", "password", "private_key"} {
		cursor := 0
		for {
			lower := strings.ToLower(value)
			start := strings.Index(lower[cursor:], name)
			if start < 0 {
				break
			}
			start += cursor
			separator := start + len(name)
			for separator < len(value) && (value[separator] == ' ' || value[separator] == '\t' || value[separator] == '"' || value[separator] == '\'') {
				separator++
			}
			if separator >= len(value) || (value[separator] != '=' && value[separator] != ':') {
				cursor = start + len(name)
				continue
			}
			valueStart := separator + 1
			for valueStart < len(value) && strings.ContainsRune(" \t\"'", rune(value[valueStart])) {
				valueStart++
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

func sanitizeProviderBody(body []byte, webAPIKey string) json.RawMessage {
	if len(bytes.TrimSpace(body)) == 0 {
		return nil
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	var decoded any
	if decoder.Decode(&decoded) == nil {
		var extra any
		if decoder.Decode(&extra) == io.EOF {
			encoded, err := json.Marshal(sanitizeJSONValue(decoded, webAPIKey))
			if err == nil {
				if webAPIKey != "" && bytes.Contains(encoded, []byte(webAPIKey)) {
					return json.RawMessage(`{"truncated":true}`)
				}
				if len(encoded) <= maxErrorRawBytes {
					return append(json.RawMessage(nil), encoded...)
				}
				return json.RawMessage(`{"truncated":true}`)
			}
		}
	}
	text := redactNamedValues(redactExact(strings.ToValidUTF8(string(body), ""), webAPIKey))
	encoded, err := json.Marshal(text)
	if err != nil || len(encoded) > maxErrorRawBytes {
		return json.RawMessage(`{"truncated":true}`)
	}
	return encoded
}

func sanitizeJSONValue(value any, webAPIKey string) any {
	switch typed := value.(type) {
	case map[string]any:
		clean := make(map[string]any, len(typed))
		for key, child := range typed {
			cleanKey := redactExact(key, webAPIKey)
			if sensitiveJSONKey(key) {
				clean[cleanKey] = "[REDACTED]"
				continue
			}
			clean[cleanKey] = sanitizeJSONValue(child, webAPIKey)
		}
		return clean
	case []any:
		clean := make([]any, len(typed))
		for index, child := range typed {
			clean[index] = sanitizeJSONValue(child, webAPIKey)
		}
		return clean
	case string:
		return redactNamedValues(redactExact(typed, webAPIKey))
	case json.Number:
		if webAPIKey != "" && strings.Contains(string(typed), webAPIKey) {
			return "[REDACTED]"
		}
		return value
	default:
		return value
	}
}

func sensitiveJSONKey(value string) bool {
	normalized := strings.NewReplacer("_", "", "-", "", ".", "").Replace(strings.ToLower(value))
	switch normalized {
	case "key", "xwebapikey", "webapikey", "apikey", "accesstoken", "authorization", "clientsecret", "password", "privatekey":
		return true
	default:
		return false
	}
}

func providerMessage(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var text string
	if json.Unmarshal(raw, &text) == nil {
		return text
	}
	var object map[string]any
	if json.Unmarshal(raw, &object) == nil {
		for _, key := range []string{"message", "error", "detail"} {
			if value, ok := object[key].(string); ok && strings.TrimSpace(value) != "" {
				return value
			}
		}
	}
	return string(raw)
}

func sanitizeCause(err error) error {
	var urlError *url.Error
	if errors.As(err, &urlError) && urlError.Err != nil {
		return urlError.Err
	}
	return err
}

func sanitizeCredentialCause(err error, webAPIKey string) error {
	if err == nil {
		return nil
	}
	clean := sanitizeCause(err)
	return errors.New(boundedMessage(redactNamedValues(redactExact(clean.Error(), webAPIKey)), 1024))
}
