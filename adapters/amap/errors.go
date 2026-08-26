package amap

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

type errorEnvelope struct {
	Status   string
	InfoCode string
}

type APIError struct {
	Hub  *socialhub.Error
	Meta ResponseMeta
}

func (value *APIError) Error() string {
	if value == nil || value.Hub == nil {
		return "socialhub: amap: platform_error"
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
		return newAPIError(status, header, body, clock)
	}
}

func providerErrorFromBody(status int, header http.Header, body []byte, clock socialhub.Clock) (error, bool) {
	provider, found := decodeErrorEnvelope(body)
	if !found || provider.Status == "1" && provider.InfoCode == "10000" {
		return nil, false
	}
	return newAPIError(status, header, body, clock), true
}

func newAPIError(status int, header http.Header, body []byte, clock socialhub.Clock) error {
	provider, _ := decodeErrorEnvelope(body)
	provider.Status = safeProviderCode(provider.Status, 8)
	provider.InfoCode = safeProviderCode(provider.InfoCode, 32)
	platformCode := provider.InfoCode
	if platformCode == "" {
		platformCode = "http_" + strconv.Itoa(status)
	}
	code, class := classifyError(status, provider.InfoCode)
	hub := &socialhub.Error{
		Code: code, Class: class, Platform: platformName, Product: productName,
		HTTPStatus: status, PlatformCode: platformCode, PlatformMessage: "Amap API request failed",
		RetryAfter: parseRetryAfter(header.Get("Retry-After"), clock.Now()),
	}
	if code == socialhub.CodeUnauthenticated || code == socialhub.CodePermissionDenied || code == socialhub.CodeApprovalRequired {
		hub.ApprovalURL = keyManagementURL
	}
	return &APIError{
		Hub:  hub,
		Meta: ResponseMeta{HTTPStatus: status, Status: provider.Status, InfoCode: provider.InfoCode},
	}
}

func decodeErrorEnvelope(body []byte) (errorEnvelope, bool) {
	var wire struct {
		Status   json.RawMessage `json:"status"`
		InfoCode json.RawMessage `json:"infocode"`
	}
	if json.Unmarshal(body, &wire) != nil {
		return errorEnvelope{}, false
	}
	status, statusFound := scalarText(wire.Status)
	infoCode, codeFound := scalarText(wire.InfoCode)
	if !statusFound && !codeFound {
		return errorEnvelope{}, false
	}
	return errorEnvelope{Status: status, InfoCode: infoCode}, true
}

func safeProviderCode(value string, maximum int) string {
	if value == "" || len(value) > maximum {
		return ""
	}
	for _, character := range value {
		if character < '0' || character > '9' {
			return ""
		}
	}
	return value
}

func classifyError(status int, infoCode string) (socialhub.ErrorCode, socialhub.ErrorClass) {
	switch infoCode {
	case "10001", "10007", "10008", "10009", "10013":
		return socialhub.CodeUnauthenticated, socialhub.ClassUserAction
	case "10002", "10012", "10041", "20011", "40000", "40002", "40003":
		return socialhub.CodeApprovalRequired, socialhub.ClassUserAction
	case "10005", "10006", "10026":
		return socialhub.CodePermissionDenied, socialhub.ClassUserAction
	case "10003", "10010", "10029", "10044", "10045":
		return socialhub.CodeRateLimited, socialhub.ClassUserAction
	case "10004", "10014", "10019", "10020", "10021":
		return socialhub.CodeRateLimited, socialhub.ClassRetryable
	case "10015", "10016", "10017":
		return socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable
	case "20000", "20001", "20002", "20012":
		return socialhub.CodeInvalidArgument, socialhub.ClassPermanent
	}
	if status >= 300 && status < 400 {
		return socialhub.CodeConflict, socialhub.ClassPermanent
	}
	switch status {
	case http.StatusBadRequest, http.StatusMethodNotAllowed, http.StatusUnprocessableEntity:
		return socialhub.CodeInvalidArgument, socialhub.ClassPermanent
	case http.StatusUnauthorized:
		return socialhub.CodeUnauthenticated, socialhub.ClassUserAction
	case http.StatusForbidden:
		return socialhub.CodePermissionDenied, socialhub.ClassUserAction
	case http.StatusNotFound:
		return socialhub.CodeNotFound, socialhub.ClassPermanent
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

func authenticationError(operation string) error {
	return &socialhub.Error{
		Code: socialhub.CodeUnauthenticated, Class: socialhub.ClassUserAction,
		Platform: platformName, Product: productName, Op: operation,
		PlatformMessage: "Amap Web Service credentials could not be resolved or validated",
		ApprovalURL:     keyManagementURL,
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
	if seconds, err := strconv.ParseInt(value, 10, 64); err == nil && seconds >= 0 && seconds <= int64((48*time.Hour)/time.Second) {
		return time.Duration(seconds) * time.Second
	}
	when, err := http.ParseTime(value)
	if err != nil || !when.After(now) {
		return 0
	}
	delay := when.Sub(now)
	if delay > 48*time.Hour {
		return 0
	}
	return delay
}

func boundedMessage(value string, maximum int) string {
	if utf8.RuneCountInString(value) <= maximum {
		return value
	}
	return string([]rune(value)[:maximum])
}

func sanitizeCause(err error) error {
	var urlError *url.Error
	if errors.As(err, &urlError) && urlError.Err != nil {
		return urlError.Err
	}
	return err
}
