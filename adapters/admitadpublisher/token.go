package admitadpublisher

import (
	"context"
	"errors"
	"sync"
	"time"

	"social-hub/pkg/socialhub"
)

type managedTokenSource struct {
	mu    sync.Mutex
	oauth OAuthClient
	store socialhub.TokenStore
	key   socialhub.TokenKey
	token socialhub.Token
}

func (source *managedTokenSource) Token(ctx context.Context) (socialhub.Token, error) {
	source.mu.Lock()
	defer source.mu.Unlock()
	now := source.oauth.Clock.Now()
	if source.token.Valid(now.Add(2 * time.Minute)) {
		return source.token, nil
	}
	candidate := source.token
	if source.store != nil {
		stored, err := source.store.Get(ctx, source.key)
		if err == nil {
			candidate = stored
			if stored.Valid(now.Add(2 * time.Minute)) {
				source.token = stored
				return stored, nil
			}
		} else if !errors.Is(err, socialhub.ErrNotFound) {
			return socialhub.Token{}, platformError("token_cache_get", socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
		}
	}
	if validOpaque(candidate.RefreshToken, 16_384) {
		refreshed, err := source.oauth.Refresh(ctx, candidate.RefreshToken)
		if err == nil {
			return source.remember(ctx, refreshed)
		}
		if !reauthorizationRequired(err) {
			return socialhub.Token{}, err
		}
	}
	token, err := source.oauth.ClientCredentials(ctx)
	if err != nil {
		return socialhub.Token{}, err
	}
	return source.remember(ctx, token)
}

func (source *managedTokenSource) remember(ctx context.Context, token socialhub.Token) (socialhub.Token, error) {
	if source.store != nil {
		if err := source.store.Put(ctx, source.key, token); err != nil {
			return socialhub.Token{}, platformError("token_cache_put", socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
		}
	}
	source.token = token
	return token, nil
}

func reauthorizationRequired(err error) bool {
	var hub *socialhub.Error
	return errors.As(err, &hub) && (hub.Code == socialhub.CodeUnauthenticated || hub.Code == socialhub.CodeConflict)
}
