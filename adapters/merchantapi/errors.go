package merchantapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

var (
	ErrOutcomeUnknown       = errors.New("merchantapi: mutation outcome is unknown")
	errCredentialResolution = errors.New("merchantapi: credential resolution failed")
)

type requestIDFilter struct {
	mu      sync.RWMutex
	blocked map[string]struct{}
	closed  bool
}

func newRequestIDFilter(values ...string) *requestIDFilter {
	filter := &requestIDFilter{blocked: make(map[string]struct{}, len(values))}
	filter.add(values...)
	return filter
}

func (filter *requestIDFilter) add(values ...string) {
	if filter == nil {
		return
	}
	filter.mu.Lock()
	defer filter.mu.Unlock()
	if filter.closed {
		return
	}
	for _, value := range values {
		if value != "" {
			filter.blocked[value] = struct{}{}
		}
	}
}

func (filter *requestIDFilter) with(values ...string) *requestIDFilter {
	result := newRequestIDFilter(values...)
	if filter == nil {
		return result
	}
	filter.mu.RLock()
	defer filter.mu.RUnlock()
	if filter.closed {
		return result
	}
	for value := range filter.blocked {
		result.blocked[value] = struct{}{}
	}
	return result
}

func (filter *requestIDFilter) clear() {
	if filter == nil {
		return
	}
	filter.mu.Lock()
	clear(filter.blocked)
	filter.closed = true
	filter.mu.Unlock()
}

func (filter *requestIDFilter) safeRequestID(value string) string {
	if !validOpaque(value, 256) || containsSensitiveMarker(value) || filter == nil {
		return ""
	}
	filter.mu.RLock()
	defer filter.mu.RUnlock()
	if filter.closed {
		return ""
	}
	for blocked := range filter.blocked {
		if strings.Contains(value, blocked) {
			return ""
		}
	}
	return value
}

func (filter *requestIDFilter) redact(value string) string {
	if filter == nil {
		return redactSensitive(value)
	}
	filter.mu.RLock()
	defer filter.mu.RUnlock()
	if filter.closed {
		return ""
	}
	for blocked := range filter.blocked {
		value = strings.ReplaceAll(value, blocked, "[REDACTED]")
	}
	return redactSensitive(value)
}

type apiErrorEnvelope struct {
	Error struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Status  string `json:"status"`
		Details []struct {
			Type      string            `json:"@type"`
			Reason    string            `json:"reason"`
			Domain    string            `json:"domain"`
			RequestID string            `json:"requestId"`
			Metadata  map[string]string `json:"metadata"`
			Errors    []struct {
				ErrorCode map[string]string `json:"errorCode"`
				Message   string            `json:"message"`
			} `json:"errors"`
		} `json:"details"`
	} `json:"error"`
}

func newHTTPErrorDecoder(clock socialhub.Clock, requestIDs *requestIDFilter) func(int, http.Header, []byte) error {
	return func(status int, header http.Header, body []byte) error {
		return decodeHTTPError(status, header, body, clock.Now(), requestIDs)
	}
}

func decodeHTTPError(status int, header http.Header, body []byte, now time.Time, requestIDs *requestIDFilter) error {
	var response apiErrorEnvelope
	_ = json.Unmarshal(body, &response)
	platformCode := response.Error.Status
	message := response.Error.Message
	bodyRequestID := ""
	for _, detail := range response.Error.Details {
		bodyRequestID = firstNonEmpty(bodyRequestID, detail.RequestID, detail.Metadata["requestId"])
		platformCode = firstNonEmpty(detail.Reason, platformCode)
		if len(detail.Errors) > 0 {
			platformCode = firstNonEmpty(firstErrorCode(detail.Errors[0].ErrorCode), platformCode)
			message = firstNonEmpty(detail.Errors[0].Message, message)
		}
		if platformCode != response.Error.Status {
			break
		}
	}
	platformCode = requestIDs.redact(platformCode)
	message = requestIDs.redact(message)
	code, class := classifyError(status, response.Error.Status, platformCode)
	return &socialhub.Error{
		Code: code, Class: class, Platform: platformName, Product: productName, HTTPStatus: status,
		PlatformCode: boundedMessage(platformCode, 256), PlatformMessage: boundedMessage(message, 1024),
		RequestID:  firstSafeRequestID(requestIDs, header.Get("x-goog-request-id"), bodyRequestID),
		RetryAfter: parseRetryAfter(header.Get("Retry-After"), now),
	}
}

func firstErrorCode(values map[string]string) string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if values[key] != "" {
			return values[key]
		}
	}
	return ""
}

func classifyError(status int, platformStatus, platformCode string) (socialhub.ErrorCode, socialhub.ErrorClass) {
	normalized := strings.ToUpper(strings.TrimSpace(platformCode))
	switch normalized {
	case "RESOURCE_EXHAUSTED", "RATE_LIMIT_EXCEEDED", "QUOTA_EXCEEDED", "TOO_MANY_REQUESTS":
		return socialhub.CodeRateLimited, socialhub.ClassRetryable
	case "DAILY_LIMIT_EXCEEDED":
		return socialhub.CodeRateLimited, socialhub.ClassUserAction
	case "UNAUTHENTICATED", "AUTHENTICATION_ERROR", "INVALID_CREDENTIALS":
		return socialhub.CodeUnauthenticated, socialhub.ClassUserAction
	case "PERMISSION_DENIED", "AUTHORIZATION_ERROR", "USER_PERMISSION_DENIED":
		return socialhub.CodePermissionDenied, socialhub.ClassUserAction
	case "NOT_FOUND":
		return socialhub.CodeNotFound, socialhub.ClassPermanent
	case "ABORTED", "CONCURRENT_MODIFICATION", "VERSION_MISMATCH":
		return socialhub.CodeConflict, socialhub.ClassPermanent
	case "INTERNAL_ERROR", "SERVER_ERROR", "TEMPORARILY_UNAVAILABLE":
		return socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable
	}
	switch strings.ToUpper(strings.TrimSpace(platformStatus)) {
	case "INVALID_ARGUMENT", "FAILED_PRECONDITION", "OUT_OF_RANGE":
		return socialhub.CodeInvalidArgument, socialhub.ClassPermanent
	case "RESOURCE_EXHAUSTED":
		return socialhub.CodeRateLimited, socialhub.ClassRetryable
	case "UNAUTHENTICATED":
		return socialhub.CodeUnauthenticated, socialhub.ClassUserAction
	case "PERMISSION_DENIED":
		return socialhub.CodePermissionDenied, socialhub.ClassUserAction
	case "NOT_FOUND":
		return socialhub.CodeNotFound, socialhub.ClassPermanent
	case "ALREADY_EXISTS", "ABORTED":
		return socialhub.CodeConflict, socialhub.ClassPermanent
	case "UNAVAILABLE", "DEADLINE_EXCEEDED", "INTERNAL":
		return socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable
	}
	switch status {
	case http.StatusBadRequest, http.StatusUnprocessableEntity, http.StatusRequestEntityTooLarge:
		return socialhub.CodeInvalidArgument, socialhub.ClassPermanent
	case http.StatusUnauthorized:
		return socialhub.CodeUnauthenticated, socialhub.ClassUserAction
	case http.StatusForbidden:
		return socialhub.CodePermissionDenied, socialhub.ClassUserAction
	case http.StatusNotFound, http.StatusGone:
		return socialhub.CodeNotFound, socialhub.ClassPermanent
	case http.StatusConflict:
		return socialhub.CodeConflict, socialhub.ClassPermanent
	case http.StatusRequestTimeout:
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
		Platform: platformName, Product: productName, Op: operation, PlatformMessage: boundedMessage(message, 1024),
	}
}

func platformContractError(operation, message string) error {
	return &socialhub.Error{
		Code: socialhub.CodePlatformError, Class: socialhub.ClassPermanent,
		Platform: platformName, Product: productName, Op: operation, PlatformMessage: boundedMessage(message, 1024),
	}
}

func outcomeUnknownError(operation string, cause error, requestID string, requestIDs *requestIDFilter) error {
	var hub *socialhub.Error
	if requestID == "" && errors.As(cause, &hub) {
		requestID = hub.RequestID
	}
	return &socialhub.Error{
		Code: socialhub.CodeConflict, Class: socialhub.ClassUserAction,
		Platform: platformName, Product: productName, Op: operation,
		PlatformMessage: "Merchant API mutation outcome is unknown; reconcile Merchant Center state before retrying",
		RequestID:       requestIDs.safeRequestID(requestID),
		Cause:           errors.Join(ErrOutcomeUnknown, mutationCause(cause)),
	}
}

func mutationCause(err error) error {
	var hub *socialhub.Error
	if errors.As(err, &hub) {
		return sanitizeCause(hub.Cause)
	}
	return sanitizeCause(err)
}

func ambiguousMutationError(err error) bool {
	var hub *socialhub.Error
	if !errors.As(err, &hub) {
		return true
	}
	return hub.HTTPStatus == 0 || hub.HTTPStatus == http.StatusRequestTimeout || hub.HTTPStatus >= 500 ||
		hub.HTTPStatus >= 200 && hub.HTTPStatus < 300
}

func responseRequestID(header http.Header, requestIDs *requestIDFilter) string {
	return requestIDs.safeRequestID(header.Get("x-goog-request-id"))
}

func firstSafeRequestID(requestIDs *requestIDFilter, values ...string) string {
	for _, value := range values {
		if safe := requestIDs.safeRequestID(value); safe != "" {
			return safe
		}
	}
	return ""
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

func containsSensitiveMarker(value string) bool {
	lower := strings.ToLower(value)
	for _, marker := range []string{"access_token", "refresh_token", "client_secret", "authorization", "bearer"} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

func boundedMessage(value string, maximum int) string {
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

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func redactSensitive(value string) string {
	for _, key := range []string{"access_token", "refresh_token", "client_secret", "authorization", "bearer"} {
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
			for valueEnd < len(value) && !strings.ContainsRune(" \t\r\n,;&\"'", rune(value[valueEnd])) {
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
