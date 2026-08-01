package telegram

import (
	"context"
	"errors"
	"io"
	"net"
	"net/url"
	"strconv"
	"time"

	tgbot "github.com/go-telegram/bot"

	"social-hub/pkg/socialhub"
)

var errUploadTooLarge = errors.New("telegram upload exceeds documented limit")
var errResponseTooLarge = errors.New("telegram response exceeds size limit")

func mapError(operation string, err error) error {
	if err == nil {
		return nil
	}
	var rateLimit *tgbot.TooManyRequestsError
	if errors.As(err, &rateLimit) {
		return &socialhub.Error{
			Code: socialhub.CodeRateLimited, Class: socialhub.ClassRetryable, Platform: "telegram", Product: "bot-api", Op: operation,
			HTTPStatus: 429, PlatformCode: "429", RetryAfter: time.Duration(rateLimit.RetryAfter) * time.Second,
		}
	}
	var migration *tgbot.MigrateError
	if errors.As(err, &migration) {
		return &socialhub.Error{
			Code: socialhub.CodeConflict, Class: socialhub.ClassUserAction, Platform: "telegram", Product: "bot-api", Op: operation,
			HTTPStatus: 400, PlatformCode: "400", PlatformMessage: "chat migrated to " + strconv.Itoa(migration.MigrateToChatID), Cause: err,
		}
	}
	switch {
	case errors.Is(err, errUploadTooLarge):
		return wrapError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	case errors.Is(err, errResponseTooLarge):
		return wrapError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	case errors.Is(err, tgbot.ErrorBadRequest):
		return &socialhub.Error{Code: socialhub.CodeInvalidArgument, Class: socialhub.ClassPermanent, Platform: "telegram", Product: "bot-api", Op: operation, HTTPStatus: 400, PlatformCode: "400", Cause: err}
	case errors.Is(err, tgbot.ErrorUnauthorized):
		return &socialhub.Error{Code: socialhub.CodeUnauthenticated, Class: socialhub.ClassUserAction, Platform: "telegram", Product: "bot-api", Op: operation, HTTPStatus: 401, PlatformCode: "401", Cause: err}
	case errors.Is(err, tgbot.ErrorForbidden):
		return &socialhub.Error{Code: socialhub.CodePermissionDenied, Class: socialhub.ClassUserAction, Platform: "telegram", Product: "bot-api", Op: operation, HTTPStatus: 403, PlatformCode: "403", Cause: err}
	case errors.Is(err, tgbot.ErrorNotFound):
		return &socialhub.Error{Code: socialhub.CodeNotFound, Class: socialhub.ClassPermanent, Platform: "telegram", Product: "bot-api", Op: operation, HTTPStatus: 404, PlatformCode: "404", Cause: err}
	case errors.Is(err, tgbot.ErrorConflict):
		return &socialhub.Error{Code: socialhub.CodeConflict, Class: socialhub.ClassUserAction, Platform: "telegram", Product: "bot-api", Op: operation, HTTPStatus: 409, PlatformCode: "409", Cause: err}
	case errors.Is(err, context.Canceled):
		return wrapError(operation, socialhub.CodeTemporarilyUnavailable, socialhub.ClassUserAction, err)
	case errors.Is(err, context.DeadlineExceeded):
		return wrapError(operation, socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
	}
	var networkError net.Error
	var urlError *url.Error
	if errors.As(err, &networkError) || errors.As(err, &urlError) {
		return wrapError(operation, socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
	}
	return wrapError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, err)
}

func wrapError(operation string, code socialhub.ErrorCode, class socialhub.ErrorClass, cause error) error {
	return &socialhub.Error{Code: code, Class: class, Platform: "telegram", Product: "bot-api", Op: operation, Cause: cause}
}

func invalidArgument(operation, message string) error {
	return &socialhub.Error{Code: socialhub.CodeInvalidArgument, Class: socialhub.ClassPermanent, Platform: "telegram", Product: "bot-api", Op: operation, PlatformMessage: message}
}

func unsupported(operation, message string) error {
	return &socialhub.Error{Code: socialhub.CodeUnsupported, Class: socialhub.ClassPermanent, Platform: "telegram", Product: "bot-api", Op: operation, PlatformMessage: message}
}

type limitErrorReader struct {
	reader  io.Reader
	maximum int64
	read    int64
}

func (r *limitErrorReader) Read(buffer []byte) (int, error) {
	remaining := r.maximum - r.read
	if remaining < 0 {
		return 0, errResponseTooLarge
	}
	if int64(len(buffer)) > remaining+1 {
		buffer = buffer[:remaining+1]
	}
	n, err := r.reader.Read(buffer)
	r.read += int64(n)
	if r.read > r.maximum {
		return 0, errResponseTooLarge
	}
	return n, err
}

type boundedReadCloser struct {
	io.Reader
	io.Closer
}
