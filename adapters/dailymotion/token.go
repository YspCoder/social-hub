package dailymotion

import (
	"context"
	"errors"
	"sync"
	"time"

	"social-hub/pkg/socialhub"
)

type clientTokenSource struct {
	mu     sync.Mutex
	oauth  OAuthClient
	scopes []string
	store  socialhub.TokenStore
	key    socialhub.TokenKey
	token  socialhub.Token
}

func (s *clientTokenSource) Token(ctx context.Context) (socialhub.Token, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.oauth.Clock.Now()
	if s.token.Valid(now.Add(2 * time.Minute)) {
		return s.token, nil
	}
	if s.store != nil {
		stored, err := s.store.Get(ctx, s.key)
		if err == nil && stored.Valid(now.Add(2*time.Minute)) {
			s.token = stored
			return stored, nil
		}
		if err != nil && !errors.Is(err, socialhub.ErrNotFound) {
			return socialhub.Token{}, platformError("token_cache_get", socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
		}
	}
	token, err := s.oauth.ClientCredentials(ctx, s.scopes)
	if err != nil {
		return socialhub.Token{}, err
	}
	if s.store != nil {
		if err := s.store.Put(ctx, s.key, token); err != nil {
			return socialhub.Token{}, platformError("token_cache_put", socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
		}
	}
	s.token = token
	return token, nil
}
