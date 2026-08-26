package cjpublisher

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

type GraphQLLocation struct {
	Line   int `json:"line"`
	Column int `json:"column"`
}

type GraphQLErrorExtensions struct {
	Code           string          `json:"code"`
	Classification string          `json:"classification"`
	Raw            json.RawMessage `json:"-"`
}

func (value *GraphQLErrorExtensions) UnmarshalJSON(data []byte) error {
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		*value = GraphQLErrorExtensions{}
		return nil
	}
	type wire GraphQLErrorExtensions
	var decoded wire
	if err := decodeProviderObject(data, &decoded); err != nil {
		return err
	}
	*value = GraphQLErrorExtensions(decoded)
	value.Raw = append(value.Raw[:0], data...)
	return nil
}

type GraphQLError struct {
	Message    string                 `json:"message"`
	Locations  []GraphQLLocation      `json:"locations"`
	Path       []json.RawMessage      `json:"path"`
	Extensions GraphQLErrorExtensions `json:"extensions"`
}

// APIError augments socialhub.Error with CJ's GraphQL or REST error details.
// Raw is sanitized because legacy REST errors can echo the rejected token.
type APIError struct {
	Hub     *socialhub.Error
	GraphQL []GraphQLError
	Message string
	Raw     []byte
}

func (value *APIError) Error() string {
	if value == nil || value.Hub == nil {
		return "socialhub: cj: platform_error"
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
		return decodeHTTPError(status, header, body, clock.Now(), accessToken)
	}
}

func decodeHTTPError(status int, header http.Header, body []byte, now time.Time, accessToken string) error {
	var envelope struct {
		Errors  []GraphQLError `json:"errors"`
		Message string         `json:"message"`
		Error   string         `json:"error"`
	}
	_ = json.Unmarshal(body, &envelope)
	message := firstNonEmpty(envelope.Message, envelope.Error)
	platformCode := "http_" + strconv.Itoa(status)
	if len(envelope.Errors) > 0 {
		message = envelope.Errors[0].Message
		platformCode = firstNonEmpty(envelope.Errors[0].Extensions.Code, envelope.Errors[0].Extensions.Classification, platformCode)
	}
	if message == "" {
		message = string(bytes.TrimSpace(body))
	}
	if message == "" {
		message = firstNonEmpty(http.StatusText(status), "CJ rejected the request")
	}
	message = boundedMessage(redactSensitive(redactExact(message, accessToken)), 1024)
	code, class := classifyHTTPError(status)
	hub := &socialhub.Error{
		Code: code, Class: class, Platform: platformName, Product: productName,
		HTTPStatus: status, PlatformCode: boundedMessage(platformCode, 256), PlatformMessage: message,
		RequestID:  boundedMessage(firstHeader(header, "X-Request-ID", "X-Correlation-ID"), 256),
		RetryAfter: parseRetryAfter(header.Get("Retry-After"), now),
	}
	if code == socialhub.CodePermissionDenied || code == socialhub.CodeApprovalRequired {
		hub.ApprovalURL = documentationURL
	}
	return &APIError{
		Hub: hub, GraphQL: sanitizeGraphQLErrors(envelope.Errors), Message: message,
		Raw: sanitizeProviderBody(body, accessToken),
	}
}

func graphQLOperationError(operation string, provider []GraphQLError, raw []byte, requestID string) error {
	provider = sanitizeGraphQLErrors(provider)
	message, platformCode := "CJ returned a GraphQL error", "graphql_error"
	if len(provider) > 0 {
		message = firstNonEmpty(provider[0].Message, message)
		platformCode = firstNonEmpty(provider[0].Extensions.Code, provider[0].Extensions.Classification, platformCode)
	}
	code, class := classifyGraphQLError(platformCode)
	hub := &socialhub.Error{
		Code: code, Class: class, Platform: platformName, Product: productName, Op: operation,
		HTTPStatus: http.StatusOK, PlatformCode: boundedMessage(platformCode, 256),
		PlatformMessage: boundedMessage(message, 1024), RequestID: boundedMessage(requestID, 256),
	}
	if code == socialhub.CodePermissionDenied || code == socialhub.CodeApprovalRequired {
		hub.ApprovalURL = documentationURL
	}
	return &APIError{Hub: hub, GraphQL: provider, Message: hub.PlatformMessage, Raw: sanitizeProviderBody(raw, "")}
}

func sanitizeGraphQLErrors(values []GraphQLError) []GraphQLError {
	result := make([]GraphQLError, len(values))
	for index, value := range values {
		value.Message = boundedMessage(redactSensitive(value.Message), 1024)
		value.Extensions.Code = boundedMessage(value.Extensions.Code, 256)
		value.Extensions.Classification = boundedMessage(value.Extensions.Classification, 256)
		value.Extensions.Raw = append(json.RawMessage(nil), value.Extensions.Raw...)
		value.Locations = append([]GraphQLLocation(nil), value.Locations...)
		value.Path = append([]json.RawMessage(nil), value.Path...)
		result[index] = value
	}
	return result
}

func classifyGraphQLError(value string) (socialhub.ErrorCode, socialhub.ErrorClass) {
	normalized := strings.ToUpper(strings.ReplaceAll(strings.ReplaceAll(value, "-", "_"), " ", "_"))
	switch {
	case strings.Contains(normalized, "UNAUTHENTICATED"), strings.Contains(normalized, "UNAUTHORIZED"):
		return socialhub.CodeUnauthenticated, socialhub.ClassUserAction
	case strings.Contains(normalized, "FORBIDDEN"), strings.Contains(normalized, "PERMISSION"):
		return socialhub.CodePermissionDenied, socialhub.ClassUserAction
	case strings.Contains(normalized, "RATE"), strings.Contains(normalized, "THROTTL"), strings.Contains(normalized, "TOO_MANY"):
		return socialhub.CodeRateLimited, socialhub.ClassRetryable
	case strings.Contains(normalized, "NOT_FOUND"):
		return socialhub.CodeNotFound, socialhub.ClassPermanent
	case strings.Contains(normalized, "VALIDATION"), strings.Contains(normalized, "BAD_REQUEST"), strings.Contains(normalized, "BAD_USER_INPUT"), strings.Contains(normalized, "PARSE"):
		return socialhub.CodeInvalidArgument, socialhub.ClassPermanent
	case strings.Contains(normalized, "INTERNAL"), strings.Contains(normalized, "UNAVAILABLE"), strings.Contains(normalized, "TIMEOUT"):
		return socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable
	default:
		return socialhub.CodePlatformError, socialhub.ClassPermanent
	}
}

func classifyHTTPError(status int) (socialhub.ErrorCode, socialhub.ErrorClass) {
	switch status {
	case http.StatusBadRequest, http.StatusMethodNotAllowed, http.StatusRequestEntityTooLarge,
		http.StatusUnsupportedMediaType, http.StatusUnprocessableEntity:
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

func firstHeader(header http.Header, names ...string) string {
	for _, name := range names {
		if value := header.Get(name); value != "" {
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

func redactExact(value, secret string) string {
	if secret == "" {
		return value
	}
	return strings.ReplaceAll(value, secret, "[REDACTED]")
}

func redactSensitive(value string) string {
	for _, key := range []string{
		"access_token", "authorization", "bearer", "developer key", "not authenticated", "password", "secret",
	} {
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

func sanitizeProviderBody(body []byte, secret string) []byte {
	if len(body) == 0 {
		return nil
	}
	sanitized := redactSensitive(redactExact(string(body), secret))
	return []byte(sanitized)
}

func sanitizeCause(err error) error {
	var urlError *url.Error
	if errors.As(err, &urlError) && urlError.Err != nil {
		return urlError.Err
	}
	return err
}
