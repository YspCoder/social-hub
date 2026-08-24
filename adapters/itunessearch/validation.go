package itunessearch

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

const (
	maxSearchTermBytes = 4_096
	maxIdentifierCount = 200
)

var entitiesByMedia = map[Media]map[Entity]struct{}{
	MediaMovie:      entitySet(EntityMovieArtist, EntityMovie),
	MediaPodcast:    entitySet(EntityPodcastAuthor, EntityPodcast, EntityPodcastEpisode),
	MediaMusic:      entitySet(EntityMusicArtist, EntityMusicTrack, EntityAlbum, EntityMusicVideo, EntityMix, EntitySong),
	MediaMusicVideo: entitySet(EntityMusicArtist, EntityMusicVideo),
	MediaAudiobook:  entitySet(EntityAudiobookAuthor, EntityAudiobook),
	MediaShortFilm:  entitySet(EntityShortFilmArtist, EntityShortFilm),
	MediaTVShow:     entitySet(EntityTVEpisode, EntityTVSeason),
	MediaSoftware:   entitySet(EntitySoftware, EntityIPadSoftware, EntityDesktopSoftware),
	MediaEBook:      entitySet(EntityEBook),
	MediaAll:        entitySet(EntityMovie, EntityAlbum, EntityAllArtist, EntityPodcast, EntityMusicVideo, EntityMix, EntityAudiobook, EntityTVSeason, EntityAllTrack),
}

var attributesByMedia = map[Media]map[Attribute]struct{}{
	MediaMovie: attributeSet(
		AttributeActorTerm, AttributeGenreIndex, AttributeArtistTerm, AttributeShortFilmTerm,
		AttributeProducerTerm, AttributeRatingTerm, AttributeDirectorTerm, AttributeReleaseYearTerm,
		AttributeFeatureFilmTerm, AttributeMovieArtistTerm, AttributeMovieTerm, AttributeRatingIndex,
		AttributeDescriptionTerm,
	),
	MediaPodcast: attributeSet(
		AttributeTitleTerm, AttributeLanguageTerm, AttributeAuthorTerm, AttributeGenreIndex,
		AttributeArtistTerm, AttributeRatingIndex, AttributeKeywordsTerm, AttributeDescriptionTerm,
	),
	MediaMusic: attributeSet(
		AttributeMixTerm, AttributeGenreIndex, AttributeArtistTerm, AttributeComposerTerm,
		AttributeAlbumTerm, AttributeRatingIndex, AttributeSongTerm,
	),
	MediaMusicVideo: attributeSet(
		AttributeGenreIndex, AttributeArtistTerm, AttributeAlbumTerm, AttributeRatingIndex, AttributeSongTerm,
	),
	MediaAudiobook: attributeSet(AttributeTitleTerm, AttributeAuthorTerm, AttributeGenreIndex, AttributeRatingIndex),
	MediaShortFilm: attributeSet(
		AttributeGenreIndex, AttributeArtistTerm, AttributeShortFilmTerm, AttributeRatingIndex, AttributeDescriptionTerm,
	),
	MediaTVShow: attributeSet(
		AttributeGenreIndex, AttributeTVEpisodeTerm, AttributeShowTerm, AttributeTVSeasonTerm,
		AttributeRatingIndex, AttributeDescriptionTerm,
	),
	MediaSoftware: attributeSet(AttributeSoftwareDeveloper),
	MediaEBook:    {},
	MediaAll: attributeSet(
		AttributeActorTerm, AttributeLanguageTerm, AttributeAllArtistTerm, AttributeTVEpisodeTerm,
		AttributeShortFilmTerm, AttributeDirectorTerm, AttributeReleaseYearTerm, AttributeTitleTerm,
		AttributeFeatureFilmTerm, AttributeRatingIndex, AttributeKeywordsTerm, AttributeDescriptionTerm,
		AttributeAuthorTerm, AttributeGenreIndex, AttributeMixTerm, AttributeAllTrackTerm,
		AttributeArtistTerm, AttributeComposerTerm, AttributeTVSeasonTerm, AttributeProducerTerm,
		AttributeRatingTerm, AttributeSongTerm, AttributeMovieArtistTerm, AttributeShowTerm,
		AttributeMovieTerm, AttributeAlbumTerm,
	),
}

func entitySet(values ...Entity) map[Entity]struct{} {
	result := make(map[Entity]struct{}, len(values))
	for _, value := range values {
		result[value] = struct{}{}
	}
	return result
}

func attributeSet(values ...Attribute) map[Attribute]struct{} {
	result := make(map[Attribute]struct{}, len(values))
	for _, value := range values {
		result[value] = struct{}{}
	}
	return result
}

func normalizeSearchRequest(input SearchRequest) (SearchRequest, error) {
	if !validSearchTerm(input.Term) {
		return SearchRequest{}, invalidArgument("search", "term is required and must be bounded UTF-8 without surrounding whitespace or control characters")
	}
	if input.Country == "" {
		input.Country = "US"
	}
	if !validCountry(input.Country) {
		return SearchRequest{}, invalidArgument("search", "country must be a two-letter ISO 3166-1 alpha-2 code")
	}
	input.Country = strings.ToUpper(input.Country)
	if input.Media == "" {
		input.Media = MediaAll
	}
	allowedEntities, validMedia := entitiesByMedia[input.Media]
	if !validMedia {
		return SearchRequest{}, invalidArgument("search", "media is not supported by the iTunes Search API")
	}
	if input.Entity != "" {
		if _, ok := allowedEntities[input.Entity]; !ok {
			return SearchRequest{}, invalidArgument("search", "entity is not valid for the selected media")
		}
	}
	if input.Attribute != "" {
		if _, ok := attributesByMedia[input.Media][input.Attribute]; !ok {
			return SearchRequest{}, invalidArgument("search", "attribute is not valid for the selected media")
		}
	}
	if input.Limit == 0 {
		input.Limit = DefaultSearchLimit
	}
	if input.Limit < 1 || input.Limit > MaximumLimit {
		return SearchRequest{}, invalidArgument("search", "limit must be between 1 and 200")
	}
	if input.Language == "" {
		input.Language = LanguageEnglishUS
	}
	if input.Language != LanguageEnglishUS && input.Language != LanguageJapanese {
		return SearchRequest{}, invalidArgument("search", "language must be en_us or ja_jp")
	}
	if input.Version == 0 {
		input.Version = ResultVersionTwo
	}
	if input.Version != ResultVersionOne && input.Version != ResultVersionTwo {
		return SearchRequest{}, invalidArgument("search", "result version must be 1 or 2")
	}
	if input.Explicit == "" {
		input.Explicit = ExplicitInclude
	}
	if input.Explicit != ExplicitInclude && input.Explicit != ExplicitExclude {
		return SearchRequest{}, invalidArgument("search", "explicit must be Yes or No")
	}
	return input, nil
}

func searchQuery(input SearchRequest) url.Values {
	query := url.Values{
		"term":     {input.Term},
		"country":  {input.Country},
		"media":    {string(input.Media)},
		"limit":    {strconv.Itoa(input.Limit)},
		"lang":     {string(input.Language)},
		"version":  {strconv.Itoa(int(input.Version))},
		"explicit": {string(input.Explicit)},
	}
	if input.Entity != "" {
		query.Set("entity", string(input.Entity))
	}
	if input.Attribute != "" {
		query.Set("attribute", string(input.Attribute))
	}
	return query
}

func normalizeLookupRequest(input LookupRequest) (LookupRequest, error) {
	input.IDs = append([]int64(nil), input.IDs...)
	input.AMGArtistIDs = append([]int64(nil), input.AMGArtistIDs...)
	input.AMGAlbumIDs = append([]int64(nil), input.AMGAlbumIDs...)
	input.AMGVideoIDs = append([]int64(nil), input.AMGVideoIDs...)

	families := 0
	for _, values := range [][]int64{input.IDs, input.AMGArtistIDs, input.AMGAlbumIDs, input.AMGVideoIDs} {
		if len(values) > 0 {
			families++
		}
		if err := validatePositiveIDs(values); err != nil {
			return LookupRequest{}, invalidArgument("lookup", err.Error())
		}
	}
	if input.UPC != "" {
		families++
		if !decimalLength(input.UPC, 8, 14) {
			return LookupRequest{}, invalidArgument("lookup", "upc must be an 8 to 14 digit UPC/EAN value")
		}
	}
	if input.ISBN != "" {
		families++
		if !decimalLength(input.ISBN, 13, 13) {
			return LookupRequest{}, invalidArgument("lookup", "isbn must be a 13 digit ISBN")
		}
	}
	if families != 1 {
		return LookupRequest{}, invalidArgument("lookup", "exactly one identifier family is required")
	}
	if input.Entity != "" && !validLookupEntity(input.Entity) {
		return LookupRequest{}, invalidArgument("lookup", "entity is not an official iTunes Search entity")
	}
	if input.Limit < 0 || input.Limit > MaximumLimit {
		return LookupRequest{}, invalidArgument("lookup", "limit must be between 1 and 200 when set")
	}
	if input.Sort != "" && input.Sort != LookupSortRecent {
		return LookupRequest{}, invalidArgument("lookup", "sort must be recent when set")
	}
	if input.Sort != "" && input.Entity == "" {
		return LookupRequest{}, invalidArgument("lookup", "sort requires an entity expansion")
	}
	return input, nil
}

func lookupQuery(input LookupRequest) url.Values {
	query := make(url.Values)
	switch {
	case len(input.IDs) > 0:
		query.Set("id", joinIDs(input.IDs))
	case len(input.AMGArtistIDs) > 0:
		query.Set("amgArtistId", joinIDs(input.AMGArtistIDs))
	case len(input.AMGAlbumIDs) > 0:
		query.Set("amgAlbumId", joinIDs(input.AMGAlbumIDs))
	case len(input.AMGVideoIDs) > 0:
		query.Set("amgVideoId", joinIDs(input.AMGVideoIDs))
	case input.UPC != "":
		query.Set("upc", input.UPC)
	case input.ISBN != "":
		query.Set("isbn", input.ISBN)
	}
	if input.Entity != "" {
		query.Set("entity", string(input.Entity))
	}
	if input.Limit > 0 {
		query.Set("limit", strconv.Itoa(input.Limit))
	}
	if input.Sort != "" {
		query.Set("sort", string(input.Sort))
	}
	return query
}

func prepareCallOptions(operation string, options []socialhub.CallOption) (socialhub.CallOptions, error) {
	resolved, err := socialhub.ResolveCallOptions(options...)
	if err != nil {
		return socialhub.CallOptions{}, platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	if resolved.RequestID != "" {
		return socialhub.CallOptions{}, invalidArgument(operation, "caller request IDs are not documented by the iTunes Search API")
	}
	if resolved.IdempotencyKey != "" {
		return socialhub.CallOptions{}, invalidArgument(operation, "idempotency keys are not used by public reads")
	}
	if len(resolved.Fields) != 0 {
		return socialhub.CallOptions{}, invalidArgument(operation, "field selection is fixed by the typed result model")
	}
	return resolved, nil
}

func validSearchTerm(value string) bool {
	if value == "" || len(value) > maxSearchTermBytes || !utf8.ValidString(value) || strings.TrimSpace(value) != value {
		return false
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return false
		}
	}
	return true
}

func validCountry(value string) bool {
	return len(value) == 2 && asciiLetter(value[0]) && asciiLetter(value[1])
}

func asciiLetter(value byte) bool {
	return value >= 'a' && value <= 'z' || value >= 'A' && value <= 'Z'
}

func validatePositiveIDs(values []int64) error {
	if len(values) > maxIdentifierCount {
		return fmt.Errorf("an identifier family cannot contain more than %d values", maxIdentifierCount)
	}
	for _, value := range values {
		if value <= 0 {
			return fmt.Errorf("numeric identifiers must be positive")
		}
	}
	return nil
}

func decimalLength(value string, minimum, maximum int) bool {
	if len(value) < minimum || len(value) > maximum {
		return false
	}
	for index := range value {
		if value[index] < '0' || value[index] > '9' {
			return false
		}
	}
	return true
}

func validLookupEntity(value Entity) bool {
	for _, entities := range entitiesByMedia {
		if _, ok := entities[value]; ok {
			return true
		}
	}
	return false
}

func joinIDs(values []int64) string {
	encoded := make([]string, len(values))
	for index, value := range values {
		encoded[index] = strconv.FormatInt(value, 10)
	}
	return strings.Join(encoded, ",")
}
