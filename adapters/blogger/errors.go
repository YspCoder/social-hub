package blogger

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

const maxErrorRawBytes = 64 << 10

// LegacyErrorDetail preserves Google's older JSON error reason shape.
type LegacyErrorDetail struct {
	Domain       string `json:"domain"`
	Reason       string `json:"reason"`
	Message      string `json:"message"`
	LocationType string `json:"locationType"`
	Location     string `json:"location"`
}

// GoogleError is the google.rpc.Status-compatible Google JSON error object.
type GoogleError struct {
	Code    int                 `json:"code"`
	Message string              `json:"message"`
	Status  string              `json:"status"`
	Details []json.RawMessage   `json:"details"`
	Errors  []LegacyErrorDetail `json:"errors"`
}

// ErrorEnvelope is the standard top-level Google API failure shape.
type ErrorEnvelope struct {
	Error GoogleError `json:"error"`
}

// APIError retains a bounded, sanitized provider error and response metadata.
type APIError struct {
	Hub      *socialhub.Error
	Provider ErrorEnvelope
	Meta     ResponseMeta
	Raw      json.RawMessage
}

func (value *APIError) Error() string {
	if value == nil || value.Hub == nil {
		return "socialhub: blogger: platform_error"
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
		meta := responseMeta(header, clock, accessToken)
		var provider ErrorEnvelope
		decoded := json.Unmarshal(body, &provider) == nil
		reason := firstErrorReason(provider.Error)
		message := provider.Error.Message
		if message == "" && !decoded {
			message = string(bytes.TrimSpace(body))
		}
		if message == "" {
			message = http.StatusText(status)
		}

		classificationStatus := status
		if status >= 200 && status < 300 && provider.Error.Code >= 400 && provider.Error.Code <= 599 {
			classificationStatus = provider.Error.Code
		}
		code, class := classifyHTTPError(classificationStatus, provider.Error.Status, reason, message)
		platformCode := firstNonEmpty(reason, provider.Error.Status)
		if platformCode == "" && provider.Error.Code != 0 {
			platformCode = strconv.Itoa(provider.Error.Code)
		}
		if platformCode == "" {
			platformCode = "http_" + strconv.Itoa(status)
		}

		retryAfter := meta.RetryAfter
		if retryAfter == 0 {
			retryAfter = retryInfoDelay(provider.Error.Details)
		}
		hub := &socialhub.Error{
			Code: code, Class: class, Platform: platformName, Product: productName,
			HTTPStatus: status, PlatformCode: boundedMessage(redactText(platformCode, accessToken), 256),
			PlatformMessage: boundedMessage(redactText(message, accessToken), 1024),
			RequestID:       meta.RequestID, RetryAfter: retryAfter,
		}
		if code == socialhub.CodePermissionDenied {
			hub.RequiredScopes = []string{ScopeReadOnly}
			hub.ApprovalURL = authorizationURL
		}
		sanitizeErrorEnvelope(&provider, accessToken)
		return &APIError{
			Hub: hub, Provider: provider, Meta: meta,
			Raw: sanitizeProviderBody(body, accessToken),
		}
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

func retryInfoDelay(details []json.RawMessage) time.Duration {
	for _, detail := range details {
		var typed struct {
			Type       string `json:"@type"`
			RetryDelay string `json:"retryDelay"`
		}
		if json.Unmarshal(detail, &typed) != nil || !strings.HasSuffix(typed.Type, "/google.rpc.RetryInfo") {
			continue
		}
		delay, err := time.ParseDuration(typed.RetryDelay)
		if err == nil && delay > 0 && delay <= 48*time.Hour {
			return delay
		}
	}
	return 0
}

func classifyHTTPError(status int, platformStatus, reason, message string) (socialhub.ErrorCode, socialhub.ErrorClass) {
	normalizedReason := strings.NewReplacer("_", "", "-", "", ".", "", " ", "").Replace(strings.ToLower(reason))
	switch normalizedReason {
	case "ratelimitexceeded", "quotaexceeded", "exceededquota", "resourceexhausted":
		return socialhub.CodeRateLimited, socialhub.ClassRetryable
	case "dailylimitexceeded":
		return socialhub.CodeRateLimited, socialhub.ClassUserAction
	case "autherror", "invalidcredentials", "unauthenticated":
		return socialhub.CodeUnauthenticated, socialhub.ClassUserAction
	case "insufficientpermissions", "permissiondenied", "forbidden", "accesstokenscopeinsufficient":
		return socialhub.CodePermissionDenied, socialhub.ClassUserAction
	case "notfound":
		return socialhub.CodeNotFound, socialhub.ClassPermanent
	case "backenderror", "internalerror", "unavailable":
		return socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable
	case "invalid", "invalidargument", "outofrange":
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

func sanitizeErrorEnvelope(provider *ErrorEnvelope, accessToken string) {
	provider.Error.Message = boundedMessage(redactText(provider.Error.Message, accessToken), 1024)
	provider.Error.Status = boundedMessage(redactText(provider.Error.Status, accessToken), 256)
	for index := range provider.Error.Details {
		provider.Error.Details[index] = sanitizeProviderBody(provider.Error.Details[index], accessToken)
	}
	for index := range provider.Error.Errors {
		provider.Error.Errors[index].Domain = boundedMessage(redactText(provider.Error.Errors[index].Domain, accessToken), 256)
		provider.Error.Errors[index].Reason = boundedMessage(redactText(provider.Error.Errors[index].Reason, accessToken), 256)
		provider.Error.Errors[index].Message = boundedMessage(redactText(provider.Error.Errors[index].Message, accessToken), 1024)
		provider.Error.Errors[index].LocationType = boundedMessage(redactText(provider.Error.Errors[index].LocationType, accessToken), 256)
		provider.Error.Errors[index].Location = boundedMessage(redactText(provider.Error.Errors[index].Location, accessToken), 1024)
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

func sanitizeProviderBody(body []byte, accessToken string) json.RawMessage {
	if sanitized, ok := sanitizeProviderJSON(body, accessToken, maxErrorRawBytes); ok {
		return sanitized
	}
	if json.Valid(body) {
		return json.RawMessage(`{"error":"[REDACTED OVERSIZED PROVIDER JSON]"}`)
	}
	encoded, err := json.Marshal(redactText(strings.ToValidUTF8(string(body), ""), accessToken))
	if err != nil || len(encoded) > maxErrorRawBytes {
		return json.RawMessage(`{"error":"[REDACTED OVERSIZED PROVIDER BODY]"}`)
	}
	return encoded
}

func sanitizeProviderJSON(body []byte, accessToken string, maximum int) (json.RawMessage, bool) {
	if len(body) == 0 || len(body) > maxProviderObjectBytes {
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
	value = sanitizeJSONValue(value, accessToken)
	var encoded bytes.Buffer
	encoder := json.NewEncoder(&encoded)
	encoder.SetEscapeHTML(false)
	if encoder.Encode(value) != nil {
		return nil, false
	}
	result := bytes.TrimSpace(encoded.Bytes())
	if len(result) == 0 || len(result) > maximum {
		return nil, false
	}
	return append(json.RawMessage(nil), result...), true
}

func sanitizeJSONValue(value any, accessToken string) any {
	switch typed := value.(type) {
	case map[string]any:
		clean := make(map[string]any, len(typed))
		for key, child := range typed {
			if sensitiveKey(key) {
				clean[key] = "[REDACTED]"
				continue
			}
			clean[key] = sanitizeJSONValue(child, accessToken)
		}
		return clean
	case []any:
		clean := make([]any, len(typed))
		for index, child := range typed {
			clean[index] = sanitizeJSONValue(child, accessToken)
		}
		return clean
	case string:
		return redactText(typed, accessToken)
	default:
		return value
	}
}

func sensitiveKey(key string) bool {
	normalized := strings.ToLower(strings.NewReplacer("-", "", "_", "", " ", "").Replace(key))
	switch normalized {
	case "authorization", "accesstoken", "token", "key", "apikey", "clientsecret", "password":
		return true
	default:
		return false
	}
}

func redactText(value, accessToken string) string {
	if accessToken == "" {
		return value
	}
	return strings.ReplaceAll(value, accessToken, "[REDACTED]")
}

var _ error = (*APIError)(nil)
var _ interface{ Retryable() bool } = (*APIError)(nil)
