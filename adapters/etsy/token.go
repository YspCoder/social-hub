package etsy

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	"social-hub/pkg/socialhub"
)

type apiKeyOnlyTokenSource struct{}

func (apiKeyOnlyTokenSource) Token(context.Context) (socialhub.Token, error) {
	return socialhub.Token{}, nil
}

type credentialAuthenticator struct{ APIKey string }

func (authenticator credentialAuthenticator) Authenticate(request *http.Request, token socialhub.Token) error {
	if !validOpaque(authenticator.APIKey, 17_409) {
		return errors.New("etsy: invalid API key credential")
	}
	request.Header.Set("x-api-key", authenticator.APIKey)
	if token.AccessToken == "" {
		return nil
	}
	if !validOpaque(token.AccessToken, 16_384) || token.TokenType != "" && !strings.EqualFold(token.TokenType, "bearer") {
		return errors.New("etsy: invalid OAuth token")
	}
	request.Header.Set("Authorization", "Bearer "+token.AccessToken)
	return nil
}

type refreshTokenSource struct {
	mu           sync.Mutex
	client       OAuthClient
	refreshToken string
	store        socialhub.TokenStore
	key          socialhub.TokenKey
	token        socialhub.Token
}

func (source *refreshTokenSource) Token(ctx context.Context) (socialhub.Token, error) {
	source.mu.Lock()
	defer source.mu.Unlock()
	now := source.client.Clock.Now()
	if validCachedToken(source.token, now.Add(2*time.Minute)) {
		return source.token, nil
	}
	if source.store != nil {
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
	if source.store != nil {
		if err := source.store.Put(ctx, source.key, token); err != nil {
			return socialhub.Token{}, platformError("token_cache_put", socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
		}
	}
	source.refreshToken, source.token = token.RefreshToken, token
	return token, nil
}

func validCachedToken(token socialhub.Token, at time.Time) bool {
	return token.Valid(at) && validOpaque(token.AccessToken, 16_384) && validOpaque(token.RefreshToken, 16_384) &&
		(token.TokenType == "" || strings.EqualFold(token.TokenType, "bearer"))
}
