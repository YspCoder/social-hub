package etsy

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
	maxErrorRawBytes = 64 << 10
	maxRetryAfter    = 24 * time.Hour
)

// ErrOutcomeUnknown means an Etsy mutation may have reached the platform and
// must be reconciled before it is retried.
var ErrOutcomeUnknown = errors.New("etsy: mutation outcome is unknown")

type ProviderError struct {
	Code    string
	Message string
}

type APIError struct {
	Hub      *socialhub.Error
	Provider ProviderError
	Raw      []byte
}

func (value *APIError) Error() string {
	if value == nil || value.Hub == nil {
		return "socialhub: etsy: platform_error"
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

func newHTTPErrorDecoder(clock socialhub.Clock, secrets ...string) func(int, http.Header, []byte) error {
	return func(status int, header http.Header, body []byte) error {
		return decodeHTTPError(status, header, body, clock.Now(), secrets...)
	}
}

func decodeHTTPError(status int, header http.Header, body []byte, now time.Time, secrets ...string) error {
	provider := decodeProviderError(body)
	for _, secret := range secrets {
		if secret != "" {
			provider.Code = strings.ReplaceAll(provider.Code, secret, "[REDACTED]")
			provider.Message = strings.ReplaceAll(provider.Message, secret, "[REDACTED]")
		}
	}
	code, class := classifyHTTPError(status, provider.Code)
	message := firstNonEmpty(provider.Message, http.StatusText(status), "Etsy rejected the request")
	platformCode := provider.Code
	if platformCode == "" {
		platformCode = "http_" + strconv.Itoa(status)
	}
	hub := &socialhub.Error{
		Code: code, Class: class, Platform: platformName, Product: productName,
		HTTPStatus: status, PlatformCode: boundedMessage(redactSensitive(platformCode), 256),
		PlatformMessage: boundedMessage(redactSensitive(message), 1024),
		RequestID:       boundedMessage(firstHeader(header, "X-Request-ID", "X-Correlation-ID"), 256),
		RetryAfter:      parseRetryAfter(header.Get("Retry-After"), now),
	}
	if code == socialhub.CodePermissionDenied || code == socialhub.CodeApprovalRequired {
		hub.ApprovalURL = documentationURL
	}
	provider.Code = boundedMessage(redactSensitive(provider.Code), 256)
	provider.Message = boundedMessage(redactSensitive(provider.Message), 1024)
	return &APIError{Hub: hub, Provider: provider, Raw: boundedRedactedRaw(body, secrets...)}
}

func oauthResponseError(operation string, status int, header http.Header, code, message string, body []byte, now time.Time, secrets ...string) error {
	for _, secret := range secrets {
		if secret != "" {
			code = strings.ReplaceAll(code, secret, "[REDACTED]")
			message = strings.ReplaceAll(message, secret, "[REDACTED]")
		}
	}
	errorCode, class := classifyHTTPError(status, code)
	return &APIError{
		Hub: &socialhub.Error{
			Code: errorCode, Class: class, Op: operation, Platform: platformName, Product: productName,
			HTTPStatus: status, PlatformCode: boundedMessage(redactSensitive(code), 256),
			PlatformMessage: boundedMessage(redactSensitive(message), 1024),
			RequestID:       boundedMessage(firstHeader(header, "X-Request-ID", "X-Correlation-ID"), 256),
			RetryAfter:      parseRetryAfter(header.Get("Retry-After"), now),
		},
		Provider: ProviderError{Code: boundedMessage(redactSensitive(code), 256), Message: boundedMessage(redactSensitive(message), 1024)},
		Raw:      boundedRedactedRaw(body, secrets...),
	}
}

func classifyHTTPError(status int, providerCode string) (socialhub.ErrorCode, socialhub.ErrorClass) {
	switch strings.ToLower(strings.TrimSpace(providerCode)) {
	case "invalid_client", "invalid_grant", "invalid_token", "unauthorized_client":
		return socialhub.CodeUnauthenticated, socialhub.ClassUserAction
	case "invalid_scope", "insufficient_scope", "access_denied":
		return socialhub.CodeApprovalRequired, socialhub.ClassUserAction
	case "invalid_request", "unsupported_grant_type", "unsupported_response_type":
		return socialhub.CodeInvalidArgument, socialhub.ClassPermanent
	case "temporarily_unavailable", "server_error":
		return socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable
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

func decodeProviderError(body []byte) ProviderError {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 || trimmed[0] != '{' || !json.Valid(trimmed) {
		return ProviderError{}
	}
	var payload struct {
		Error            string `json:"error"`
		ErrorDescription string `json:"error_description"`
	}
	if json.Unmarshal(trimmed, &payload) != nil {
		return ProviderError{}
	}
	if payload.ErrorDescription != "" || isOAuthErrorCode(payload.Error) {
		return ProviderError{Code: payload.Error, Message: payload.ErrorDescription}
	}
	return ProviderError{Message: payload.Error}
}

func isOAuthErrorCode(value string) bool {
	switch value {
	case "invalid_client", "invalid_grant", "invalid_token", "unauthorized_client", "invalid_scope",
		"insufficient_scope", "access_denied", "invalid_request", "unsupported_grant_type",
		"unsupported_response_type", "temporarily_unavailable", "server_error":
		return true
	default:
		return false
	}
}

func withOperation(err error, operation string) error {
	if err == nil {
		return nil
	}
	var apiError *APIError
	if errors.As(err, &apiError) && apiError.Hub != nil {
		apiError.Hub.Op = operation
		return apiError
	}
	var hubError *socialhub.Error
	if errors.As(err, &hubError) {
		hubError.Op = operation
		hubError.Platform = platformName
		hubError.Product = productName
		return hubError
	}
	return platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, err)
}

func platformError(operation string, code socialhub.ErrorCode, class socialhub.ErrorClass, cause error) error {
	return &socialhub.Error{
		Code: code, Class: class, Op: operation, Platform: platformName, Product: productName,
		Cause: sanitizeCause(cause),
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

func outcomeUnknownError(operation string, cause error, requestID string) error {
	if requestID == "" {
		var hub *socialhub.Error
		if errors.As(cause, &hub) {
			requestID = hub.RequestID
		}
	}
	return &socialhub.Error{
		Code: socialhub.CodeConflict, Class: socialhub.ClassUserAction,
		Op: operation, Platform: platformName, Product: productName,
		PlatformMessage: "Etsy mutation outcome is unknown; reconcile shop state before retrying",
		RequestID:       boundedMessage(redactSensitive(requestID), 256),
		Cause:           errors.Join(ErrOutcomeUnknown, sanitizeCause(cause)),
	}
}

func withMutationOutcome(operation string, mutation bool, requestID string, err error) error {
	if err != nil && mutation && ambiguousMutationError(err) {
		return outcomeUnknownError(operation, err, requestID)
	}
	return err
}

func ambiguousMutationError(err error) bool {
	var hub *socialhub.Error
	if !errors.As(err, &hub) {
		return true
	}
	return hub.HTTPStatus == 0 || hub.HTTPStatus == http.StatusRequestTimeout || hub.HTTPStatus >= 500 ||
		hub.HTTPStatus >= 200 && hub.HTTPStatus < 300
}

func approvalRequired(operation, scope string) error {
	result := &socialhub.Error{
		Code: socialhub.CodeApprovalRequired, Class: socialhub.ClassUserAction,
		Op: operation, Platform: platformName, Product: productName,
		PlatformMessage: "an Etsy OAuth token is required", ApprovalURL: documentationURL,
	}
	if scope != "" {
		result.RequiredScopes = []string{scope}
	}
	return result
}

func firstHeader(header http.Header, names ...string) string {
	for _, name := range names {
		if value := strings.TrimSpace(header.Get(name)); value != "" {
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

func parseRetryAfter(value string, now time.Time) time.Duration {
	value = strings.TrimSpace(value)
	if value != "" && strings.IndexFunc(value, func(character rune) bool {
		return character < '0' || character > '9'
	}) == -1 {
		seconds, err := strconv.ParseUint(value, 10, 64)
		if err != nil || seconds > uint64(maxRetryAfter/time.Second) {
			return maxRetryAfter
		}
		return time.Duration(seconds) * time.Second
	}
	if at, err := http.ParseTime(value); err == nil && at.After(now) {
		delay := at.Sub(now)
		if delay > maxRetryAfter {
			return maxRetryAfter
		}
		return delay
	}
	return 0
}

func boundedMessage(value string, maximum int) string {
	if utf8.RuneCountInString(value) <= maximum {
		return value
	}
	return string([]rune(value)[:maximum])
}

func boundedRedactedRaw(value []byte, secrets ...string) []byte {
	text := string(value)
	for _, secret := range secrets {
		if secret != "" {
			text = strings.ReplaceAll(text, secret, "[REDACTED]")
		}
	}
	redacted := []byte(redactSensitive(text))
	if len(redacted) > maxErrorRawBytes {
		redacted = redacted[:maxErrorRawBytes]
	}
	return append([]byte(nil), redacted...)
}

func redactSensitive(value string) string {
	for _, key := range []string{"access_token", "authorization", "client_secret", "refresh_token", "shared_secret", "x-api-key"} {
		cursor := 0
		for cursor < len(value) {
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
			for valueEnd < len(value) && !strings.ContainsRune(" \t\r\n,;&}\"'", rune(value[valueEnd])) {
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
