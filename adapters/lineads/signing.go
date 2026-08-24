package lineads

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"social-hub/pkg/socialhub"
)

type joseHeader struct {
	Algorithm string `json:"alg"`
	KeyID     string `json:"kid"`
	Type      string `json:"typ"`
}

func authenticateRequest(request *http.Request, accessKey, secretKey string, clock socialhub.Clock) error {
	if request == nil || request.URL == nil || clock == nil || !validOpaque(accessKey, 256) || !validOpaque(secretKey, 16_384) {
		return socialhub.ErrUnauthenticated
	}
	if request.Body != nil {
		return fmt.Errorf("lineads: signer only supports bodyless read requests")
	}
	now := clock.Now().UTC()
	header, err := json.Marshal(joseHeader{Algorithm: "HS256", KeyID: accessKey, Type: "text/plain"})
	if err != nil {
		return err
	}
	emptyDigest := sha256.Sum256(nil)
	canonicalURI := request.URL.EscapedPath()
	if canonicalURI == "" || !strings.HasPrefix(canonicalURI, "/") {
		return fmt.Errorf("lineads: invalid canonical URI")
	}
	payload := fmt.Sprintf("%x\n\n%s\n%s", emptyDigest, now.Format("20060102"), canonicalURI)
	encodedHeader := base64.URLEncoding.EncodeToString(header)
	encodedPayload := base64.URLEncoding.EncodeToString([]byte(payload))
	signingInput := encodedHeader + "." + encodedPayload
	mac := hmac.New(sha256.New, []byte(secretKey))
	_, _ = mac.Write([]byte(signingInput))
	signature := base64.URLEncoding.EncodeToString(mac.Sum(nil))
	request.Header.Set("Date", now.Format(http.TimeFormat))
	request.Header.Set("Authorization", "Bearer "+signingInput+"."+signature)
	return nil
}
