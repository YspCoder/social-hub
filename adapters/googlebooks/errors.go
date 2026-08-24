package googlebooks

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

const maxErrorRawBytes = 64 << 10

var errInvalidCredential = errors.New("googlebooks: invalid resolved credential")

type LegacyErrorDetail struct {
	Domain       string `json:"domain"`
	Reason       string `json:"reason"`
	Message      string `json:"message"`
	LocationType string `json:"locationType"`
	Location     string `json:"location"`
}

// GoogleError accepts both google.rpc.Status fields and the legacy errors list.
type GoogleError struct {
	Code    int                 `json:"code"`
	Message string              `json:"message"`
	Status  string              `json:"status"`
	Details []json.RawMessage   `json:"details"`
	Errors  []LegacyErrorDetail `json:"errors"`
}

type ErrorEnvelope struct {
	Error GoogleError `json:"error"`
}

// APIError retains bounded, credential-sanitized provider JSON and metadata.
// Raw is always valid JSON, including for non-JSON or oversized provider bodies.
type APIError struct {
	Hub      *socialhub.Error
	Provider ErrorEnvelope
	Meta     ResponseMeta
	Raw      json.RawMessage
}

func (value *APIError) Error() string {
	if value == nil || value.Hub == nil {
		return "socialhub: google-books: platform_error"
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

func decodeAPIError(
	operation string,
	status int,
	header http.Header,
	body []byte,
	clock socialhub.Clock,
	credentials ...string,
) error {
	meta := responseMeta(header, clock, credentials...)
	var provider ErrorEnvelope
	decoded := json.Unmarshal(body, &provider) == nil
	reason := firstErrorReason(provider.Error)
	message := provider.Error.Message
	if message == "" && !decoded {
		message = http.StatusText(status)
	}
	if message == "" {
		message = "Google Books API rejected the request"
	}
	classificationStatus := status
	if status >= 200 && status < 300 && provider.Error.Code >= 400 && provider.Error.Code <= 599 {
		classificationStatus = provider.Error.Code
	}
	code, class := classifyAPIError(classificationStatus, provider.Error.Status, reason, message)
	platformCode := firstNonEmpty(reason, provider.Error.Status)
	if platformCode == "" && provider.Error.Code != 0 {
		platformCode = strconv.Itoa(provider.Error.Code)
	}
	if platformCode == "" {
		platformCode = "http_" + strconv.Itoa(status)
	}
	hub := &socialhub.Error{
		Code: code, Class: class, Platform: platformName, Product: productName, Op: operation,
		HTTPStatus: status, PlatformCode: boundedMessage(redactText(platformCode, credentials...), 256),
		PlatformMessage: boundedMessage(redactText(message, credentials...), 1_024),
		RequestID:       meta.RequestID, RetryAfter: meta.RetryAfter,
	}
	if code == socialhub.CodeUnauthenticated || code == socialhub.CodePermissionDenied {
		hub.ApprovalURL = authorizationURL
	}
	sanitizeErrorEnvelope(&provider, credentials...)
	return &APIError{
		Hub: hub, Provider: provider, Meta: meta,
		Raw: sanitizeProviderBody(body, credentials...),
	}
}

func firstErrorReason(value GoogleError) string {
	for _, detail := range value.Details {
		var typed struct {
			Reason string `json:"reason"`
		}
		if json.Unmarshal(detail, &typed) == nil && typed.Reason != "" {
			return typed.Reason
		}
	}
	for _, detail := range value.Errors {
		if detail.Reason != "" {
			return detail.Reason
		}
	}
	return ""
}

func classifyAPIError(status int, platformStatus, reason, message string) (socialhub.ErrorCode, socialhub.ErrorClass) {
	normalizedReason := normalizeErrorToken(reason)
	switch normalizedReason {
	case "apikeyinvalid", "keyinvalid", "autherror", "invalidcredentials", "unauthenticated":
		return socialhub.CodeUnauthenticated, socialhub.ClassUserAction
	case "apikeyserviceblocked", "apikeyhttprefererblocked", "forbidden", "insufficientpermissions",
		"permissiondenied", "accesstokenscopeinsufficient":
		return socialhub.CodePermissionDenied, socialhub.ClassUserAction
	case "ratelimitexceeded", "userratelimitexceeded", "quotaexceeded", "exceededquota", "resourceexhausted":
		return socialhub.CodeRateLimited, socialhub.ClassRetryable
	case "dailylimitexceeded":
		return socialhub.CodeRateLimited, socialhub.ClassUserAction
	case "notfound":
		return socialhub.CodeNotFound, socialhub.ClassPermanent
	case "backenderror", "internalerror", "unavailable":
		return socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable
	case "invalid", "invalidargument", "badrequest", "outofrange":
		return socialhub.CodeInvalidArgument, socialhub.ClassPermanent
	case "failedprecondition", "alreadyexists", "aborted":
		return socialhub.CodeConflict, socialhub.ClassPermanent
	}

	switch strings.ToUpper(strings.TrimSpace(platformStatus)) {
	case "INVALID_ARGUMENT", "OUT_OF_RANGE":
		return socialhub.CodeInvalidArgument, socialhub.ClassPermanent
	case "UNAUTHENTICATED":
		return socialhub.CodeUnauthenticated, socialhub.ClassUserAction
	case "PERMISSION_DENIED":
		return socialhub.CodePermissionDenied, socialhub.ClassUserAction
	case "NOT_FOUND":
		return socialhub.CodeNotFound, socialhub.ClassPermanent
	case "RESOURCE_EXHAUSTED":
		return socialhub.CodeRateLimited, socialhub.ClassRetryable
	case "FAILED_PRECONDITION", "ALREADY_EXISTS", "ABORTED":
		return socialhub.CodeConflict, socialhub.ClassPermanent
	case "UNAVAILABLE", "DEADLINE_EXCEEDED", "INTERNAL":
		return socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable
	}

	normalizedMessage := strings.ToLower(message)
	if strings.Contains(normalizedMessage, "api key not valid") || strings.Contains(normalizedMessage, "invalid api key") {
		return socialhub.CodeUnauthenticated, socialhub.ClassUserAction
	}
	if strings.Contains(normalizedMessage, "rate limit") || strings.Contains(normalizedMessage, "quota exceeded") {
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
	case http.StatusConflict, http.StatusPreconditionFailed:
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

func normalizeErrorToken(value string) string {
	return strings.NewReplacer("_", "", "-", "", ".", "", " ", "").Replace(strings.ToLower(value))
}

func responseMeta(header http.Header, clock socialhub.Clock, credentials ...string) ResponseMeta {
	retryAfter := boundedMessage(redactText(header.Get("Retry-After"), credentials...), 128)
	return ResponseMeta{
		RequestID: boundedMessage(redactText(firstHeaderValue(header,
			"X-Goog-Request-ID", "X-Google-Request-ID", "X-Request-ID"), credentials...), 512),
		RetryAfter:   parseRetryAfter(retryAfter, clock.Now()),
		QuotaHeaders: dynamicQuotaHeaders(header, credentials...),
	}
}

func dynamicQuotaHeaders(header http.Header, credentials ...string) map[string]string {
	names := make([]string, 0, len(header))
	for name := range header {
		normalized := strings.ToLower(name)
		if strings.Contains(normalized, "quota") || strings.Contains(normalized, "ratelimit") ||
			strings.Contains(normalized, "rate-limit") {
			names = append(names, name)
		}
	}
	if len(names) == 0 {
		return nil
	}
	sort.Strings(names)
	if len(names) > 32 {
		names = names[:32]
	}
	result := make(map[string]string, len(names))
	for _, name := range names {
		result[boundedMessage(name, 256)] = boundedMessage(redactText(strings.Join(header.Values(name), ", "), credentials...), 2_048)
	}
	return result
}

func firstHeaderValue(header http.Header, names ...string) string {
	for _, name := range names {
		if value := strings.TrimSpace(header.Get(name)); value != "" {
			return value
		}
	}
	return ""
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
	delay := when.Sub(now)
	if delay <= 0 || delay > 48*time.Hour {
		return 0
	}
	return delay
}

func sanitizeErrorEnvelope(provider *ErrorEnvelope, credentials ...string) {
	provider.Error.Message = boundedMessage(redactText(provider.Error.Message, credentials...), 1_024)
	provider.Error.Status = boundedMessage(redactText(provider.Error.Status, credentials...), 256)
	for index := range provider.Error.Details {
		provider.Error.Details[index] = sanitizeProviderBody(provider.Error.Details[index], credentials...)
	}
	for index := range provider.Error.Errors {
		detail := &provider.Error.Errors[index]
		detail.Domain = boundedMessage(redactText(detail.Domain, credentials...), 256)
		detail.Reason = boundedMessage(redactText(detail.Reason, credentials...), 256)
		detail.Message = boundedMessage(redactText(detail.Message, credentials...), 1_024)
		detail.LocationType = boundedMessage(redactText(detail.LocationType, credentials...), 256)
		detail.Location = boundedMessage(redactText(detail.Location, credentials...), 1_024)
	}
}

func sanitizeProviderBody(body []byte, credentials ...string) json.RawMessage {
	if sanitized, ok := sanitizeProviderJSON(body, maxErrorRawBytes, credentials...); ok {
		return sanitized
	}
	if json.Valid(body) {
		return json.RawMessage(`{"error":{"message":"[REDACTED OVERSIZED PROVIDER JSON]"}}`)
	}
	return json.RawMessage(`{"error":{"message":"[REDACTED NON-JSON PROVIDER BODY]"}}`)
}

func sanitizeProviderJSON(body []byte, maximum int, credentials ...string) (json.RawMessage, bool) {
	if len(body) == 0 || int64(len(body)) > maxProviderObjectBytes {
		return nil, false
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	var value any
	if decoder.Decode(&value) != nil {
		return nil, false
	}
	var trailing any
	if decoder.Decode(&trailing) != io.EOF {
		return nil, false
	}
	value = sanitizeJSONValue(value, credentials...)
	encoded, err := json.Marshal(value)
	if err != nil || len(encoded) == 0 || len(encoded) > maximum {
		return nil, false
	}
	return append(json.RawMessage(nil), encoded...), true
}

func sanitizeJSONValue(value any, credentials ...string) any {
	switch typed := value.(type) {
	case map[string]any:
		clean := make(map[string]any, len(typed))
		for key, child := range typed {
			if sensitiveKey(key) {
				clean[key] = "[REDACTED]"
				continue
			}
			clean[key] = sanitizeJSONValue(child, credentials...)
		}
		return clean
	case []any:
		clean := make([]any, len(typed))
		for index, child := range typed {
			clean[index] = sanitizeJSONValue(child, credentials...)
		}
		return clean
	case string:
		return redactText(typed, credentials...)
	default:
		return value
	}
}

func sensitiveKey(key string) bool {
	switch normalizeErrorToken(key) {
	case "authorization", "accesstoken", "token", "key", "apikey", "clientsecret", "password":
		return true
	default:
		return false
	}
}

func redactText(value string, credentials ...string) string {
	for _, credential := range credentials {
		if credential != "" {
			value = strings.ReplaceAll(value, credential, "[REDACTED]")
		}
	}
	return value
}

func transportError(operation string, cause error, credentials ...string) error {
	clean := sanitizeCause(cause)
	if clean != nil {
		message := redactText(clean.Error(), credentials...)
		if message != clean.Error() {
			clean = errors.New(boundedMessage(message, 1_024))
		}
	}
	return platformError(operation, socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, clean)
}

func authenticationError(operation string, cause error) error {
	return &socialhub.Error{
		Code: socialhub.CodeUnauthenticated, Class: socialhub.ClassUserAction,
		Platform: platformName, Product: productName, Op: operation, Cause: sanitizeCause(cause),
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
		PlatformMessage: boundedMessage(message, 1_024),
	}
}

func platformContractError(operation, message string) error {
	return &socialhub.Error{
		Code: socialhub.CodePlatformError, Class: socialhub.ClassPermanent,
		Platform: platformName, Product: productName, Op: operation,
		PlatformMessage: boundedMessage(message, 1_024),
	}
}

func sanitizeCause(err error) error {
	var urlError *url.Error
	if errors.As(err, &urlError) && urlError.Err != nil {
		return urlError.Err
	}
	if errors.Is(err, context.Canceled) {
		return context.Canceled
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return context.DeadlineExceeded
	}
	return err
}

func boundedMessage(value string, maximum int) string {
	if utf8.RuneCountInString(value) <= maximum {
		return value
	}
	return string([]rune(value)[:maximum])
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

var _ error = (*APIError)(nil)
var _ interface{ Retryable() bool } = (*APIError)(nil)
