package applemusic

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"strings"
	"testing"
	"time"
)

func TestDeveloperTokenSourceSignsCachesAndRefreshes(t *testing.T) {
	clock := &testClock{now: testNow}
	source, err := newDeveloperTokenSource("TEAM123456", "KEY1234567", testPrivateKeyPEM(t, elliptic.P256()), time.Hour, clock)
	if err != nil {
		t.Fatal(err)
	}
	first, err := source.Token(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	second, err := source.Token(context.Background())
	if err != nil || first.AccessToken != second.AccessToken {
		t.Fatalf("cached token mismatch err=%v", err)
	}
	parts := strings.Split(first.AccessToken, ".")
	if len(parts) != 3 {
		t.Fatalf("JWT parts=%d", len(parts))
	}
	var header struct {
		Algorithm string `json:"alg"`
		KeyID     string `json:"kid"`
		Type      string `json:"typ"`
	}
	decodeJSONPart(t, parts[0], &header)
	var claims struct {
		Issuer   string `json:"iss"`
		IssuedAt int64  `json:"iat"`
		Expires  int64  `json:"exp"`
	}
	decodeJSONPart(t, parts[1], &claims)
	if header.Algorithm != "ES256" || header.KeyID != "KEY1234567" || header.Type != "JWT" ||
		claims.Issuer != "TEAM123456" || claims.IssuedAt != testNow.Unix() || claims.Expires != testNow.Add(time.Hour).Unix() {
		t.Fatalf("header=%#v claims=%#v", header, claims)
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || len(signature) != 64 {
		t.Fatalf("signature length=%d err=%v", len(signature), err)
	}
	digest := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	r := new(big.Int).SetBytes(signature[:32])
	s := new(big.Int).SetBytes(signature[32:])
	if !ecdsa.Verify(&source.privateKey.PublicKey, digest[:], r, s) {
		t.Fatal("developer token signature did not verify")
	}
	clock.now = testNow.Add(56 * time.Minute)
	refreshed, err := source.Token(context.Background())
	if err != nil || refreshed.AccessToken == first.AccessToken || !refreshed.ExpiresAt.Equal(clock.now.Add(time.Hour)) {
		t.Fatalf("refreshed token=%#v err=%v", refreshed, err)
	}
}

func TestDeveloperTokenSourceRejectsInvalidKeysAndSignatures(t *testing.T) {
	clock := &testClock{now: testNow}
	for _, key := range []string{"not pem", testPrivateKeyPEM(t, elliptic.P384())} {
		if _, err := newDeveloperTokenSource("TEAM123456", "KEY1234567", key, time.Hour, clock); err == nil {
			t.Fatalf("key must fail: %.16q", key)
		}
	}
	valid := testPrivateKeyPEM(t, elliptic.P256())
	if _, err := newDeveloperTokenSource("TEAM123456", "KEY1234567", valid+valid, time.Hour, clock); err == nil {
		t.Fatal("multiple PEM blocks must fail")
	}
	if _, err := rawECDSASignature(nil, big.NewInt(1)); err == nil {
		t.Fatal("nil signature value must fail")
	}
	tooLarge := new(big.Int).Lsh(big.NewInt(1), 257)
	if _, err := rawECDSASignature(tooLarge, big.NewInt(1)); err == nil {
		t.Fatal("oversized signature value must fail")
	}
}

func decodeJSONPart(t *testing.T, value string, target any) {
	t.Helper()
	decoded, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil || json.Unmarshal(decoded, target) != nil {
		t.Fatalf("decode JWT part err=%v", err)
	}
}
