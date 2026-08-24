package youtubereporting

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
	return source.cachedOrRefresh(ctx, source.oauth.Clock.Now(), func() (socialhub.Token, error) {
		return source.oauth.Refresh(ctx, source.refreshToken)
	})
}

func (source *refreshTokenSource) cachedOrRefresh(ctx context.Context, now time.Time, refresh func() (socialhub.Token, error)) (socialhub.Token, error) {
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
	token, err := refresh()
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

type serviceAccountTokenSource struct {
	mu     sync.Mutex
	client ServiceAccountClient
	store  socialhub.TokenStore
	key    socialhub.TokenKey
	token  socialhub.Token
}

func (source *serviceAccountTokenSource) Token(ctx context.Context) (socialhub.Token, error) {
	source.mu.Lock()
	defer source.mu.Unlock()
	helper := refreshTokenSource{store: source.store, key: source.key, token: source.token}
	token, err := helper.cachedOrRefresh(ctx, source.client.Clock.Now(), func() (socialhub.Token, error) {
		return source.client.Token(ctx)
	})
	if err == nil {
		source.token = token
	}
	return token, err
}
