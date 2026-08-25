package yahoosearchads

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

var ErrOutcomeUnknown = errors.New("yahoosearchads: mutation outcome unknown")
var ErrPartialMutation = errors.New("yahoosearchads: partial mutation")

type errorEnvelope struct {
	Errors []ErrorItem `json:"errors"`
	RID    string      `json:"rid"`
}

// APIError augments the platform-neutral error with bounded provider codes.
// Provider messages and request values are deliberately not retained.
type APIError struct {
	Hub    *socialhub.Error
	Errors []ErrorItem
	RID    string
}

func (err *APIError) Error() string {
	if err == nil || err.Hub == nil {
		return "socialhub: line-yahoo: platform_error"
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

type requestIDFilter struct {
	mu      sync.RWMutex
	blocked map[string]struct{}
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
	for _, value := range values {
		if value != "" {
			filter.blocked[value] = struct{}{}
		}
	}
}

func (filter *requestIDFilter) safe(values ...string) string {
	value := boundedOpaque(firstNonEmpty(values...), 256)
	if value == "" || filter == nil {
		return ""
	}
	filter.mu.RLock()
	defer filter.mu.RUnlock()
	if _, blocked := filter.blocked[value]; blocked {
		return ""
	}
	return value
}

func newHTTPErrorDecoder(clock socialhub.Clock, requestIDs *requestIDFilter) func(int, http.Header, []byte) error {
	return func(status int, header http.Header, body []byte) error {
		var envelope errorEnvelope
		_ = json.Unmarshal(body, &envelope)
		return apiErrorValue("", status, header, envelope.RID, envelope.Errors, clock.Now(), requestIDs)
	}
}

func apiErrorValue(
	operation string,
	status int,
	header http.Header,
	rid string,
	items []ErrorItem,
	now time.Time,
	requestIDs *requestIDFilter,
) error {
	providerCode := ""
	if len(items) > 0 {
		providerCode = validPlatformCode(items[0].Code)
	}
	code, class := classifyError(status, providerCode)
	requestID := requestIDs.safe(rid, header.Get("x-z-rid"))
	retryAfter := retryDelay(header, now)
	if providerCode == "0003" && retryAfter == 0 {
		retryAfter = 30 * time.Second
	}
	return &APIError{Hub: &socialhub.Error{
		Code: code, Class: class, Platform: platformName, Product: productName, Op: operation,
		HTTPStatus: status, PlatformCode: providerCode,
		PlatformMessage: "LINE Yahoo Ads API request failed", RequestID: requestID,
		RetryAfter: retryAfter,
	}, Errors: sanitizeErrorItems(items), RID: requestID}
}

func (client *Client) apiErrorValue(operation string, status int, header http.Header, rid string, items []ErrorItem) error {
	return apiErrorValue(operation, status, header, rid, items, client.clock.Now(), client.requestIDs)
}

func classifyError(status int, providerCode string) (socialhub.ErrorCode, socialhub.ErrorClass) {
	switch strings.TrimSpace(providerCode) {
	case "0110", "0111", "0114":
		return socialhub.CodeUnauthenticated, socialhub.ClassUserAction
	case "0112", "I0001":
		return socialhub.CodeNotFound, socialhub.ClassPermanent
	case "0113":
		return socialhub.CodeConflict, socialhub.ClassUserAction
	case "0098":
		return socialhub.CodePermissionDenied, socialhub.ClassUserAction
	case "0003":
		return socialhub.CodeRateLimited, socialhub.ClassRetryable
	case "0002", "0099", "130000", "130001":
		return socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable
	case "0001", "R0001", "F0001", "V0001", "L0001", "L0002", "D0001", "RL001", "240003", "240004":
		return socialhub.CodeInvalidArgument, socialhub.ClassPermanent
	case "S0001":
		return socialhub.CodeConflict, socialhub.ClassPermanent
	case "LT001", "210405":
		return socialhub.CodeConflict, socialhub.ClassUserAction
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

func invalidArgument(operation, message string) error {
	return &socialhub.Error{
		Code: socialhub.CodeInvalidArgument, Class: socialhub.ClassPermanent,
		Platform: platformName, Product: productName, Op: operation,
		PlatformMessage: boundedText(message, 512),
	}
}

func notFound(operation, message string) error {
	return &socialhub.Error{
		Code: socialhub.CodeNotFound, Class: socialhub.ClassPermanent,
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

func outcomeUnknownError(operation string, cause error, rid string) error {
	requestID := boundedOpaque(rid, 256)
	var hub *socialhub.Error
	if requestID == "" && errors.As(cause, &hub) {
		requestID = hub.RequestID
	}
	return &APIError{Hub: &socialhub.Error{
		Code: socialhub.CodeConflict, Class: socialhub.ClassUserAction,
		Platform: platformName, Product: productName, Op: operation,
		PlatformMessage: "LINE Yahoo mutation outcome is unknown; reconcile advertiser state before retrying",
		RequestID:       requestID, Cause: errors.Join(ErrOutcomeUnknown, sanitizeCause(cause)),
	}, RID: requestID}
}

func partialMutationError(operation, rid string, cause error) error {
	return &APIError{Hub: &socialhub.Error{
		Code: socialhub.CodeConflict, Class: socialhub.ClassUserAction,
		Platform: platformName, Product: productName, Op: operation,
		PlatformMessage: "LINE Yahoo applied only part of the batch; reconcile per-item results before retrying",
		RequestID:       rid, Cause: errors.Join(ErrPartialMutation, sanitizeCause(cause)),
	}, RID: rid}
}

func ambiguousMutationError(err error) bool {
	var hub *socialhub.Error
	if !errors.As(err, &hub) {
		return true
	}
	return hub.HTTPStatus == 0 || hub.HTTPStatus == http.StatusRequestTimeout || hub.HTTPStatus >= 500 ||
		hub.HTTPStatus >= 200 && hub.HTTPStatus < 300
}

func retryDelay(header http.Header, now time.Time) time.Duration {
	value := header.Get("Retry-After")
	if value == "" || len(value) > 128 || !utf8.ValidString(value) {
		return 0
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return 0
		}
	}
	value = strings.TrimSpace(value)
	if seconds, err := strconv.ParseUint(value, 10, 64); err == nil && seconds <= uint64((24*time.Hour)/time.Second) {
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

func sanitizeErrorItems(items []ErrorItem) []ErrorItem {
	result := make([]ErrorItem, 0, len(items))
	for _, item := range items {
		code := validPlatformCode(item.Code)
		if code == "" {
			continue
		}
		result = append(result, ErrorItem{Code: code, Message: "LINE Yahoo Ads API item failed"})
	}
	return result
}

func validErrorItems(items []ErrorItem) bool {
	for _, item := range items {
		if validPlatformCode(item.Code) == "" || !validText(item.Message, 2048) {
			return false
		}
		for _, detail := range item.Details {
			if detail.RequestKey != "" && !validOpaque(detail.RequestKey, 1024) ||
				detail.RequestValue != "" && !validOpaque(detail.RequestValue, 4096) {
				return false
			}
		}
	}
	return true
}

func validPlatformCode(value string) string {
	if value == "" || len(value) > 64 {
		return ""
	}
	for _, character := range value {
		if character >= 'A' && character <= 'Z' || character >= 'a' && character <= 'z' ||
			character >= '0' && character <= '9' || character == '_' {
			continue
		}
		return ""
	}
	return value
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
