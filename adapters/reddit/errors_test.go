package reddit

import (
	"encoding/json"
	"errors"
	"net/http"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

func TestRedditErrorMapping(t *testing.T) {
	header := http.Header{"X-Reddit-Loid": {"request-1"}, "X-Ratelimit-Reset": {"3"}}
	err := decodeHTTPError(http.StatusTooManyRequests, header, []byte(`{"error":429,"message":"Too Many Requests"}`))
	var typed *socialhub.Error
	if !errors.As(err, &typed) || !errors.Is(err, socialhub.ErrRateLimited) || typed.PlatformCode != "429" || typed.RequestID != "request-1" || typed.RetryAfter != 3*time.Second {
		t.Fatalf("error=%#v", err)
	}
	response := redditAPIResponse{}
	response.JSON.Errors = [][]json.RawMessage{{json.RawMessage(`"RATELIMIT"`), json.RawMessage(`"try later"`), json.RawMessage(`"ratelimit"`)}}
	if err := checkAPIResponse("submit", response); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("api error=%v", err)
	}
}
