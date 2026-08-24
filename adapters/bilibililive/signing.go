package bilibililive

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

const maxSignedBodyBytes int64 = 1 << 20

type requestSigner struct {
	accessKeyID     string
	accessKeySecret string
	clock           socialhub.Clock
}

func (signer *requestSigner) Authenticate(request *http.Request, _ socialhub.Token) error {
	if signer.accessKeyID == "" || signer.accessKeySecret == "" || signer.clock == nil {
		return fmt.Errorf("bilibililive: incomplete request signer")
	}
	var body []byte
	if request.Body != nil {
		var err error
		body, err = io.ReadAll(io.LimitReader(request.Body, maxSignedBodyBytes+1))
		if err != nil {
			return err
		}
		if int64(len(body)) > maxSignedBodyBytes {
			return fmt.Errorf("bilibililive: signed request body exceeded size limit")
		}
		request.Body = io.NopCloser(bytes.NewReader(body))
	}
	contentMD5 := md5.Sum(body)
	nonce, err := randomUUID()
	if err != nil {
		return err
	}
	headers := map[string]string{
		"x-bili-accesskeyid":       signer.accessKeyID,
		"x-bili-content-md5":       hex.EncodeToString(contentMD5[:]),
		"x-bili-signature-method":  "HMAC-SHA256",
		"x-bili-signature-nonce":   nonce,
		"x-bili-signature-version": "1.0",
		"x-bili-timestamp":         strconv.FormatInt(signer.clock.Now().Unix(), 10),
	}
	for name, value := range headers {
		request.Header.Set(name, value)
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", signatureFor(headers, signer.accessKeySecret))
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

func randomUUID() (string, error) {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", err
	}
	value[6] = (value[6] & 0x0f) | 0x40
	value[8] = (value[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		value[0:4], value[4:6], value[6:8], value[8:10], value[10:16]), nil
}
