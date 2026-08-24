package criteo

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

// Problem is Criteo's JSON:API/RFC 7807-style error detail.
type Problem struct {
	TraceID  string `json:"traceId,omitempty"`
	Type     string `json:"type,omitempty"`
	Code     string `json:"code,omitempty"`
	Instance string `json:"instance,omitempty"`
	Title    string `json:"title,omitempty"`
	Detail   string `json:"detail,omitempty"`
}

// RateLimit contains Criteo's per-response quota state when supplied.
type RateLimit struct {
	Limit     int
	Remaining int
	ResetAt   time.Time
}

// APIError augments socialhub.Error with Criteo problem and quota details.
type APIError struct {
	Hub       *socialhub.Error
	Problems  []Problem
	RateLimit RateLimit
}

func (err *APIError) Error() string {
	if err == nil || err.Hub == nil {
		return "socialhub: criteo: platform_error"
	}
	return err.Hub.Error()
}

func (err *APIError) Unwrap() error {
	if err == nil {
		return nil
	}
	return err.Hub
}

func (err *APIError) Retryable() bool {
	return err != nil && err.Hub != nil && err.Hub.Retryable()
}

func decodeHTTPError(status int, header http.Header, body []byte) error {
	var envelope struct {
		Errors []Problem `json:"errors"`
	}
	_ = json.Unmarshal(body, &envelope)
	return newAPIError(status, header, envelope.Errors)
}

func newAPIError(status int, header http.Header, problems []Problem) error {
	code, class := classifyHTTPError(status)
	if len(problems) > 0 && strings.TrimSpace(problems[0].Type) != "" {
		problemCode, problemClass := classifyProblem(problems[0].Type)
		if problemCode != socialhub.CodePlatformError || status >= 200 && status < 300 {
			code, class = problemCode, problemClass
		}
	}
	sanitized := sanitizeProblems(problems)
	var platformCode, message, traceID string
	if len(sanitized) > 0 {
		platformCode = firstNonEmpty(sanitized[0].Code, sanitized[0].Type)
		message = firstNonEmpty(sanitized[0].Detail, sanitized[0].Title)
		traceID = sanitized[0].TraceID
	}
	return &APIError{
		Hub: &socialhub.Error{
			Code: code, Class: class, Platform: platformName, Product: productName, HTTPStatus: status,
			PlatformCode: platformCode, PlatformMessage: message,
			RequestID:  boundedMessage(firstNonEmpty(traceID, header.Get("x-request-id"), header.Get("x-correlation-id")), 256),
			RetryAfter: retryDelay(header),
		},
		Problems:  sanitized,
		RateLimit: parseRateLimit(header),
	}
}

func businessError(operation string, problems []Problem) error {
	err := newAPIError(http.StatusOK, nil, problems)
	var api *APIError
	if errors.As(err, &api) && api.Hub != nil {
		api.Hub.Op = operation
	}
	return err
}

func classifyProblem(problemType string) (socialhub.ErrorCode, socialhub.ErrorClass) {
	switch strings.ToLower(strings.TrimSpace(problemType)) {
	case "authentication":
		return socialhub.CodeUnauthenticated, socialhub.ClassUserAction
	case "access-control", "access_control", "authorization":
		return socialhub.CodePermissionDenied, socialhub.ClassUserAction
	case "availability":
		return socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable
	case "quota":
		return socialhub.CodeRateLimited, socialhub.ClassRetryable
	case "validation":
		return socialhub.CodeInvalidArgument, socialhub.ClassPermanent
	default:
		return socialhub.CodePlatformError, socialhub.ClassPermanent
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
	return &socialhub.Error{
		Code: code, Class: class, Platform: platformName, Product: productName,
		Op: operation, Cause: sanitizeCause(cause),
	}
}

func invalidArgument(operation, message string) error {
	return &socialhub.Error{
		Code: socialhub.CodeInvalidArgument, Class: socialhub.ClassPermanent,
		Platform: platformName, Product: productName, Op: operation,
		PlatformMessage: message,
	}
}

func ownershipError(operation string) error {
	return &socialhub.Error{
		Code: socialhub.CodePermissionDenied, Class: socialhub.ClassUserAction,
		Platform: platformName, Product: productName, Op: operation,
		PlatformMessage: "resource is not owned by the configured advertiser",
	}
}

func platformContractError(operation, message string) error {
	return &socialhub.Error{
		Code: socialhub.CodePlatformError, Class: socialhub.ClassPermanent,
		Platform: platformName, Product: productName, Op: operation,
		PlatformMessage: message,
	}
}

func conflictError(operation, message string) error {
	return &socialhub.Error{
		Code: socialhub.CodeConflict, Class: socialhub.ClassPermanent,
		Platform: platformName, Product: productName, Op: operation,
		PlatformMessage: message,
	}
}

func retryDelay(header http.Header) time.Duration {
	if header == nil {
		return 0
	}
	if value := strings.TrimSpace(header.Get("Retry-After")); value != "" {
		if seconds, err := strconv.ParseFloat(value, 64); err == nil && seconds >= 0 && seconds <= float64((24*time.Hour)/time.Second) {
			return time.Duration(seconds * float64(time.Second))
		}
		if when, err := http.ParseTime(value); err == nil {
			return boundedDelay(time.Until(when))
		}
	}
	if reset, err := strconv.ParseInt(strings.TrimSpace(header.Get("x-ratelimit-reset")), 10, 64); err == nil && reset > 0 {
		return boundedDelay(time.Until(time.Unix(reset, 0)))
	}
	return 0
}

func boundedDelay(delay time.Duration) time.Duration {
	if delay < 0 || delay > 24*time.Hour {
		return 0
	}
	return delay
}

func parseRateLimit(header http.Header) RateLimit {
	if header == nil {
		return RateLimit{}
	}
	limit, _ := strconv.Atoi(strings.TrimSpace(header.Get("x-ratelimit-limit")))
	remaining, _ := strconv.Atoi(strings.TrimSpace(header.Get("x-ratelimit-remaining")))
	reset, _ := strconv.ParseInt(strings.TrimSpace(header.Get("x-ratelimit-reset")), 10, 64)
	result := RateLimit{Limit: limit, Remaining: remaining}
	if reset > 0 {
		result.ResetAt = time.Unix(reset, 0)
	}
	return result
}

func sanitizeProblems(problems []Problem) []Problem {
	result := make([]Problem, 0, len(problems))
	for _, problem := range problems {
		result = append(result, Problem{
			TraceID:  boundedMessage(problem.TraceID, 256),
			Type:     boundedMessage(redactSensitive(problem.Type), 128),
			Code:     boundedMessage(redactSensitive(problem.Code), 256),
			Instance: boundedMessage(problem.Instance, 512),
			Title:    boundedMessage(redactSensitive(problem.Title), 512),
			Detail:   boundedMessage(redactSensitive(problem.Detail), 1024),
		})
	}
	return result
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
	for _, marker := range []string{"client_secret", "clientsecret", "access_token", "accesstoken", "authorization", "bearer"} {
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
			if marker == "authorization" || marker == "bearer" {
				stops = "\r\n,;}&"
			}
			for end < len(value) && !strings.ContainsRune(stops, rune(value[end])) {
				end++
			}
			value = value[:start] + "[REDACTED]" + value[end:]
			cursor = start + len("[REDACTED]")
		}
	}
	return value
}
