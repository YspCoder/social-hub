package applemusic

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"sync"
	"time"

	"social-hub/pkg/socialhub"
)

type developerTokenSource struct {
	mu         sync.Mutex
	teamID     string
	keyID      string
	privateKey *ecdsa.PrivateKey
	ttl        time.Duration
	clock      socialhub.Clock
	token      socialhub.Token
}

func newDeveloperTokenSource(teamID, keyID, privateKeyPEM string, ttl time.Duration, clock socialhub.Clock) (*developerTokenSource, error) {
	block, rest := pem.Decode([]byte(privateKeyPEM))
	if block == nil || block.Type != "PRIVATE KEY" || strings.TrimSpace(string(rest)) != "" {
		return nil, errors.New("Apple private key must be one PKCS#8 PEM block")
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse Apple PKCS#8 private key: %w", err)
	}
	privateKey, ok := parsed.(*ecdsa.PrivateKey)
	if !ok || privateKey.Curve != elliptic.P256() {
		return nil, errors.New("Apple private key must be an ECDSA P-256 key")
	}
	return &developerTokenSource{teamID: teamID, keyID: keyID, privateKey: privateKey, ttl: ttl, clock: clock}, nil
}

func (s *developerTokenSource) Token(context.Context) (socialhub.Token, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.clock.Now().UTC()
	leeway := s.ttl / 10
	if leeway > 5*time.Minute {
		leeway = 5 * time.Minute
	}
	if s.token.Valid(now.Add(leeway)) {
		return s.token, nil
	}
	header, err := json.Marshal(struct {
		Algorithm string `json:"alg"`
		KeyID     string `json:"kid"`
		Type      string `json:"typ"`
	}{Algorithm: "ES256", KeyID: s.keyID, Type: "JWT"})
	if err != nil {
		return socialhub.Token{}, err
	}
	expiresAt := now.Add(s.ttl)
	claims, err := json.Marshal(struct {
		Issuer   string `json:"iss"`
		IssuedAt int64  `json:"iat"`
		Expires  int64  `json:"exp"`
	}{Issuer: s.teamID, IssuedAt: now.Unix(), Expires: expiresAt.Unix()})
	if err != nil {
		return socialhub.Token{}, err
	}
	encoded := base64.RawURLEncoding
	signingInput := encoded.EncodeToString(header) + "." + encoded.EncodeToString(claims)
	digest := sha256.Sum256([]byte(signingInput))
	r, signatureS, err := ecdsa.Sign(rand.Reader, s.privateKey, digest[:])
	if err != nil {
		return socialhub.Token{}, fmt.Errorf("sign Apple developer token: %w", err)
	}
	signature, err := rawECDSASignature(r, signatureS)
	if err != nil {
		return socialhub.Token{}, err
	}
	s.token = socialhub.Token{
		AccessToken: signingInput + "." + encoded.EncodeToString(signature),
		TokenType:   "Bearer", ExpiresAt: expiresAt,
	}
	return s.token, nil
}

func rawECDSASignature(r, s *big.Int) ([]byte, error) {
	if r == nil || s == nil || r.Sign() <= 0 || s.Sign() <= 0 || r.BitLen() > 256 || s.BitLen() > 256 {
		return nil, errors.New("invalid ECDSA signature values")
	}
	signature := make([]byte, 64)
	r.FillBytes(signature[:32])
	s.FillBytes(signature[32:])
	return signature, nil
}

var _ socialhub.TokenSource = (*developerTokenSource)(nil)
