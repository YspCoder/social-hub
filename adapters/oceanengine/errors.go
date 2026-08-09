package oceanengine

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

type apiEnvelope[T any] struct {
	Code      *int64 `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"request_id"`
	Data      *T     `json:"data"`
}

func requireEnvelope[T any](operation string, envelope apiEnvelope[T]) (*T, error) {
	if envelope.Code == nil {
		return nil, platformContractError(operation, "Ocean Engine response omitted code")
	}
	if *envelope.Code != 0 {
		return nil, businessError(operation, *envelope.Code, envelope.Message, envelope.RequestID)
	}
	if envelope.Data == nil {
		return nil, platformContractError(operation, "Ocean Engine success response omitted data")
	}
	return envelope.Data, nil
}

func businessError(operation string, code int64, message, requestID string) error {
	return &socialhub.Error{
		Code: socialhub.CodePlatformError, Class: socialhub.ClassPermanent,
		Platform: platformName, Product: productName, Op: operation,
		PlatformCode: strconv.FormatInt(code, 10), PlatformMessage: boundedMessage(message, 512),
		RequestID: boundedMessage(requestID, 256),
	}
}

func decodeHTTPError(status int, header http.Header, body []byte) error {
	var envelope struct {
		Code      *int64 `json:"code"`
		Message   string `json:"message"`
		RequestID string `json:"request_id"`
	}
	_ = json.Unmarshal(body, &envelope)
	code, class := classifyHTTPError(status)
	platformCode := ""
	if envelope.Code != nil {
		platformCode = strconv.FormatInt(*envelope.Code, 10)
	}
	return &socialhub.Error{
		Code: code, Class: class, Platform: platformName, Product: productName, HTTPStatus: status,
		PlatformCode: boundedMessage(platformCode, 128), PlatformMessage: boundedMessage(envelope.Message, 512),
		RequestID:  boundedMessage(firstNonEmpty(envelope.RequestID, header.Get("X-Request-ID")), 256),
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

func mutationError(operation string, failure providerMutationError, requestID string) error {
	code := failure.ErrorCode
	if code == 0 {
		code = -1
	}
	err := businessError(operation, code, failure.ErrorMessage, requestID)
	return err
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
