package deviantart

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

type apiErrorResponse struct {
	Error            string          `json:"error"`
	ErrorDescription string          `json:"error_description"`
	ErrorDetails     map[string]any  `json:"error_details"`
	ErrorCode        json.RawMessage `json:"error_code"`
}

func decodeHTTPError(status int, header http.Header, body []byte) error {
	var response apiErrorResponse
	_ = json.Unmarshal(body, &response)
	code, class := classifyError(status, response.Error)
	platformCode := boundedMessage(strings.TrimSpace(response.Error), 256)
	if raw := strings.Trim(string(response.ErrorCode), " \t\r\n\""); raw != "" && raw != "null" {
		if platformCode == "" {
			platformCode = raw
		} else {
			platformCode += ":" + raw
		}
	}
	return &socialhub.Error{
		Code: code, Class: class, Platform: "deviantart", Product: productName, HTTPStatus: status,
		PlatformCode: platformCode, PlatformMessage: boundedMessage(response.ErrorDescription, 512),
		RequestID:  boundedMessage(firstNonEmpty(header.Get("X-Request-Id"), header.Get("X-Correlation-Id")), 256),
		RetryAfter: parseRetryAfter(header.Get("Retry-After")),
	}
}

func classifyError(status int, platformCode string) (socialhub.ErrorCode, socialhub.ErrorClass) {
	switch strings.TrimSpace(platformCode) {
	case "unauthorized", "invalid_client", "invalid_grant":
		return socialhub.CodeUnauthenticated, socialhub.ClassUserAction
	case "unverified_account":
		return socialhub.CodePermissionDenied, socialhub.ClassUserAction
	case "server_error", "temporarily_unavailable":
		return socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable
	case "invalid_scope", "unsupported_grant_type", "version_error":
		return socialhub.CodeInvalidArgument, socialhub.ClassPermanent
	}
	switch status {
	case http.StatusBadRequest, http.StatusMethodNotAllowed, http.StatusNotAcceptable, http.StatusUnprocessableEntity, http.StatusRequestEntityTooLarge:
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
	return &socialhub.Error{Code: code, Class: class, Platform: "deviantart", Product: productName, Op: operation, Cause: cause}
}

func invalidArgument(operation, message string) error {
	return &socialhub.Error{
		Code: socialhub.CodeInvalidArgument, Class: socialhub.ClassPermanent, Platform: "deviantart", Product: productName,
		Op: operation, PlatformMessage: message,
	}
}

func unsupported(operation, message string) error {
	return &socialhub.Error{
		Code: socialhub.CodeUnsupported, Class: socialhub.ClassPermanent, Platform: "deviantart", Product: productName,
		Op: operation, PlatformMessage: message,
	}
}

func parseRetryAfter(value string) time.Duration {
	seconds, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err != nil || seconds < 0 || seconds > float64((24*time.Hour)/time.Second) {
		return 0
	}
	return time.Duration(seconds * float64(time.Second))
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
