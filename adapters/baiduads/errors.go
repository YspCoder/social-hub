package baiduads

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

type apiFailure struct {
	Code     int64  `json:"code"`
	Field    string `json:"field,omitempty"`
	Position string `json:"position,omitempty"`
	ID       int64  `json:"id,omitempty"`
	Message  string `json:"message"`
}

type responseHeader struct {
	Status   *int         `json:"status"`
	Desc     string       `json:"desc"`
	TraceID  string       `json:"traceid"`
	Failures []apiFailure `json:"failures"`
	Quota    int          `json:"quota"`
	RQuota   int          `json:"rquota"`
	Oprs     int          `json:"oprs"`
	OprTime  int          `json:"oprtime"`
}

type responseBody[T any] struct {
	Data *T `json:"data"`
}

type apiEnvelope[T any] struct {
	Header *responseHeader  `json:"header"`
	Body   *responseBody[T] `json:"body"`
}

func requireEnvelope[T any](operation string, envelope apiEnvelope[T], httpHeader http.Header) (*T, error) {
	requestID := boundedMessage(firstNonEmpty(httpHeader.Get("X-B3-Traceid"), headerTraceID(envelope.Header)), 256)
	if envelope.Header == nil || envelope.Header.Status == nil {
		return nil, platformContractError(operation, "Baidu Ads response omitted header.status")
	}
	if *envelope.Header.Status != 0 || len(envelope.Header.Failures) > 0 {
		return nil, businessHeaderError(operation, *envelope.Header, requestID)
	}
	if envelope.Body == nil || envelope.Body.Data == nil {
		return nil, platformContractError(operation, "Baidu Ads success response omitted body.data")
	}
	return envelope.Body.Data, nil
}

func businessHeaderError(operation string, header responseHeader, requestID string) error {
	failure := apiFailure{Message: header.Desc}
	if len(header.Failures) > 0 {
		failure = header.Failures[0]
	}
	code, class, retryAfter := classifyBusinessError(failure.Code, header.Status)
	return &socialhub.Error{
		Code: code, Class: class, Platform: platformName, Product: productName, Op: operation,
		PlatformCode: strconv.FormatInt(failure.Code, 10), PlatformMessage: boundedMessage(failure.Message, 512),
		RequestID: requestID, RetryAfter: retryAfter,
	}
}

func classifyBusinessError(code int64, status *int) (socialhub.ErrorCode, socialhub.ErrorClass, time.Duration) {
	switch code {
	case 8401, 8402, 8403, 8408, 8410, 8412, 8414, 8415, 8423,
		84260, 84261, 84262, 84263, 84270, 84271, 89403, 89405, 89406, 89407, 894061, 894062, 894063, 894064:
		return socialhub.CodeUnauthenticated, socialhub.ClassUserAction, 0
	case 8104, 8303, 8406, 8407, 8409, 8411, 841210, 9012, 90121, 90122, 90123, 90124, 901913:
		return socialhub.CodePermissionDenied, socialhub.ClassUserAction, 0
	case 8501, 8502:
		return socialhub.CodeRateLimited, socialhub.ClassRetryable, time.Second
	case 9011028, 901160, 901245:
		return socialhub.CodeRateLimited, socialhub.ClassRetryable, time.Minute
	}
	if status != nil && *status == 3 {
		return socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, 0
	}
	return socialhub.CodePlatformError, socialhub.ClassPermanent, 0
}

func decodeHTTPError(status int, header http.Header, body []byte) error {
	var envelope struct {
		Header  *responseHeader `json:"header"`
		Code    *int64          `json:"code"`
		Message string          `json:"message"`
	}
	_ = json.Unmarshal(body, &envelope)
	code, class := classifyHTTPError(status)
	platformCode := ""
	platformMessage := envelope.Message
	if envelope.Header != nil && len(envelope.Header.Failures) > 0 {
		failure := envelope.Header.Failures[0]
		platformCode = strconv.FormatInt(failure.Code, 10)
		platformMessage = failure.Message
	} else if envelope.Code != nil {
		platformCode = strconv.FormatInt(*envelope.Code, 10)
	}
	return &socialhub.Error{
		Code: code, Class: class, Platform: platformName, Product: productName, HTTPStatus: status,
		PlatformCode: boundedMessage(platformCode, 128), PlatformMessage: boundedMessage(platformMessage, 512),
		RequestID:  boundedMessage(firstNonEmpty(header.Get("X-B3-Traceid"), headerTraceID(envelope.Header)), 256),
		RetryAfter: parseRetryAfter(header.Get("Retry-After")),
	}
}

func classifyHTTPError(status int) (socialhub.ErrorCode, socialhub.ErrorClass) {
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
	return &socialhub.Error{Code: code, Class: class, Platform: platformName, Product: productName, Op: operation, Cause: cause}
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
	seconds, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err == nil && seconds >= 0 && seconds <= float64((24*time.Hour)/time.Second) {
		return time.Duration(seconds * float64(time.Second))
	}
	return 0
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

func headerTraceID(header *responseHeader) string {
	if header == nil {
		return ""
	}
	return header.TraceID
}
