package toutiao

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

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

type apiResponse struct {
	ErrorCode   flexibleInt64 `json:"error_code"`
	Description string        `json:"description"`
}

type responseExtra struct {
	apiResponse
	SubErrorCode   flexibleInt64 `json:"sub_error_code"`
	SubDescription string        `json:"sub_description"`
	LogID          string        `json:"logid"`
	LogIDAlt       string        `json:"log_id"`
}

func responseError(data apiResponse, extra responseExtra, operation string, status int, header http.Header) error {
	provider := data
	if provider.ErrorCode == 0 {
		provider = extra.apiResponse
	}
	if provider.ErrorCode == 0 {
		return nil
	}
	code, class := classifyError(int64(provider.ErrorCode), status)
	platformCode := strconv.FormatInt(int64(provider.ErrorCode), 10)
	if extra.SubErrorCode != 0 {
		platformCode += "/" + strconv.FormatInt(int64(extra.SubErrorCode), 10)
	}
	return &socialhub.Error{
		Code: code, Class: class, Platform: platformName, Product: productName, Op: operation,
		HTTPStatus: status, PlatformCode: bounded(platformCode, 128),
		PlatformMessage: bounded(firstNonEmpty(provider.Description, extra.SubDescription, extra.Description), 512),
		RequestID:       bounded(firstNonEmpty(extra.LogID, extra.LogIDAlt, headerValue(header, "X-Tt-Logid"), headerValue(header, "X-Request-ID")), 256),
		RetryAfter:      parseRetryAfter(headerValue(header, "Retry-After")),
		ApprovalURL:     approvalURL(code),
	}
}

func decodeHTTPError(status int, header http.Header, body []byte) error {
	var envelope struct {
		Data  apiResponse   `json:"data"`
		Extra responseExtra `json:"extra"`
	}
	if json.Unmarshal(body, &envelope) == nil {
		if err := responseError(envelope.Data, envelope.Extra, "http", status, header); err != nil {
			return err
		}
	}
	code, class := classifyError(0, status)
	return &socialhub.Error{
		Code: code, Class: class, Platform: platformName, Product: productName, Op: "http",
		HTTPStatus: status, RequestID: bounded(firstNonEmpty(headerValue(header, "X-Tt-Logid"), headerValue(header, "X-Request-ID")), 256),
		RetryAfter: parseRetryAfter(headerValue(header, "Retry-After")),
	}
}

func classifyError(platformCode int64, status int) (socialhub.ErrorCode, socialhub.ErrorClass) {
	switch platformCode {
	case 10002, 10003, 10005, 10006, 2100005, 2190005, 2190006, 2190007, 2190015:
		return socialhub.CodeInvalidArgument, socialhub.ClassPermanent
	case 10007, 10008, 10010, 10013, 10014, 2190002, 2190008:
		return socialhub.CodeUnauthenticated, socialhub.ClassUserAction
	case 10004, 10012, 2190003, 2190004:
		return socialhub.CodeApprovalRequired, socialhub.ClassUserAction
	case 2100007, 2100009, 2114005, 2190016:
		return socialhub.CodePermissionDenied, socialhub.ClassUserAction
	case 2190001:
		return socialhub.CodeRateLimited, socialhub.ClassRetryable
	case 2114007:
		return socialhub.CodeRateLimited, socialhub.ClassUserAction
	case 2100004:
		return socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable
	}
	switch status {
	case http.StatusBadRequest, http.StatusRequestEntityTooLarge, http.StatusUnprocessableEntity:
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
	return &socialhub.Error{Code: code, Class: class, Platform: platformName, Product: productName, Op: operation, Cause: cause}
}

func invalidArgument(operation, message string) error {
	return &socialhub.Error{
		Code: socialhub.CodeInvalidArgument, Class: socialhub.ClassPermanent,
		Platform: platformName, Product: productName, Op: operation, PlatformMessage: bounded(message, 512),
	}
}

func invalidPlatformResponse(operation, message string) error {
	return &socialhub.Error{
		Code: socialhub.CodePlatformError, Class: socialhub.ClassPermanent,
		Platform: platformName, Product: productName, Op: operation, PlatformMessage: bounded(message, 512),
	}
}

func approvalURL(code socialhub.ErrorCode) string {
	if code == socialhub.CodeApprovalRequired {
		return "https://open.douyin.com/platform/management/"
	}
	return ""
}

func parseRetryAfter(value string) time.Duration {
	seconds, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil || seconds <= 0 || seconds > int64((24*time.Hour)/time.Second) {
		return 0
	}
	return time.Duration(seconds) * time.Second
}

func bounded(value string, maximum int) string {
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

func headerValue(header http.Header, name string) string {
	if header == nil {
		return ""
	}
	return header.Get(name)
}
