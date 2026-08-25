package yandexdirect

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

var ErrOutcomeUnknown = errors.New("yandexdirect: mutation outcome unknown")
var ErrPartialMutation = errors.New("yandexdirect: partial mutation")

type apiError struct {
	RequestID string `json:"request_id"`
	Code      int    `json:"error_code"`
	Message   string `json:"error_string"`
	Detail    string `json:"error_detail"`
}

type errorEnvelope struct {
	Error *apiError `json:"error"`
}

// APIError augments the platform-neutral error with Yandex response metadata.
// Callers can use errors.As to inspect point balances even on failed requests.
type APIError struct {
	Hub      *socialhub.Error
	Metadata ResponseMetadata
}

func (err *APIError) Error() string {
	if err == nil || err.Hub == nil {
		return "socialhub: yandex: platform_error"
	}
	return err.Hub.Error()
}

func (err *APIError) Unwrap() error {
	if err == nil {
		return nil
	}
	return err.Hub
}

func (err *APIError) Retryable() bool {
	return err != nil && err.Hub != nil && err.Hub.Retryable()
}

func newHTTPErrorDecoder(clock socialhub.Clock, requestIDValues ...string) func(int, http.Header, []byte) error {
	return func(status int, header http.Header, body []byte) error {
		return decodeHTTPError(status, header, body, clock.Now(), requestIDValues...)
	}
}

func decodeHTTPError(status int, header http.Header, body []byte, now time.Time, requestIDValues ...string) error {
	var envelope errorEnvelope
	_ = json.Unmarshal(body, &envelope)
	if envelope.Error == nil {
		envelope.Error = &apiError{}
	}
	return apiErrorValue("", status, header, *envelope.Error, now, requestIDValues...)
}

func apiErrorValue(operation string, status int, header http.Header, provider apiError, now time.Time, requestIDValues ...string) error {
	code, class := classifyError(status, provider.Code)
	requestID := responseRequestID(requestIDValues, provider.RequestID, header.Get("RequestId"))
	metadata := responseMetadata(header, requestIDValues...)
	metadata.RequestID = requestID
	return &APIError{Hub: &socialhub.Error{
		Code: code, Class: class, Platform: platformName, Product: productName, Op: operation,
		HTTPStatus: status, PlatformCode: providerCode(provider.Code),
		PlatformMessage: providerMessage(status, provider.Detail, provider.Message),
		RequestID:       metadata.RequestID, RetryAfter: retryDelay(header, now),
	}, Metadata: metadata}
}

func notificationError(operation string, status int, notification Notification, metadata ResponseMetadata) error {
	code, class := classifyError(status, notification.Code)
	return &APIError{Hub: &socialhub.Error{
		Code: code, Class: class, Platform: platformName, Product: productName, Op: operation,
		HTTPStatus: status, PlatformCode: providerCode(notification.Code),
		PlatformMessage: providerMessage(status, notification.Details, notification.Message),
		RequestID:       metadata.RequestID,
	}, Metadata: metadata}
}

func classifyError(status, providerCode int) (socialhub.ErrorCode, socialhub.ErrorClass) {
	switch providerCode {
	case 53:
		return socialhub.CodeUnauthenticated, socialhub.ClassUserAction
	case 58, 513, 3000:
		return socialhub.CodeApprovalRequired, socialhub.ClassUserAction
	case 54, 3001:
		return socialhub.CodePermissionDenied, socialhub.ClassUserAction
	case 152, 506:
		return socialhub.CodeRateLimited, socialhub.ClassRetryable
	case 52, 1000, 1001, 1002, 1003, 1004, 1020:
		return socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable
	case 3500, 3600:
		return socialhub.CodeUnsupported, socialhub.ClassPermanent
	case 6000, 9601, 9801:
		return socialhub.CodeConflict, socialhub.ClassPermanent
	case 7001:
		return socialhub.CodeConflict, socialhub.ClassUserAction
	case 8000:
		return socialhub.CodeInvalidArgument, socialhub.ClassPermanent
	case 8800:
		return socialhub.CodeNotFound, socialhub.ClassPermanent
	}
	if providerCode >= 4000 && providerCode < 6000 || providerCode == 6001 || providerCode == 7000 || providerCode == 8312 ||
		providerCode == 9300 || providerCode == 9301 || providerCode == 9600 || providerCode == 9800 || providerCode == 9802 {
		return socialhub.CodeInvalidArgument, socialhub.ClassPermanent
	}
	switch status {
	case http.StatusBadRequest, http.StatusUnprocessableEntity, http.StatusRequestEntityTooLarge, http.StatusUnsupportedMediaType:
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

func platformError(operation string, code socialhub.ErrorCode, class socialhub.ErrorClass, cause error) error {
	return &socialhub.Error{
		Code: code, Class: class, Platform: platformName, Product: productName,
		Op: operation, Cause: sanitizeCause(cause),
	}
}

func authenticationError(operation, message string, cause error, credentials ...string) error {
	var sanitized error
	if clean := sanitizeCause(cause); clean != nil {
		causeMessage := clean.Error()
		for _, credential := range credentials {
			causeMessage = redactExact(causeMessage, credential)
		}
		causeMessage = boundedSingleLine(redactSensitive(causeMessage), 1024)
		if causeMessage != "" {
			sanitized = errors.New(causeMessage)
		}
	}
	return &socialhub.Error{
		Code: socialhub.CodeUnauthenticated, Class: socialhub.ClassUserAction,
		Platform: platformName, Product: productName, Op: operation,
		PlatformMessage: boundedText(message, 512), Cause: sanitized,
	}
}

func invalidArgument(operation, message string) error {
	return &socialhub.Error{
		Code: socialhub.CodeInvalidArgument, Class: socialhub.ClassPermanent,
		Platform: platformName, Product: productName, Op: operation, PlatformMessage: message,
	}
}

func notFound(operation, message string) error {
	return &socialhub.Error{
		Code: socialhub.CodeNotFound, Class: socialhub.ClassPermanent,
		Platform: platformName, Product: productName, Op: operation, PlatformMessage: message,
	}
}

func platformContractError(operation, message string) error {
	return &socialhub.Error{
		Code: socialhub.CodePlatformError, Class: socialhub.ClassPermanent,
		Platform: platformName, Product: productName, Op: operation, PlatformMessage: boundedText(message, 512),
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

func withRequestMetadata(err error, operation string, metadata ResponseMetadata) error {
	if err == nil {
		return nil
	}
	var hub *socialhub.Error
	if errors.As(err, &hub) {
		hub.Op = operation
		if hub.RequestID == "" {
			hub.RequestID = metadata.RequestID
		}
	}
	var api *APIError
	if errors.As(err, &api) {
		api.Metadata = mergeResponseMetadata(api.Metadata, metadata)
	}
	return err
}

func outcomeUnknownError(operation string, cause error, metadata ResponseMetadata) error {
	requestID := metadata.RequestID
	var hub *socialhub.Error
	if requestID == "" && errors.As(cause, &hub) {
		requestID = hub.RequestID
	}
	metadata.RequestID = requestID
	return &APIError{Hub: &socialhub.Error{
		Code: socialhub.CodeConflict, Class: socialhub.ClassUserAction,
		Platform: platformName, Product: productName, Op: operation,
		PlatformMessage: "Yandex mutation outcome is unknown; reconcile advertiser state before retrying",
		RequestID:       requestID, Cause: errors.Join(ErrOutcomeUnknown, sanitizeCause(cause)),
	}, Metadata: metadata}
}

func partialMutationError(operation string, metadata ResponseMetadata, cause error) error {
	return &APIError{Hub: &socialhub.Error{
		Code: socialhub.CodeConflict, Class: socialhub.ClassUserAction,
		Platform: platformName, Product: productName, Op: operation,
		PlatformMessage: "Yandex applied only part of the batch; reconcile per-item results before retrying",
		RequestID:       metadata.RequestID, Cause: errors.Join(ErrPartialMutation, sanitizeCause(cause)),
	}, Metadata: metadata}
}

func mergeResponseMetadata(primary, fallback ResponseMetadata) ResponseMetadata {
	if primary.RequestID == "" {
		primary.RequestID = fallback.RequestID
	}
	if primary.Units == nil {
		primary.Units = fallback.Units
	}
	if primary.UnitsUsedLogin == "" {
		primary.UnitsUsedLogin = fallback.UnitsUsedLogin
	}
	return primary
}

func ambiguousMutationError(err error) bool {
	var hub *socialhub.Error
	if !errors.As(err, &hub) {
		return true
	}
	return hub.HTTPStatus == 0 || hub.HTTPStatus == http.StatusRequestTimeout || hub.HTTPStatus >= 500 ||
		hub.HTTPStatus >= 200 && hub.HTTPStatus < 300
}

func batchResultError(operation string, result BatchResult) error {
	failures := 0
	var first error
	for _, item := range result.Items {
		if len(item.Errors) == 0 {
			continue
		}
		failures++
		if first == nil {
			first = notificationError(operation, http.StatusOK, item.Errors[0], result.Metadata)
		}
	}
	if failures == 0 {
		return nil
	}
	if failures < len(result.Items) {
		return partialMutationError(operation, result.Metadata, first)
	}
	return first
}

func retryDelay(header http.Header, now time.Time) time.Duration {
	value := boundedRetryHeader(header.Get("Retry-After"))
	if seconds, err := strconv.ParseInt(value, 10, 64); err == nil && seconds >= 0 && seconds <= 86_400 {
		return time.Duration(seconds) * time.Second
	}
	if when, err := http.ParseTime(value); err == nil {
		delay := when.Sub(now)
		if delay >= 0 && delay <= 24*time.Hour {
			return delay
		}
	}
	value = boundedRetryHeader(header.Get("retryIn"))
	if seconds, err := strconv.ParseInt(value, 10, 64); err == nil && seconds >= 0 && seconds <= 86_400 {
		return time.Duration(seconds) * time.Second
	}
	return 0
}

func boundedRetryHeader(value string) string {
	if value == "" || len(value) > 128 || !utf8.ValidString(value) || strings.ContainsFunc(value, unicode.IsControl) {
		return ""
	}
	return strings.TrimSpace(value)
}

func providerCode(value int) string {
	if value == 0 {
		return ""
	}
	return strconv.Itoa(value)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func boundedText(value string, maximum int) string {
	if maximum <= 0 || !utf8.ValidString(value) {
		return ""
	}
	if utf8.RuneCountInString(value) <= maximum {
		return value
	}
	return string([]rune(value)[:maximum])
}

func boundedOpaque(value string, maximum int) string {
	value = strings.TrimSpace(value)
	if !validOpaque(value, maximum) {
		return ""
	}
	return value
}

func firstBoundedOpaque(maximum int, values ...string) string {
	for _, value := range values {
		if value := boundedOpaque(value, maximum); value != "" {
			return value
		}
	}
	return ""
}

func responseRequestID(blockedValues []string, values ...string) string {
	value := firstBoundedOpaque(256, values...)
	for _, blocked := range blockedValues {
		if blocked != "" && strings.Contains(value, blocked) {
			return ""
		}
	}
	return value
}

func boundedLogin(value string) string {
	value = boundedOpaque(value, 255)
	if value != "" && !validLogin(value) {
		return ""
	}
	return value
}

func boundedSingleLine(value string, maximum int) string {
	value = strings.TrimSpace(value)
	if strings.ContainsFunc(value, unicode.IsControl) {
		return ""
	}
	return boundedText(value, maximum)
}

func providerMessage(status int, values ...string) string {
	message := boundedSingleLine(redactSensitive(firstNonEmpty(values...)), 512)
	if message != "" {
		return message
	}
	if status >= 400 {
		if message := http.StatusText(status); message != "" {
			return message
		}
	}
	return "Yandex Direct request failed"
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

func redactExact(value, credential string) string {
	if credential == "" {
		return value
	}
	return strings.ReplaceAll(value, credential, "[REDACTED]")
}

func redactSensitive(value string) string {
	markers := []string{"authorization", "oauth", "access_token", "token", "client-login", "client_login"}
	for _, marker := range markers {
		for cursor := 0; cursor < len(value); {
			index := strings.Index(strings.ToLower(value[cursor:]), marker)
			if index < 0 {
				break
			}
			index += cursor
			start := index + len(marker)
			for start < len(value) && strings.ContainsRune(" \t:=\"'", rune(value[start])) {
				start++
			}
			if start == index+len(marker) {
				cursor = start
				continue
			}
			end := start
			for end < len(value) && !strings.ContainsRune("\r\n,;}&\"' \t", rune(value[end])) {
				end++
			}
			value = value[:start] + "[REDACTED]" + value[end:]
			cursor = start + len("[REDACTED]")
		}
	}
	return value
}
