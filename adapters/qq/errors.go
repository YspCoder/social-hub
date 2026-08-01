package qq

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

// APIError is the error envelope used by QQ OpenAPI and its token endpoint.
type APIError struct {
	ErrCode int    `json:"err_code"`
	Code    int    `json:"code"`
	Message string `json:"message"`
	TraceID string `json:"trace_id"`
}

func (e APIError) EffectiveCode() int {
	if e.ErrCode != 0 {
		return e.ErrCode
	}
	return e.Code
}

func (e APIError) Err(operation string) error {
	codeValue := e.EffectiveCode()
	if codeValue == 0 {
		return nil
	}
	code, class := classifyError(0, codeValue)
	err := &socialhub.Error{
		Code: code, Class: class, Platform: "qq", Product: productName, Op: operation,
		PlatformCode: strconv.Itoa(codeValue), PlatformMessage: boundedMessage(e.Message, 512),
		RequestID: boundedMessage(e.TraceID, 512),
	}
	if code == socialhub.CodeApprovalRequired {
		err.ApprovalURL = "https://q.qq.com/"
	}
	return err
}

func decodeHTTPError(status int, header http.Header, body []byte) error {
	var response APIError
	if json.Unmarshal(body, &response) == nil && response.EffectiveCode() != 0 {
		err := response.Err("http").(*socialhub.Error)
		err.HTTPStatus = status
		if err.RequestID == "" {
			err.RequestID = requestID(header)
		}
		err.RetryAfter = retryAfter(header.Get("Retry-After"))
		return err
	}
	code, class := classifyError(status, 0)
	return &socialhub.Error{
		Code: code, Class: class, Platform: "qq", Product: productName, Op: "http",
		HTTPStatus: status, RequestID: requestID(header), RetryAfter: retryAfter(header.Get("Retry-After")),
	}
}

func classifyError(status, platformCode int) (socialhub.ErrorCode, socialhub.ErrorClass) {
	switch platformCode {
	case 100007, 100016, 11241, 11242, 11243, 11251, 11261:
		return socialhub.CodeUnauthenticated, socialhub.ClassUserAction
	case 11253, 11254, 304004, 304036, 304037, 40034105:
		return socialhub.CodeApprovalRequired, socialhub.ClassUserAction
	case 10001, 10003, 10004, 503012:
		return socialhub.CodeNotFound, socialhub.ClassPermanent
	case 11282, 11264, 11274, 306004, 620005, 40054004, 40054013:
		return socialhub.CodePermissionDenied, socialhub.ClassUserAction
	case 100001, 20028, 304019, 304045, 304049, 304050, 503004, 610013, 620006, 1100100, 1100308, 40034100, 40034122, 40034128, 40093002:
		return socialhub.CodeRateLimited, socialhub.ClassRetryable
	case 11281, 11252, 11263, 306003, 306005, 306006, 40093001, 850026, 850027, 1100300, 50055002:
		return socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable
	case 12002, 22006, 50006, 50035, 50037, 50041, 50042, 50048, 50054, 50055, 50056, 50057,
		620001, 850019, 850031, 304061, 304080, 40034005, 40034024, 40034025, 40034026,
		40034027, 40034029, 40054005, 40054007, 40054018:
		return socialhub.CodeInvalidArgument, socialhub.ClassPermanent
	}
	switch status {
	case http.StatusBadRequest, http.StatusMethodNotAllowed, http.StatusRequestEntityTooLarge, http.StatusUnprocessableEntity:
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
	return &socialhub.Error{Code: code, Class: class, Platform: "qq", Product: productName, Op: operation, Cause: cause}
}

func invalidArgument(operation, message string) error {
	return &socialhub.Error{Code: socialhub.CodeInvalidArgument, Class: socialhub.ClassPermanent, Platform: "qq", Product: productName, Op: operation, PlatformMessage: message}
}

func unsupported(operation, message string) error {
	return &socialhub.Error{Code: socialhub.CodeUnsupported, Class: socialhub.ClassPermanent, Platform: "qq", Product: productName, Op: operation, PlatformMessage: message}
}

func retryAfter(value string) time.Duration {
	seconds, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil || seconds <= 0 || seconds > int64((24*time.Hour)/time.Second) {
		return 0
	}
	return time.Duration(seconds) * time.Second
}

func requestID(header http.Header) string {
	for _, name := range []string{"X-Tps-Trace-Id", "X-Request-Id", "X-Correlation-Id"} {
		if value := strings.TrimSpace(header.Get(name)); value != "" {
			return boundedMessage(value, 512)
		}
	}
	return ""
}

func boundedMessage(value string, maximum int) string {
	if utf8.RuneCountInString(value) <= maximum {
		return value
	}
	return string([]rune(value)[:maximum])
}
