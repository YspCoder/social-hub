package rakutenadvertising

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"social-hub/pkg/socialhub"
)

const maxTokenResponseBytes int64 = 1 << 20

type tokenClient struct {
	TokenURL    string
	TokenKey    string
	PublisherID string
	HTTPClient  *http.Client
	Clock       socialhub.Clock
}

func (client tokenClient) Refresh(ctx context.Context, refreshToken string) (socialhub.Token, error) {
	if !validEndpoint(client.TokenURL) || !validOpaque(client.TokenKey, 16_384) ||
		!validPositiveID(client.PublisherID) || client.HTTPClient == nil || client.Clock == nil ||
		!validOpaque(refreshToken, 16_384) {
		return socialhub.Token{}, invalidArgument("token_refresh", "token endpoint, token-key, publisher ID, HTTP client, clock, and refresh token are required")
	}
	form := url.Values{"refresh_token": {refreshToken}, "scope": {client.PublisherID}}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, client.TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return socialhub.Token{}, platformError("token_refresh", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Authorization", "Bearer "+client.TokenKey)
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, err := client.HTTPClient.Do(request)
	if err != nil {
		return socialhub.Token{}, platformError("token_refresh", socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, maxTokenResponseBytes+1))
	if err != nil {
		return socialhub.Token{}, platformError("token_refresh", socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
	}
	if int64(len(body)) > maxTokenResponseBytes {
		return socialhub.Token{}, platformError("token_refresh", socialhub.CodePlatformError, socialhub.ClassPermanent, fmt.Errorf("token response exceeded 1 MiB"))
	}
	var payload struct {
		AccessToken      string `json:"access_token"`
		ExpiresIn        int64  `json:"expires_in"`
		RefreshToken     string `json:"refresh_token"`
		TokenType        string `json:"token_type"`
		Error            string `json:"error"`
		ErrorDescription string `json:"error_description"`
		Message          string `json:"message"`
	}
	decodeErr := json.Unmarshal(body, &payload)
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return socialhub.Token{}, withOperation(decodeHTTPError(
			response.StatusCode, response.Header, body, client.Clock.Now(), client.TokenKey, refreshToken,
		), "token_refresh")
	}
	if response.StatusCode != http.StatusOK {
		return socialhub.Token{}, platformContractError(
			"token_refresh", "Rakuten Advertising returned an unexpected token success status", response.StatusCode,
		)
	}
	if !validJSONContentType(response.Header.Get("Content-Type")) {
		return socialhub.Token{}, platformContractError(
			"token_refresh", "Rakuten Advertising returned a non-JSON token response", response.StatusCode,
		)
	}
	if decodeErr != nil {
		return socialhub.Token{}, withHTTPStatus(
			platformError("token_refresh", socialhub.CodePlatformError, socialhub.ClassPermanent, decodeErr), response.StatusCode,
		)
	}
	if payload.Error != "" {
		provider := ProviderError{Code: payload.Error, Message: firstNonEmpty(payload.ErrorDescription, payload.Message)}
		return socialhub.Token{}, providerResponseError(
			"token_refresh", response.StatusCode, response.Header, provider, body, client.Clock.Now(), client.TokenKey, refreshToken,
		)
	}
	rotatedRefreshToken := payload.RefreshToken
	if rotatedRefreshToken == "" {
		rotatedRefreshToken = refreshToken
	}
	if !validOpaque(payload.AccessToken, 16_384) || !validOpaque(rotatedRefreshToken, 16_384) ||
		payload.ExpiresIn <= 0 || payload.ExpiresIn > int64((24*time.Hour)/time.Second) ||
		(payload.TokenType != "" && !strings.EqualFold(payload.TokenType, "bearer")) {
		return socialhub.Token{}, withHTTPStatus(
			platformError("token_refresh", socialhub.CodePlatformError, socialhub.ClassPermanent, fmt.Errorf("invalid token response fields")),
			response.StatusCode,
		)
	}
	return socialhub.Token{
		AccessToken: payload.AccessToken, RefreshToken: rotatedRefreshToken, TokenType: "Bearer",
		ExpiresAt: client.Clock.Now().Add(time.Duration(payload.ExpiresIn) * time.Second),
		Scopes:    []string{client.PublisherID},
	}, nil
}

type refreshTokenSource struct {
	mu                sync.Mutex
	client            tokenClient
	refreshToken      string
	configuredSecrets []string
	store             socialhub.TokenStore
	key               socialhub.TokenKey
	token             socialhub.Token
	storeDirty        bool
}

func (source *refreshTokenSource) redactionSecrets() []string {
	source.mu.Lock()
	defer source.mu.Unlock()
	secrets := append([]string(nil), source.configuredSecrets...)
	return append(secrets, source.refreshToken, source.token.AccessToken, source.token.RefreshToken)
}

func (source *refreshTokenSource) Token(ctx context.Context) (socialhub.Token, error) {
	source.mu.Lock()
	defer source.mu.Unlock()
	now := source.client.Clock.Now()
	if validCachedToken(source.token, now.Add(2*time.Minute)) {
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
		if err == nil {
			if validOpaque(stored.RefreshToken, 16_384) {
				source.refreshToken = stored.RefreshToken
			}
			if validCachedToken(stored, now.Add(2*time.Minute)) {
				source.token = stored
				return stored, nil
			}
		}
		if err != nil && !errors.Is(err, socialhub.ErrNotFound) {
			return socialhub.Token{}, platformError("token_cache_get", socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
		}
	}
	token, err := source.client.Refresh(ctx, source.refreshToken)
	if err != nil {
		return socialhub.Token{}, err
	}
	source.refreshToken, source.token = token.RefreshToken, token
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
	return token.Valid(at) && validOpaque(token.AccessToken, 16_384) && validOpaque(token.RefreshToken, 16_384) &&
		(token.TokenType == "" || strings.EqualFold(token.TokenType, "bearer"))
}
