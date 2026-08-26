package mintegral

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	mtg "github.com/jageros/mintegral-go"

	"social-hub/pkg/socialhub"
)

func mapError(operation string, err error) error {
	if err == nil {
		return nil
	}
	var hub *socialhub.Error
	if errors.As(err, &hub) {
		return err
	}
	if errors.Is(err, mtg.ErrOutcomeUnknown) {
		return fixedError(operation, socialhub.CodePlatformError, socialhub.ClassUserAction,
			"Mintegral write outcome is unknown; reconcile remote state before retrying", err)
	}
	if errors.Is(err, mtg.ErrPartialDelivery) {
		return fixedError(operation, socialhub.CodePlatformError, socialhub.ClassUserAction,
			"Mintegral report delivery stopped after one or more acknowledged batches", err)
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return platformError(operation, socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
	}
	if errors.Is(err, mtg.ErrCredentialsRequired) || errors.Is(err, mtg.ErrInvalidCredentials) {
		return platformError(operation, socialhub.CodeUnauthenticated, socialhub.ClassUserAction, nil)
	}
	if errors.Is(err, mtg.ErrPermissionDenied) {
		return platformError(operation, socialhub.CodePermissionDenied, socialhub.ClassUserAction, nil)
	}
	if errors.Is(err, mtg.ErrRateLimited) {
		return apiError(operation, err, socialhub.CodeRateLimited, socialhub.ClassRetryable)
	}
	if errors.Is(err, mtg.ErrInvalidRequest) {
		return platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	if errors.Is(err, mtg.ErrUploadExpired) {
		return fixedError(operation, socialhub.CodeConflict, socialhub.ClassUserAction,
			"Mintegral audience upload plan expired before completion", err)
	}
	if errors.Is(err, mtg.ErrReportTimeout) {
		return fixedError(operation, socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable,
			"Mintegral report did not become available before the configured deadline", err)
	}
	if errors.Is(err, mtg.ErrTransport) {
		return platformError(operation, socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
	}
	if errors.Is(err, mtg.ErrInvalidReport) || errors.Is(err, mtg.ErrUnexpectedResponse) {
		return platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	var upstream *mtg.APIError
	if errors.As(err, &upstream) {
		code, class := classifyHTTPStatus(upstream.HTTPStatus)
		if upstream.Code >= 10_000 && upstream.Code < 20_000 {
			code, class = socialhub.CodeInvalidArgument, socialhub.ClassPermanent
		}
		return apiError(operation, err, code, class)
	}
	if errors.Is(err, mtg.ErrAPI) {
		return platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, err)
}

func apiError(operation string, err error, code socialhub.ErrorCode, class socialhub.ErrorClass) error {
	result := &socialhub.Error{
		Code: code, Class: class, Platform: platformName, Product: productName, Op: operation,
		PlatformMessage: "Mintegral rejected the AppGrowth Open API request",
	}
	var upstream *mtg.APIError
	if errors.As(err, &upstream) {
		result.HTTPStatus = upstream.HTTPStatus
		result.RetryAfter = upstream.RetryAfter
		if upstream.Code != 0 {
			result.PlatformCode = strconv.Itoa(upstream.Code)
		}
	}
	return result
}

func classifyHTTPStatus(status int) (socialhub.ErrorCode, socialhub.ErrorClass) {
	switch status {
	case http.StatusBadRequest, http.StatusRequestEntityTooLarge, http.StatusUnsupportedMediaType, http.StatusUnprocessableEntity:
		return socialhub.CodeInvalidArgument, socialhub.ClassPermanent
	case http.StatusUnauthorized:
		return socialhub.CodeUnauthenticated, socialhub.ClassUserAction
	case http.StatusPaymentRequired:
		return socialhub.CodeApprovalRequired, socialhub.ClassUserAction
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
		Op: operation, Cause: cause,
	}
}

func fixedError(operation string, code socialhub.ErrorCode, class socialhub.ErrorClass, message string, cause error) error {
	return &socialhub.Error{
		Code: code, Class: class, Platform: platformName, Product: productName,
		Op: operation, PlatformMessage: message, Cause: cause,
	}
}

func invalidArgument(operation, message string) error {
	return fixedError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, message, nil)
}
