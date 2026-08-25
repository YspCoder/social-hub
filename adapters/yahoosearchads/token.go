package yahoosearchads

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"social-hub/pkg/socialhub"
)

type refreshTokenSource struct {
	mu           sync.Mutex
	oauth        OAuthClient
	refreshToken string
	store        socialhub.TokenStore
	key          socialhub.TokenKey
	token        socialhub.Token
}

func (source *refreshTokenSource) Token(ctx context.Context) (socialhub.Token, error) {
	source.mu.Lock()
	defer source.mu.Unlock()
	now := source.oauth.clock.Now()
	if source.token.Valid(now.Add(2 * time.Minute)) {
		return source.token, nil
	}
	if source.store != nil {
		stored, err := source.store.Get(ctx, source.key)
		if err == nil && stored.RefreshToken != "" {
			if !validOpaque(stored.RefreshToken, 16_384) {
				return socialhub.Token{}, tokenStoreError("token_cache_get")
			}
			source.refreshToken = stored.RefreshToken
			source.oauth.requestIDs.add(stored.RefreshToken)
		}
		if err == nil && stored.Valid(now.Add(2*time.Minute)) {
			if !validOpaque(stored.AccessToken, 16_384) || !strings.EqualFold(stored.TokenType, "bearer") {
				return socialhub.Token{}, tokenStoreError("token_cache_get")
			}
			source.oauth.requestIDs.add(stored.AccessToken, stored.RefreshToken)
			source.token = stored
			return stored, nil
		}
		if err != nil && !errors.Is(err, socialhub.ErrNotFound) {
			return socialhub.Token{}, tokenStoreError("token_cache_get")
		}
	}
	token, err := source.oauth.Refresh(ctx, source.refreshToken)
	if err != nil {
		return socialhub.Token{}, err
	}
	if token.RefreshToken != "" {
		source.refreshToken = token.RefreshToken
	}
	if source.store != nil {
		if err := source.store.Put(ctx, source.key, token); err != nil {
			return socialhub.Token{}, tokenStoreError("token_cache_put")
		}
	}
	source.token = token
	return token, nil
}

func tokenStoreError(operation string) error {
	return &socialhub.Error{
		Code: socialhub.CodeTemporarilyUnavailable, Class: socialhub.ClassRetryable,
		Platform: platformName, Product: productName, Op: operation,
		PlatformMessage: "configured token store operation failed",
	}
}
