package shopeeads

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

type errorEnvelope struct {
	Error     string `json:"error"`
	Message   string `json:"message"`
	RequestID string `json:"request_id"`
}

func newHTTPErrorDecoder(clock socialhub.Clock) func(int, http.Header, []byte) error {
	return func(status int, header http.Header, body []byte) error {
		return decodeHTTPError(status, header, body, clock.Now())
	}
}

func decodeHTTPError(status int, header http.Header, body []byte, now time.Time, sensitiveValues ...string) error {
	var envelope errorEnvelope
	_ = json.Unmarshal(body, &envelope)
	return apiErrorValue("http", status, header, envelope.Error, envelope.Message, envelope.RequestID, now, sensitiveValues...)
}

func apiErrorValue(operation string, status int, header http.Header, platformCode, message, requestID string, now time.Time, sensitiveValues ...string) error {
	platformCode = redactExact(redactSensitive(platformCode), sensitiveValues)
	message = redactExact(redactSensitive(message), sensitiveValues)
	code, class := classifyError(status, platformCode)
	result := &socialhub.Error{
		Code: code, Class: class, Platform: platformName, Product: productName, Op: operation,
		HTTPStatus: status, PlatformCode: boundedOpaque(platformCode, 128),
		PlatformMessage: boundedText(message, 512),
		RequestID:       safeRequestID(firstNonEmpty(requestID, header.Get("X-Shopee-Request-ID")), sensitiveValues),
		RetryAfter:      retryDelay(header, now),
	}
	if code == socialhub.CodeApprovalRequired {
		result.ApprovalURL = documentationURL
	}
	return result
}

func classifyError(status int, platformCode string) (socialhub.ErrorCode, socialhub.ErrorClass) {
	code := strings.ToLower(strings.TrimSpace(platformCode))
	switch {
	case strings.Contains(code, "rate_limit") || code == "error_limit":
		return socialhub.CodeRateLimited, socialhub.ClassRetryable
	case code == "error_server" || code == "error_network":
		return socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable
	case code == "error_auth" || code == "error_sign" || code == "invalid_code" ||
		code == "invalid_acceess_token" || code == "invalid_access_token" ||
		code == "error_partner_key_expired" || strings.Contains(code, "access_expired") ||
		strings.Contains(code, "refresh_token"):
		return socialhub.CodeUnauthenticated, socialhub.ClassUserAction
	case code == "error_api_permission" || code == "error_ashop_api_permission" || code == "error_kyc_auth":
		return socialhub.CodeApprovalRequired, socialhub.ClassUserAction
	case code == "error_api_call_restricted" || code == "source_ip_undeclared" ||
		code == "shop_no_linked" || code == "partner_shop_no_link" ||
		code == "merchant_no_linked" || code == "supplier_no_linked" || code == "shop_banned":
		return socialhub.CodePermissionDenied, socialhub.ClassUserAction
	case code == "error_not_found" || code == "ads.shop.campaign_not_match":
		return socialhub.CodeNotFound, socialhub.ClassPermanent
	case code == "error_param" || strings.HasPrefix(code, "invalid_") ||
		code == "error_shop" || code == "ads.api.http_method_not_allowed":
		return socialhub.CodeInvalidArgument, socialhub.ClassPermanent
	case code == "api_suspended":
		return socialhub.CodeUnsupported, socialhub.ClassPermanent
	}
	switch status {
	case http.StatusBadRequest, http.StatusMethodNotAllowed, http.StatusUnprocessableEntity,
		http.StatusRequestEntityTooLarge, http.StatusUnsupportedMediaType:
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
	case http.StatusRequestTimeout:
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

func invalidArgument(operation, message string) error {
	return &socialhub.Error{
		Code: socialhub.CodeInvalidArgument, Class: socialhub.ClassPermanent,
		Platform: platformName, Product: productName, Op: operation, PlatformMessage: boundedText(message, 512),
	}
}

func platformContractError(operation, message string) error {
	return &socialhub.Error{
		Code: socialhub.CodePlatformError, Class: socialhub.ClassPermanent,
		Platform: platformName, Product: productName, Op: operation, PlatformMessage: boundedText(message, 512),
	}
}

func credentialError(operation string) error {
	return &socialhub.Error{
		Code: socialhub.CodeUnauthenticated, Class: socialhub.ClassUserAction,
		Platform: platformName, Product: productName, Op: operation,
		PlatformMessage: "credential resolution failed",
	}
}

func dependencyError(operation, message string) error {
	return &socialhub.Error{
		Code: socialhub.CodeTemporarilyUnavailable, Class: socialhub.ClassRetryable,
		Platform: platformName, Product: productName, Op: operation,
		PlatformMessage: boundedText(message, 512),
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

func retryDelay(header http.Header, now time.Time) time.Duration {
	value := header.Get("Retry-After")
	if !validOpaque(value, 128) {
		return 0
	}
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

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func boundedText(value string, maximum int) string {
	if !utf8.ValidString(value) {
		return ""
	}
	value = strings.Map(func(character rune) rune {
		if unicode.IsControl(character) {
			return -1
		}
		return character
	}, value)
	if utf8.RuneCountInString(value) <= maximum {
		return value
	}
	return string([]rune(value)[:maximum])
}

func boundedOpaque(value string, maximum int) string {
	value = boundedText(strings.TrimSpace(value), maximum)
	if !validOpaque(value, maximum) {
		return ""
	}
	return value
}

func safeRequestID(value string, sensitiveValues []string) string {
	if !validOpaque(value, 256) || containsSensitiveMarker(value) {
		return ""
	}
	for _, sensitive := range sensitiveValues {
		if sensitive != "" && strings.Contains(value, sensitive) {
			return ""
		}
	}
	return value
}

func containsSensitiveMarker(value string) bool {
	lower := strings.ToLower(value)
	for _, marker := range []string{"access_token", "refresh_token", "partner_key", "authorization", "bearer", "sign"} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

func sanitizeCause(err error) error {
	var urlError *url.Error
	if errors.As(err, &urlError) && urlError.Err != nil {
		return urlError.Err
	}
	return err
}

func redactSensitive(value string) string {
	markers := []string{"access_token", "refresh_token", "partner_key", "secret", "authorization", "bearer", "sign"}
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

func redactExact(value string, sensitiveValues []string) string {
	for _, sensitive := range sensitiveValues {
		if sensitive != "" {
			value = strings.ReplaceAll(value, sensitive, "[REDACTED]")
		}
	}
	return value
}
