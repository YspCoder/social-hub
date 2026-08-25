package xandr

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"sync"
	"time"

	"social-hub/pkg/socialhub"
)

const (
	maxAuthenticationResponseBytes int64 = 1 << 20
	sessionLifetime                      = 2 * time.Hour
	sessionExpirySkew                    = 30 * time.Second
)

type sessionTokenSource struct {
	mu         sync.Mutex
	username   string
	password   string
	httpClient *http.Client
	clock      socialhub.Clock
	token      socialhub.Token
	requestIDs *requestIDFilter
	closed     bool
}

func (source *sessionTokenSource) Token(ctx context.Context) (socialhub.Token, error) {
	source.mu.Lock()
	defer source.mu.Unlock()
	if source.closed {
		return socialhub.Token{}, platformError("session", socialhub.CodeConflict, socialhub.ClassPermanent, nil)
	}
	now := source.clock.Now()
	if now.Unix() <= 0 {
		return socialhub.Token{}, invalidArgument("session", "clock must return a time after the Unix epoch")
	}
	if source.token.Valid(now.Add(sessionExpirySkew)) {
		return source.token, nil
	}
	token, err := source.authenticate(ctx)
	if err != nil {
		return socialhub.Token{}, err
	}
	source.token = token
	return token, nil
}

func (source *sessionTokenSource) Invalidate(failedToken string) {
	source.mu.Lock()
	if source.token.AccessToken == failedToken {
		source.token = socialhub.Token{}
	}
	source.mu.Unlock()
}

func (source *sessionTokenSource) Close() {
	source.mu.Lock()
	source.username, source.password = "", ""
	source.token, source.closed = socialhub.Token{}, true
	if source.requestIDs != nil {
		source.requestIDs.clear()
	}
	source.mu.Unlock()
}

func (source *sessionTokenSource) authenticate(ctx context.Context) (socialhub.Token, error) {
	const operation = "authentication"
	payload, err := json.Marshal(struct {
		Auth struct {
			Username string `json:"username"`
			Password string `json:"password"`
		} `json:"auth"`
	}{Auth: struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}{Username: source.username, Password: source.password}})
	if err != nil {
		return socialhub.Token{}, platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, defaultAuthURL, bytes.NewReader(payload))
	if err != nil {
		return socialhub.Token{}, platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Accept-Encoding", "identity")
	request.Header.Set("Content-Type", "application/json")
	started := source.clock.Now()
	if started.Unix() <= 0 {
		return socialhub.Token{}, invalidArgument(operation, "clock must return a time after the Unix epoch")
	}
	response, err := source.httpClient.Do(request)
	if err != nil {
		return socialhub.Token{}, platformError(operation, socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, sanitizeCause(err))
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, maxAuthenticationResponseBytes+1))
	if err != nil {
		return socialhub.Token{}, platformError(operation, socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
	}
	if int64(len(body)) > maxAuthenticationResponseBytes {
		return socialhub.Token{}, platformContractError(operation, "authentication response exceeded 1 MiB")
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return socialhub.Token{}, withOperation(decodeHTTPError(response.StatusCode, response.Header, body, source.clock.Now(), source.requestIDs), operation)
	}
	if response.StatusCode != http.StatusOK || !validJSONContentType(response.Header.Get("Content-Type")) {
		return socialhub.Token{}, platformContractError(operation, "Xandr returned an invalid authentication success response")
	}
	var envelope apiEnvelope
	if err := json.Unmarshal(body, &envelope); err != nil || envelope.Response == nil {
		return socialhub.Token{}, platformContractError(operation, "Xandr returned an invalid authentication envelope")
	}
	wire := envelope.Response
	if wire.ErrorID != "" {
		return socialhub.Token{}, businessError(operation, response.StatusCode, response.Header, *wire, source.clock.Now(), source.requestIDs)
	}
	if wire.Status != "OK" || !validOpaque(wire.Token, 16_384) {
		return socialhub.Token{}, platformContractError(operation, "Xandr authentication did not return an OK status and token")
	}
	if source.requestIDs != nil {
		source.requestIDs.add(wire.Token)
	}
	return socialhub.Token{
		AccessToken: wire.Token, TokenType: "XandrSession", ExpiresAt: started.Add(sessionLifetime),
	}, nil
}

var _ socialhub.TokenSource = (*sessionTokenSource)(nil)
