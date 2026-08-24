package wechatstore

import (
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"social-hub/pkg/socialhub"
)

type apiResponse struct {
	ErrCode int    `json:"errcode"`
	RID     string `json:"rid"`
}

// APIError deliberately excludes WeChat's errmsg and response body because
// either may echo credentials or commercially sensitive product content.
type APIError struct {
	Hub     *socialhub.Error
	ErrCode int
}

func (value *APIError) Error() string {
	if value == nil || value.Hub == nil {
		return "socialhub: wechat: store-shop: platform_error"
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

func businessError(operation string, status int, response apiResponse, retryAfter time.Duration) error {
	code, class := classifyBusinessError(response.ErrCode)
	requestID := ""
	if validOptionalSensitive(response.RID, maxRequestIDLength) {
		requestID = response.RID
	}
	hub := &socialhub.Error{
		Code: code, Class: class, Platform: platformName, Product: productName, Op: operation,
		HTTPStatus: status, PlatformCode: strconv.Itoa(response.ErrCode),
		PlatformMessage: safeBusinessMessage(response.ErrCode), RequestID: requestID, RetryAfter: retryAfter,
	}
	if code == socialhub.CodeApprovalRequired {
		hub.ApprovalURL = developerGuideURL
	}
	return &APIError{Hub: hub, ErrCode: response.ErrCode}
}

func classifyBusinessError(code int) (socialhub.ErrorCode, socialhub.ErrorClass) {
	switch code {
	case -1:
		return socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable
	case 40001, 40013, 40014, 40125, 42001:
		return socialhub.CodeUnauthenticated, socialhub.ClassUserAction
	case 40002, 40003, 40066, 40097, 41001, 41002, 41004, 41018, 43001, 43002, 43003,
		44002, 45002, 47001, 10020051, 10020207:
		return socialhub.CodeInvalidArgument, socialhub.ClassUserAction
	case 89503, 48001, 50001, 61007, 10020050, 10020177:
		return socialhub.CodeApprovalRequired, socialhub.ClassUserAction
	case 40164, 45035, 48004, 50002, 61004, 89506, 89507:
		return socialhub.CodePermissionDenied, socialhub.ClassUserAction
	case 10020052:
		return socialhub.CodeNotFound, socialhub.ClassPermanent
	case 43119, 43120, 10080000, 10080001, 10080002, 10020065:
		return socialhub.CodeConflict, socialhub.ClassUserAction
	case 45009, 45011:
		return socialhub.CodeRateLimited, socialhub.ClassRetryable
	default:
		return socialhub.CodePlatformError, socialhub.ClassPermanent
	}
}

func safeBusinessMessage(code int) string {
	switch code {
	case 40001, 40014, 42001:
		return "the WeChat access token is invalid, stale, or expired"
	case 40125:
		return "the WeChat Store AppSecret is invalid"
	case 45009, 45011:
		return "the WeChat API quota or rate limit was reached"
	case 48001, 10020050, 10020177:
		return "the store does not currently have permission to call this API"
	case 10020051:
		return "WeChat rejected the product request parameters"
	case 10020052:
		return "the requested product does not exist"
	case 10020065:
		return "the requested product has no online version"
	case 10020207:
		return "this product type does not expose draft data"
	default:
		return "WeChat Store API rejected the request"
	}
}

func httpError(operation string, status int, header http.Header, retryAfter time.Duration) error {
	code, class := socialhub.CodePlatformError, socialhub.ClassPermanent
	switch status {
	case http.StatusBadRequest, http.StatusMethodNotAllowed, http.StatusNotAcceptable,
		http.StatusRequestEntityTooLarge, http.StatusUnsupportedMediaType, http.StatusUnprocessableEntity:
		code = socialhub.CodeInvalidArgument
	case http.StatusUnauthorized:
		code, class = socialhub.CodeUnauthenticated, socialhub.ClassUserAction
	case http.StatusForbidden:
		code, class = socialhub.CodePermissionDenied, socialhub.ClassUserAction
	case http.StatusNotFound:
		code = socialhub.CodeNotFound
	case http.StatusConflict:
		code = socialhub.CodeConflict
	case http.StatusTooManyRequests:
		code, class = socialhub.CodeRateLimited, socialhub.ClassRetryable
	case http.StatusRequestTimeout, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		code, class = socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable
	default:
		if status >= 500 {
			code, class = socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable
		}
	}
	requestID := firstSafeHeader(header, "X-Request-ID", "X-WX-Request-ID")
	return &socialhub.Error{
		Code: code, Class: class, Platform: platformName, Product: productName, Op: operation,
		HTTPStatus: status, PlatformCode: "http_" + strconv.Itoa(status), RequestID: requestID,
		RetryAfter: retryAfter,
	}
}

func authenticationError(operation string, cause error) error {
	return &socialhub.Error{
		Code: socialhub.CodeUnauthenticated, Class: socialhub.ClassUserAction,
		Platform: platformName, Product: productName, Op: operation, Cause: sanitizeCause(cause),
	}
}

func platformError(operation string, code socialhub.ErrorCode, class socialhub.ErrorClass, cause error) error {
	return &socialhub.Error{
		Code: code, Class: class, Platform: platformName, Product: productName, Op: operation,
		Cause: sanitizeCause(cause),
	}
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

func localRateLimitError(operation string, retryAfter time.Duration) error {
	return &socialhub.Error{
		Code: socialhub.CodeRateLimited, Class: socialhub.ClassRetryable,
		Platform: platformName, Product: productName, Op: operation,
		PlatformMessage: "stable-token force refresh requires at least 30 seconds between calls",
		RetryAfter:      retryAfter,
	}
}

func firstSafeHeader(header http.Header, names ...string) string {
	for _, name := range names {
		value := header.Get(name)
		if value != "" && validSensitive(value, maxRequestIDLength) {
			return value
		}
	}
	return ""
}

func parseRetryAfter(value string, now time.Time) time.Duration {
	value = strings.TrimSpace(value)
	if seconds, err := strconv.ParseFloat(value, 64); err == nil && seconds >= 0 && seconds <= float64((24*time.Hour)/time.Second) {
		return time.Duration(seconds * float64(time.Second))
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

func sanitizeCause(err error) error {
	var urlError *url.Error
	if errors.As(err, &urlError) && urlError.Err != nil {
		return urlError.Err
	}
	return err
}
