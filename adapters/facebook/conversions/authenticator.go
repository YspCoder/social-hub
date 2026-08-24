package conversions

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"

	"social-hub/pkg/socialhub"
)

type graphAuthenticator struct{ appSecret string }

func (authenticator graphAuthenticator) Authenticate(request *http.Request, token socialhub.Token) error {
	if !validOpaque(token.AccessToken, 16_384) {
		return socialhub.ErrUnauthenticated
	}
	request.Header.Set("Authorization", "Bearer "+token.AccessToken)
	if authenticator.appSecret == "" {
		return nil
	}
	digest := hmac.New(sha256.New, []byte(authenticator.appSecret))
	_, _ = digest.Write([]byte(token.AccessToken))
	query := request.URL.Query()
	query.Set("appsecret_proof", hex.EncodeToString(digest.Sum(nil)))
	request.URL.RawQuery = query.Encode()
	return nil
}
