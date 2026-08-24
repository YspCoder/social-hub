package tiktokresearch

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
	"unicode"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

const maxErrorRawBytes = 64 << 10

// ProviderError is TikTok's standard v2 error object.
type ProviderError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	LogID   string `json:"log_id"`
}

// APIError augments socialhub.Error with a sanitized TikTok error envelope.
type APIError struct {
	Hub      *socialhub.Error
	Provider ProviderError
	Meta     ResponseMeta
	Raw      json.RawMessage
}

func (value *APIError) Error() string {
	if value == nil || value.Hub == nil {
		return "socialhub: tiktok: research-api: platform_error"
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
		var envelope struct {
			Error *ProviderError `json:"error"`
		}
		decoded := json.Unmarshal(body, &envelope) == nil
		provider := ProviderError{}
		if envelope.Error != nil {
			provider = *envelope.Error
		}
		message := provider.Message
		if message == "" && !decoded {
			message = string(bytes.TrimSpace(body))
		}
		if message == "" {
			message = http.StatusText(status)
		}
		code, class := classifyProviderError(provider.Code, status)
		platformCode := provider.Code
		if platformCode == "" {
			platformCode = "http_" + strconv.Itoa(status)
		}
		logID := firstNonEmpty(provider.LogID, meta.LogID)
		hub := &socialhub.Error{
			Code: code, Class: class, Platform: platformName, Product: productName,
			HTTPStatus: status, PlatformCode: boundedSafeValue(platformCode, 256),
			PlatformMessage: boundedMessage(redactText(message, accessToken), 1024),
			RequestID:       boundedSafeValue(logID, 512), RetryAfter: meta.RetryAfterDuration,
		}
		setApprovalDetails(hub)
		sanitizeProviderError(&provider, accessToken)
		return &APIError{
			Hub: hub, Provider: provider, Meta: meta,
			Raw: sanitizeProviderBody(body, accessToken),
		}
	}
}

func requireEnvelope[T any](
	operation string,
	envelope responseEnvelope[T],
	raw json.RawMessage,
	meta ResponseMeta,
	sensitiveValues ...string,
) (*T, ResponseMeta, error) {
	if envelope.Error == nil || envelope.Error.Code == "" {
		return nil, meta, platformContractError(operation, "TikTok response omitted error metadata")
	}
	if envelope.Error.Code != "ok" {
		return nil, meta, businessError(operation, *envelope.Error, raw, meta, sensitiveValues...)
	}
	if envelope.Data == nil {
		return nil, meta, platformContractError(operation, "TikTok success response omitted data")
	}
	if logID := boundedSafeValue(envelope.Error.LogID, 512); logID != "" {
		meta.LogID = logID
	}
	return envelope.Data, meta, nil
}

func businessError(operation string, provider ProviderError, raw json.RawMessage, meta ResponseMeta, sensitiveValues ...string) error {
	code, class := classifyProviderError(provider.Code, http.StatusOK)
	logID := firstNonEmpty(provider.LogID, meta.LogID)
	hub := &socialhub.Error{
		Code: code, Class: class, Platform: platformName, Product: productName, Op: operation,
		PlatformCode:    boundedSafeValue(provider.Code, 256),
		PlatformMessage: boundedMessage(redactText(provider.Message, sensitiveValues...), 1024),
		RequestID:       boundedSafeValue(logID, 512), RetryAfter: meta.RetryAfterDuration,
	}
	setApprovalDetails(hub)
	sanitizeProviderError(&provider, sensitiveValues...)
	return &APIError{
		Hub: hub, Provider: provider, Meta: meta,
		Raw: sanitizeProviderBody(raw, sensitiveValues...),
	}
}

func classifyProviderError(providerCode string, status int) (socialhub.ErrorCode, socialhub.ErrorClass) {
	switch strings.ToLower(strings.TrimSpace(providerCode)) {
	case "access_token_invalid", "invalid_access_token", "access_token_expired":
		return socialhub.CodeUnauthenticated, socialhub.ClassUserAction
	case "scope_not_authorized":
		return socialhub.CodeApprovalRequired, socialhub.ClassUserAction
	case "rate_limit_exceeded":
		return socialhub.CodeRateLimited, socialhub.ClassRetryable
	case "invalid_params":
		return socialhub.CodeInvalidArgument, socialhub.ClassPermanent
	case "internal_error":
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
	case http.StatusConflict:
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

func setApprovalDetails(hub *socialhub.Error) {
	if hub.Code == socialhub.CodeUnauthenticated || hub.Code == socialhub.CodePermissionDenied || hub.Code == socialhub.CodeApprovalRequired {
		hub.ApprovalURL = approvalURL
		hub.RequiredScopes = []string{RequiredScope}
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

func boundedMessage(value string, maximum int) string {
	if utf8.RuneCountInString(value) <= maximum {
		return value
	}
	return string([]rune(value)[:maximum])
}

func boundedSafeValue(value string, maximum int) string {
	value = strings.TrimSpace(value)
	if value == "" || !utf8.ValidString(value) || strings.ContainsFunc(value, unicode.IsControl) {
		return ""
	}
	return boundedMessage(value, maximum)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func sanitizeProviderError(provider *ProviderError, sensitiveValues ...string) {
	provider.Code = boundedSafeValue(provider.Code, 256)
	provider.Message = boundedMessage(redactText(provider.Message, sensitiveValues...), 1024)
	provider.LogID = boundedSafeValue(provider.LogID, 512)
}

func boundedRaw(value []byte) json.RawMessage {
	if len(value) > maxErrorRawBytes {
		return json.RawMessage(`{"truncated":true}`)
	}
	return append(json.RawMessage(nil), value...)
}

func sanitizeProviderBody(body []byte, sensitiveValues ...string) json.RawMessage {
	if len(body) == 0 {
		return nil
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	var value any
	if decoder.Decode(&value) == nil {
		var trailing any
		if decoder.Decode(&trailing) == io.EOF {
			value = sanitizeJSONValue(value, sensitiveValues...)
			if encoded, err := json.Marshal(value); err == nil {
				return boundedRaw(encoded)
			}
		}
	}
	encoded, err := json.Marshal(redactText(strings.ToValidUTF8(string(body), ""), sensitiveValues...))
	if err != nil {
		return json.RawMessage(`{"truncated":true}`)
	}
	return boundedRaw(encoded)
}

func sanitizeJSONValue(value any, sensitiveValues ...string) any {
	switch typed := value.(type) {
	case map[string]any:
		clean := make(map[string]any, len(typed))
		for key, child := range typed {
			if sensitiveProviderKey(key) {
				clean[key] = "[REDACTED]"
				continue
			}
			clean[key] = sanitizeJSONValue(child, sensitiveValues...)
		}
		return clean
	case []any:
		clean := make([]any, len(typed))
		for index, child := range typed {
			clean[index] = sanitizeJSONValue(child, sensitiveValues...)
		}
		return clean
	case string:
		return redactText(typed, sensitiveValues...)
	default:
		return value
	}
}

func sensitiveProviderKey(key string) bool {
	normalized := strings.ToLower(strings.NewReplacer("-", "", "_", "", " ", "").Replace(key))
	switch normalized {
	case "accesstoken", "authorization", "clientsecret", "commentid", "effectid", "fieldvalues",
		"id", "musicid", "playlistid", "query", "searchid", "secret", "token", "username", "videoid":
		return true
	default:
		return false
	}
}

func redactText(value string, sensitiveValues ...string) string {
	for _, sensitive := range sensitiveValues {
		if sensitive != "" {
			value = strings.ReplaceAll(value, sensitive, "[REDACTED]")
		}
	}
	lower := strings.ToLower(value)
	for _, marker := range []string{
		"access_token", "authorization", "bearer", "client_secret", "comment_id", "effect_id",
		"field_values", "music_id", "playlist_id", "search_id", "username", "video_id",
	} {
		if strings.Contains(lower, marker) {
			return "TikTok rejected the request; provider message was redacted"
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
