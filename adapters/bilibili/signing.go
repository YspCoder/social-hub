package bilibili

import (
	"bytes"
	"crypto/hmac"
	"crypto/md5"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"social-hub/pkg/socialhub"
)

const maxSignedBodyBytes int64 = 8 << 20

type requestSigner struct {
	clientID  string
	appSecret string
	clock     socialhub.Clock
}

func (s *requestSigner) Authenticate(request *http.Request, token socialhub.Token) error {
	var body []byte
	if request.Body != nil {
		var err error
		body, err = io.ReadAll(io.LimitReader(request.Body, maxSignedBodyBytes+1))
		if err != nil {
			return err
		}
		if int64(len(body)) > maxSignedBodyBytes {
			return fmt.Errorf("bilibili: signed request body exceeded size limit")
		}
		request.Body = io.NopCloser(bytes.NewReader(body))
	}
	sum := md5.Sum(body)
	request.Header.Set("Accept", "application/json")
	if request.Header.Get("Content-Type") == "" {
		request.Header.Set("Content-Type", "application/json")
	}
	return s.sign(request, token, hex.EncodeToString(sum[:]))
}

func (s *requestSigner) sign(request *http.Request, token socialhub.Token, contentMD5 string) error {
	if s.clientID == "" || s.appSecret == "" || s.clock == nil || token.AccessToken == "" {
		return fmt.Errorf("bilibili: incomplete request signer")
	}
	nonceBytes := make([]byte, 16)
	if _, err := rand.Read(nonceBytes); err != nil {
		return err
	}
	nonce := hex.EncodeToString(nonceBytes)
	timestamp := strconv.FormatInt(s.clock.Now().Unix(), 10)
	headers := map[string]string{
		"x-bili-accesskeyid":       s.clientID,
		"x-bili-content-md5":       contentMD5,
		"x-bili-signature-method":  "HMAC-SHA256",
		"x-bili-signature-nonce":   nonce,
		"x-bili-signature-version": "2.0",
		"x-bili-timestamp":         timestamp,
	}
	signature := signatureFor(headers, s.appSecret)
	for name, value := range headers {
		request.Header.Set(name, value)
	}
	request.Header.Set("Access-Token", token.AccessToken)
	request.Header.Set("Authorization", signature)
	return nil
}

func signatureFor(headers map[string]string, secret string) string {
	keys := []string{
		"x-bili-accesskeyid",
		"x-bili-content-md5",
		"x-bili-signature-method",
		"x-bili-signature-nonce",
		"x-bili-signature-version",
		"x-bili-timestamp",
	}
	lines := make([]string, 0, len(keys))
	for _, key := range keys {
		lines = append(lines, key+":"+headers[key])
	}
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(strings.Join(lines, "\n")))
	return hex.EncodeToString(mac.Sum(nil))
}
