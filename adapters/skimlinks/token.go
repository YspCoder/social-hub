package skimlinks

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"sync"
	"time"

	"social-hub/pkg/socialhub"
)

const maxTokenResponseBytes int64 = 1 << 20

type tokenClient struct {
	URL          string
	ClientID     string
	ClientSecret string
	HTTPClient   *http.Client
	Clock        socialhub.Clock
	PublisherID  int64
}

func (client tokenClient) Issue(ctx context.Context) (socialhub.Token, error) {
	if !validEndpoint(client.URL) || !validOpaque(client.ClientID, 1024) ||
		!validOpaque(client.ClientSecret, 16_384) || client.HTTPClient == nil || client.Clock == nil ||
		client.PublisherID <= 0 {
		return socialhub.Token{}, invalidArgument("token_issue", "token endpoint, client credentials, HTTP client, clock, and publisher ID are required")
	}
	payload := struct {
		ClientID     string `json:"client_id"`
		ClientSecret string `json:"client_secret"`
		GrantType    string `json:"grant_type"`
	}{ClientID: client.ClientID, ClientSecret: client.ClientSecret, GrantType: "client_credentials"}
	body, err := json.Marshal(payload)
	if err != nil {
		return socialhub.Token{}, platformError("token_issue", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, client.URL, bytes.NewReader(body))
	if err != nil {
		return socialhub.Token{}, platformError("token_issue", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Content-Type", "application/json")
	response, err := client.HTTPClient.Do(request)
	if err != nil {
		return socialhub.Token{}, platformError("token_issue", socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
	}
	defer response.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(response.Body, maxTokenResponseBytes+1))
	if err != nil {
		return socialhub.Token{}, withHTTPStatus(
			platformError("token_issue", socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err), response.StatusCode,
		)
	}
	if int64(len(responseBody)) > maxTokenResponseBytes {
		return socialhub.Token{}, platformContractError("token_issue", "Skimlinks token response exceeded 1 MiB", response.StatusCode)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return socialhub.Token{}, withOperation(decodeHTTPError(
			response.StatusCode, response.Header, responseBody, client.Clock.Now(), client.ClientID, client.ClientSecret,
		), "token_issue")
	}
	if response.StatusCode != http.StatusOK {
		return socialhub.Token{}, platformContractError(
			"token_issue", "Skimlinks returned an unexpected token success status", response.StatusCode,
		)
	}
	if len(responseBody) == 0 || !json.Valid(responseBody) {
		return socialhub.Token{}, platformContractError(
			"token_issue", "Skimlinks returned an empty or invalid JSON token response", response.StatusCode,
		)
	}
	if !validJSONContentType(response.Header.Get("Content-Type")) {
		return socialhub.Token{}, platformContractError(
			"token_issue", "Skimlinks returned a non-JSON token response", response.StatusCode,
		)
	}
	var decoded struct {
		AccessToken     string `json:"access_token"`
		Timestamp       int64  `json:"timestamp"`
		ExpiryTimestamp int64  `json:"expiry_timestamp"`
	}
	if err := json.Unmarshal(responseBody, &decoded); err != nil {
		return socialhub.Token{}, withHTTPStatus(
			platformError("token_issue", socialhub.CodePlatformError, socialhub.ClassPermanent, err), response.StatusCode,
		)
	}
	expiresAt := time.Unix(decoded.ExpiryTimestamp, 0)
	if !validOpaque(decoded.AccessToken, 16_384) || decoded.Timestamp <= 0 ||
		decoded.ExpiryTimestamp <= decoded.Timestamp || !expiresAt.After(client.Clock.Now()) {
		return socialhub.Token{}, platformContractError("token_issue", "Skimlinks returned invalid token fields", response.StatusCode)
	}
	return socialhub.Token{
		AccessToken: decoded.AccessToken, ExpiresAt: expiresAt,
		Scopes: []string{strconv.FormatInt(client.PublisherID, 10)},
	}, nil
}

type clientCredentialsTokenSource struct {
	mu                sync.Mutex
	client            tokenClient
	configuredSecrets []string
	store             socialhub.TokenStore
	key               socialhub.TokenKey
	token             socialhub.Token
	storeDirty        bool
}

func (source *clientCredentialsTokenSource) redactionSecrets() []string {
	source.mu.Lock()
	defer source.mu.Unlock()
	secrets := append([]string(nil), source.configuredSecrets...)
	return append(secrets, source.token.AccessToken)
}

func (source *clientCredentialsTokenSource) Token(ctx context.Context) (socialhub.Token, error) {
	source.mu.Lock()
	defer source.mu.Unlock()
	now := source.client.Clock.Now()
	if validCachedToken(source.token, now.Add(time.Minute)) {
		if source.storeDirty {
			if err := source.store.Put(ctx, source.key, source.token); err != nil {
				return socialhub.Token{}, platformError("token_cache_put", socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
			}
			source.storeDirty = false
		}
		return source.token, nil
	}
	if source.store != nil && !source.storeDirty {
		stored, err := source.store.Get(ctx, source.key)
		if err == nil && validCachedToken(stored, now.Add(time.Minute)) {
			source.token = stored
			return stored, nil
		}
		if err != nil && !errors.Is(err, socialhub.ErrNotFound) {
			return socialhub.Token{}, platformError("token_cache_get", socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
		}
	}
	token, err := source.client.Issue(ctx)
	if err != nil {
		return socialhub.Token{}, err
	}
	source.token = token
	if source.store != nil {
		source.storeDirty = true
		if err := source.store.Put(ctx, source.key, token); err != nil {
			return socialhub.Token{}, platformError("token_cache_put", socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
		}
		source.storeDirty = false
	}
	return token, nil
}

func validCachedToken(token socialhub.Token, at time.Time) bool {
	return !token.ExpiresAt.IsZero() && token.Valid(at) && validOpaque(token.AccessToken, 16_384)
}
