package xandr

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

// APIError preserves sanitized Digital Platform API diagnostics.
type APIError struct {
	Hub            *socialhub.Error
	RateLimitCode  string
	RateLimitCount string
}

func (err *APIError) Error() string {
	if err == nil || err.Hub == nil {
		return "socialhub: xandr: platform_error"
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

func (filter *requestIDFilter) clear() {
	if filter == nil {
		return
	}
	filter.mu.Lock()
	clear(filter.blocked)
	filter.closed = true
	filter.mu.Unlock()
}

func (filter *requestIDFilter) safe(value string) string {
	if value == "" || filter == nil {
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

func newHTTPErrorDecoder(clock socialhub.Clock, requestIDs *requestIDFilter) func(int, http.Header, []byte) error {
	return func(status int, header http.Header, body []byte) error {
		return decodeHTTPError(status, header, body, clock.Now(), requestIDs)
	}
}

func decodeHTTPError(status int, header http.Header, body []byte, now time.Time, requestIDs *requestIDFilter) error {
	var envelope apiEnvelope
	if json.Unmarshal(body, &envelope) == nil && envelope.Response != nil {
		return businessError("", status, header, *envelope.Response, now, requestIDs)
	}
	code, class := classifyError(status, header, "")
	return &APIError{
		Hub: &socialhub.Error{
			Code: code, Class: class, Platform: platformName, Product: productName,
			HTTPStatus: status, PlatformCode: firstNonEmpty(boundedUnsignedHeader(header.Get("X-RateLimit-Code"), 32), strconv.Itoa(status)),
			PlatformMessage: "Xandr rejected the request", RequestID: responseRequestID(header, requestIDs), RetryAfter: retryDelay(header, now),
		},
		RateLimitCode:  boundedUnsignedHeader(header.Get("X-RateLimit-Code"), 32),
		RateLimitCount: boundedUnsignedHeader(header.Get("X-RateLimit-Count"), 32),
	}
}

func businessError(operation string, status int, header http.Header, wire responseWire, now time.Time, requestIDs *requestIDFilter) error {
	code, class := classifyError(status, header, wire.ErrorID)
	platformCode := boundedMachineCode(wire.ErrorID, 128)
	if platformCode == "" {
		platformCode = firstNonEmpty(boundedUnsignedHeader(header.Get("X-RateLimit-Code"), 32), strconv.Itoa(status))
	}
	return &APIError{
		Hub: &socialhub.Error{
			Code: code, Class: class, Platform: platformName, Product: productName, Op: operation,
			HTTPStatus: status, PlatformCode: platformCode, PlatformMessage: "Xandr rejected the request",
			RequestID: responseRequestID(header, requestIDs), RetryAfter: retryDelay(header, now),
		},
		RateLimitCode:  boundedUnsignedHeader(header.Get("X-RateLimit-Code"), 32),
		RateLimitCount: boundedUnsignedHeader(header.Get("X-RateLimit-Count"), 32),
	}
}

func classifyError(status int, header http.Header, platformCode string) (socialhub.ErrorCode, socialhub.ErrorClass) {
	if status == http.StatusTooManyRequests || status == http.StatusServiceUnavailable && boundedUnsignedHeader(header.Get("X-RateLimit-Code"), 32) != "" {
		return socialhub.CodeRateLimited, socialhub.ClassRetryable
	}
	switch strings.ToUpper(platformCode) {
	case "NOAUTH", "NOAUTH_DISABLED", "NOAUTH_EXPIRED":
		return socialhub.CodeUnauthenticated, socialhub.ClassUserAction
	case "UNAUTH":
		return socialhub.CodePermissionDenied, socialhub.ClassUserAction
	case "SYNTAX":
		return socialhub.CodeInvalidArgument, socialhub.ClassPermanent
	case "INTEGRITY":
		return socialhub.CodeConflict, socialhub.ClassPermanent
	case "LIMIT":
		return socialhub.CodeConflict, socialhub.ClassPermanent
	case "SYSTEM":
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
	if value := header.Get("Retry-After"); value == "" || len(value) > 128 || !utf8.ValidString(value) || strings.ContainsFunc(value, unicode.IsControl) {
		return 0
	}
	value := strings.TrimSpace(header.Get("Retry-After"))
	if seconds, err := strconv.ParseInt(value, 10, 64); err == nil && seconds >= 0 && seconds <= int64((24*time.Hour)/time.Second) {
		return time.Duration(seconds) * time.Second
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
	for _, character := range value {
		if unicode.IsControl(character) {
			return ""
		}
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

func boundedMachineCode(value string, maximum int) string {
	value = boundedOpaque(value, maximum)
	for _, character := range value {
		if character != '_' && character != '-' &&
			(character < '0' || character > '9') &&
			(character < 'A' || character > 'Z') &&
			(character < 'a' || character > 'z') {
			return ""
		}
	}
	return value
}

func boundedUnsignedHeader(value string, maximum int) string {
	value = boundedOpaque(value, maximum)
	for _, character := range value {
		if character < '0' || character > '9' {
			return ""
		}
	}
	return value
}

func responseRequestID(header http.Header, requestIDs *requestIDFilter) string {
	value := boundedOpaque(header.Get("X-B3-TraceId"), 32)
	if len(value) != 16 && len(value) != 32 {
		return ""
	}
	for _, character := range value {
		if (character < '0' || character > '9') && (character < 'a' || character > 'f') && (character < 'A' || character > 'F') {
			return ""
		}
	}
	return requestIDs.safe(value)
}

func sanitizeCause(err error) error {
	var urlError *url.Error
	if errors.As(err, &urlError) && urlError.Err != nil {
		return urlError.Err
	}
	return err
}
