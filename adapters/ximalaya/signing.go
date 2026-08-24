package ximalaya

import (
	"crypto/hmac"
	"crypto/md5"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"social-hub/pkg/socialhub"
)

type requestSigner struct {
	appKey       string
	signingKey   []byte
	clientOSType int
	deviceID     string
	deviceIDType DeviceIDType
	clock        socialhub.Clock
}

func (signer *requestSigner) Authenticate(request *http.Request, _ socialhub.Token) error {
	if request.Method != http.MethodGet {
		return errors.New("ximalaya: signer supports GET requests only")
	}
	query := request.URL.Query()
	for _, reserved := range []string{
		"app_key", "client_os_type", "device_id", "device_id_type", "nonce", "server_api_version", "sig", "timestamp",
	} {
		if _, exists := query[reserved]; exists {
			return errors.New("ximalaya: request contains a reserved signing parameter")
		}
	}
	nonce, err := newNonce()
	if err != nil {
		return errors.New("ximalaya: generate nonce")
	}
	timestamp := signer.clock.Now().UnixMilli()
	if timestamp <= 0 {
		return errors.New("ximalaya: invalid signing time")
	}
	query.Set("app_key", signer.appKey)
	query.Set("client_os_type", strconv.Itoa(signer.clientOSType))
	query.Set("device_id", signer.deviceID)
	query.Set("device_id_type", string(signer.deviceIDType))
	query.Set("nonce", nonce)
	query.Set("server_api_version", apiVersion)
	query.Set("timestamp", strconv.FormatInt(timestamp, 10))
	signature, err := signValues(query, signer.signingKey)
	if err != nil {
		return err
	}
	query.Set("sig", signature)
	request.URL.RawQuery = query.Encode()
	return nil
}

func newNonce() (string, error) {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(value[:]), nil
}

func signValues(values url.Values, signingKey []byte) (string, error) {
	keys := make([]string, 0, len(values))
	for key, entries := range values {
		if key == "sig" {
			continue
		}
		if len(entries) != 1 {
			return "", errors.New("ximalaya: signing parameters must be single-valued")
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var canonical strings.Builder
	for index, key := range keys {
		if index > 0 {
			canonical.WriteByte('&')
		}
		canonical.WriteString(key)
		canonical.WriteByte('=')
		canonical.WriteString(values.Get(key))
	}
	encoded := base64.StdEncoding.EncodeToString([]byte(canonical.String()))
	mac := hmac.New(sha1.New, signingKey)
	_, _ = mac.Write([]byte(encoded))
	result := md5.Sum(mac.Sum(nil))
	return hex.EncodeToString(result[:]), nil
}
