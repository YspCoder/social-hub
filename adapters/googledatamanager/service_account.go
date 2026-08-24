package googledatamanager

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"social-hub/pkg/socialhub"
)

const jwtBearerGrantType = "urn:ietf:params:oauth:grant-type:jwt-bearer"

type ServiceAccountClient struct {
	Email      string
	PrivateKey *rsa.PrivateKey
	TokenURL   string
	HTTPClient *http.Client
	Clock      socialhub.Clock
	Scopes     []string
}

func (client *ServiceAccountClient) Token(ctx context.Context) (socialhub.Token, error) {
	if !validServiceAccountEmail(client.Email) || client.PrivateKey == nil || client.HTTPClient == nil || client.Clock == nil ||
		client.TokenURL != defaultTokenURL || !validOAuthScopes(client.Scopes) {
		return socialhub.Token{}, invalidArgument("service_account_token", "service-account client is incomplete")
	}
	issuedAt := client.Clock.Now().UTC().Unix()
	header, _ := json.Marshal(map[string]string{"alg": "RS256", "typ": "JWT"})
	claims, _ := json.Marshal(map[string]any{
		"iss": client.Email, "scope": strings.Join(client.Scopes, " "), "aud": client.TokenURL,
		"iat": issuedAt, "exp": issuedAt + int64(time.Hour/time.Second),
	})
	unsigned := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(claims)
	digest := sha256.Sum256([]byte(unsigned))
	signature, err := rsa.SignPKCS1v15(rand.Reader, client.PrivateKey, crypto.SHA256, digest[:])
	if err != nil {
		return socialhub.Token{}, platformError("service_account_token", socialhub.CodeUnauthenticated, socialhub.ClassUserAction, err)
	}
	assertion := unsigned + "." + base64.RawURLEncoding.EncodeToString(signature)
	return requestToken(ctx, client.HTTPClient, client.TokenURL, client.Clock, "service_account_token", url.Values{
		"grant_type": {jwtBearerGrantType}, "assertion": {assertion},
	}, "", false, client.Scopes)
}

func parseRSAPrivateKey(value string) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(value))
	if block == nil {
		return nil, fmt.Errorf("PEM block is missing")
	}
	if parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
		key, ok := parsed.(*rsa.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("PKCS#8 key is not RSA")
		}
		return key, key.Validate()
	}
	key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	return key, key.Validate()
}
