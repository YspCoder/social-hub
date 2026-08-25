package thetradedesk

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

var (
	ErrOutcomeUnknown       = errors.New("thetradedesk: mutation outcome is unknown")
	errCredentialResolution = errors.New("thetradedesk: credential resolution failed")
	errTokenStoreOperation  = errors.New("thetradedesk: token store operation failed")
)

// PlatformProblem preserves the documented property-validation error details.
type PlatformProblem struct {
	Property  string
	Reasons   []string
	ErrorCode string
}

// APIError augments the common error with sanitized TTD error fields.
type APIError struct {
	Hub     *socialhub.Error
	Problem PlatformProblem
}

func (err *APIError) Error() string {
	if err == nil || err.Hub == nil {
		return "socialhub: thetradedesk: platform_error"
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

type problemWire struct {
	Property  string          `json:"Property"`
	Reasons   []string        `json:"Reasons"`
	ErrorCode json.RawMessage `json:"ErrorCode"`
}

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
	var wire problemWire
	_ = json.Unmarshal(body, &wire)
	property := boundedText(wire.Property, 512)
	sensitiveProperty := containsSensitiveMarker(property)
	if sensitiveProperty {
		property = ""
	}
	problem := PlatformProblem{
		Property:  property,
		ErrorCode: safePlatformCode(rawScalar(wire.ErrorCode)),
	}
	for _, reason := range wire.Reasons {
		if sensitiveProperty {
			break
		}
		if len(problem.Reasons) == 32 {
			break
		}
		if sanitized := boundedText(redactSensitive(reason), 512); sanitized != "" {
			problem.Reasons = append(problem.Reasons, sanitized)
		}
	}
	message := ""
	if len(problem.Reasons) > 0 {
		message = strings.Join(problem.Reasons, "; ")
		if problem.Property != "" {
			message = problem.Property + ": " + message
		}
	}
	if message == "" {
		message = http.StatusText(status)
	}
	code, class := classifyError(status)
	platformCode := problem.ErrorCode
	if platformCode == "" {
		platformCode = strconv.Itoa(status)
	}
	approvalURL := ""
	if status == http.StatusUnauthorized {
		approvalURL = authenticationDocURL
	} else if status == http.StatusForbidden {
		approvalURL = accountSetupURL
	}
	hub := &socialhub.Error{
		Code: code, Class: class, Platform: platformName, Product: productName,
		HTTPStatus: status, PlatformCode: platformCode, PlatformMessage: message,
		RequestID:   responseRequestID(header, requestIDs),
		RetryAfter:  retryDelay(header, now),
		ApprovalURL: approvalURL,
	}
	return &APIError{Hub: hub, Problem: problem}
}

func classifyError(status int) (socialhub.ErrorCode, socialhub.ErrorClass) {
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
	case http.StatusGone:
		return socialhub.CodeUnsupported, socialhub.ClassPermanent
	case http.StatusTooManyRequests:
		return socialhub.CodeRateLimited, socialhub.ClassRetryable
	case http.StatusRequestTimeout, http.StatusServiceUnavailable:
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

func outcomeUnknownError(operation string, cause error, requestID string, requestIDs *requestIDFilter) error {
	var hub *socialhub.Error
	if requestID == "" && errors.As(cause, &hub) {
		requestID = hub.RequestID
	}
	return &APIError{Hub: &socialhub.Error{
		Code: socialhub.CodeConflict, Class: socialhub.ClassUserAction,
		Platform: platformName, Product: productName, Op: operation,
		PlatformMessage: "Platform API mutation outcome is unknown; reconcile advertiser state before retrying",
		RequestID:       safeRequestID(requestID, requestIDs),
		Cause:           errors.Join(ErrOutcomeUnknown, sanitizeCause(cause)),
	}}
}

func ambiguousMutationError(err error) bool {
	var hub *socialhub.Error
	if !errors.As(err, &hub) {
		return true
	}
	return hub.HTTPStatus == 0 || hub.HTTPStatus == http.StatusRequestTimeout || hub.HTTPStatus >= 500 ||
		hub.HTTPStatus >= 200 && hub.HTTPStatus < 300
}

func rawScalar(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var text string
	if json.Unmarshal(raw, &text) == nil {
		return text
	}
	trimmed := strings.TrimSpace(string(raw))
	if len(trimmed) <= 128 && json.Valid(raw) && !strings.ContainsAny(trimmed, "{}[]\"") {
		return trimmed
	}
	return ""
}

func retryDelay(header http.Header, now time.Time) time.Duration {
	value := safeHeaderValue(header.Get("Retry-After"), 128)
	if value == "" {
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

func boundedText(value string, maximum int) string {
	value = strings.TrimSpace(value)
	if value == "" || !utf8.ValidString(value) {
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
	value = boundedText(value, maximum)
	if !validOpaque(value, maximum) {
		return ""
	}
	return value
}

func responseRequestID(header http.Header, requestIDs *requestIDFilter) string {
	return safeRequestID(header.Get("X-TTD-Request-ID"), requestIDs)
}

func safeRequestID(value string, requestIDs *requestIDFilter) string {
	return requestIDs.safe(safeHeaderValue(value, 256))
}

func safeHeaderValue(value string, maximum int) string {
	if len(value) > maximum || !utf8.ValidString(value) {
		return ""
	}
	value = strings.TrimSpace(value)
	if !validOpaque(value, maximum) || containsSensitiveMarker(value) {
		return ""
	}
	return value
}

func safePlatformCode(value string) string {
	value = boundedOpaque(value, 128)
	if value == "" || containsSensitiveMarker(value) {
		return ""
	}
	for _, character := range value {
		if character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' ||
			character >= '0' && character <= '9' || strings.ContainsRune("._:-", character) {
			continue
		}
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
	markers := []string{"authorization", "ttd-auth", "password", "token", "login", "secret", "api-key", "apikey"}
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

func containsSensitiveMarker(value string) bool {
	lower := strings.ToLower(value)
	for _, marker := range []string{"authorization", "ttd-auth", "password", "token", "login", "secret", "api-key", "apikey"} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}
