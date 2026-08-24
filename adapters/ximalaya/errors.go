package ximalaya

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

// ErrorEnvelope is Ximalaya's documented business-error response.
type ErrorEnvelope struct {
	ErrorNo   json.RawMessage `json:"error_no"`
	ErrorCode json.RawMessage `json:"error_code"`
	ErrorDesc string          `json:"error_desc"`
	Service   string          `json:"service"`
}

// APIError augments socialhub.Error with Ximalaya's sanitized error envelope.
type APIError struct {
	Hub      *socialhub.Error
	Provider ErrorEnvelope
	Meta     ResponseMeta
	Raw      json.RawMessage
}

func (value *APIError) Error() string {
	if value == nil || value.Hub == nil {
		return "socialhub: ximalaya: platform_error"
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

func newHTTPErrorDecoder(clock socialhub.Clock, redactions []string) func(int, http.Header, []byte) error {
	return func(status int, header http.Header, body []byte) error {
		return newAPIError(status, header, body, clock, redactions)
	}
}

func providerErrorFromBody(
	status int,
	header http.Header,
	body []byte,
	clock socialhub.Clock,
	redactions []string,
) (error, bool) {
	var probe ErrorEnvelope
	if json.Unmarshal(body, &probe) != nil {
		return nil, false
	}
	number, present := providerNumber(probe.ErrorNo)
	if !present || number == 0 {
		return nil, false
	}
	return newAPIError(status, header, body, clock, redactions), true
}

func newAPIError(status int, header http.Header, body []byte, clock socialhub.Clock, redactions []string) error {
	meta := responseMeta(status, header, clock, redactions)
	raw := sanitizeProviderBody(body, redactions)
	var provider ErrorEnvelope
	_ = json.Unmarshal(raw, &provider)
	number, _ := providerNumber(provider.ErrorNo)
	platformCode := providerCode(provider.ErrorCode)
	if platformCode == "" && number != 0 {
		platformCode = strconv.FormatInt(number, 10)
	}
	if platformCode == "" {
		platformCode = "http_" + strconv.Itoa(status)
	}
	message := provider.ErrorDesc
	if message == "" {
		message = providerMessage(raw)
	}
	if message == "" {
		message = http.StatusText(status)
	}
	code, class := classifyError(status, number)
	hub := &socialhub.Error{
		Code: code, Class: class, Platform: platformName, Product: productName,
		HTTPStatus: status, PlatformCode: boundedMessage(platformCode, 256),
		PlatformMessage: boundedMessage(message, 1024), RetryAfter: meta.RetryAfterDuration,
	}
	if code == socialhub.CodeApprovalRequired || code == socialhub.CodePermissionDenied || code == socialhub.CodeUnauthenticated {
		hub.ApprovalURL = applicationURL
	}
	provider.ErrorNo = boundedRaw(provider.ErrorNo)
	provider.ErrorCode = boundedRaw(provider.ErrorCode)
	provider.ErrorDesc = boundedMessage(provider.ErrorDesc, 1024)
	provider.Service = boundedMessage(provider.Service, 256)
	return &APIError{Hub: hub, Provider: provider, Meta: meta, Raw: raw}
}

func classifyError(status int, providerNo int64) (socialhub.ErrorCode, socialhub.ErrorClass) {
	switch providerNo {
	case 100, 105, 108, 200, 201, 205, 211, 213, 215, 400:
		return socialhub.CodeInvalidArgument, socialhub.ClassPermanent
	case 101, 103, 203, 206, 207, 208, 209, 210, 212, 214, 216, 217, 301, 702:
		return socialhub.CodeUnauthenticated, socialhub.ClassUserAction
	case 102, 111, 219:
		return socialhub.CodeApprovalRequired, socialhub.ClassUserAction
	case 202, 204:
		return socialhub.CodePermissionDenied, socialhub.ClassUserAction
	case 104:
		return socialhub.CodeRateLimited, socialhub.ClassRetryable
	case 110:
		return socialhub.CodeRateLimited, socialhub.ClassUserAction
	case 225:
		return socialhub.CodeConflict, socialhub.ClassPermanent
	case 500, 501, 502:
		return socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable
	case 611, 612, 643:
		return socialhub.CodePermissionDenied, socialhub.ClassUserAction
	}
	if status >= 300 && status < 400 {
		return socialhub.CodeConflict, socialhub.ClassPermanent
	}
	switch status {
	case http.StatusBadRequest, http.StatusMethodNotAllowed, http.StatusUnprocessableEntity:
		return socialhub.CodeInvalidArgument, socialhub.ClassPermanent
	case http.StatusUnauthorized:
		return socialhub.CodeUnauthenticated, socialhub.ClassUserAction
	case http.StatusForbidden:
		return socialhub.CodePermissionDenied, socialhub.ClassUserAction
	case http.StatusNotFound:
		return socialhub.CodeNotFound, socialhub.ClassPermanent
	case http.StatusTooManyRequests:
		return socialhub.CodeRateLimited, socialhub.ClassRetryable
	default:
		if status >= 500 {
			return socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable
		}
		return socialhub.CodePlatformError, socialhub.ClassPermanent
	}
}

func providerNumber(raw json.RawMessage) (int64, bool) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return 0, false
	}
	var number int64
	if json.Unmarshal(trimmed, &number) == nil {
		return number, true
	}
	var text string
	if json.Unmarshal(trimmed, &text) == nil {
		value, err := strconv.ParseInt(strings.TrimSpace(text), 10, 64)
		return value, err == nil
	}
	return 0, false
}

func providerCode(raw json.RawMessage) string {
	var text string
	if json.Unmarshal(raw, &text) == nil {
		return text
	}
	var number json.Number
	if json.Unmarshal(raw, &number) == nil {
		return number.String()
	}
	return ""
}

func platformError(operation string, code socialhub.ErrorCode, class socialhub.ErrorClass, cause error) error {
	return &socialhub.Error{
		Code: code, Class: class, Op: operation, Platform: platformName, Product: productName,
		Cause: sanitizeCause(cause),
	}
}

func authenticationError(operation string, cause error, secrets ...string) error {
	return &socialhub.Error{
		Code: socialhub.CodeUnauthenticated, Class: socialhub.ClassUserAction,
		Op: operation, Platform: platformName, Product: productName,
		Cause: sanitizeCredentialCause(cause, secrets...), ApprovalURL: applicationURL,
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

func boundedRaw(value []byte) json.RawMessage {
	if len(value) > maxErrorRawBytes {
		return json.RawMessage(`{"truncated":true}`)
	}
	return append(json.RawMessage(nil), value...)
}

func redactExact(value string, secrets ...string) string {
	for _, secret := range secrets {
		if secret != "" {
			value = strings.ReplaceAll(value, secret, "[REDACTED]")
		}
	}
	return value
}

func redactNamedValues(value string) string {
	for _, name := range []string{
		"access_token", "app_key", "app_secret", "authorization", "client_secret", "device_id",
		"nonce", "password", "private_key", "serverauthenticatestatickey", "server_auth_static_key", "sig",
	} {
		cursor := 0
		for {
			lower := strings.ToLower(value)
			start := strings.Index(lower[cursor:], name)
			if start < 0 {
				break
			}
			start += cursor
			separator := start + len(name)
			for separator < len(value) && strings.ContainsRune(" \t\"'", rune(value[separator])) {
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

func sanitizeProviderBody(body []byte, redactions []string) json.RawMessage {
	if len(bytes.TrimSpace(body)) == 0 {
		return nil
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	var decoded any
	if decoder.Decode(&decoded) == nil {
		var extra any
		if decoder.Decode(&extra) == io.EOF {
			encoded, err := json.Marshal(sanitizeJSONValue(decoded, redactions))
			if err == nil && len(encoded) <= maxErrorRawBytes {
				for _, secret := range redactions {
					if secret != "" && bytes.Contains(encoded, []byte(secret)) {
						return json.RawMessage(`{"truncated":true}`)
					}
				}
				return append(json.RawMessage(nil), encoded...)
			}
			return json.RawMessage(`{"truncated":true}`)
		}
	}
	text := redactNamedValues(redactExact(strings.ToValidUTF8(string(body), ""), redactions...))
	encoded, err := json.Marshal(text)
	if err != nil || len(encoded) > maxErrorRawBytes {
		return json.RawMessage(`{"truncated":true}`)
	}
	return encoded
}

func sanitizeJSONValue(value any, redactions []string) any {
	switch typed := value.(type) {
	case map[string]any:
		clean := make(map[string]any, len(typed))
		for key, child := range typed {
			cleanKey := redactExact(key, redactions...)
			if sensitiveJSONKey(key) {
				clean[cleanKey] = "[REDACTED]"
				continue
			}
			clean[cleanKey] = sanitizeJSONValue(child, redactions)
		}
		return clean
	case []any:
		clean := make([]any, len(typed))
		for index, child := range typed {
			clean[index] = sanitizeJSONValue(child, redactions)
		}
		return clean
	case string:
		return redactNamedValues(redactExact(typed, redactions...))
	case json.Number:
		text := string(typed)
		for _, secret := range redactions {
			if secret != "" && strings.Contains(text, secret) {
				return "[REDACTED]"
			}
		}
		return value
	default:
		return value
	}
}

func sensitiveJSONKey(value string) bool {
	normalized := strings.NewReplacer("_", "", "-", "", ".", "").Replace(strings.ToLower(value))
	switch normalized {
	case "accesstoken", "appkey", "appsecret", "authorization", "clientsecret", "deviceid", "nonce", "password", "privatekey", "serverauthenticatestatickey", "serverauthstatickey", "sig":
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
	return string(raw)
}

func sanitizeCause(err error) error {
	var urlError *url.Error
	if errors.As(err, &urlError) && urlError.Err != nil {
		return urlError.Err
	}
	return err
}

func sanitizeCredentialCause(err error, secrets ...string) error {
	if err == nil {
		return nil
	}
	clean := sanitizeCause(err)
	return errors.New(boundedMessage(redactNamedValues(redactExact(clean.Error(), secrets...)), 1024))
}
