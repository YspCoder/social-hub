package socialhub

import (
	"context"
	"net/http"
)

// ChallengeHandler handles platform webhook verification handshakes.
type ChallengeHandler interface {
	HandleChallenge(context.Context, *http.Request) (status int, body []byte, err error)
}
