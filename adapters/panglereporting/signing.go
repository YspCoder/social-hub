package panglereporting

import (
	"crypto/md5" // #nosec G501 -- Pangle Reporting API 2.0 mandates MD5 signatures.
	"encoding/hex"
	"net/url"
	"sort"
	"strings"
)

func signValues(values url.Values, securityKey string) string {
	keys := make([]string, 0, len(values))
	for key := range values {
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
	canonical.WriteString(securityKey)
	sum := md5.Sum([]byte(canonical.String())) // #nosec G401 -- required by the upstream wire contract.
	return hex.EncodeToString(sum[:])
}
