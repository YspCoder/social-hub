package pixabay

import (
	"net/url"
	"strings"
	"unicode"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

const maximumPageNumber = 1_000_000

var supportedLanguages = map[Language]struct{}{
	LanguageCS: {}, LanguageDA: {}, LanguageDE: {}, LanguageEN: {}, LanguageES: {}, LanguageFR: {},
	LanguageID: {}, LanguageIT: {}, LanguageHU: {}, LanguageNL: {}, LanguageNO: {}, LanguagePL: {},
	LanguagePT: {}, LanguageRO: {}, LanguageSK: {}, LanguageFI: {}, LanguageSV: {}, LanguageTR: {},
	LanguageVI: {}, LanguageTH: {}, LanguageBG: {}, LanguageRU: {}, LanguageEL: {}, LanguageJA: {},
	LanguageKO: {}, LanguageZH: {},
}

var supportedCategories = map[Category]struct{}{
	CategoryBackgrounds: {}, CategoryFashion: {}, CategoryNature: {}, CategoryScience: {},
	CategoryEducation: {}, CategoryFeelings: {}, CategoryHealth: {}, CategoryPeople: {},
	CategoryReligion: {}, CategoryPlaces: {}, CategoryAnimals: {}, CategoryIndustry: {},
	CategoryComputer: {}, CategoryFood: {}, CategorySports: {}, CategoryTransportation: {},
	CategoryTravel: {}, CategoryBuildings: {}, CategoryBusiness: {}, CategoryMusic: {},
}

var supportedColors = map[Color]struct{}{
	ColorGrayscale: {}, ColorTransparent: {}, ColorRed: {}, ColorOrange: {}, ColorYellow: {},
	ColorGreen: {}, ColorTurquoise: {}, ColorBlue: {}, ColorLilac: {}, ColorPink: {},
	ColorWhite: {}, ColorGray: {}, ColorBlack: {}, ColorBrown: {},
}

func validOpaque(value string, maximum int) bool {
	if value == "" || value != strings.TrimSpace(value) || len(value) > maximum || !utf8.ValidString(value) {
		return false
	}
	return !strings.ContainsFunc(value, unicode.IsControl)
}

func validAPIKey(value string) bool {
	if !validOpaque(value, 4096) {
		return false
	}
	return !strings.ContainsFunc(value, unicode.IsSpace)
}

func validQuery(value string) bool {
	if value == "" {
		return true
	}
	return utf8.ValidString(value) && utf8.RuneCountInString(value) <= 100 &&
		strings.TrimSpace(value) != "" && !strings.ContainsFunc(value, unicode.IsControl)
}

func validCommonSearch(value SearchRequest) bool {
	if !validQuery(value.Query) || value.MinimumWidth < 0 || value.MinimumHeight < 0 ||
		value.Page < 0 || value.Page > maximumPageNumber || value.PerPage < 0 {
		return false
	}
	if value.Language != "" {
		if _, ok := supportedLanguages[value.Language]; !ok {
			return false
		}
	}
	if value.Category != "" {
		if _, ok := supportedCategories[value.Category]; !ok {
			return false
		}
	}
	if value.Order != "" && value.Order != OrderPopular && value.Order != OrderLatest {
		return false
	}
	return value.PerPage == 0 || value.PerPage >= MinimumPageSize && value.PerPage <= MaximumPageSize
}

func validImageSearch(value ImageSearchRequest) bool {
	if !validCommonSearch(value.SearchRequest) {
		return false
	}
	if !validImageType(value.ImageType, true) {
		return false
	}
	switch value.Orientation {
	case "", OrientationAll, OrientationHorizontal, OrientationVertical:
	default:
		return false
	}
	if len(value.Colors) > len(supportedColors) {
		return false
	}
	seen := make(map[Color]struct{}, len(value.Colors))
	for _, color := range value.Colors {
		if _, ok := supportedColors[color]; !ok {
			return false
		}
		if _, duplicate := seen[color]; duplicate {
			return false
		}
		seen[color] = struct{}{}
	}
	return true
}

func validVideoSearch(value VideoSearchRequest) bool {
	if !validCommonSearch(value.SearchRequest) {
		return false
	}
	return validVideoType(value.VideoType, true)
}

func validImageType(value ImageType, allowAll bool) bool {
	return value == ImageTypePhoto || value == ImageTypeIllustration || value == ImageTypeVector ||
		allowAll && (value == "" || value == ImageTypeAll)
}

func validVideoType(value VideoType, allowAll bool) bool {
	return value == VideoTypeFilm || value == VideoTypeAnimation || allowAll && (value == "" || value == VideoTypeAll)
}

func effectivePagination(page, perPage int) (int, int) {
	if page == 0 {
		page = 1
	}
	if perPage == 0 {
		perPage = DefaultPageSize
	}
	return page, perPage
}

func validHTTPSURL(value string) bool {
	if !validOpaque(value, 16_384) {
		return false
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil {
		return false
	}
	query, err := url.ParseQuery(parsed.RawQuery)
	if err != nil {
		return false
	}
	for key := range query {
		switch strings.ToLower(key) {
		case "key", "api_key", "apikey", "access_token", "token", "authorization", "client_secret":
			return false
		}
	}
	return true
}

func validOptionalHTTPSURL(value string) bool { return value == "" || validHTTPSURL(value) }

func validImage(value Image) bool {
	if value.ID <= 0 || !validImageType(value.Type, false) || !validHTTPSURL(value.PageURL) || value.PreviewWidth < 0 || value.PreviewHeight < 0 ||
		value.WebformatWidth < 0 || value.WebformatHeight < 0 || value.ImageWidth < 0 || value.ImageHeight < 0 ||
		value.ImageSize < 0 || value.Views < 0 || value.Downloads < 0 || value.Likes < 0 || value.Comments < 0 || value.UserID < 0 {
		return false
	}
	for _, mediaURL := range []string{
		value.PreviewURL, value.WebformatURL, value.LargeImageURL, value.FullHDURL,
		value.ImageURL, value.VectorURL, value.UserImageURL,
	} {
		if !validOptionalHTTPSURL(mediaURL) {
			return false
		}
	}
	return true
}

func validVideo(value Video) bool {
	if value.ID <= 0 || !validVideoType(value.Type, false) || !validHTTPSURL(value.PageURL) || value.Duration < 0 || value.Views < 0 ||
		value.Downloads < 0 || value.Likes < 0 || value.Comments < 0 || value.UserID < 0 ||
		!validOptionalHTTPSURL(value.UserImageURL) {
		return false
	}
	for _, rendition := range []VideoRendition{value.Videos.Large, value.Videos.Medium, value.Videos.Small, value.Videos.Tiny} {
		if rendition.Width < 0 || rendition.Height < 0 || rendition.Size < 0 ||
			!validOptionalHTTPSURL(rendition.URL) || !validOptionalHTTPSURL(rendition.Thumbnail) {
			return false
		}
	}
	return true
}

func validImageSearchResponse(value ImageSearchResponse, request ImageSearchRequest) bool {
	_, perPage := effectivePagination(request.Page, request.PerPage)
	if value.Total < 0 || value.TotalHits < 0 || len(value.Hits) > perPage {
		return false
	}
	for _, image := range value.Hits {
		if !validImage(image) {
			return false
		}
	}
	return true
}

func validVideoSearchResponse(value VideoSearchResponse, request VideoSearchRequest) bool {
	_, perPage := effectivePagination(request.Page, request.PerPage)
	if value.Total < 0 || value.TotalHits < 0 || len(value.Hits) > perPage {
		return false
	}
	for _, video := range value.Hits {
		if !validVideo(video) {
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
		return invalidArgument(operation, "Pixabay does not document a caller request-ID header")
	}
	if resolved.IdempotencyKey != "" {
		return invalidArgument(operation, "read-only Pixabay searches do not use idempotency keys")
	}
	if len(resolved.Fields) != 0 {
		return invalidArgument(operation, "field selection is fixed by the typed Pixabay search")
	}
	return nil
}
