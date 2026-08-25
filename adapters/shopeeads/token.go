package shopeeads

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
	shopID       int64
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
	store, key, storeDirty, shopID := source.store, source.key, source.storeDirty, source.shopID
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
		if storeDirty {
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
	if !storeDirty {
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
	if !validOpaque(refreshToken, 16_384) || !validShopID(shopID) {
		return socialhub.Token{}, credentialError("token")
	}
	token, err := source.oauth.Refresh(ctx, refreshToken, shopID)
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
	source.storeDirty = true
	source.mu.Unlock()
	if err := store.Put(ctx, key, token); err != nil {
		return socialhub.Token{}, dependencyError("token_cache_put", "token store write failed")
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
	source.shopID, source.storeDirty, source.closed = 0, false, true
	source.store, source.key = nil, socialhub.TokenKey{}
	source.mu.Unlock()
	source.oauth.Close()
}

func validStoredToken(token socialhub.Token, now time.Time) bool {
	return validOpaque(token.AccessToken, 16_384) && validOpaque(token.RefreshToken, 16_384) &&
		strings.EqualFold(token.TokenType, "Bearer") && len(token.Scopes) == 0 &&
		!token.ExpiresAt.IsZero() && token.ExpiresAt.After(now.Add(2*time.Minute)) &&
		!token.ExpiresAt.After(now.Add(24*time.Hour))
}
