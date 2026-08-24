package vipunion

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

func prepareCallOptions(operation string, options []socialhub.CallOption) (string, []socialhub.CallOption, error) {
	resolved, err := socialhub.ResolveCallOptions(options...)
	if err != nil {
		return "", nil, platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	if resolved.IdempotencyKey != "" {
		return "", nil, invalidArgument(operation, "Vipshop Union query and link workflows do not define idempotency keys")
	}
	if len(resolved.Fields) > 0 {
		return "", nil, invalidArgument(operation, "field selection is fixed by the typed Vipshop method")
	}
	requestID := resolved.RequestID
	if requestID != "" && !validRequestID(requestID) {
		return "", nil, invalidArgument(operation, "request ID is invalid")
	}
	if requestID == "" {
		var bytes [16]byte
		if _, err := rand.Read(bytes[:]); err != nil {
			return "", nil, platformError(operation, socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
		}
		requestID = hex.EncodeToString(bytes[:])
	}
	forwarded := append([]socialhub.CallOption(nil), options...)
	forwarded = append(forwarded, socialhub.WithRequestID(requestID))
	return requestID, forwarded, nil
}

func validOpaque(value string, maximum int) bool {
	if value == "" || value != strings.TrimSpace(value) || len(value) > maximum || !utf8.ValidString(value) {
		return false
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return false
		}
	}
	return true
}

func validOptionalOpaque(value string, maximum int) bool {
	return value == "" || validOpaque(value, maximum)
}

func validIdentifier(value string, maximum int) bool {
	if !validOpaque(value, maximum) {
		return false
	}
	for index := range value {
		character := value[index]
		if !asciiLetterOrDigit(character) && character != '_' && character != '-' {
			return false
		}
	}
	return true
}

func validRequestID(value string) bool { return validIdentifier(value, 128) }

func validChanTag(value string) bool {
	if !validOpaque(value, 64) {
		return false
	}
	for index := range value {
		character := value[index]
		if !asciiLetterOrDigit(character) && character != '_' {
			return false
		}
	}
	return true
}

func validOpenID(value string) bool {
	if !validOpaque(value, 32) {
		return false
	}
	for index := range value {
		character := value[index]
		if !asciiLetterOrDigit(character) && character != '_' {
			return false
		}
	}
	return true
}

func asciiLetterOrDigit(character byte) bool {
	return character >= '0' && character <= '9' || character >= 'A' && character <= 'Z' ||
		character >= 'a' && character <= 'z'
}

func validGoodsID(value string) bool {
	if !validOpaque(value, 128) {
		return false
	}
	for _, character := range value {
		if unicode.IsSpace(character) || character == ',' {
			return false
		}
	}
	return true
}

func validGoodsIDs(values []string, maximum int) bool {
	if len(values) == 0 || len(values) > maximum {
		return false
	}
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if !validGoodsID(value) {
			return false
		}
		if _, found := seen[value]; found {
			return false
		}
		seen[value] = struct{}{}
	}
	return true
}

func validSizeIDs(values map[string]string, goodsIDs []string) bool {
	if len(values) == 0 {
		return true
	}
	allowed := make(map[string]struct{}, len(goodsIDs))
	for _, goodsID := range goodsIDs {
		allowed[goodsID] = struct{}{}
	}
	for goodsID, sizeID := range values {
		if _, found := allowed[goodsID]; !found || !validGoodsID(sizeID) {
			return false
		}
	}
	return true
}

func validDecimal(value string) bool {
	if value == "" || len(value) > 64 || value != strings.TrimSpace(value) {
		return false
	}
	digits, points := 0, 0
	for _, character := range value {
		switch {
		case character >= '0' && character <= '9':
			digits++
		case character == '.':
			points++
		default:
			return false
		}
	}
	if digits == 0 || points > 1 {
		return false
	}
	_, ok := new(big.Rat).SetString(value)
	return ok
}

func validPriceRange(start, end string) bool {
	if start == "" && end == "" {
		return true
	}
	if start != "" && !validDecimal(start) || end != "" && !validDecimal(end) {
		return false
	}
	if start == "" || end == "" {
		return true
	}
	left, _ := new(big.Rat).SetString(start)
	right, _ := new(big.Rat).SetString(end)
	return left.Cmp(right) <= 0
}

func validTimeRange(start, end time.Time) bool {
	if start.IsZero() && end.IsZero() {
		return true
	}
	return !start.IsZero() && !end.IsZero() && start.UnixMilli() > 0 && !end.Before(start) &&
		end.Sub(start) <= time.Hour
}

func validOrderSNs(values []string) bool {
	if len(values) > 50 {
		return false
	}
	for _, value := range values {
		if !validOpaque(value, 128) {
			return false
		}
	}
	return true
}

func milliseconds(value time.Time) int64 {
	if value.IsZero() {
		return 0
	}
	return value.UnixMilli()
}

func invalidEnum(operation, field string) error {
	return invalidArgument(operation, fmt.Sprintf("%s is invalid", field))
}
