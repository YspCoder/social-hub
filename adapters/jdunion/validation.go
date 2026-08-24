package jdunion

import (
	"fmt"
	"net/url"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

func validateCallOptions(operation string, options []socialhub.CallOption) error {
	resolved, err := socialhub.ResolveCallOptions(options...)
	if err != nil {
		return platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	if resolved.RequestID != "" {
		return invalidArgument(operation, "JD assigns request IDs; caller request IDs are not supported")
	}
	if resolved.IdempotencyKey != "" {
		return invalidArgument(operation, "JD Union query workflows do not define idempotency keys")
	}
	if len(resolved.Fields) > 0 {
		return invalidArgument(operation, "field selection must use the typed JD request fields")
	}
	return nil
}

func validGatewayURL(value string) bool {
	return value == defaultBaseURL
}

func splitGatewayURL(value string) (string, string, error) {
	parsed, err := url.Parse(value)
	if err != nil || !validGatewayURL(value) {
		return "", "", fmt.Errorf("invalid JD gateway URL")
	}
	return parsed.Scheme + "://" + parsed.Host, parsed.Path, nil
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

func validIdentifier(value string, maximum int) bool {
	if !validOpaque(value, maximum) {
		return false
	}
	for _, character := range value {
		if unicode.IsSpace(character) || character == ',' {
			return false
		}
	}
	return true
}

func validOptionalIdentifier(value string, maximum int) bool {
	return value == "" || validIdentifier(value, maximum)
}

func validASCIITag(value string, maximum int) bool {
	if value == "" {
		return true
	}
	if len(value) > maximum {
		return false
	}
	for index := range value {
		character := value[index]
		if character < '0' || character > '9' {
			if character < 'A' || character > 'Z' {
				if character < 'a' || character > 'z' {
					if character != '_' && character != '-' {
						return false
					}
				}
			}
		}
	}
	return true
}

func validPID(value string) bool {
	if value == "" {
		return true
	}
	parts := strings.Split(value, "_")
	if len(parts) != 3 {
		return false
	}
	for _, part := range parts {
		if !validDigits(part, 32) {
			return false
		}
	}
	return true
}

func validDigits(value string, maximum int) bool {
	if value == "" || len(value) > maximum || value[0] == '0' {
		return false
	}
	for index := range value {
		if value[index] < '0' || value[index] > '9' {
			return false
		}
	}
	return true
}

func validOptionalHTTPURL(value string) bool {
	if value == "" {
		return true
	}
	if !validOpaque(value, 8192) {
		return false
	}
	parsed, err := url.Parse(value)
	return err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host != "" && parsed.User == nil
}

func validOrderRange(start, end time.Time) bool {
	return !start.IsZero() && !end.IsZero() && !end.Before(start) && end.Sub(start) <= time.Hour
}

func uniqueGoodsFields(values []GoodsField) bool {
	seen := make(map[GoodsField]struct{}, len(values))
	for _, value := range values {
		switch value {
		case GoodsFieldVideoInfo, GoodsFieldHotWords, GoodsFieldSimilar, GoodsFieldDocumentInfo,
			GoodsFieldSKULabelInfo, GoodsFieldPromotionLabelInfo, GoodsFieldCompanyType,
			GoodsFieldSeckillSpecialPriceInfo:
		default:
			return false
		}
		if _, found := seen[value]; found {
			return false
		}
		seen[value] = struct{}{}
	}
	return true
}

func uniqueOrderFields(values []OrderField) bool {
	seen := make(map[OrderField]struct{}, len(values))
	for _, value := range values {
		switch value {
		case OrderFieldGoodsInfo, OrderFieldCategoryInfo, OrderFieldKeyword:
		default:
			return false
		}
		if _, found := seen[value]; found {
			return false
		}
		seen[value] = struct{}{}
	}
	return true
}
