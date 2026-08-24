package aliexpressaffiliate

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"social-hub/pkg/socialhub"
)

const maxRequestBytes = 1 << 20

var topTimeZone = time.FixedZone("GMT+8", 8*60*60)

type topAuthenticator struct {
	appKey string
	clock  socialhub.Clock
}

func (authenticator *topAuthenticator) Authenticate(request *http.Request, token socialhub.Token) error {
	if request.Body == nil || token.AccessToken == "" {
		return fmt.Errorf("aliexpressaffiliate: request body and app secret are required")
	}
	body, err := io.ReadAll(io.LimitReader(request.Body, maxRequestBytes+1))
	_ = request.Body.Close()
	if err != nil {
		return fmt.Errorf("aliexpressaffiliate: read request form: %w", err)
	}
	if len(body) > maxRequestBytes {
		return fmt.Errorf("aliexpressaffiliate: request form exceeds 1 MiB")
	}
	business, err := url.ParseQuery(string(body))
	if err != nil {
		return fmt.Errorf("aliexpressaffiliate: parse request form: %w", err)
	}
	query := request.URL.Query()
	methodValues := query["method"]
	if len(methodValues) != 1 || !validAPIMethod(methodValues[0]) {
		return fmt.Errorf("aliexpressaffiliate: one supported route method is required")
	}
	method := methodValues[0]
	common := map[string]string{
		"app_key":     authenticator.appKey,
		"format":      "json",
		"method":      method,
		"sign_method": "sha256",
		"simplify":    "true",
		"timestamp":   authenticator.clock.Now().In(topTimeZone).Format("2006-01-02 15:04:05"),
		"v":           apiVersion,
	}
	all := make(map[string]string, len(common)+len(business))
	for key, value := range common {
		all[key] = value
	}
	for key, values := range business {
		if _, reserved := common[key]; reserved || key == "sign" || len(values) != 1 {
			return fmt.Errorf("aliexpressaffiliate: invalid or reserved business parameter %q", key)
		}
		if values[0] != "" {
			all[key] = values[0]
		}
	}
	common["sign"] = signHMACSHA256(all, token.AccessToken)
	for _, key := range []string{"app_key", "format", "method", "sign_method", "simplify", "timestamp", "v", "sign"} {
		query.Set(key, common[key])
	}
	request.URL.RawQuery = query.Encode()
	if len(request.URL.RawQuery)+len(body) > maxRequestBytes {
		return fmt.Errorf("aliexpressaffiliate: signed request exceeds 1 MiB")
	}
	request.Body = io.NopCloser(strings.NewReader(string(body)))
	request.GetBody = func() (io.ReadCloser, error) { return io.NopCloser(strings.NewReader(string(body))), nil }
	request.ContentLength = int64(len(body))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded;charset=utf-8")
	return nil
}

func signHMACSHA256(values map[string]string, secret string) string {
	keys := make([]string, 0, len(values))
	for key, value := range values {
		if key != "" && value != "" {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	var builder strings.Builder
	for _, key := range keys {
		builder.WriteString(key)
		builder.WriteString(values[key])
	}
	digest := hmac.New(sha256.New, []byte(secret))
	_, _ = digest.Write([]byte(builder.String()))
	return strings.ToUpper(hex.EncodeToString(digest.Sum(nil)))
}

func validAPIMethod(value string) bool {
	switch value {
	case productQueryMethod, productDetailMethod, linkGenerateMethod, orderListMethod, orderGetMethod:
		return true
	default:
		return false
	}
}
