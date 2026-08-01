package tumblr

import (
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

func TestTumblrHTTPErrorMapping(t *testing.T) {
	cases := []struct {
		status int
		code   socialhub.ErrorCode
		target error
		class  socialhub.ErrorClass
	}{
		{http.StatusBadRequest, socialhub.CodeInvalidArgument, socialhub.ErrInvalidArgument, socialhub.ClassPermanent},
		{http.StatusUnprocessableEntity, socialhub.CodeInvalidArgument, socialhub.ErrInvalidArgument, socialhub.ClassPermanent},
		{http.StatusUnauthorized, socialhub.CodeUnauthenticated, socialhub.ErrUnauthenticated, socialhub.ClassUserAction},
		{http.StatusForbidden, socialhub.CodePermissionDenied, socialhub.ErrPermissionDenied, socialhub.ClassUserAction},
		{http.StatusNotFound, socialhub.CodeNotFound, socialhub.ErrNotFound, socialhub.ClassPermanent},
		{http.StatusGone, socialhub.CodeNotFound, socialhub.ErrNotFound, socialhub.ClassPermanent},
		{http.StatusConflict, socialhub.CodeConflict, socialhub.ErrConflict, socialhub.ClassPermanent},
		{http.StatusTooManyRequests, socialhub.CodeRateLimited, socialhub.ErrRateLimited, socialhub.ClassRetryable},
		{http.StatusServiceUnavailable, socialhub.CodeTemporarilyUnavailable, socialhub.ErrUnavailable, socialhub.ClassRetryable},
	}
	for _, test := range cases {
		header := http.Header{"X-Tumblr-Request-Id": {"request-1"}, "Retry-After": {"2.5"}}
		err := decodeHTTPError(test.status, header, []byte(`{"meta":{"status":400,"msg":"fallback"},"errors":[{"title":"Denied","code":1001,"detail":"specific detail"}]}`))
		if !errors.Is(err, test.target) {
			t.Fatalf("status %d error=%v", test.status, err)
		}
		var platformErr *socialhub.Error
		if !errors.As(err, &platformErr) || platformErr.Code != test.code || platformErr.Class != test.class || platformErr.PlatformCode != "1001" || platformErr.PlatformMessage != "specific detail" || platformErr.RequestID != "request-1" || platformErr.RetryAfter != 2500*time.Millisecond {
			t.Fatalf("mapped status %d=%#v", test.status, err)
		}
	}
	err := decodeHTTPError(http.StatusTeapot, http.Header{"X-Request-Id": {"request-2"}}, []byte(`{"meta":{"msg":"teapot"}}`))
	var platformErr *socialhub.Error
	if !errors.As(err, &platformErr) || platformErr.Code != socialhub.CodePlatformError || platformErr.RequestID != "request-2" || platformErr.PlatformMessage != "teapot" {
		t.Fatalf("fallback error=%#v", err)
	}
}

func TestEnvelopeAndErrorHelpers(t *testing.T) {
	err := decodeEnvelope(tumblrEnvelope{Meta: tumblrMeta{Status: 403, Msg: "denied"}}, nil)
	if !errors.Is(err, socialhub.ErrPermissionDenied) {
		t.Fatalf("envelope error=%v", err)
	}
	var output map[string]string
	if err := decodeEnvelope(tumblrEnvelope{Meta: tumblrMeta{Status: 200}}, &output); err == nil {
		t.Fatal("empty response accepted")
	}
	if err := decodeEnvelope(tumblrEnvelope{Meta: tumblrMeta{Status: 200}, Response: []byte(`[]`)}, &output); err == nil {
		t.Fatal("wrong response shape accepted")
	}
	if err := decodeEnvelope(tumblrEnvelope{Meta: tumblrMeta{Status: 200}, Response: []byte(`{"ok":"yes"}`)}, &output); err != nil || output["ok"] != "yes" {
		t.Fatalf("decoded output=%v error=%v", output, err)
	}
	if retryAfter("bad") != 0 || retryAfter("-1") != 0 || retryAfter("1.25") != 1250*time.Millisecond {
		t.Fatal("retry-after helper mismatch")
	}
	if boundedMessage(strings.Repeat("x", 520), 512) != strings.Repeat("x", 512) || firstNonEmpty(" ", "value") != "value" {
		t.Fatal("message helper mismatch")
	}
	var value flexString
	if err := value.UnmarshalJSON([]byte(`false`)); err == nil {
		t.Fatal("boolean flex string accepted")
	}
}
