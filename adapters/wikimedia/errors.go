package wikimedia

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

const maxErrorRawBytes = 64 << 10

type ProviderError struct {
	Code         string
	Message      string
	HTTPCode     int
	HTTPReason   string
	Translations map[string]string
}

type APIError struct {
	Hub      *socialhub.Error
	Provider ProviderError
	Raw      []byte
}

func (value *APIError) Error() string {
	if value == nil || value.Hub == nil {
		return "socialhub: wikimedia: platform_error"
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

func decodeHTTPError(status int, header http.Header, body []byte, now time.Time) error {
	provider := decodeProviderError(body)
	code, class := classifyHTTPError(status)
	platformCode := firstNonEmpty(provider.Code, "http_"+strconv.Itoa(status))
	message := firstNonEmpty(provider.Message, provider.Translations["en"], provider.HTTPReason, http.StatusText(status))
	retryAfter := parseRetryAfter(header.Get("Retry-After"), now)
	if retryAfter == 0 && (status == http.StatusTooManyRequests || status == http.StatusServiceUnavailable) {
		retryAfter = 5 * time.Second
	}
	hub := &socialhub.Error{
		Code: code, Class: class, Platform: platformName, Product: productName,
		HTTPStatus: status, PlatformCode: boundedMessage(platformCode, 256),
		PlatformMessage: boundedMessage(message, 1024),
		RequestID:       boundedMessage(firstNonEmpty(header.Get("X-Request-ID"), header.Get("X-Correlation-ID")), 256),
		RetryAfter:      retryAfter,
	}
	return &APIError{Hub: hub, Provider: provider, Raw: boundedRaw(body)}
}

func classifyHTTPError(status int) (socialhub.ErrorCode, socialhub.ErrorClass) {
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
		Error               string            `json:"error"`
		ErrorKey            string            `json:"errorKey"`
		FailureCode         string            `json:"failureCode"`
		HTTPCode            int               `json:"httpCode"`
		HTTPReason          string            `json:"httpReason"`
		HTTPMessage         string            `json:"httpMessage"`
		Message             string            `json:"message"`
		MessageTranslations map[string]string `json:"messageTranslations"`
	}
	if json.Unmarshal(trimmed, &payload) != nil {
		return ProviderError{}
	}
	return ProviderError{
		Code:    firstNonEmpty(payload.ErrorKey, payload.Error, payload.FailureCode),
		Message: firstNonEmpty(payload.Message, payload.HTTPMessage), HTTPCode: payload.HTTPCode,
		HTTPReason:   firstNonEmpty(payload.HTTPReason, payload.HTTPMessage),
		Translations: payload.MessageTranslations,
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

func parseRetryAfter(value string, now time.Time) time.Duration {
	value = strings.TrimSpace(value)
	if seconds, err := strconv.ParseInt(value, 10, 64); err == nil && seconds >= 0 && seconds <= int64((24*time.Hour)/time.Second) {
		return time.Duration(seconds) * time.Second
	}
	if at, err := http.ParseTime(value); err == nil && at.After(now) {
		return at.Sub(now)
	}
	return 0
}

func boundedMessage(value string, maximum int) string {
	if utf8.RuneCountInString(value) <= maximum {
		return value
	}
	return string([]rune(value)[:maximum])
}

func boundedRaw(value []byte) []byte {
	if len(value) > maxErrorRawBytes {
		value = value[:maxErrorRawBytes]
	}
	return append([]byte(nil), value...)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func sanitizeCause(err error) error {
	var urlError *url.Error
	if errors.As(err, &urlError) && urlError.Err != nil {
		return urlError.Err
	}
	return err
}
