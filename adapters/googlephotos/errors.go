package googlephotos

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

const maxErrorRawBytes = 64 << 10

// LegacyErrorDetail preserves the older Google JSON error reason shape.
type LegacyErrorDetail struct {
	Domain       string `json:"domain"`
	Reason       string `json:"reason"`
	Message      string `json:"message"`
	LocationType string `json:"locationType"`
	Location     string `json:"location"`
}

// GoogleError is the standard Google JSON error object. Details remain raw
// because google.rpc detail message types vary by failure.
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

// APIError augments socialhub.Error with a sanitized Google provider envelope
// and response quota metadata.
type APIError struct {
	Hub      *socialhub.Error
	Provider ErrorEnvelope
	Meta     ResponseMeta
	Raw      json.RawMessage
}

func (value *APIError) Error() string {
	if value == nil || value.Hub == nil {
		return "socialhub: google-photos: platform_error"
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
		meta := responseMeta(header, clock)
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
		code, class := classifyHTTPError(status, provider.Error.Status, reason, message)
		platformCode := redactText(reason, accessToken)
		if platformCode == "" {
			platformCode = redactText(provider.Error.Status, accessToken)
		}
		if platformCode == "" && provider.Error.Code != 0 {
			platformCode = strconv.Itoa(provider.Error.Code)
		}
		if platformCode == "" {
			platformCode = "http_" + strconv.Itoa(status)
		}
		hub := &socialhub.Error{
			Code: code, Class: class, Platform: platformName, Product: productName,
			HTTPStatus: status, PlatformCode: boundedMessage(platformCode, 256),
			PlatformMessage: boundedMessage(redactText(message, accessToken), 1024),
			RequestID:       meta.RequestID, RetryAfter: meta.RetryAfterDuration,
		}
		if code == socialhub.CodePermissionDenied {
			hub.RequiredScopes = []string{ScopeReadAppCreatedData}
			hub.ApprovalURL = authorizationURL
		}
		if code == socialhub.CodeUnauthenticated {
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

func classifyHTTPError(status int, platformStatus, reason, message string) (socialhub.ErrorCode, socialhub.ErrorClass) {
	normalized := strings.ToLower(strings.Join([]string{platformStatus, reason, message}, " "))
	if status == http.StatusTooManyRequests || strings.Contains(normalized, "resource_exhausted") ||
		strings.Contains(normalized, "rate limit") || strings.Contains(normalized, "ratelimit") ||
		strings.Contains(normalized, "quotaexceeded") || strings.Contains(normalized, "quota exceeded") ||
		strings.Contains(normalized, "dailylimit") {
		return socialhub.CodeRateLimited, socialhub.ClassRetryable
	}
	switch platformStatus {
	case "UNAUTHENTICATED":
		return socialhub.CodeUnauthenticated, socialhub.ClassUserAction
	case "PERMISSION_DENIED":
		return socialhub.CodePermissionDenied, socialhub.ClassUserAction
	case "NOT_FOUND":
		return socialhub.CodeNotFound, socialhub.ClassPermanent
	case "ALREADY_EXISTS", "ABORTED", "FAILED_PRECONDITION":
		return socialhub.CodeConflict, socialhub.ClassPermanent
	case "UNAVAILABLE", "DEADLINE_EXCEEDED":
		return socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable
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

func sanitizeProviderBody(body []byte, accessToken string) json.RawMessage {
	if len(body) == 0 {
		return nil
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	var value any
	if decoder.Decode(&value) == nil {
		var trailing any
		if decoder.Decode(&trailing) == io.EOF {
			value = sanitizeJSONValue(value, accessToken)
			if encoded, err := json.Marshal(value); err == nil {
				return boundedRaw(encoded)
			}
		}
	}
	return boundedRaw([]byte(redactText(string(body), accessToken)))
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
	case "accesstoken", "authorization", "clientsecret", "cookie", "password", "refreshtoken", "secret", "token":
		return true
	default:
		return false
	}
}

func redactText(value, accessToken string) string {
	if accessToken != "" {
		value = strings.ReplaceAll(value, accessToken, "[REDACTED]")
	}
	for _, key := range []string{"access_token", "authorization", "client_secret", "password", "refresh_token"} {
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

func sanitizeCause(err error) error {
	var urlError *url.Error
	if errors.As(err, &urlError) && urlError.Err != nil {
		return urlError.Err
	}
	return err
}
