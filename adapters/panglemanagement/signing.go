package panglemanagement

import (
	"crypto/rand"
	"crypto/sha1" // #nosec G505 -- Pangle Management API mandates SHA-1 signatures.
	"encoding/hex"
	"math/big"
	"sort"
	"strconv"
	"strings"

	"social-hub/pkg/socialhub"
)

type authWire struct {
	UserID    ID     `json:"user_id"`
	RoleID    ID     `json:"role_id"`
	Timestamp int64  `json:"timestamp"`
	Nonce     int64  `json:"nonce"`
	Sign      string `json:"sign"`
	Version   string `json:"version"`
}

func (client *Client) newAuth(operation string) (authWire, error) {
	timestamp := client.clock.Now().Unix()
	if timestamp <= 0 {
		return authWire{}, invalidArgument(operation, "clock must return a time after the Unix epoch")
	}
	limit := new(big.Int).Lsh(big.NewInt(1), 63)
	random, err := rand.Int(rand.Reader, limit)
	if err != nil {
		return authWire{}, platformError(operation, socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
	}
	nonce := random.Int64()
	return authWire{
		UserID: client.userID, RoleID: client.roleID, Timestamp: timestamp, Nonce: nonce,
		Sign: signManagement(client.securityKey, timestamp, nonce), Version: wireVersion,
	}, nil
}

func signManagement(securityKey string, timestamp, nonce int64) string {
	parts := []string{securityKey, strconv.FormatInt(timestamp, 10), strconv.FormatInt(nonce, 10)}
	sort.Strings(parts)
	sum := sha1.Sum([]byte(strings.Join(parts, ""))) // #nosec G401 -- required by the upstream wire contract.
	return hex.EncodeToString(sum[:])
}
