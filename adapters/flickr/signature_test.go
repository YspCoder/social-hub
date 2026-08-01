package flickr

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"testing"

	"github.com/dghubble/oauth1"
)

func verifyOAuthSignature(t *testing.T, request *http.Request, consumerSecret, tokenSecret string, multipartRequest bool) {
	t.Helper()
	oauthParameters, err := parseOAuthHeader(request.Header.Get("Authorization"))
	if err != nil {
		t.Fatal(err)
	}
	signature := oauthParameters["oauth_signature"]
	delete(oauthParameters, "oauth_signature")
	delete(oauthParameters, "realm")
	parameters := map[string]string{}
	for key, values := range request.URL.Query() {
		parameters[key] = values[0]
	}
	if multipartRequest {
		if err := request.ParseMultipartForm(1 << 20); err != nil {
			t.Fatalf("ParseMultipartForm: %v", err)
		}
		for key, values := range request.MultipartForm.Value {
			parameters[key] = values[0]
		}
	} else if request.Method == http.MethodPost {
		if err := request.ParseForm(); err != nil {
			t.Fatalf("ParseForm: %v", err)
		}
		for key, values := range request.PostForm {
			parameters[key] = values[0]
		}
	}
	for key, value := range oauthParameters {
		parameters[key] = value
	}
	pairs := make([]string, 0, len(parameters))
	for key, value := range parameters {
		pairs = append(pairs, oauth1.PercentEncode(key)+"="+oauth1.PercentEncode(value))
	}
	sort.Strings(pairs)
	baseURI := "http://" + request.Host + request.URL.EscapedPath()
	signatureBase := request.Method + "&" + oauth1.PercentEncode(baseURI) + "&" + oauth1.PercentEncode(strings.Join(pairs, "&"))
	key := oauth1.PercentEncode(consumerSecret) + "&" + oauth1.PercentEncode(tokenSecret)
	mac := hmac.New(sha1.New, []byte(key))
	_, _ = mac.Write([]byte(signatureBase))
	expected := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(signature), []byte(expected)) {
		t.Fatalf("signature=%q expected=%q base=%s", signature, expected, signatureBase)
	}
}

func parseOAuthHeader(header string) (map[string]string, error) {
	if !strings.HasPrefix(header, "OAuth ") {
		return nil, fmt.Errorf("missing OAuth header: %q", header)
	}
	parameters := map[string]string{}
	for _, pair := range strings.Split(strings.TrimPrefix(header, "OAuth "), ",") {
		parts := strings.SplitN(strings.TrimSpace(pair), "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid OAuth pair %q", pair)
		}
		value := strings.Trim(parts[1], `"`)
		decoded, err := url.QueryUnescape(value)
		if err != nil {
			return nil, err
		}
		parameters[parts[0]] = decoded
	}
	return parameters, nil
}
