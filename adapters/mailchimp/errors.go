package mailchimp

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

type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// ProblemDetail is Mailchimp's RFC 7807-style provider error document.
type ProblemDetail struct {
	Type     string       `json:"type"`
	Title    string       `json:"title"`
	Status   int          `json:"status"`
	Detail   string       `json:"detail"`
	Instance string       `json:"instance"`
	Errors   []FieldError `json:"errors,omitempty"`
}

// APIError retains a bounded, sanitized provider problem and concurrency metadata.
type APIError struct {
	Hub      *socialhub.Error
	Provider ProblemDetail
	Meta     ResponseMeta
	Raw      json.RawMessage
}

func (value *APIError) Error() string {
	if value == nil || value.Hub == nil {
		return "socialhub: mailchimp: platform_error"
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

func newHTTPErrorDecoder(clock socialhub.Clock, apiKey, authorization string) func(int, http.Header, []byte) error {
	return func(status int, header http.Header, body []byte) error {
		meta := responseMeta(status, header, clock, apiKey, authorization)
		var provider ProblemDetail
		decoded := json.Unmarshal(body, &provider) == nil
		message := provider.Detail
		if message == "" {
			message = provider.Title
		}
		if message == "" && !decoded {
			message = string(bytes.TrimSpace(body))
		}
		if message == "" {
			message = http.StatusText(status)
		}
		classificationStatus := status
		if status >= 200 && status < 300 && provider.Status >= 400 && provider.Status <= 599 {
			classificationStatus = provider.Status
		}
		code, class := classifyHTTPError(classificationStatus, provider.Title, provider.Detail)
		platformCode := provider.Title
		if platformCode == "" {
			platformCode = "http_" + strconv.Itoa(status)
		}
		requestID := meta.RequestID
		if requestID == "" {
			requestID = provider.Instance
			meta.RequestID = boundedMessage(redactText(requestID, apiKey, authorization), 512)
		}
		hub := &socialhub.Error{
			Code: code, Class: class, Platform: platformName, Product: productName,
			HTTPStatus: status, PlatformCode: boundedMessage(redactText(platformCode, apiKey, authorization), 256),
			PlatformMessage: boundedMessage(redactText(message, apiKey, authorization), 1024),
			RequestID:       meta.RequestID, RetryAfter: meta.RetryAfter,
		}
		if code == socialhub.CodePermissionDenied || code == socialhub.CodeUnauthenticated {
			hub.ApprovalURL = documentationURL
		}
		sanitizeProblem(&provider, apiKey, authorization)
		return &APIError{
			Hub: hub, Provider: provider, Meta: meta,
			Raw: sanitizeProviderBody(body, apiKey, authorization),
		}
	}
}

func classifyHTTPError(status int, title, detail string) (socialhub.ErrorCode, socialhub.ErrorClass) {
	normalized := strings.ToLower(strings.Join([]string{title, detail}, " "))
	compactTitle := strings.NewReplacer(" ", "", "-", "", "_", "", ":", "").Replace(strings.ToLower(title))
	if status == http.StatusTooManyRequests || compactTitle == "toomanyrequests" || strings.Contains(normalized, "simultaneous connection") ||
		strings.Contains(normalized, "rate limit") || strings.Contains(normalized, "throttl") {
		return socialhub.CodeRateLimited, socialhub.ClassRetryable
	}
	switch compactTitle {
	case "apikeyinvalid", "unauthorized":
		return socialhub.CodeUnauthenticated, socialhub.ClassUserAction
	case "forbidden", "userdisabled":
		return socialhub.CodePermissionDenied, socialhub.ClassUserAction
	case "resourcenotfound", "notfound":
		return socialhub.CodeNotFound, socialhub.ClassPermanent
	case "invalidresource", "badrequest":
		return socialhub.CodeInvalidArgument, socialhub.ClassPermanent
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

func sanitizeProblem(provider *ProblemDetail, apiKey, authorization string) {
	provider.Type = boundedMessage(redactText(provider.Type, apiKey, authorization), 1024)
	provider.Title = boundedMessage(redactText(provider.Title, apiKey, authorization), 256)
	provider.Detail = boundedMessage(redactText(provider.Detail, apiKey, authorization), 1024)
	provider.Instance = boundedMessage(redactText(provider.Instance, apiKey, authorization), 512)
	if len(provider.Errors) > 64 {
		provider.Errors = append([]FieldError(nil), provider.Errors[:64]...)
	}
	for index := range provider.Errors {
		provider.Errors[index].Field = boundedMessage(redactText(provider.Errors[index].Field, apiKey, authorization), 256)
		provider.Errors[index].Message = boundedMessage(redactText(provider.Errors[index].Message, apiKey, authorization), 1024)
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

func boundedRaw(value []byte, maximum int) json.RawMessage {
	trimmed := bytes.TrimSpace(value)
	if len(trimmed) == 0 {
		return json.RawMessage("null")
	}
	if len(trimmed) <= maximum && json.Valid(trimmed) {
		return append(json.RawMessage(nil), trimmed...)
	}
	if len(trimmed) > maximum {
		return json.RawMessage(`{"truncated":true}`)
	}
	encoded, _ := json.Marshal(string(bytes.ToValidUTF8(trimmed, []byte("?"))))
	if len(encoded) > maximum {
		return json.RawMessage(`{"truncated":true}`)
	}
	return encoded
}

func sanitizeProviderBody(body []byte, apiKey, authorization string) json.RawMessage {
	if sanitized, ok := sanitizeProviderJSON(body, apiKey, authorization, maxErrorRawBytes); ok {
		return sanitized
	}
	if json.Valid(body) {
		return json.RawMessage(`{"error":"[REDACTED OVERSIZED PROVIDER JSON]"}`)
	}
	return boundedRaw([]byte(redactText(string(body), apiKey, authorization)), maxErrorRawBytes)
}

func sanitizeProviderJSON(body []byte, apiKey, authorization string, maximum int) (json.RawMessage, bool) {
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
	value = sanitizeJSONValue(value, apiKey, authorization)
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

func sanitizeJSONValue(value any, apiKey, authorization string) any {
	switch typed := value.(type) {
	case map[string]any:
		clean := make(map[string]any, len(typed))
		for key, child := range typed {
			if sensitiveKey(key) {
				clean[key] = "[REDACTED]"
				continue
			}
			clean[key] = sanitizeJSONValue(child, apiKey, authorization)
		}
		return clean
	case []any:
		clean := make([]any, len(typed))
		for index, child := range typed {
			clean[index] = sanitizeJSONValue(child, apiKey, authorization)
		}
		return clean
	case string:
		return redactText(typed, apiKey, authorization)
	default:
		return value
	}
}

func sensitiveKey(key string) bool {
	normalized := strings.ToLower(strings.NewReplacer("-", "", "_", "", " ", "").Replace(key))
	switch normalized {
	case "authorization", "apikey", "key", "password", "token", "accesstoken", "clientsecret", "secret":
		return true
	default:
		return false
	}
}

func redactText(value, apiKey, authorization string) string {
	if apiKey != "" {
		value = strings.ReplaceAll(value, apiKey, "[REDACTED]")
	}
	if authorization != "" {
		value = strings.ReplaceAll(value, authorization, "[REDACTED]")
	}
	return value
}

func sanitizeCause(err error) error {
	if err == nil {
		return nil
	}
	var urlError *url.Error
	if errors.As(err, &urlError) && urlError.Err != nil {
		return urlError.Err
	}
	return err
}

var _ error = (*APIError)(nil)
var _ interface{ Retryable() bool } = (*APIError)(nil)
