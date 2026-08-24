package openverse

import (
	"encoding/json"
	"math"
	"net/url"
	"strings"
	"unicode"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

var supportedLicenses = map[License]struct{}{
	LicenseBY: {}, LicenseBYSA: {}, LicenseBYND: {}, LicenseBYNC: {}, LicenseBYNCSA: {},
	LicenseBYNCND: {}, LicenseCC0: {}, LicensePDM: {}, LicenseSampling: {}, LicenseNCSampling: {},
}

var supportedLicenseTypes = map[LicenseType]struct{}{
	LicenseTypeAll: {}, LicenseTypeAllCC: {}, LicenseTypeCommercial: {}, LicenseTypeModification: {},
}

func validOpaque(value string, maximum int) bool {
	if value == "" || value != strings.TrimSpace(value) || len(value) > maximum || !utf8.ValidString(value) {
		return false
	}
	return !strings.ContainsFunc(value, unicode.IsControl)
}

func validOptionalOpaque(value string, maximum int) bool {
	return value == "" || validOpaque(value, maximum)
}

func validBearerToken(value string) bool {
	if !validOpaque(value, 16_384) {
		return false
	}
	for _, character := range value {
		if character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' ||
			character >= '0' && character <= '9' || strings.ContainsRune("-._~+/=", character) {
			continue
		}
		return false
	}
	return true
}

func validSearchText(value string) bool {
	if !utf8.ValidString(value) || utf8.RuneCountInString(value) > 200 || strings.ContainsFunc(value, unicode.IsControl) {
		return false
	}
	return strings.TrimSpace(value) != ""
}

func validOptionalSearchText(value string) bool { return value == "" || validSearchText(value) }

func validSource(value string) bool {
	if !validOpaque(value, 128) {
		return false
	}
	for _, character := range value {
		if character >= 'a' && character <= 'z' || character >= '0' && character <= '9' || character == '_' || character == '-' {
			continue
		}
		return false
	}
	return true
}

func validSources(values []string) bool {
	if len(values) > 50 {
		return false
	}
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if !validSource(value) {
			return false
		}
		if _, duplicate := seen[value]; duplicate {
			return false
		}
		seen[value] = struct{}{}
	}
	return true
}

func validEnumSet[T ~string](values []T, supported map[T]struct{}) bool {
	if len(values) > len(supported) {
		return false
	}
	seen := make(map[T]struct{}, len(values))
	for _, value := range values {
		if _, ok := supported[value]; !ok {
			return false
		}
		if _, duplicate := seen[value]; duplicate {
			return false
		}
		seen[value] = struct{}{}
	}
	return true
}

func validExtension(value string) bool {
	if value == "" {
		return true
	}
	if len(value) > 16 {
		return false
	}
	for _, character := range value {
		if character >= 'a' && character <= 'z' || character >= '0' && character <= '9' {
			continue
		}
		return false
	}
	return true
}

func validCommonSearch(input SearchRequest, authenticated bool) bool {
	if !validOptionalSearchText(input.Query) || !validOptionalSearchText(input.Creator) ||
		!validOptionalSearchText(input.Tags) || !validOptionalSearchText(input.Title) ||
		!validSources(input.Sources) || !validSources(input.ExcludedSources) ||
		!validEnumSet(input.Licenses, supportedLicenses) || !validEnumSet(input.LicenseTypes, supportedLicenseTypes) ||
		!validExtension(input.Extension) {
		return false
	}
	if input.Query == "" && input.Creator == "" && input.Tags == "" && input.Title == "" {
		return false
	}
	if input.Query != "" && (input.Tags != "" || input.Title != "") {
		return false
	}
	if len(input.Sources) != 0 && len(input.ExcludedSources) != 0 {
		return false
	}
	page, pageSize := effectivePagination(input.Page, input.PageSize)
	maximumPageSize := AnonymousMaximumPageSize
	if authenticated {
		maximumPageSize = AuthenticatedMaximumPageSize
	}
	return input.Page >= 0 && input.PageSize >= 0 && pageSize <= maximumPageSize && page <= MaximumSearchDepth/pageSize
}

func validImageSearch(input ImageSearchRequest, authenticated bool) bool {
	if !validCommonSearch(input.SearchRequest, authenticated) {
		return false
	}
	switch input.Category {
	case "", ImageCategoryDigitizedArtwork, ImageCategoryIllustration, ImageCategoryPhotograph:
	default:
		return false
	}
	switch input.AspectRatio {
	case "", ImageAspectTall, ImageAspectWide, ImageAspectSquare:
	default:
		return false
	}
	switch input.Size {
	case "", ImageSizeSmall, ImageSizeMedium, ImageSizeLarge:
		return true
	default:
		return false
	}
}

func validAudioSearch(input AudioSearchRequest, authenticated bool) bool {
	if !validCommonSearch(input.SearchRequest, authenticated) {
		return false
	}
	switch input.Category {
	case "", AudioCategoryAudiobook, AudioCategoryMusic, AudioCategoryNews, AudioCategoryPodcast,
		AudioCategoryPronunciation, AudioCategorySoundEffect:
	default:
		return false
	}
	switch input.Length {
	case "", AudioLengthShortest, AudioLengthShort, AudioLengthMedium, AudioLengthLong:
		return true
	default:
		return false
	}
}

func effectivePagination(page, pageSize int) (int, int) {
	if page == 0 {
		page = 1
	}
	if pageSize == 0 {
		pageSize = AnonymousMaximumPageSize
	}
	return page, pageSize
}

func validUUID(value string) bool {
	if len(value) != 36 || value[8] != '-' || value[13] != '-' || value[18] != '-' || value[23] != '-' {
		return false
	}
	for index, character := range value {
		if index == 8 || index == 13 || index == 18 || index == 23 {
			continue
		}
		if character >= '0' && character <= '9' || character >= 'a' && character <= 'f' || character >= 'A' && character <= 'F' {
			continue
		}
		return false
	}
	return true
}

func validMedia(value Media) bool {
	if !validUUID(value.ID) || value.Filesize != nil && *value.Filesize < 0 ||
		!validOptionalProviderURL(value.ForeignLandingURL) || !validOptionalProviderURL(value.URL) ||
		!validOptionalProviderURL(value.CreatorURL) || !validOptionalProviderURL(value.LicenseURL) ||
		!validOptionalProviderURL(value.Thumbnail) || !validOptionalProviderURL(value.DetailURL) ||
		!validOptionalProviderURL(value.RelatedURL) || value.License == "" || value.Provider == "" || value.Source == "" {
		return false
	}
	for _, tag := range value.Tags {
		if tag.Name == "" || tag.Accuracy != nil && (math.IsNaN(*tag.Accuracy) || math.IsInf(*tag.Accuracy, 0)) {
			return false
		}
	}
	return len(value.UnstableSensitivity) == 0 || jsonValid(value.UnstableSensitivity)
}

func validOptionalProviderURL(value string) bool {
	if value == "" {
		return true
	}
	if !validOpaque(value, 16_384) {
		return false
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.User != nil {
		return false
	}
	switch parsed.Scheme {
	case "https", "http":
		return parsed.Host != ""
	case "data":
		return parsed.Opaque != ""
	default:
		return false
	}
}

func validImage(value Image) bool {
	return validMedia(value.Media) && (value.Height == nil || *value.Height >= 0) && (value.Width == nil || *value.Width >= 0)
}

func validAudio(value Audio) bool {
	if !validMedia(value.Media) || !validOptionalNonnegative(value.Duration) || !validOptionalNonnegative(value.BitRate) ||
		!validOptionalNonnegative(value.SampleRate) || !validOptionalProviderURL(value.Waveform) {
		return false
	}
	for _, file := range value.AltFiles {
		if !validOptionalProviderURL(file.URL) || !validOptionalNonnegative(file.BitRate) ||
			!validOptionalNonnegative(file.Filesize) || !validOptionalNonnegative(file.SampleRate) {
			return false
		}
	}
	if value.AudioSet != nil && (!validOptionalProviderURL(value.AudioSet.ForeignLandingURL) ||
		!validOptionalProviderURL(value.AudioSet.CreatorURL) || !validOptionalProviderURL(value.AudioSet.URL) ||
		!validOptionalNonnegative(value.AudioSet.Filesize)) {
		return false
	}
	return true
}

func validOptionalNonnegative(value *int64) bool { return value == nil || *value >= 0 }

func jsonValid(value []byte) bool {
	return json.Valid(value)
}

func validImageSearchResponse(value ImageSearchResponse, input ImageSearchRequest) bool {
	page, pageSize := effectivePagination(input.Page, input.PageSize)
	if value.ResultCount < 0 || value.PageCount < 0 || value.Page != page || value.PageSize != pageSize || len(value.Results) > value.PageSize {
		return false
	}
	for _, image := range value.Results {
		if !validImage(image) {
			return false
		}
	}
	return true
}

func validAudioSearchResponse(value AudioSearchResponse, input AudioSearchRequest) bool {
	page, pageSize := effectivePagination(input.Page, input.PageSize)
	if value.ResultCount < 0 || value.PageCount < 0 || value.Page != page || value.PageSize != pageSize || len(value.Results) > value.PageSize {
		return false
	}
	for _, audio := range value.Results {
		if !validAudio(audio) {
			return false
		}
	}
	return true
}

func prepareCallOptions(operation string, options []socialhub.CallOption) error {
	resolved, err := socialhub.ResolveCallOptions(options...)
	if err != nil {
		return platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	if resolved.RequestID != "" {
		return invalidArgument(operation, "Openverse does not document a caller request-ID header")
	}
	if resolved.IdempotencyKey != "" {
		return invalidArgument(operation, "read-only Openverse operations do not use idempotency keys")
	}
	if len(resolved.Fields) != 0 {
		return invalidArgument(operation, "field selection is fixed by the typed Openverse operation")
	}
	return nil
}
