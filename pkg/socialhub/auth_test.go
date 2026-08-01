package socialhub

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestTokenValidity(t *testing.T) {
	t.Parallel()
	now := time.Unix(100, 0)
	if !(Token{AccessToken: "token"}).Valid(now) {
		t.Fatal("token without expiry should be valid")
	}
	if (Token{AccessToken: "token", ExpiresAt: now}).Valid(now) {
		t.Fatal("expired token should be invalid")
	}
}

func TestMemoryTokenStoreLifecycle(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := NewMemoryTokenStore()
	key := TokenKey{Platform: "x", Account: "primary"}
	if _, err := store.Get(ctx, key); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing token error = %v", err)
	}
	if err := store.Put(ctx, key, Token{AccessToken: "secret"}); err != nil {
		t.Fatal(err)
	}
	got, err := store.Get(ctx, key)
	if err != nil {
		t.Fatal(err)
	}
	if got.AccessToken != "secret" {
		t.Fatalf("access token = %q", got.AccessToken)
	}
	if err := store.Delete(ctx, key); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Get(ctx, key); !errors.Is(err, ErrNotFound) {
		t.Fatalf("deleted token error = %v", err)
	}
}
