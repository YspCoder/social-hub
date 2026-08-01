package microsoftteams

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

type graphErrorEnvelope struct {
	Error struct {
		Code       string `json:"code"`
		Message    string `json:"message"`
		InnerError struct {
			RequestID       string `json:"request-id"`
			ClientRequestID string `json:"client-request-id"`
		} `json:"innerError"`
	} `json:"error"`
}

func decodeHTTPError(status int, header http.Header, body []byte) error {
	var envelope graphErrorEnvelope
	_ = json.Unmarshal(body, &envelope)
	code, class := classifyError(status, envelope.Error.Code)
	requestID := firstNonEmpty(header.Get("request-id"), header.Get("client-request-id"), envelope.Error.InnerError.RequestID)
	return &socialhub.Error{
		Code: code, Class: class, Platform: "microsoft-teams", Product: productName, Op: "http",
		HTTPStatus: status, PlatformCode: boundedMessage(envelope.Error.Code, 256),
		PlatformMessage: boundedMessage(envelope.Error.Message, 512), RequestID: boundedMessage(requestID, 512),
		RetryAfter: retryAfter(header.Get("Retry-After")),
	}
}

func classifyError(status int, platformCode string) (socialhub.ErrorCode, socialhub.ErrorClass) {
	switch strings.ToLower(strings.TrimSpace(platformCode)) {
	case "invalidauthenticationtoken", "authentication_error", "tokenexpired":
		return socialhub.CodeUnauthenticated, socialhub.ClassUserAction
	case "authorization_requestdenied", "accessdenied", "erroraccessdenied", "forbidden":
		return socialhub.CodePermissionDenied, socialhub.ClassUserAction
	case "itemnotfound", "request_resourcenotfound", "notfound":
		return socialhub.CodeNotFound, socialhub.ClassPermanent
	case "toomanyrequests", "activitylimitreached", "throttledrequest":
		return socialhub.CodeRateLimited, socialhub.ClassRetryable
	case "serviceunavailable", "temporarilyunavailable", "internalservererror":
		return socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable
	}
	switch status {
	case http.StatusBadRequest, http.StatusLengthRequired, http.StatusRequestEntityTooLarge, http.StatusUnprocessableEntity:
		return socialhub.CodeInvalidArgument, socialhub.ClassPermanent
	case http.StatusUnauthorized:
		return socialhub.CodeUnauthenticated, socialhub.ClassUserAction
	case http.StatusForbidden:
		return socialhub.CodePermissionDenied, socialhub.ClassUserAction
	case http.StatusNotFound, http.StatusGone:
		return socialhub.CodeNotFound, socialhub.ClassPermanent
	case http.StatusConflict, http.StatusPreconditionFailed:
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
	return &socialhub.Error{Code: code, Class: class, Platform: "microsoft-teams", Product: productName, Op: operation, Cause: cause}
}

func invalidArgument(operation, message string) error {
	return &socialhub.Error{Code: socialhub.CodeInvalidArgument, Class: socialhub.ClassPermanent, Platform: "microsoft-teams", Product: productName, Op: operation, PlatformMessage: message}
}

func unsupported(operation, message string) error {
	return &socialhub.Error{Code: socialhub.CodeUnsupported, Class: socialhub.ClassPermanent, Platform: "microsoft-teams", Product: productName, Op: operation, PlatformMessage: message}
}

func operationError(err error, operation string) error {
	if platformErr, ok := err.(*socialhub.Error); ok {
		platformErr.Op = operation
	}
	return err
}

func retryAfter(value string) time.Duration {
	seconds, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil || seconds <= 0 || seconds > int64((24*time.Hour)/time.Second) {
		return 0
	}
	return time.Duration(seconds) * time.Second
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
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func stringPointer(value string) *string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	copy := value
	return &copy
}
