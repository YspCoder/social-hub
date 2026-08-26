package amap

import (
	"crypto/md5"
	"encoding/hex"
	"errors"
	"net/http"
	"net/url"
	"sort"
	"strings"

	"social-hub/pkg/socialhub"
)

type queryAuthenticator struct {
	signingSecret string
}

func (authenticator *queryAuthenticator) Authenticate(request *http.Request, token socialhub.Token) error {
	if request.Method != http.MethodGet {
		return errors.New("amap: authenticator supports GET requests only")
	}
	query := request.URL.Query()
	if _, found := query["key"]; found {
		return errors.New("amap: request contains reserved key parameter")
	}
	if _, found := query["sig"]; found {
		return errors.New("amap: request contains reserved sig parameter")
	}
	query.Set("key", token.AccessToken)
	if authenticator.signingSecret != "" {
		signature, err := signValues(query, authenticator.signingSecret)
		if err != nil {
			return err
		}
		query.Set("sig", signature)
	}
	request.URL.RawQuery = query.Encode()
	return nil
}

func signValues(values url.Values, signingSecret string) (string, error) {
	keys := make([]string, 0, len(values))
	for key, entries := range values {
		if key == "sig" {
			continue
		}
		if len(entries) != 1 {
			return "", errors.New("amap: signing parameters must be single-valued")
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
	canonical.WriteString(signingSecret)
	digest := md5.Sum([]byte(canonical.String()))
	return hex.EncodeToString(digest[:]), nil
}
