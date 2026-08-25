package mercadodisplayads

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"social-hub/pkg/socialhub"
)

type closeableTokenSource interface {
	socialhub.TokenSource
	Close()
}

type staticTokenSource struct {
	mu     sync.RWMutex
	token  socialhub.Token
	closed bool
}

func (source *staticTokenSource) Token(ctx context.Context) (socialhub.Token, error) {
	if source == nil || ctx == nil {
		return socialhub.Token{}, invalidArgument("token", "token source and context are required")
	}
	source.mu.RLock()
	defer source.mu.RUnlock()
	if source.closed {
		return socialhub.Token{}, platformError("token", socialhub.CodeConflict, socialhub.ClassPermanent, nil)
	}
	if !validOpaque(source.token.AccessToken, 16_384) {
		return socialhub.Token{}, credentialError("token")
	}
	return source.token, nil
}

func (source *staticTokenSource) Close() {
	if source == nil {
		return
	}
	source.mu.Lock()
	source.token, source.closed = socialhub.Token{}, true
	source.mu.Unlock()
}

type refreshTokenSource struct {
	mu           sync.RWMutex
	refreshMu    sync.Mutex
	oauth        OAuthClient
	refreshToken string
	store        socialhub.TokenStore
	key          socialhub.TokenKey
	token        socialhub.Token
	storeDirty   bool
	closed       bool
}

func (source *refreshTokenSource) Token(ctx context.Context) (socialhub.Token, error) {
	if source == nil || ctx == nil {
		return socialhub.Token{}, invalidArgument("token", "token source and context are required")
	}
	source.refreshMu.Lock()
	defer source.refreshMu.Unlock()

	source.mu.RLock()
	if source.closed {
		source.mu.RUnlock()
		return socialhub.Token{}, platformError("token", socialhub.CodeConflict, socialhub.ClassPermanent, nil)
	}
	cached, refreshToken := source.token, source.refreshToken
	store, key, storeDirty := source.store, source.key, source.storeDirty
	source.mu.RUnlock()

	snapshot, err := source.oauth.snapshot("token")
	if err != nil {
		return socialhub.Token{}, err
	}
	now := snapshot.clock.Now()
	if !now.After(time.Unix(0, 0)) {
		return socialhub.Token{}, invalidArgument("token", "clock must return a time after the Unix epoch")
	}
	if cached.Valid(now.Add(2 * time.Minute)) {
		if storeDirty && store != nil {
			if err := store.Put(ctx, key, cached); err != nil {
				return socialhub.Token{}, dependencyError("token_cache_put", "token store write failed")
			}
		}
		source.mu.Lock()
		defer source.mu.Unlock()
		if source.closed {
			return socialhub.Token{}, platformError("token", socialhub.CodeConflict, socialhub.ClassPermanent, nil)
		}
		if source.token.AccessToken == cached.AccessToken {
			source.storeDirty = false
		}
		return cached, nil
	}
	if store != nil && !storeDirty {
		stored, err := store.Get(ctx, key)
		if err == nil {
			if validStoredToken(stored, now) {
				source.mu.Lock()
				defer source.mu.Unlock()
				if source.closed {
					return socialhub.Token{}, platformError("token", socialhub.CodeConflict, socialhub.ClassPermanent, nil)
				}
				source.token = stored
				source.refreshToken = stored.RefreshToken
				source.storeDirty = false
				return stored, nil
			}
			if validOpaque(stored.RefreshToken, 16_384) {
				refreshToken = stored.RefreshToken
			}
		} else if !errors.Is(err, socialhub.ErrNotFound) {
			return socialhub.Token{}, dependencyError("token_cache_get", "token store read failed")
		}
	}
	if !validOpaque(refreshToken, 16_384) {
		return socialhub.Token{}, credentialError("token")
	}
	token, err := source.oauth.Refresh(ctx, refreshToken)
	if err != nil {
		return socialhub.Token{}, err
	}
	source.mu.Lock()
	if source.closed {
		source.mu.Unlock()
		return socialhub.Token{}, platformError("token", socialhub.CodeConflict, socialhub.ClassPermanent, nil)
	}
	source.token = token
	source.refreshToken = token.RefreshToken
	source.storeDirty = store != nil
	source.mu.Unlock()
	if store != nil {
		if err := store.Put(ctx, key, token); err != nil {
			return socialhub.Token{}, dependencyError("token_cache_put", "token store write failed")
		}
	}
	source.mu.Lock()
	defer source.mu.Unlock()
	if source.closed {
		return socialhub.Token{}, platformError("token", socialhub.CodeConflict, socialhub.ClassPermanent, nil)
	}
	if source.token.AccessToken == token.AccessToken {
		source.storeDirty = false
	}
	return token, nil
}

func (source *refreshTokenSource) Close() {
	if source == nil {
		return
	}
	source.refreshMu.Lock()
	defer source.refreshMu.Unlock()
	source.mu.Lock()
	source.refreshToken, source.token = "", socialhub.Token{}
	source.store, source.key, source.storeDirty, source.closed = nil, socialhub.TokenKey{}, false, true
	source.mu.Unlock()
	source.oauth.Close()
}

func validStoredToken(token socialhub.Token, now time.Time) bool {
	return validOpaque(token.AccessToken, 16_384) && validOpaque(token.RefreshToken, 16_384) &&
		strings.EqualFold(token.TokenType, "Bearer") && !token.ExpiresAt.IsZero() &&
		token.ExpiresAt.After(now.Add(2*time.Minute)) && !token.ExpiresAt.After(now.Add(24*time.Hour)) &&
		validStoredScopes(token.Scopes)
}

func validStoredScopes(scopes []string) bool {
	if len(scopes) > 32 {
		return false
	}
	seen := make(map[string]struct{}, len(scopes))
	for _, scope := range scopes {
		if !validOpaque(scope, 256) {
			return false
		}
		if _, duplicate := seen[scope]; duplicate {
			return false
		}
		seen[scope] = struct{}{}
	}
	return true
}
