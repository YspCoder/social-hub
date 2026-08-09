package microsoftads

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

// Failure preserves a sanitized Microsoft Advertising operation or batch
// failure without exposing credentials or raw response bodies.
type Failure struct {
	Code      int
	ErrorCode string
	Message   string
	Details   string
	Index     *int
	FieldPath string
}

// APIError augments the common social-hub error with typed Microsoft
// Advertising failures.
type APIError struct {
	Hub        *socialhub.Error
	TrackingID string
	Failures   []Failure
}

func (err *APIError) Error() string {
	if err == nil || err.Hub == nil {
		return "socialhub: microsoftads: platform_error"
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

type wireFailure struct {
	Code      int    `json:"Code"`
	ErrorCode string `json:"ErrorCode"`
	Message   string `json:"Message"`
	Details   string `json:"Details"`
	Index     *int   `json:"Index"`
	FieldPath string `json:"FieldPath"`
}

type faultEnvelope struct {
	TrackingID      string         `json:"TrackingId"`
	OperationErrors []wireFailure  `json:"OperationErrors"`
	BatchErrors     []wireFailure  `json:"BatchErrors"`
	PartialErrors   []wireFailure  `json:"PartialErrors"`
	AdAPIFault      *faultEnvelope `json:"AdApiFaultDetail"`
	APIFault        *faultEnvelope `json:"ApiFaultDetail"`
	EditorialFault  *faultEnvelope `json:"EditorialApiFaultDetail"`
}

func decodeHTTPError(status int, header http.Header, body []byte) error {
	return decodeAPIError("", status, header, body)
}

func decodeAPIError(operation string, status int, header http.Header, body []byte) error {
	var envelope faultEnvelope
	_ = json.Unmarshal(body, &envelope)
	envelope = unwrapFault(envelope)
	failures := append([]wireFailure(nil), envelope.OperationErrors...)
	failures = append(failures, envelope.BatchErrors...)
	failures = append(failures, envelope.PartialErrors...)
	return newAPIError(operation, status, header, envelope.TrackingID, failures)
}

func unwrapFault(envelope faultEnvelope) faultEnvelope {
	for _, nested := range []*faultEnvelope{envelope.AdAPIFault, envelope.APIFault, envelope.EditorialFault} {
		if nested != nil {
			if nested.TrackingID == "" {
				nested.TrackingID = envelope.TrackingID
			}
			return *nested
		}
	}
	return envelope
}

func checkPartialErrors(operation string, header http.Header, failures []wireFailure) error {
	if len(failures) == 0 {
		return nil
	}
	return newAPIError(operation, http.StatusOK, header, header.Get("TrackingId"), failures)
}

func newAPIError(operation string, status int, header http.Header, trackingID string, wire []wireFailure) error {
	failures := make([]Failure, 0, len(wire))
	for _, failure := range wire {
		failures = append(failures, Failure{
			Code: failure.Code, ErrorCode: boundedMessage(failure.ErrorCode, 256),
			Message: boundedMessage(redactSensitive(failure.Message), 512),
			Details: boundedMessage(redactSensitive(failure.Details), 512),
			Index:   failure.Index, FieldPath: boundedMessage(failure.FieldPath, 256),
		})
	}
	code, class := classifyError(status, failures)
	platformCode, message := "", ""
	if len(failures) > 0 {
		platformCode = failures[0].ErrorCode
		if platformCode == "" && failures[0].Code != 0 {
			platformCode = strconv.Itoa(failures[0].Code)
		}
		message = firstNonEmpty(failures[0].Message, failures[0].Details)
	}
	requestID := firstNonEmpty(trackingID, header.Get("TrackingId"), header.Get("x-ms-request-id"), header.Get("request-id"))
	hub := &socialhub.Error{
		Code: code, Class: class, Platform: platformName, Product: productName, Op: operation,
		HTTPStatus: status, PlatformCode: platformCode, PlatformMessage: message,
		RequestID: boundedMessage(requestID, 256), RetryAfter: parseRetryAfter(header.Get("Retry-After")),
	}
	return &APIError{Hub: hub, TrackingID: hub.RequestID, Failures: failures}
}

func classifyError(status int, failures []Failure) (socialhub.ErrorCode, socialhub.ErrorClass) {
	for _, failure := range failures {
		upper := strings.ToUpper(failure.ErrorCode)
		switch {
		case failure.Code == 117 || failure.Code == 207 || upper == "CALLRATEEXCEEDED" || upper == "CONCURRENTREQUESTOVERLIMIT":
			return socialhub.CodeRateLimited, socialhub.ClassRetryable
		case strings.Contains(upper, "AUTHENTICATION") || strings.Contains(upper, "INVALIDCREDENTIAL") || strings.Contains(upper, "TOKENEXPIRED"):
			return socialhub.CodeUnauthenticated, socialhub.ClassUserAction
		case strings.Contains(upper, "NOTAUTHORIZED") || strings.Contains(upper, "PERMISSION") || strings.Contains(upper, "ACCESSDENIED"):
			return socialhub.CodePermissionDenied, socialhub.ClassUserAction
		case strings.Contains(upper, "NOTFOUND"):
			return socialhub.CodeNotFound, socialhub.ClassPermanent
		case strings.Contains(upper, "INVALID") || strings.Contains(upper, "REQUIRED"):
			return socialhub.CodeInvalidArgument, socialhub.ClassPermanent
		}
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
	return &socialhub.Error{Code: code, Class: class, Platform: platformName, Product: productName, Op: operation, Cause: sanitizeCause(cause)}
}

func invalidArgument(operation, message string) error {
	return &socialhub.Error{
		Code: socialhub.CodeInvalidArgument, Class: socialhub.ClassPermanent,
		Platform: platformName, Product: productName, Op: operation, PlatformMessage: message,
	}
}

func platformContractError(operation, message string) error {
	return &socialhub.Error{
		Code: socialhub.CodePlatformError, Class: socialhub.ClassPermanent,
		Platform: platformName, Product: productName, Op: operation, PlatformMessage: message,
	}
}

func parseRetryAfter(value string) time.Duration {
	value = strings.TrimSpace(value)
	seconds, err := strconv.ParseFloat(value, 64)
	if err == nil && seconds >= 0 && seconds <= float64((24*time.Hour)/time.Second) {
		return time.Duration(seconds * float64(time.Second))
	}
	when, err := http.ParseTime(value)
	if err != nil {
		return 0
	}
	delay := time.Until(when)
	if delay < 0 || delay > 24*time.Hour {
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

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func sanitizeCause(err error) error {
	var urlError *url.Error
	if errors.As(err, &urlError) && urlError.Err != nil {
		return urlError.Err
	}
	return err
}

func redactSensitive(value string) string {
	for _, marker := range []string{
		"authorization", "access_token", "accesstoken", "refresh_token", "refreshtoken",
		"developer_token", "developertoken", "client_secret", "clientsecret",
	} {
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
			stops := " \t\r\n,;}&\"'"
			if marker == "authorization" {
				stops = "\r\n,;}&"
			}
			for end < len(value) && !strings.ContainsRune(stops, rune(value[end])) {
				end++
			}
			value = value[:start] + "[REDACTED]" + value[end:]
			cursor = start + len("[REDACTED]")
		}
	}
	return redactURLQueries(value)
}

func redactURLQueries(value string) string {
	for cursor := 0; cursor < len(value); {
		lower := strings.ToLower(value[cursor:])
		httpIndex, httpsIndex := strings.Index(lower, "http://"), strings.Index(lower, "https://")
		index := httpIndex
		if index < 0 || (httpsIndex >= 0 && httpsIndex < index) {
			index = httpsIndex
		}
		if index < 0 {
			break
		}
		index += cursor
		end := index
		for end < len(value) && !strings.ContainsRune(" \t\r\n,;\"'<>[](){}", rune(value[end])) {
			end++
		}
		raw := value[index:end]
		query := strings.IndexByte(raw, '?')
		if query < 0 {
			cursor = end
			continue
		}
		replacement := raw[:query+1] + "[REDACTED]"
		value = value[:index] + replacement + value[end:]
		cursor = index + len(replacement)
	}
	return value
}
