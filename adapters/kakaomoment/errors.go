package kakaomoment

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

var ErrOutcomeUnknown = errors.New("kakaomoment: mutation outcome unknown")
var ErrReconciliationRequired = errors.New("kakaomoment: reconciliation required")

type ErrorExtras struct {
	DetailCode int    `json:"detailCode"`
	DetailMsg  string `json:"detailMsg"`
	Status     int    `json:"status"`
	Message    string `json:"message"`
}

type errorEnvelope struct {
	Code    *int        `json:"code"`
	Msg     string      `json:"msg"`
	Message string      `json:"message"`
	Extras  ErrorExtras `json:"extras"`
}

// APIError augments the platform-neutral error with bounded Kakao diagnostics.
type APIError struct {
	Hub        *socialhub.Error
	Code       int
	Message    string
	Extras     ErrorExtras
	ResourceID int64
}

func (err *APIError) Error() string {
	if err == nil || err.Hub == nil {
		return "socialhub: kakao: platform_error"
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

func decodeHTTPError(status int, header http.Header, body []byte) error {
	return decodeHTTPErrorAt(status, header, body, time.Now())
}

func newHTTPErrorDecoder(clock socialhub.Clock) func(int, http.Header, []byte) error {
	return func(status int, header http.Header, body []byte) error {
		return decodeHTTPErrorAt(status, header, body, clock.Now())
	}
}

func decodeHTTPErrorAt(status int, header http.Header, body []byte, now time.Time) error {
	var envelope errorEnvelope
	if json.Unmarshal(body, &envelope) != nil {
		envelope = errorEnvelope{}
	}
	return apiErrorValue("", status, header, envelope, now)
}

func apiErrorValue(operation string, status int, header http.Header, envelope errorEnvelope, now time.Time) error {
	providerCode := 0
	if envelope.Code != nil {
		providerCode = *envelope.Code
	}
	detailCode := envelope.Extras.DetailCode
	classificationStatus := status
	if status >= 200 && status < 300 && envelope.Extras.Status >= 400 && envelope.Extras.Status <= 599 {
		classificationStatus = envelope.Extras.Status
	}
	code, class := classifyError(classificationStatus, providerCode, detailCode)
	extras := ErrorExtras{
		DetailCode: detailCode,
		Status:     envelope.Extras.Status,
	}
	requestID := boundedOpaque(firstNonEmpty(
		header.Get("X-Kakao-Request-Id"), header.Get("X-Request-Id"), header.Get("X-Correlation-Id"),
	), 256)
	hub := &socialhub.Error{
		Code: code, Class: class, Platform: platformName, Product: productName, Op: operation,
		HTTPStatus: status, PlatformMessage: "Kakao Moment rejected the request",
		RequestID: requestID, RetryAfter: retryDelay(header, now),
	}
	if envelope.Code != nil {
		hub.PlatformCode = strconv.Itoa(providerCode)
		if detailCode != 0 {
			hub.PlatformCode += "/" + strconv.Itoa(detailCode)
		}
	}
	if code == socialhub.CodeApprovalRequired {
		hub.ApprovalURL = approvalURL
	}
	return &APIError{
		Hub: hub, Code: providerCode,
		Message: "Kakao Moment rejected the request", Extras: extras,
	}
}

func classifyError(status, providerCode, detailCode int) (socialhub.ErrorCode, socialhub.ErrorClass) {
	if providerCode == -813 && status == http.StatusTooManyRequests {
		return socialhub.CodeRateLimited, socialhub.ClassRetryable
	}
	switch providerCode {
	case -2:
		return socialhub.CodeInvalidArgument, socialhub.ClassPermanent
	case -3:
		return socialhub.CodeApprovalRequired, socialhub.ClassUserAction
	case -5, -12, -402:
		return socialhub.CodePermissionDenied, socialhub.ClassUserAction
	case -10:
		return socialhub.CodeRateLimited, socialhub.ClassRetryable
	case -101:
		return socialhub.CodeUnauthenticated, socialhub.ClassUserAction
	}
	switch detailCode {
	case 6003, 21006, 31001, 32001, 33003, 75014:
		return socialhub.CodeNotFound, socialhub.ClassPermanent
	case 7001, 9004, 75303, 75401, 75601:
		return socialhub.CodePermissionDenied, socialhub.ClassUserAction
	case 21400, 21500, 32013, 32040, 32041, 38022:
		return socialhub.CodeConflict, socialhub.ClassUserAction
	case 60005, 60007, 60008:
		return socialhub.CodeInvalidArgument, socialhub.ClassPermanent
	}
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

func conflict(operation, message string) error {
	return &socialhub.Error{
		Code: socialhub.CodeConflict, Class: socialhub.ClassUserAction,
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

func outcomeUnknownError(operation string, cause error, requestID string) error {
	var hub *socialhub.Error
	if requestID == "" && errors.As(cause, &hub) {
		requestID = hub.RequestID
	}
	return &APIError{Hub: &socialhub.Error{
		Code: socialhub.CodeConflict, Class: socialhub.ClassUserAction,
		Platform: platformName, Product: productName, Op: operation,
		PlatformMessage: "Kakao Moment mutation outcome is unknown; reconcile ad-account state before retrying",
		RequestID:       boundedOpaque(requestID, 256), Cause: errors.Join(ErrOutcomeUnknown, sanitizeCause(cause)),
	}}
}

func reconciliationError(operation string, resourceID int64, cause error) error {
	return &APIError{Hub: &socialhub.Error{
		Code: socialhub.CodeConflict, Class: socialhub.ClassUserAction,
		Platform: platformName, Product: productName, Op: operation,
		PlatformMessage: "Campaign was created but OFF state was not confirmed; reconcile it before creating child resources",
		Cause:           errors.Join(ErrReconciliationRequired, sanitizeCause(cause)),
	}, ResourceID: resourceID}
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
	value := boundedOpaque(header.Get("Retry-After"), 128)
	if value == "" {
		return 0
	}
	seconds, err := strconv.ParseInt(value, 10, 64)
	if err == nil && seconds >= 0 && seconds <= 86_400 {
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

func sanitizeCause(err error) error {
	var urlError *url.Error
	if errors.As(err, &urlError) && urlError.Err != nil {
		return urlError.Err
	}
	return err
}
