package mercadoads

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

type PlatformProblem struct {
	Code        string
	Message     string
	Description string
	BlockedBy   string
	Causes      []string
}

type APIError struct {
	Hub     *socialhub.Error
	Problem PlatformProblem
}

func (err *APIError) Error() string {
	if err == nil || err.Hub == nil {
		return "socialhub: mercadolibre: platform_error"
	}
	return err.Hub.Error()
}

func (err *APIError) Unwrap() error {
	if err == nil {
		return nil
	}
	return err.Hub
}

func (err *APIError) Retryable() bool { return err != nil && err.Hub != nil && err.Hub.Retryable() }

type errorWire struct {
	Message          string            `json:"message"`
	Error            string            `json:"error"`
	Code             string            `json:"code"`
	ErrorDescription string            `json:"error_description"`
	BlockedBy        string            `json:"blocked_by"`
	Status           int               `json:"status"`
	Cause            []json.RawMessage `json:"cause"`
}

func newHTTPErrorDecoder(clock socialhub.Clock) func(int, http.Header, []byte) error {
	return func(status int, header http.Header, body []byte) error {
		return decodeHTTPError(status, header, body, clock.Now())
	}
}

func decodeHTTPError(status int, header http.Header, body []byte, now time.Time, sensitiveValues ...string) error {
	var wire errorWire
	_ = json.Unmarshal(body, &wire)
	problem := PlatformProblem{
		Code:        boundedOpaque(redactExact(redactSensitive(firstNonEmpty(wire.Code, wire.Error)), sensitiveValues), 128),
		Message:     boundedText(redactExact(redactSensitive(wire.Message), sensitiveValues), 512),
		Description: boundedText(redactExact(redactSensitive(wire.ErrorDescription), sensitiveValues), 512),
		BlockedBy:   boundedOpaque(redactExact(redactSensitive(wire.BlockedBy), sensitiveValues), 128),
	}
	for _, raw := range wire.Cause {
		if len(problem.Causes) == 32 {
			break
		}
		var value any
		if json.Unmarshal(raw, &value) != nil {
			continue
		}
		encoded, err := json.Marshal(value)
		if err == nil {
			if text := boundedText(redactExact(redactSensitive(string(encoded)), sensitiveValues), 512); text != "" {
				problem.Causes = append(problem.Causes, text)
			}
		}
	}
	message := firstNonEmpty(problem.Description, problem.Message)
	code, class := classifyError(status, problem.Code, message)
	platformCode := problem.Code
	if platformCode == "" {
		platformCode = strconv.Itoa(status)
	}
	hub := &socialhub.Error{
		Code: code, Class: class, Platform: platformName, Product: productName,
		HTTPStatus: status, PlatformCode: platformCode, PlatformMessage: message,
		RequestID:  safeRequestID(header.Get("X-Request-Id"), sensitiveValues),
		RetryAfter: retryDelay(header, now),
	}
	if code == socialhub.CodeApprovalRequired {
		hub.ApprovalURL = documentationURL
	}
	return &APIError{Hub: hub, Problem: problem}
}

func classifyError(status int, platformCode, message string) (socialhub.ErrorCode, socialhub.ErrorClass) {
	value := strings.ToLower(strings.TrimSpace(platformCode + " " + message))
	switch {
	case strings.Contains(value, "local_rate_limited") || strings.Contains(value, "too_many_requests") || status == http.StatusTooManyRequests:
		return socialhub.CodeRateLimited, socialhub.ClassRetryable
	case strings.Contains(value, "invalid_grant") || strings.Contains(value, "invalid_client") ||
		strings.Contains(value, "invalid_token") || strings.Contains(value, "pa_unauthorized"):
		return socialhub.CodeUnauthenticated, socialhub.ClassUserAction
	case status == http.StatusNotFound && strings.Contains(value, "permission"):
		return socialhub.CodeApprovalRequired, socialhub.ClassUserAction
	}
	switch status {
	case http.StatusBadRequest, http.StatusMethodNotAllowed, http.StatusRequestEntityTooLarge,
		http.StatusUnsupportedMediaType, http.StatusUnprocessableEntity:
		return socialhub.CodeInvalidArgument, socialhub.ClassPermanent
	case http.StatusUnauthorized:
		return socialhub.CodeUnauthenticated, socialhub.ClassUserAction
	case http.StatusForbidden:
		return socialhub.CodePermissionDenied, socialhub.ClassUserAction
	case http.StatusNotFound:
		return socialhub.CodeNotFound, socialhub.ClassPermanent
	case http.StatusConflict:
		return socialhub.CodeConflict, socialhub.ClassPermanent
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
		Platform: platformName, Product: productName, Op: operation,
		PlatformMessage: boundedText(message, 512),
	}
}

func platformContractError(operation, message string) error {
	return &socialhub.Error{
		Code: socialhub.CodePlatformError, Class: socialhub.ClassPermanent,
		Platform: platformName, Product: productName, Op: operation,
		PlatformMessage: boundedText(message, 512),
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
	for _, marker := range []string{"access_token", "refresh_token", "client_secret", "authorization", "code_verifier", "bearer"} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
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

func redactSensitive(value string) string {
	markers := []string{"access_token", "refresh_token", "client_secret", "authorization", "code_verifier", "bearer"}
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
