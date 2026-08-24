package bitbucket

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

// ErrorDetail is Bitbucket's documented provider error object. Fields and Data
// stay raw because their endpoint-specific structures vary.
type ErrorDetail struct {
	Message string          `json:"message"`
	Fields  json.RawMessage `json:"fields"`
	Detail  string          `json:"detail"`
	ID      string          `json:"id"`
	Data    json.RawMessage `json:"data"`
}

// ErrorEnvelope is Bitbucket's standard REST failure shape.
type ErrorEnvelope struct {
	Type  string      `json:"type"`
	Error ErrorDetail `json:"error"`
}

// APIError augments socialhub.Error with Bitbucket's sanitized provider body
// and response metadata.
type APIError struct {
	Hub      *socialhub.Error
	Provider ErrorEnvelope
	Meta     ResponseMeta
	Raw      json.RawMessage
}

func (value *APIError) Error() string {
	if value == nil || value.Hub == nil {
		return "socialhub: bitbucket: platform_error"
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

func newHTTPErrorDecoder(clock socialhub.Clock, credential string) func(int, http.Header, []byte) error {
	return func(status int, header http.Header, body []byte) error {
		meta := responseMeta(header, clock)
		var provider ErrorEnvelope
		decoded := json.Unmarshal(body, &provider) == nil
		message := provider.Error.Message
		if message == "" && !decoded {
			message = string(bytes.TrimSpace(body))
		}
		if message == "" {
			message = http.StatusText(status)
		}
		code, class := classifyHTTPError(status)
		platformCode := provider.Error.ID
		if platformCode == "" {
			platformCode = provider.Type
		}
		if platformCode == "" {
			platformCode = "http_" + strconv.Itoa(status)
		}
		hub := &socialhub.Error{
			Code: code, Class: class, Platform: platformName, Product: productName,
			HTTPStatus: status, PlatformCode: boundedMessage(redactText(platformCode, credential), 256),
			PlatformMessage: boundedMessage(redactText(message, credential), 1024),
			RequestID:       meta.RequestID, RetryAfter: meta.RetryAfterDuration,
		}
		if code == socialhub.CodeUnauthenticated || code == socialhub.CodePermissionDenied {
			hub.ApprovalURL = authenticationURL
		}
		provider.Type = boundedMessage(redactText(provider.Type, credential), 256)
		provider.Error.Message = boundedMessage(redactText(provider.Error.Message, credential), 1024)
		provider.Error.Detail = boundedMessage(redactText(provider.Error.Detail, credential), 4096)
		provider.Error.ID = boundedMessage(redactText(provider.Error.ID, credential), 256)
		provider.Error.Fields = sanitizeProviderBody(provider.Error.Fields, credential)
		provider.Error.Data = sanitizeProviderBody(provider.Error.Data, credential)
		return &APIError{
			Hub: hub, Provider: provider, Meta: meta,
			Raw: sanitizeProviderBody(body, credential),
		}
	}
}

func classifyHTTPError(status int) (socialhub.ErrorCode, socialhub.ErrorClass) {
	if status == http.StatusTooManyRequests {
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
	case http.StatusMovedPermanently, http.StatusFound, http.StatusTemporaryRedirect,
		http.StatusPermanentRedirect, http.StatusConflict, http.StatusPreconditionFailed:
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

func sanitizeProviderBody(body []byte, credential string) json.RawMessage {
	if len(body) == 0 {
		return nil
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	var value any
	if decoder.Decode(&value) == nil {
		var trailing any
		if decoder.Decode(&trailing) == io.EOF {
			value = sanitizeJSONValue(value, credential)
			if encoded, err := json.Marshal(value); err == nil {
				return boundedRaw(encoded)
			}
		}
	}
	return boundedRaw([]byte(redactText(string(body), credential)))
}

func sanitizeJSONValue(value any, credential string) any {
	switch typed := value.(type) {
	case map[string]any:
		clean := make(map[string]any, len(typed))
		for key, child := range typed {
			if sensitiveKey(key) {
				clean[key] = "[REDACTED]"
				continue
			}
			clean[key] = sanitizeJSONValue(child, credential)
		}
		return clean
	case []any:
		clean := make([]any, len(typed))
		for index, child := range typed {
			clean[index] = sanitizeJSONValue(child, credential)
		}
		return clean
	case string:
		return redactText(typed, credential)
	default:
		return value
	}
}

func sensitiveKey(key string) bool {
	normalized := strings.ToLower(strings.NewReplacer("-", "", "_", "", " ", "").Replace(key))
	switch normalized {
	case "accesstoken", "apitoken", "authorization", "clientsecret", "cookie", "password", "refreshtoken", "secret", "token":
		return true
	default:
		return false
	}
}

func redactText(value, credential string) string {
	if credential != "" {
		value = strings.ReplaceAll(value, credential, "[REDACTED]")
	}
	for _, key := range []string{"access_token", "api_token", "authorization", "client_secret", "password", "refresh_token"} {
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
