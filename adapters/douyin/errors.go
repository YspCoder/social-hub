package douyin

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"social-hub/pkg/socialhub"
)

type flexibleInt64 int64

func (value *flexibleInt64) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if len(data) == 0 || bytes.Equal(data, []byte("null")) {
		*value = 0
		return nil
	}
	if data[0] == '"' {
		var text string
		if err := json.Unmarshal(data, &text); err != nil {
			return err
		}
		parsed, err := strconv.ParseInt(text, 10, 64)
		if err != nil {
			return err
		}
		*value = flexibleInt64(parsed)
		return nil
	}
	parsed, err := strconv.ParseInt(string(data), 10, 64)
	if err != nil {
		return err
	}
	*value = flexibleInt64(parsed)
	return nil
}

type APIResponse struct {
	ErrorCode   flexibleInt64 `json:"error_code"`
	Description string        `json:"description"`
}

type responseExtra struct {
	APIResponse
	SubErrorCode   flexibleInt64 `json:"sub_error_code"`
	SubDescription string        `json:"sub_description"`
	LogID          string        `json:"logid"`
	LogIDAlt       string        `json:"log_id"`
}

func (r APIResponse) Err(operation string, extra responseExtra, status int, header http.Header) error {
	if r.ErrorCode == 0 {
		return nil
	}
	code, class := classifyError(int64(r.ErrorCode), status)
	return &socialhub.Error{
		Code:            code,
		Class:           class,
		Platform:        "douyin",
		Product:         "openapi",
		Op:              operation,
		HTTPStatus:      status,
		PlatformCode:    strconv.FormatInt(int64(r.ErrorCode), 10),
		PlatformMessage: firstNonEmpty(r.Description, extra.SubDescription, extra.Description),
		RequestID:       firstNonEmpty(extra.LogID, extra.LogIDAlt, header.Get("X-Tt-Logid"), header.Get("X-Request-ID")),
		RetryAfter:      parseRetryAfter(header.Get("Retry-After")),
	}
}

func responseError(data APIResponse, extra responseExtra, operation string, status int, header http.Header) error {
	if data.ErrorCode != 0 {
		return data.Err(operation, extra, status, header)
	}
	return extra.APIResponse.Err(operation, extra, status, header)
}

func decodeHTTPError(status int, header http.Header, body []byte) error {
	var response struct {
		Data  APIResponse   `json:"data"`
		Extra responseExtra `json:"extra"`
	}
	_ = json.Unmarshal(body, &response)
	provider := response.Data
	if provider.ErrorCode == 0 {
		provider = response.Extra.APIResponse
	}
	if provider.ErrorCode != 0 {
		return provider.Err("http", response.Extra, status, header)
	}
	code, class := classifyError(0, status)
	return &socialhub.Error{Code: code, Class: class, Platform: "douyin", Product: "openapi", Op: "http", HTTPStatus: status, RequestID: firstNonEmpty(response.Extra.LogID, response.Extra.LogIDAlt, header.Get("X-Tt-Logid")), RetryAfter: parseRetryAfter(header.Get("Retry-After"))}
}

func classifyError(platformCode int64, status int) (socialhub.ErrorCode, socialhub.ErrorClass) {
	switch platformCode {
	case 10002, 10003, 10005, 10006, 2100005, 2190015, 2190005, 2190006, 2190007:
		return socialhub.CodeInvalidArgument, socialhub.ClassPermanent
	case 10007, 10008, 10010, 10013, 10014, 2190002, 2190008:
		return socialhub.CodeUnauthenticated, socialhub.ClassUserAction
	case 10004, 10012, 2190003, 2190004:
		return socialhub.CodeApprovalRequired, socialhub.ClassUserAction
	case 2100007, 2100009, 2190016, 2114005:
		return socialhub.CodePermissionDenied, socialhub.ClassUserAction
	case 2190001:
		return socialhub.CodeRateLimited, socialhub.ClassRetryable
	case 2114007:
		return socialhub.CodeRateLimited, socialhub.ClassUserAction
	case 2100004:
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
	return &socialhub.Error{Code: code, Class: class, Platform: "douyin", Product: "openapi", Op: operation, Cause: cause}
}

func invalidArgument(operation, message string) error {
	return &socialhub.Error{Code: socialhub.CodeInvalidArgument, Class: socialhub.ClassPermanent, Platform: "douyin", Product: "openapi", Op: operation, PlatformMessage: message}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
