package producthunt

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

const (
	maxGraphQLErrors     = 32
	maxGraphQLLocations  = 32
	maxGraphQLPath       = 32
	maxGraphQLExtensions = 32
	maxErrorRawBytes     = 64 << 10
	maxJSONFragmentBytes = 4 << 10
)

type GraphQLLocation struct {
	Line   int `json:"line"`
	Column int `json:"column"`
}

// GraphQLError accepts both the standard GraphQL error fields and Product
// Hunt's OAuth-shaped error/error_description fields.
type GraphQLError struct {
	Message          string                     `json:"message"`
	ErrorCode        string                     `json:"error"`
	ErrorDescription string                     `json:"error_description"`
	Locations        []GraphQLLocation          `json:"locations"`
	Path             []json.RawMessage          `json:"path"`
	Extensions       map[string]json.RawMessage `json:"extensions"`
}

// APIError augments socialhub.Error with Product Hunt's provider errors and
// rate-limit metadata. Raw is sanitized before exposure.
type APIError struct {
	Hub     *socialhub.Error
	GraphQL []GraphQLError
	Meta    ResponseMeta
	Raw     json.RawMessage
}

func (value *APIError) Error() string {
	if value == nil || value.Hub == nil {
		return "socialhub: producthunt: platform_error"
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
		meta := responseMeta(header)
		var envelope struct {
			Errors           []GraphQLError `json:"errors"`
			Error            string         `json:"error"`
			ErrorDescription string         `json:"error_description"`
			Message          string         `json:"message"`
		}
		decoded := json.Unmarshal(body, &envelope) == nil
		if len(envelope.Errors) == 0 && (envelope.Error != "" || envelope.ErrorDescription != "") {
			envelope.Errors = []GraphQLError{{ErrorCode: envelope.Error, ErrorDescription: envelope.ErrorDescription, Message: envelope.Message}}
		}
		platformCode, message := "http_"+strconv.Itoa(status), ""
		if len(envelope.Errors) > 0 {
			platformCode = firstNonEmpty(providerErrorCode(envelope.Errors[0]), platformCode)
			message = providerErrorMessage(envelope.Errors[0])
		} else {
			platformCode = firstNonEmpty(envelope.Error, platformCode)
			message = firstNonEmpty(envelope.ErrorDescription, envelope.Message)
		}
		if message == "" && !decoded {
			message = string(bytes.TrimSpace(body))
		}
		message = boundedMessage(redactSensitive(redactExact(firstNonEmpty(message, http.StatusText(status)), accessToken)), 1024)
		code, class := classifyHTTPError(status, platformCode)
		retryAfter := parseResetDelay(header.Get("Retry-After"), clock.Now())
		if retryAfter == 0 && status == http.StatusTooManyRequests {
			retryAfter = parseResetSeconds(meta.RateLimitReset)
		}
		hub := &socialhub.Error{
			Code: code, Class: class, Platform: platformName, Product: productName,
			HTTPStatus: status, PlatformCode: boundedMessage(redactSensitive(redactExact(platformCode, accessToken)), 256),
			PlatformMessage: message, RequestID: meta.RequestID, RetryAfter: retryAfter,
		}
		if code == socialhub.CodeUnauthenticated || code == socialhub.CodePermissionDenied || code == socialhub.CodeApprovalRequired {
			hub.ApprovalURL = dashboardURL
		}
		return &APIError{
			Hub: hub, GraphQL: sanitizeGraphQLErrors(envelope.Errors, accessToken), Meta: meta,
			Raw: sanitizeProviderBody(body, accessToken),
		}
	}
}

func graphQLOperationError(operation string, status int, provider []GraphQLError, raw []byte, meta ResponseMeta, accessToken string) error {
	provider = sanitizeGraphQLErrors(provider, accessToken)
	platformCode, message := "graphql_error", "Product Hunt returned a GraphQL error"
	if len(provider) > 0 {
		platformCode = firstNonEmpty(providerErrorCode(provider[0]), platformCode)
		message = firstNonEmpty(providerErrorMessage(provider[0]), message)
	}
	code, class := classifyGraphQLError(platformCode, message)
	hub := &socialhub.Error{
		Code: code, Class: class, Platform: platformName, Product: productName, Op: operation,
		HTTPStatus: status, PlatformCode: boundedMessage(platformCode, 256),
		PlatformMessage: boundedMessage(message, 1024), RequestID: meta.RequestID,
	}
	if code == socialhub.CodeRateLimited {
		hub.RetryAfter = parseResetSeconds(meta.RateLimitReset)
	}
	if code == socialhub.CodeUnauthenticated || code == socialhub.CodePermissionDenied || code == socialhub.CodeApprovalRequired {
		hub.ApprovalURL = dashboardURL
	}
	return &APIError{Hub: hub, GraphQL: provider, Meta: meta, Raw: sanitizeProviderBody(raw, accessToken)}
}

func providerErrorCode(value GraphQLError) string {
	return firstNonEmpty(value.ErrorCode, extensionString(value.Extensions, "code"), extensionString(value.Extensions, "classification"))
}

func providerErrorMessage(value GraphQLError) string {
	return firstNonEmpty(value.ErrorDescription, value.Message)
}

func extensionString(values map[string]json.RawMessage, key string) string {
	var value string
	if json.Unmarshal(values[key], &value) == nil {
		return value
	}
	return ""
}

func sanitizeGraphQLErrors(values []GraphQLError, accessToken string) []GraphQLError {
	if len(values) > maxGraphQLErrors {
		values = values[:maxGraphQLErrors]
	}
	result := make([]GraphQLError, len(values))
	for index, value := range values {
		value.Message = boundedMessage(redactSensitive(redactExact(value.Message, accessToken)), 1024)
		value.ErrorCode = boundedMessage(redactSensitive(redactExact(value.ErrorCode, accessToken)), 256)
		value.ErrorDescription = boundedMessage(redactSensitive(redactExact(value.ErrorDescription, accessToken)), 1024)
		if len(value.Locations) > maxGraphQLLocations {
			value.Locations = value.Locations[:maxGraphQLLocations]
		}
		value.Locations = append([]GraphQLLocation(nil), value.Locations...)
		if len(value.Path) > maxGraphQLPath {
			value.Path = value.Path[:maxGraphQLPath]
		}
		path := make([]json.RawMessage, 0, len(value.Path))
		for _, raw := range value.Path {
			path = append(path, sanitizeJSONFragment(raw, accessToken))
		}
		value.Path = path
		if value.Extensions != nil {
			value.Extensions = make(map[string]json.RawMessage, min(len(value.Extensions), maxGraphQLExtensions))
			count := 0
			for key, raw := range values[index].Extensions {
				if count == maxGraphQLExtensions {
					break
				}
				key = boundedMessage(key, 256)
				if key == "" {
					continue
				}
				if sensitiveJSONKey(key) {
					value.Extensions[key] = json.RawMessage(`"[REDACTED]"`)
				} else {
					value.Extensions[key] = sanitizeJSONFragment(raw, accessToken)
				}
				count++
			}
		}
		result[index] = value
	}
	return result
}

func classifyHTTPError(status int, platformCode string) (socialhub.ErrorCode, socialhub.ErrorClass) {
	if status == http.StatusTooManyRequests {
		return socialhub.CodeRateLimited, socialhub.ClassRetryable
	}
	if code, class, found := classifyNamedError(platformCode); found {
		return code, class
	}
	switch status {
	case http.StatusBadRequest, http.StatusMethodNotAllowed, http.StatusRequestEntityTooLarge,
		http.StatusUnsupportedMediaType, http.StatusUnprocessableEntity:
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
	default:
		if status >= 500 {
			return socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable
		}
		return socialhub.CodePlatformError, socialhub.ClassPermanent
	}
}

func classifyGraphQLError(platformCode, message string) (socialhub.ErrorCode, socialhub.ErrorClass) {
	if code, class, found := classifyNamedError(platformCode); found {
		return code, class
	}
	return classifyNamedErrorDefault(message)
}

func classifyNamedError(value string) (socialhub.ErrorCode, socialhub.ErrorClass, bool) {
	normalized := strings.ToUpper(strings.NewReplacer("-", "_", " ", "_").Replace(value))
	switch {
	case strings.Contains(normalized, "INVALID_OAUTH"), strings.Contains(normalized, "INVALID_TOKEN"),
		strings.Contains(normalized, "UNAUTHENTICATED"), strings.Contains(normalized, "UNAUTHORIZED"):
		return socialhub.CodeUnauthenticated, socialhub.ClassUserAction, true
	case strings.Contains(normalized, "FORBIDDEN"), strings.Contains(normalized, "PERMISSION"), strings.Contains(normalized, "ACCESS_DENIED"):
		return socialhub.CodePermissionDenied, socialhub.ClassUserAction, true
	case strings.Contains(normalized, "RATE"), strings.Contains(normalized, "THROTTL"), strings.Contains(normalized, "TOO_MANY"):
		return socialhub.CodeRateLimited, socialhub.ClassRetryable, true
	case strings.Contains(normalized, "NOT_FOUND"):
		return socialhub.CodeNotFound, socialhub.ClassPermanent, true
	case strings.Contains(normalized, "VALIDATION"), strings.Contains(normalized, "BAD_REQUEST"),
		strings.Contains(normalized, "BAD_USER_INPUT"), strings.Contains(normalized, "PARSE"):
		return socialhub.CodeInvalidArgument, socialhub.ClassPermanent, true
	case strings.Contains(normalized, "INTERNAL"), strings.Contains(normalized, "UNAVAILABLE"), strings.Contains(normalized, "TIMEOUT"):
		return socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, true
	default:
		return "", "", false
	}
}

func classifyNamedErrorDefault(message string) (socialhub.ErrorCode, socialhub.ErrorClass) {
	if code, class, found := classifyNamedError(message); found {
		return code, class
	}
	return socialhub.CodePlatformError, socialhub.ClassPermanent
}

func platformError(operation string, code socialhub.ErrorCode, class socialhub.ErrorClass, cause error) error {
	return &socialhub.Error{
		Code: code, Class: class, Platform: platformName, Product: productName, Op: operation,
		Cause: sanitizeCause(cause),
	}
}

func authenticationError(operation, message string, cause error, accessToken string) error {
	if cause != nil {
		clean := sanitizeCause(cause)
		cause = errors.New(boundedMessage(redactSensitive(redactExact(clean.Error(), accessToken)), 1024))
	}
	return &socialhub.Error{
		Code: socialhub.CodeUnauthenticated, Class: socialhub.ClassUserAction,
		Platform: platformName, Product: productName, Op: operation,
		PlatformMessage: boundedMessage(message, 1024), Cause: cause, ApprovalURL: dashboardURL,
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

func parseResetSeconds(value string) time.Duration {
	seconds, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil || seconds < 0 || seconds > int64((24*time.Hour)/time.Second) {
		return 0
	}
	return time.Duration(seconds) * time.Second
}

func parseResetDelay(value string, now time.Time) time.Duration {
	if delay := parseResetSeconds(value); delay > 0 {
		return delay
	}
	when, err := http.ParseTime(strings.TrimSpace(value))
	if err != nil {
		return 0
	}
	delay := when.Sub(now)
	if delay < 0 || delay > 24*time.Hour {
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

func redactSensitive(value string) string {
	for _, key := range []string{"access_token", "authorization", "bearer", "client_secret", "developer_token", "password"} {
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
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		return nil
	}
	if json.Valid(trimmed) {
		decoder := json.NewDecoder(bytes.NewReader(trimmed))
		decoder.UseNumber()
		var value any
		if decoder.Decode(&value) == nil {
			encoded, err := json.Marshal(sanitizeJSONValue(value, secret, 0))
			if err == nil && len(encoded) <= maxErrorRawBytes {
				return append(json.RawMessage(nil), encoded...)
			}
		}
		return json.RawMessage(`{"truncated":true}`)
	}
	message := strings.ToValidUTF8(string(trimmed), "")
	message = boundedMessage(redactSensitive(redactExact(message, secret)), 4096)
	encoded, err := json.Marshal(message)
	if err != nil || len(encoded) > maxErrorRawBytes {
		return json.RawMessage(`{"truncated":true}`)
	}
	return append(json.RawMessage(nil), encoded...)
}

func sanitizeJSONFragment(raw json.RawMessage, secret string) json.RawMessage {
	if len(raw) == 0 || !json.Valid(raw) {
		return json.RawMessage("null")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value any
	if decoder.Decode(&value) != nil {
		return json.RawMessage("null")
	}
	encoded, err := json.Marshal(sanitizeJSONValue(value, secret, 0))
	if err != nil || len(encoded) > maxJSONFragmentBytes {
		return json.RawMessage(`"[TRUNCATED]"`)
	}
	return append(json.RawMessage(nil), encoded...)
}

func sanitizeJSONValue(value any, secret string, depth int) any {
	if depth >= 16 {
		return "[TRUNCATED]"
	}
	switch typed := value.(type) {
	case string:
		return boundedMessage(redactSensitive(redactExact(typed, secret)), 4096)
	case []any:
		if len(typed) > 64 {
			typed = typed[:64]
		}
		result := make([]any, len(typed))
		for index := range typed {
			result[index] = sanitizeJSONValue(typed[index], secret, depth+1)
		}
		return result
	case map[string]any:
		result := make(map[string]any, min(len(typed), 64))
		count := 0
		for key, item := range typed {
			if count == 64 {
				break
			}
			key = boundedMessage(key, 256)
			if key == "" {
				continue
			}
			if sensitiveJSONKey(key) {
				result[key] = "[REDACTED]"
			} else {
				result[key] = sanitizeJSONValue(item, secret, depth+1)
			}
			count++
		}
		return result
	default:
		return typed
	}
}

func sensitiveJSONKey(value string) bool {
	normalized := strings.ToLower(strings.NewReplacer("-", "_", " ", "_").Replace(value))
	for _, key := range []string{"access_token", "authorization", "bearer", "client_secret", "developer_token", "password"} {
		if strings.Contains(normalized, key) {
			return true
		}
	}
	return false
}

func sanitizeCause(err error) error {
	var urlError *url.Error
	if errors.As(err, &urlError) && urlError.Err != nil {
		return urlError.Err
	}
	return err
}
