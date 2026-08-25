package shopeeads

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strconv"
	"sync"

	"social-hub/pkg/socialhub"
)

type shopAuthenticator struct {
	mu         sync.RWMutex
	partnerID  int64
	partnerKey string
	shopID     int64
	clock      socialhub.Clock
	closed     bool
}

func (authenticator *shopAuthenticator) Authenticate(request *http.Request, token socialhub.Token) error {
	if authenticator == nil || request == nil || request.URL == nil || !validOpaque(token.AccessToken, 16_384) {
		return fmt.Errorf("shopeeads: incomplete request signer")
	}
	authenticator.mu.RLock()
	if authenticator.closed {
		authenticator.mu.RUnlock()
		return fmt.Errorf("shopeeads: request signer is closed")
	}
	partnerID, partnerKey, shopID, clock := authenticator.partnerID, authenticator.partnerKey, authenticator.shopID, authenticator.clock
	authenticator.mu.RUnlock()
	if clock == nil || !validPartnerID(partnerID) || !validShopID(shopID) || !validOpaque(partnerKey, 16_384) {
		return fmt.Errorf("shopeeads: incomplete request signer")
	}
	timestamp := clock.Now().Unix()
	if timestamp <= 0 {
		return fmt.Errorf("shopeeads: invalid request timestamp")
	}
	query := request.URL.Query()
	query.Set("partner_id", formatID(partnerID))
	query.Set("timestamp", strconv.FormatInt(timestamp, 10))
	query.Set("access_token", token.AccessToken)
	query.Set("shop_id", formatID(shopID))
	query.Set("sign", signature(partnerKey, partnerID, request.URL.Path, timestamp, token.AccessToken, shopID))
	request.URL.RawQuery = query.Encode()
	return nil
}

func (authenticator *shopAuthenticator) Close() {
	if authenticator == nil {
		return
	}
	authenticator.mu.Lock()
	authenticator.partnerID, authenticator.shopID, authenticator.partnerKey = 0, 0, ""
	authenticator.clock, authenticator.closed = nil, true
	authenticator.mu.Unlock()
}

func publicSignature(partnerKey string, partnerID int64, path string, timestamp int64) string {
	return signature(partnerKey, partnerID, path, timestamp, "", 0)
}

func signature(partnerKey string, partnerID int64, path string, timestamp int64, accessToken string, shopID int64) string {
	base := make([]byte, 0, 256)
	base = strconv.AppendInt(base, partnerID, 10)
	base = append(base, path...)
	base = strconv.AppendInt(base, timestamp, 10)
	if accessToken != "" {
		base = append(base, accessToken...)
		base = strconv.AppendInt(base, shopID, 10)
	}
	mac := hmac.New(sha256.New, []byte(partnerKey))
	_, _ = mac.Write(base)
	return hex.EncodeToString(mac.Sum(nil))
}
