package xiaomiglobalreporting

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

func newHTTPErrorDecoder(clock socialhub.Clock, secrets ...string) func(int, http.Header, []byte) error {
	return func(status int, header http.Header, body []byte) error {
		return decodeHTTPError(status, header, body, clock.Now(), secrets...)
	}
}

func decodeHTTPError(status int, header http.Header, body []byte, now time.Time, secrets ...string) error {
	var envelope apiEnvelope
	_ = json.Unmarshal(body, &envelope)
	platformCode := scalarCode(envelope.Code)
	code, class := classifyHTTPError(status)
	if platformCode != "" && platformCode != "0" {
		businessCode, businessClass := classifyBusinessCode(platformCode)
		if businessCode != socialhub.CodePlatformError {
			code, class = businessCode, businessClass
		}
	}
	requestID := redactKnown(firstNonEmpty(envelope.TraceID, firstHeader(header, "X-Request-ID", "X-Correlation-ID")), secrets...)
	hub := &socialhub.Error{
		Code: code, Class: class, Platform: platformName, Product: productName,
		HTTPStatus: status, PlatformCode: boundedText(platformCode, 128),
		PlatformMessage: boundedText(redactKnown(envelope.Message, secrets...), 512),
		RequestID:       boundedOpaque(requestID, 256),
		RetryAfter:      parseRetryAfter(header.Get("Retry-After"), now),
	}
	if platformCode == "10003" {
		hub.ApprovalURL = documentationURL
	}
	return hub
}

func businessError(
	operation string,
	status int,
	header http.Header,
	platformCode string,
	message string,
	traceID string,
	requestUID string,
	now time.Time,
	secrets ...string,
) error {
	code, class := classifyBusinessCode(platformCode)
	requestID := firstNonEmpty(traceID, requestUID, firstHeader(header, "X-Request-ID", "X-Correlation-ID"))
	hub := &socialhub.Error{
		Code: code, Class: class, Platform: platformName, Product: productName, Op: operation,
		HTTPStatus: status, PlatformCode: boundedText(platformCode, 128),
		PlatformMessage: boundedText(redactKnown(message, secrets...), 512),
		RequestID:       boundedOpaque(redactKnown(requestID, secrets...), 256),
		RetryAfter:      parseRetryAfter(header.Get("Retry-After"), now),
	}
	if platformCode == "10003" {
		hub.ApprovalURL = documentationURL
	}
	return hub
}

func classifyBusinessCode(code string) (socialhub.ErrorCode, socialhub.ErrorClass) {
	switch code {
	case "10001", "10002":
		return socialhub.CodeUnauthenticated, socialhub.ClassUserAction
	case "10003":
		return socialhub.CodeApprovalRequired, socialhub.ClassUserAction
	case "10004":
		return socialhub.CodePermissionDenied, socialhub.ClassUserAction
	case "20001":
		return socialhub.CodeInvalidArgument, socialhub.ClassPermanent
	case "20002":
		return socialhub.CodeRateLimited, socialhub.ClassRetryable
	case "100500":
		return socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable
	default:
		return socialhub.CodePlatformError, socialhub.ClassPermanent
	}
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

func credentialError(operation string, code socialhub.ErrorCode, class socialhub.ErrorClass, cause error, secrets ...string) error {
	return &socialhub.Error{
		Code: code, Class: class, Platform: platformName, Product: productName,
		Op: operation, Cause: sanitizeCredentialCause(cause, secrets...),
	}
}

func invalidArgument(operation, message string) error {
	return &socialhub.Error{
		Code: socialhub.CodeInvalidArgument, Class: socialhub.ClassPermanent,
		Platform: platformName, Product: productName, Op: operation, PlatformMessage: message,
	}
}

func platformContractError(operation, message string, statuses ...int) error {
	hub := &socialhub.Error{
		Code: socialhub.CodePlatformError, Class: socialhub.ClassPermanent,
		Platform: platformName, Product: productName, Op: operation, PlatformMessage: boundedText(message, 512),
	}
	if len(statuses) != 0 {
		hub.HTTPStatus = statuses[0]
	}
	return hub
}

func withOperationAndRequestID(err error, operation, requestUID string, secrets ...string) error {
	if err == nil {
		return nil
	}
	var hub *socialhub.Error
	if errors.As(err, &hub) {
		hub.Op = operation
		if hub.Cause != nil && len(secrets) != 0 {
			hub.Cause = sanitizeCredentialCause(hub.Cause, secrets...)
		}
		if hub.RequestID == "" {
			hub.RequestID = boundedOpaque(redactKnown(requestUID, secrets...), 256)
		}
	}
	return err
}

func scalarCode(raw json.RawMessage) string {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" || len(trimmed) > 130 {
		return ""
	}
	if strings.HasPrefix(trimmed, "\"") {
		var value string
		if json.Unmarshal(raw, &value) == nil && validOpaque(value, 128) {
			return value
		}
		return ""
	}
	var number json.Number
	if json.Unmarshal(raw, &number) == nil {
		return number.String()
	}
	return ""
}

func decodeNonnegativeInt64(raw json.RawMessage, maximum int64) (int64, error) {
	value := scalarCode(raw)
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed < 0 || parsed > maximum {
		return 0, errInvalidReportResult
	}
	return parsed, nil
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

func boundedText(value string, maximum int) string {
	if maximum <= 0 || !utf8.ValidString(value) || strings.ContainsFunc(value, unicode.IsControl) {
		return ""
	}
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

func sanitizeCause(err error) error {
	var urlError *url.Error
	if errors.As(err, &urlError) && urlError.Err != nil {
		return urlError.Err
	}
	return err
}

func sanitizeCredentialCause(cause error, secrets ...string) error {
	if cause == nil {
		return nil
	}
	message := redactKnown(sanitizeCause(cause).Error(), secrets...)
	if message == "" || len(message) > 1024 || !utf8.ValidString(message) || strings.ContainsFunc(message, unicode.IsControl) {
		message = "credential operation failed"
	}
	return errors.New(message)
}

func redactKnown(value string, secrets ...string) string {
	ordered := append([]string(nil), secrets...)
	sort.SliceStable(ordered, func(left, right int) bool { return len(ordered[left]) > len(ordered[right]) })
	for _, secret := range ordered {
		if secret != "" {
			value = strings.ReplaceAll(value, secret, "[REDACTED]")
		}
	}
	return redactSensitive(value)
}

func redactSensitive(value string) string {
	markers := []string{
		"authorization", "access_token", "access-token", "refresh_token", "refresh-token",
		"appkey", "app_key", "api key", "bearer", "password", "secret", "credential", "token",
	}
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
