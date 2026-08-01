package snapchat

import (
	"errors"
	"net/http"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

func TestSnapchatErrorMapping(t *testing.T) {
	header := http.Header{"Retry-After": {"4"}, "X-Request-Id": {"header-request"}}
	err := decodeHTTPError(http.StatusTooManyRequests, header, []byte(`{"request_id":"body-request","request_status":"ERROR","error_code":"RATE_LIMIT","display_message":"slow down"}`))
	var typed *socialhub.Error
	if !errors.As(err, &typed) || !errors.Is(err, socialhub.ErrRateLimited) || typed.RequestID != "body-request" || typed.RetryAfter != 4*time.Second {
		t.Fatalf("rate-limit error=%#v", err)
	}
	err = decodeHTTPError(http.StatusForbidden, nil, []byte(`{"request_status":"ERROR","error_code":"AUTHORIZATION_PERMISSION_DENIED"}`))
	if !errors.Is(err, socialhub.ErrApprovalRequired) {
		t.Fatalf("approval error=%v", err)
	}
	partial := responseMeta{RequestID: "partial-request", RequestStatus: "PARTIAL"}
	err = checkResponse("search_profiles", partial, []subRequestState{{Status: "SUCCESS"}, {Status: "ERROR", Reason: "profile unavailable"}})
	if !errors.As(err, &typed) || typed.PlatformCode != "ERROR" || typed.RequestID != "partial-request" {
		t.Fatalf("partial error=%#v", err)
	}
}
