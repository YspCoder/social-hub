package messenger

import (
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

func TestGraphErrorMapping(t *testing.T) {
	tests := []struct {
		name   string
		status int
		body   string
		want   socialhub.ErrorCode
	}{
		{name: "rate despite 400", status: http.StatusBadRequest, body: `{"error":{"message":"Calls to this API have exceeded the rate limit","code":613,"fbtrace_id":"trace-613"}}`, want: socialhub.CodeRateLimited},
		{name: "expired token", status: http.StatusBadRequest, body: `{"error":{"message":"expired","code":190}}`, want: socialhub.CodeUnauthenticated},
		{name: "window closed", status: http.StatusBadRequest, body: `{"error":{"message":"outside window","code":10,"error_subcode":1545041}}`, want: socialhub.CodeConflict},
		{name: "user unavailable", status: http.StatusBadRequest, body: `{"error":{"message":"unavailable","code":551}}`, want: socialhub.CodeConflict},
		{name: "permission", status: http.StatusForbidden, body: `{"error":{"message":"permission","code":200}}`, want: socialhub.CodePermissionDenied},
		{name: "transient", status: http.StatusBadRequest, body: `{"error":{"message":"retry","code":100,"is_transient":true}}`, want: socialhub.CodeTemporarilyUnavailable},
		{name: "server", status: http.StatusServiceUnavailable, body: `{}`, want: socialhub.CodeTemporarilyUnavailable},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			header := http.Header{"Retry-After": {"1.5"}, "X-Fb-Request-Id": {"request-id"}}
			err := decodeHTTPError(test.status, header, []byte(test.body))
			if errorCode(err) != test.want {
				t.Fatalf("error=%v code=%s want=%s", err, errorCode(err), test.want)
			}
			var platformErr *socialhub.Error
			if !errors.As(err, &platformErr) || platformErr.HTTPStatus != test.status || platformErr.RetryAfter != 1500*time.Millisecond {
				t.Fatalf("platform error=%#v", platformErr)
			}
			if test.name == "rate despite 400" && (platformErr.PlatformCode != "613" || platformErr.RequestID != "trace-613" || !platformErr.Retryable()) {
				t.Fatalf("rate error=%#v", platformErr)
			}
			if test.name == "window closed" && platformErr.PlatformCode != "10/1545041" {
				t.Fatalf("window error=%#v", platformErr)
			}
			if test.name == "permission" && (len(platformErr.RequiredScopes) != 1 || platformErr.ApprovalURL == "") {
				t.Fatalf("permission guidance=%#v", platformErr)
			}
		})
	}
}

func TestGraphErrorSanitizationAndHelpers(t *testing.T) {
	message := strings.Repeat("界", 600)
	err := decodeHTTPError(http.StatusBadRequest, nil, []byte(`{"error":{"code":100,"message":"`+message+`"}}`))
	var platformErr *socialhub.Error
	if !errors.As(err, &platformErr) || len([]rune(platformErr.PlatformMessage)) != 512 {
		t.Fatalf("bounded error=%#v", platformErr)
	}
	if parseRetryAfter("bad") != 0 || parseRetryAfter("-1") != 0 || parseRetryAfter("0.25") != 250*time.Millisecond {
		t.Fatal("Retry-After parsing mismatch")
	}
	if boundedMessage("  value  ", 10) != "value" || firstNonEmpty("", " request ") != " request " {
		t.Fatal("error helper mismatch")
	}
}
