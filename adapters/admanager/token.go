package admanager

import (
	"context"
	"errors"
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
	now := source.oauth.Clock.Now()
	if source.token.Valid(now.Add(2 * time.Minute)) {
		return source.token, nil
	}
	if source.store != nil {
		stored, err := source.store.Get(ctx, source.key)
		if err == nil && stored.Valid(now.Add(2*time.Minute)) {
			source.token = stored
			return stored, nil
		}
		if err != nil && !errors.Is(err, socialhub.ErrNotFound) {
			return socialhub.Token{}, platformError("token_cache_get", socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
		}
	}
	token, err := source.oauth.Refresh(ctx, source.refreshToken)
	if err != nil {
		return socialhub.Token{}, err
	}
	if source.store != nil {
		if err := source.store.Put(ctx, source.key, token); err != nil {
			return socialhub.Token{}, platformError("token_cache_put", socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
		}
	}
	source.token = token
	return token, nil
}
