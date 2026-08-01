package socialhub

import (
	"context"
	"errors"
	"sync"
	"time"
)

// TokenKey uniquely scopes credentials across platforms, products, tenants,
// accounts, subjects, and granted scopes.
type TokenKey struct {
	Platform string
	Product  string
	Tenant   string
	Account  string
	Subject  string
	Scopes   string
}

// Token is an OAuth or platform credential bundle.
type Token struct {
	AccessToken  string
	RefreshToken string
	TokenType    string
	ExpiresAt    time.Time
	Scopes       []string
}

// Valid reports whether a token has an access token and has not expired.
func (t Token) Valid(at time.Time) bool {
	return t.AccessToken != "" && (t.ExpiresAt.IsZero() || at.Before(t.ExpiresAt))
}

// TokenSource obtains a token for one configured account.
type TokenSource interface {
	Token(context.Context) (Token, error)
}

// StaticTokenSource is useful for bot tokens and externally managed OAuth tokens.
type StaticTokenSource struct{ Value Token }

// Token returns the configured token without logging or transforming it.
func (s StaticTokenSource) Token(context.Context) (Token, error) {
	if s.Value.AccessToken == "" {
		return Token{}, ErrUnauthenticated
	}
	return s.Value, nil
}

// TokenStore persists token bundles. Implementations must encrypt tokens at rest.
type TokenStore interface {
	Get(context.Context, TokenKey) (Token, error)
	Put(context.Context, TokenKey, Token) error
	Delete(context.Context, TokenKey) error
}

// MemoryTokenStore is a process-local reference implementation intended for
// development and tests. It does not encrypt values and must not be used as a
// production secret store.
type MemoryTokenStore struct {
	mu     sync.RWMutex
	tokens map[TokenKey]Token
}

// NewMemoryTokenStore creates an empty process-local token store.
func NewMemoryTokenStore() *MemoryTokenStore {
	return &MemoryTokenStore{tokens: make(map[TokenKey]Token)}
}

func (s *MemoryTokenStore) Get(_ context.Context, key TokenKey) (Token, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	token, ok := s.tokens[key]
	if !ok {
		return Token{}, ErrNotFound
	}
	return token, nil
}

func (s *MemoryTokenStore) Put(_ context.Context, key TokenKey, token Token) error {
	if token.AccessToken == "" {
		return errors.New("socialhub: access token must not be empty")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tokens[key] = token
	return nil
}

func (s *MemoryTokenStore) Delete(_ context.Context, key TokenKey) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.tokens, key)
	return nil
}
