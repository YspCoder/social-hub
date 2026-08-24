package tenor

import (
	"math"
	"net/url"
	"strings"
	"unicode"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

var documentedMediaFormats = map[MediaFormatName]struct{}{
	FormatPreview: {}, FormatGIF: {}, FormatMediumGIF: {}, FormatTinyGIF: {}, FormatNanoGIF: {},
	FormatMP4: {}, FormatLoopedMP4: {}, FormatTinyMP4: {}, FormatNanoMP4: {},
	FormatWebM: {}, FormatTinyWebM: {}, FormatNanoWebM: {},
	FormatTransparentWebP: {}, FormatTinyTransparentWebP: {}, FormatNanoTransparentWebP: {},
	FormatTransparentGIF: {}, FormatTinyTransparentGIF: {}, FormatNanoTransparentGIF: {},
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

func validSearchQuery(value string) bool {
	if value == "" || len(value) > 2048 || !utf8.ValidString(value) {
		return false
	}
	hasContent := false
	for _, character := range value {
		if unicode.IsControl(character) {
			return false
		}
		if !unicode.IsSpace(character) {
			hasContent = true
		}
	}
	return hasContent
}

func validAPIKey(value string) bool { return validOpaque(value, 16_384) }
func validClientKey(value string) bool {
	return validOpaque(value, 256)
}
func validPostID(value string) bool { return validOpaque(value, 256) && !strings.Contains(value, ",") }
func validNextPosition(value string) bool {
	return validOptionalOpaque(value, 4096)
}

func validCountry(value string) bool {
	return value == "" || len(value) == 2 && isUpperASCII(value[0]) && isUpperASCII(value[1])
}

func validLocale(value string) bool {
	if value == "" {
		return true
	}
	if len(value) != 2 && len(value) != 5 {
		return false
	}
	if !isLowerASCII(value[0]) || !isLowerASCII(value[1]) {
		return false
	}
	return len(value) == 2 || value[2] == '_' && isUpperASCII(value[3]) && isUpperASCII(value[4])
}

func isUpperASCII(value byte) bool { return value >= 'A' && value <= 'Z' }
func isLowerASCII(value byte) bool { return value >= 'a' && value <= 'z' }

func validSafetyFilter(value SafetyFilter) bool {
	switch value {
	case "", SafetyOff, SafetyLow, SafetyMedium, SafetyHigh:
		return true
	default:
		return false
	}
}

func validContentKind(value ContentKind) bool {
	switch value {
	case ContentGIF, ContentSticker, ContentAnimatedSticker, ContentStaticSticker:
		return true
	default:
		return false
	}
}

func validAspectRatio(value AspectRatioRange) bool {
	switch value {
	case "", AspectRatioAll, AspectRatioWide, AspectRatioStandard:
		return true
	default:
		return false
	}
}

func validCategoryType(value CategoryType) bool {
	return value == "" || value == CategoryFeatured || value == CategoryTrending
}

func validMediaFormats(values []MediaFormatName) bool {
	if len(values) > len(documentedMediaFormats) {
		return false
	}
	seen := make(map[MediaFormatName]struct{}, len(values))
	for _, value := range values {
		if _, documented := documentedMediaFormats[value]; !documented {
			return false
		}
		if _, duplicate := seen[value]; duplicate {
			return false
		}
		seen[value] = struct{}{}
	}
	return true
}

func validDiscoveryOptions(value DiscoveryOptions) bool {
	return validCountry(value.Country) && validLocale(value.Locale) && validSafetyFilter(value.Safety) &&
		validMediaFormats(value.MediaFormats) && validAspectRatio(value.AspectRatio) &&
		value.Limit >= 0 && value.Limit <= MaximumPageSize && validNextPosition(value.NextPosition)
}

func validateSearch(input SearchRequest) error {
	if !validSearchQuery(input.Query) || !validContentKind(input.Content) || !validDiscoveryOptions(input.DiscoveryOptions) {
		return invalidArgument("search", "query, content, localization, filters, limit, or next position is invalid")
	}
	return nil
}

func validateFeatured(input FeaturedRequest) error {
	if !validContentKind(input.Content) || !validDiscoveryOptions(input.DiscoveryOptions) {
		return invalidArgument("featured", "content, localization, filters, limit, or next position is invalid")
	}
	return nil
}

func validateCategories(input CategoriesRequest) error {
	if !validCategoryType(input.Type) || !validCountry(input.Country) || !validLocale(input.Locale) || !validSafetyFilter(input.Safety) {
		return invalidArgument("categories", "type, country, locale, or content safety filter is invalid")
	}
	return nil
}

func validatePosts(input PostsRequest) error {
	if len(input.IDs) == 0 || len(input.IDs) > MaximumPostIDs || !validMediaFormats(input.MediaFormats) {
		return invalidArgument("posts", "one to 50 IDs and documented media formats are required")
	}
	seen := make(map[string]struct{}, len(input.IDs))
	for _, identifier := range input.IDs {
		if !validPostID(identifier) {
			return invalidArgument("posts", "IDs contain an invalid value")
		}
		if _, duplicate := seen[identifier]; duplicate {
			return invalidArgument("posts", "IDs contain a duplicate")
		}
		seen[identifier] = struct{}{}
	}
	return nil
}

func prepareCallOptions(operation string, options []socialhub.CallOption) error {
	resolved, err := socialhub.ResolveCallOptions(options...)
	if err != nil {
		return platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	if resolved.RequestID != "" {
		return invalidArgument(operation, "Tenor API v2 does not document a caller request-ID header")
	}
	if resolved.IdempotencyKey != "" {
		return invalidArgument(operation, "read-only Tenor operations do not use idempotency keys")
	}
	if len(resolved.Fields) > 0 {
		return invalidArgument(operation, "Tenor response fields are selected with media_filter, not generic fields")
	}
	return nil
}

func validProviderURL(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && (parsed.Scheme == "https" || parsed.Scheme == "http") && parsed.Host != "" && parsed.User == nil
}

func validOptionalProviderURL(value string) bool { return value == "" || validProviderURL(value) }

func validProviderPost(post Post) bool {
	if !validPostID(post.ID) || len(post.MediaFormats) == 0 || math.IsNaN(post.Created) || math.IsInf(post.Created, 0) || post.Created < 0 ||
		!validOptionalProviderURL(post.ItemURL) || !validOptionalProviderURL(post.URL) {
		return false
	}
	for name, media := range post.MediaFormats {
		if !validFormatKey(string(name)) || !validProviderURL(media.URL) || len(media.Dims) != 2 ||
			media.Dims[0] <= 0 || media.Dims[1] <= 0 || math.IsNaN(media.Duration) || math.IsInf(media.Duration, 0) ||
			media.Duration < 0 || media.Size < 0 {
			return false
		}
	}
	return true
}

func validFormatKey(value string) bool {
	if value == "" || len(value) > 128 {
		return false
	}
	for _, character := range value {
		if character >= 'a' && character <= 'z' || character >= '0' && character <= '9' || character == '_' {
			continue
		}
		return false
	}
	return true
}
