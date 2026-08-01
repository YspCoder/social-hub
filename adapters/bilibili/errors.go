package bilibili

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"social-hub/pkg/socialhub"
)

type responseEnvelope[T any] struct {
	Code      int    `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"request_id"`
	TTL       int    `json:"ttl"`
	Data      T      `json:"data"`
}

func (r responseEnvelope[T]) Err(operation string, status int, header http.Header) error {
	if r.Code == 0 {
		return nil
	}
	code, class := classifyError(r.Code, status)
	return &socialhub.Error{
		Code: code, Class: class, Platform: "bilibili", Product: "open-platform", Op: operation,
		HTTPStatus: status, PlatformCode: strconv.Itoa(r.Code), PlatformMessage: r.Message,
		RequestID: firstNonEmpty(r.RequestID, header.Get("X-Request-ID")), RetryAfter: parseRetryAfter(header.Get("Retry-After")),
	}
}

func decodeHTTPError(status int, header http.Header, body []byte) error {
	var response responseEnvelope[json.RawMessage]
	_ = json.Unmarshal(body, &response)
	if response.Code != 0 {
		return response.Err("http", status, header)
	}
	code, class := classifyError(0, status)
	return &socialhub.Error{Code: code, Class: class, Platform: "bilibili", Product: "open-platform", Op: "http", HTTPStatus: status, RequestID: firstNonEmpty(response.RequestID, header.Get("X-Request-ID")), RetryAfter: parseRetryAfter(header.Get("Retry-After"))}
}

func classifyError(platformCode, status int) (socialhub.ErrorCode, socialhub.ErrorClass) {
	switch platformCode {
	case 4000, 4005, 4006, 4007, 4008, 4009, 122000, 122002, 122008,
		123003, 123006, 123008, 123009, 123010, 123011, 123012, 123013, 123014,
		123015, 123016, 123017, 123018, 123019, 123020, 123021, 123022, 123027,
		123029, 123030, 123033, 123038, 123045, 123046, 123047, 123048, 123049,
		123053, 123054, 123055, 123056:
		return socialhub.CodeInvalidArgument, socialhub.ClassPermanent
	case 4002, 4003, 122001, 122007, 127000, 127001, 127002, 127003, 127004, 127008, 127010, 127022:
		return socialhub.CodeUnauthenticated, socialhub.ClassUserAction
	case 127005, 127006, 127007, 127011, 127304, 127305:
		return socialhub.CodeApprovalRequired, socialhub.ClassUserAction
	case 123001, 123036, 123037, 123050, 123051, 123052:
		return socialhub.CodePermissionDenied, socialhub.ClassUserAction
	case 123004, 123005, 123040, 123041:
		return socialhub.CodeNotFound, socialhub.ClassPermanent
	case 4004, 123007, 123042:
		return socialhub.CodeConflict, socialhub.ClassPermanent
	case 127009, 127306, 123026:
		return socialhub.CodeRateLimited, socialhub.ClassRetryable
	case 4001, 4010, 4011, 122009, 122010, 123002, 123023, 123028, 123039:
		return socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable
	}
	switch status {
	case http.StatusBadRequest, http.StatusUnprocessableEntity:
		return socialhub.CodeInvalidArgument, socialhub.ClassPermanent
	case http.StatusUnauthorized:
		return socialhub.CodeUnauthenticated, socialhub.ClassUserAction
	case http.StatusForbidden:
		return socialhub.CodePermissionDenied, socialhub.ClassUserAction
	case http.StatusNotFound:
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

func parseRetryAfter(value string) time.Duration {
	seconds, err := strconv.Atoi(value)
	if err != nil || seconds < 0 {
		return 0
	}
	return time.Duration(seconds) * time.Second
}

func wrapError(operation string, code socialhub.ErrorCode, class socialhub.ErrorClass, cause error) error {
	return &socialhub.Error{Code: code, Class: class, Platform: "bilibili", Product: "open-platform", Op: operation, Cause: cause}
}

func invalidArgument(operation, message string) error {
	return &socialhub.Error{Code: socialhub.CodeInvalidArgument, Class: socialhub.ClassPermanent, Platform: "bilibili", Product: "open-platform", Op: operation, PlatformMessage: message}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
