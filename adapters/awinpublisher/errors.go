package awinpublisher

import (
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

type ProviderError struct {
	Error       ExactValue `json:"error"`
	Description string     `json:"description"`
	Message     string     `json:"message"`
}

// APIError augments socialhub.Error with Awin's best-effort error envelope and
// the bounded provider response body.
type APIError struct {
	Hub      *socialhub.Error
	Provider ProviderError
	Raw      []byte
}

func (value *APIError) Error() string {
	if value == nil || value.Hub == nil {
		return "socialhub: awin: platform_error"
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

func newHTTPErrorDecoder(clock socialhub.Clock) func(int, http.Header, []byte) error {
	return func(status int, header http.Header, body []byte) error {
		return decodeHTTPError(status, header, body, clock.Now())
	}
}

func decodeHTTPError(status int, header http.Header, body []byte, now time.Time) error {
	var provider ProviderError
	_ = json.Unmarshal(body, &provider)
	message := firstNonEmpty(provider.Description, provider.Message, http.StatusText(status), "Awin rejected the request")
	platformCode := provider.Error.String()
	if platformCode == "" {
		platformCode = "http_" + strconv.Itoa(status)
	}
	code, class := classifyHTTPError(status)
	hub := &socialhub.Error{
		Code: code, Class: class, Platform: platformName, Product: productName,
		HTTPStatus: status, PlatformCode: boundedMessage(platformCode, 256),
		PlatformMessage: boundedMessage(redactSensitive(message), 1024),
		RequestID:       boundedMessage(firstHeader(header, "X-Request-ID", "X-Correlation-ID"), 256),
		RetryAfter:      parseRetryAfter(header.Get("Retry-After"), now),
	}
	if code == socialhub.CodePermissionDenied || code == socialhub.CodeApprovalRequired {
		hub.ApprovalURL = documentationURL
	}
	provider.Description = boundedMessage(redactSensitive(provider.Description), 1024)
	provider.Message = boundedMessage(redactSensitive(provider.Message), 1024)
	return &APIError{Hub: hub, Provider: provider, Raw: append([]byte(nil), body...)}
}

func trackingLinkBusinessError(description string, raw []byte) error {
	provider := ProviderError{Description: boundedMessage(redactSensitive(description), 1024)}
	return &APIError{
		Hub: &socialhub.Error{
			Code: socialhub.CodeApprovalRequired, Class: socialhub.ClassUserAction,
			Platform: platformName, Product: productName, Op: "generate_tracking_link", HTTPStatus: http.StatusOK,
			PlatformCode: "link_builder_unavailable", PlatformMessage: provider.Description, ApprovalURL: documentationURL,
		},
		Provider: provider,
		Raw:      append([]byte(nil), raw...),
	}
}

func enhancedFeedTerminalError(provider ProviderError, raw []byte) error {
	code, class := socialhub.CodePlatformError, socialhub.ClassPermanent
	if status, err := strconv.Atoi(provider.Error.String()); err == nil && status >= 400 && status <= 599 {
		code, class = classifyHTTPError(status)
	}
	message := firstNonEmpty(provider.Message, provider.Description, "Awin Enhanced Feed ended with an error")
	provider.Message = boundedMessage(redactSensitive(provider.Message), 1024)
	provider.Description = boundedMessage(redactSensitive(provider.Description), 1024)
	return &APIError{
		Hub: &socialhub.Error{
			Code: code, Class: class, Platform: platformName, Product: productName,
			Op: "download_enhanced_feed", HTTPStatus: http.StatusOK,
			PlatformCode:    boundedMessage(provider.Error.String(), 256),
			PlatformMessage: boundedMessage(redactSensitive(message), 1024),
		},
		Provider: provider,
		Raw:      append([]byte(nil), raw...),
	}
}

func classifyHTTPError(status int) (socialhub.ErrorCode, socialhub.ErrorClass) {
	switch status {
	case http.StatusBadRequest, http.StatusMethodNotAllowed, http.StatusRequestEntityTooLarge,
		http.StatusUnsupportedMediaType, http.StatusUnprocessableEntity:
		return socialhub.CodeInvalidArgument, socialhub.ClassPermanent
	case http.StatusUnauthorized:
		return socialhub.CodeUnauthenticated, socialhub.ClassUserAction
	case http.StatusPaymentRequired:
		return socialhub.CodeApprovalRequired, socialhub.ClassUserAction
	case http.StatusForbidden:
		return socialhub.CodePermissionDenied, socialhub.ClassUserAction
	case http.StatusNotFound, http.StatusGone:
		return socialhub.CodeNotFound, socialhub.ClassPermanent
	case http.StatusConflict:
		return socialhub.CodeConflict, socialhub.ClassPermanent
	case http.StatusRequestTimeout, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable
	case http.StatusTooManyRequests:
		return socialhub.CodeRateLimited, socialhub.ClassRetryable
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

func firstHeader(header http.Header, names ...string) string {
	for _, name := range names {
		if value := header.Get(name); value != "" {
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

func boundedMessage(value string, maximum int) string {
	if utf8.RuneCountInString(value) <= maximum {
		return value
	}
	return string([]rune(value)[:maximum])
}

func redactSensitive(value string) string {
	for _, key := range []string{"access_token", "authorization", "bearer", "api_token", "password", "secret"} {
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
