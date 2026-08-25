package thetradedesk

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"social-hub/pkg/socialhub"
)

const maxAuthenticationResponseBytes int64 = 1 << 20

// AuthenticationClient generates non-refreshable Platform API tokens that
// live for at most 24 hours.
type AuthenticationClient struct {
	Login      string
	Password   string
	BaseURL    string
	HTTPClient *http.Client
	Clock      socialhub.Clock
	requestIDs *requestIDFilter
}

type authenticationRequest struct {
	Login                    string `json:"Login"`
	Password                 string `json:"Password"`
	TokenExpirationInMinutes int32  `json:"TokenExpirationInMinutes"`
}

type authenticationResponse struct {
	Token string `json:"Token"`
}

// Generate creates a new short-lived TTD-Auth token. Passing zero uses the
// official 1440-minute default.
func (client AuthenticationClient) Generate(ctx context.Context, expirationMinutes int32) (socialhub.Token, error) {
	const operation = "authentication_generate"
	expirationMinutes = normalizedTokenExpiration(expirationMinutes)
	if !validOpaque(client.Login, 512) || !validOpaque(client.Password, 16_384) ||
		!validEndpoint(client.BaseURL) || client.HTTPClient == nil || client.Clock == nil ||
		expirationMinutes < 1 || expirationMinutes > 1440 {
		return socialhub.Token{}, invalidArgument(operation, "login, password, endpoint, HTTP client, clock, or token expiration is invalid")
	}
	started := client.Clock.Now()
	if !started.After(time.Unix(0, 0)) {
		return socialhub.Token{}, invalidArgument(operation, "clock must return a time after the Unix epoch")
	}
	requestIDs := client.requestIDs
	if requestIDs == nil {
		requestIDs = newRequestIDFilter(client.Login, client.Password)
		defer requestIDs.clear()
	} else {
		requestIDs.add(client.Login, client.Password)
	}
	payload, err := json.Marshal(authenticationRequest{
		Login: client.Login, Password: client.Password, TokenExpirationInMinutes: expirationMinutes,
	})
	if err != nil {
		return socialhub.Token{}, platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	endpoint, _ := url.Parse(client.BaseURL)
	endpoint.Path = strings.TrimRight(endpoint.Path, "/") + "/v3/authentication"
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.String(), bytes.NewReader(payload))
	if err != nil {
		return socialhub.Token{}, platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Accept-Encoding", "identity")
	request.Header.Set("Content-Type", "application/json")
	httpClient := cloneHTTPClient(client.HTTPClient)
	response, err := httpClient.Do(request)
	if err != nil {
		return socialhub.Token{}, platformError(operation, socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, maxAuthenticationResponseBytes+1))
	if err != nil {
		return socialhub.Token{}, platformError(operation, socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
	}
	if int64(len(body)) > maxAuthenticationResponseBytes {
		return socialhub.Token{}, platformContractError(operation, "authentication response exceeded 1 MiB")
	}
	if response.StatusCode != http.StatusOK {
		return socialhub.Token{}, withOperation(decodeHTTPError(response.StatusCode, response.Header, body, started, requestIDs), operation)
	}
	if !validJSONContentType(response.Header.Get("Content-Type")) {
		return socialhub.Token{}, platformContractError(operation, "authentication success response was not application/json")
	}
	var decoded authenticationResponse
	if err := json.Unmarshal(body, &decoded); err != nil {
		return socialhub.Token{}, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	if !validOpaque(decoded.Token, 16_384) {
		return socialhub.Token{}, platformContractError(operation, "authentication response omitted a valid token")
	}
	requestIDs.add(decoded.Token)
	return socialhub.Token{
		AccessToken: decoded.Token,
		ExpiresAt:   started.Add(time.Duration(expirationMinutes) * time.Minute),
	}, nil
}

type closableTokenSource interface {
	socialhub.TokenSource
	Close()
}

type staticTokenSource struct {
	mu         sync.RWMutex
	token      socialhub.Token
	requestIDs *requestIDFilter
	closed     bool
}

func (source *staticTokenSource) Token(context.Context) (socialhub.Token, error) {
	source.mu.RLock()
	defer source.mu.RUnlock()
	if source.closed {
		return socialhub.Token{}, platformError("token", socialhub.CodeConflict, socialhub.ClassPermanent, nil)
	}
	if !validOpaque(source.token.AccessToken, 16_384) {
		return socialhub.Token{}, socialhub.ErrUnauthenticated
	}
	return source.token, nil
}

func (source *staticTokenSource) Close() {
	source.mu.Lock()
	source.token, source.closed = socialhub.Token{}, true
	if source.requestIDs != nil {
		source.requestIDs.clear()
	}
	source.mu.Unlock()
}

type authenticationTokenSource struct {
	mu                sync.Mutex
	authentication    AuthenticationClient
	expirationMinutes int32
	store             socialhub.TokenStore
	key               socialhub.TokenKey
	token             socialhub.Token
	requestIDs        *requestIDFilter
	closed            bool
}

func (source *authenticationTokenSource) Token(ctx context.Context) (socialhub.Token, error) {
	source.mu.Lock()
	defer source.mu.Unlock()
	if source.closed {
		return socialhub.Token{}, platformError("token", socialhub.CodeConflict, socialhub.ClassPermanent, nil)
	}
	now := source.authentication.Clock.Now()
	if !now.After(time.Unix(0, 0)) {
		return socialhub.Token{}, invalidArgument("token", "clock must return a time after the Unix epoch")
	}
	if validManagedToken(source.token, now) {
		return source.token, nil
	}
	if source.store != nil {
		stored, err := source.store.Get(ctx, source.key)
		if err == nil {
			if validManagedToken(stored, now) {
				source.requestIDs.add(stored.AccessToken)
				source.token = stored
				return stored, nil
			}
		} else if !errors.Is(err, socialhub.ErrNotFound) {
			return socialhub.Token{}, platformError("token_cache_get", socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, errTokenStoreOperation)
		}
	}
	token, err := source.authentication.Generate(ctx, source.expirationMinutes)
	if err != nil {
		return socialhub.Token{}, err
	}
	source.requestIDs.add(token.AccessToken)
	source.token = token
	if source.store != nil {
		if err := source.store.Put(ctx, source.key, token); err != nil {
			return socialhub.Token{}, platformError("token_cache_put", socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, errTokenStoreOperation)
		}
	}
	return token, nil
}

func validManagedToken(token socialhub.Token, now time.Time) bool {
	return validOpaque(token.AccessToken, 16_384) && !token.ExpiresAt.IsZero() && token.Valid(now.Add(30*time.Second))
}

func (source *authenticationTokenSource) Close() {
	source.mu.Lock()
	source.authentication.Login, source.authentication.Password = "", ""
	source.token, source.closed = socialhub.Token{}, true
	if source.requestIDs != nil {
		source.requestIDs.clear()
	}
	source.mu.Unlock()
}

type requestAuthenticator struct {
	strictMode *bool
}

func (authenticator requestAuthenticator) Authenticate(request *http.Request, token socialhub.Token) error {
	if !validOpaque(token.AccessToken, 16_384) {
		return socialhub.ErrUnauthenticated
	}
	request.Header.Set("TTD-Auth", token.AccessToken)
	if authenticator.strictMode != nil {
		request.Header.Set("TTD-Strict-Mode", strconv.FormatBool(*authenticator.strictMode))
	}
	return nil
}

var (
	_ closableTokenSource = (*staticTokenSource)(nil)
	_ closableTokenSource = (*authenticationTokenSource)(nil)
)
