package tmdb

import "time"

// MediaType identifies a TMDB catalog resource.
type MediaType string

const (
	MediaAll    MediaType = "all"
	MediaMovie  MediaType = "movie"
	MediaTV     MediaType = "tv"
	MediaPerson MediaType = "person"
)

type Genre struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type ProductionCompany struct {
	ID            int64  `json:"id"`
	Name          string `json:"name"`
	LogoPath      string `json:"logo_path,omitempty"`
	OriginCountry string `json:"origin_country,omitempty"`
}

type ProductionCountry struct {
	Code string `json:"iso_3166_1"`
	Name string `json:"name"`
}

type SpokenLanguage struct {
	Code        string `json:"iso_639_1"`
	EnglishName string `json:"english_name,omitempty"`
	Name        string `json:"name"`
}

type Collection struct {
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	PosterPath   string `json:"poster_path,omitempty"`
	BackdropPath string `json:"backdrop_path,omitempty"`
}

// MediaItem is a list/search representation of a movie, TV series, or person.
type MediaItem struct {
	ID                 int64       `json:"id"`
	MediaType          MediaType   `json:"media_type,omitempty"`
	Title              string      `json:"title,omitempty"`
	OriginalTitle      string      `json:"original_title,omitempty"`
	Name               string      `json:"name,omitempty"`
	OriginalName       string      `json:"original_name,omitempty"`
	OriginalLanguage   string      `json:"original_language,omitempty"`
	Overview           string      `json:"overview,omitempty"`
	ReleaseDate        string      `json:"release_date,omitempty"`
	FirstAirDate       string      `json:"first_air_date,omitempty"`
	PosterPath         string      `json:"poster_path,omitempty"`
	BackdropPath       string      `json:"backdrop_path,omitempty"`
	ProfilePath        string      `json:"profile_path,omitempty"`
	Popularity         float64     `json:"popularity,omitempty"`
	GenreIDs           []int64     `json:"genre_ids,omitempty"`
	OriginCountry      []string    `json:"origin_country,omitempty"`
	Adult              bool        `json:"adult,omitempty"`
	Video              bool        `json:"video,omitempty"`
	VoteAverage        float64     `json:"vote_average,omitempty"`
	VoteCount          int64       `json:"vote_count,omitempty"`
	Rating             float64     `json:"rating,omitempty"`
	Gender             int         `json:"gender,omitempty"`
	KnownForDepartment string      `json:"known_for_department,omitempty"`
	KnownFor           []MediaItem `json:"known_for,omitempty"`
}

// Movie contains the primary fields returned by the movie details endpoint.
type Movie struct {
	Adult               bool                `json:"adult"`
	BackdropPath        string              `json:"backdrop_path,omitempty"`
	BelongsToCollection *Collection         `json:"belongs_to_collection,omitempty"`
	Budget              int64               `json:"budget,omitempty"`
	Genres              []Genre             `json:"genres,omitempty"`
	Homepage            string              `json:"homepage,omitempty"`
	ID                  int64               `json:"id"`
	IMDBID              string              `json:"imdb_id,omitempty"`
	OriginalLanguage    string              `json:"original_language,omitempty"`
	OriginalTitle       string              `json:"original_title,omitempty"`
	Overview            string              `json:"overview,omitempty"`
	Popularity          float64             `json:"popularity,omitempty"`
	PosterPath          string              `json:"poster_path,omitempty"`
	OriginCountry       []string            `json:"origin_country,omitempty"`
	ProductionCompanies []ProductionCompany `json:"production_companies,omitempty"`
	ProductionCountries []ProductionCountry `json:"production_countries,omitempty"`
	ReleaseDate         string              `json:"release_date,omitempty"`
	Revenue             int64               `json:"revenue,omitempty"`
	Runtime             int                 `json:"runtime,omitempty"`
	SpokenLanguages     []SpokenLanguage    `json:"spoken_languages,omitempty"`
	Status              string              `json:"status,omitempty"`
	Tagline             string              `json:"tagline,omitempty"`
	Title               string              `json:"title"`
	Video               bool                `json:"video,omitempty"`
	VoteAverage         float64             `json:"vote_average,omitempty"`
	VoteCount           int64               `json:"vote_count,omitempty"`
}

type Network struct {
	ID            int64  `json:"id"`
	Name          string `json:"name"`
	LogoPath      string `json:"logo_path,omitempty"`
	OriginCountry string `json:"origin_country,omitempty"`
}

type Season struct {
	AirDate      string `json:"air_date,omitempty"`
	EpisodeCount int    `json:"episode_count,omitempty"`
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	Overview     string `json:"overview,omitempty"`
	PosterPath   string `json:"poster_path,omitempty"`
	SeasonNumber int    `json:"season_number"`
}

// TVSeries contains the primary fields returned by the TV details endpoint.
type TVSeries struct {
	Adult               bool                `json:"adult,omitempty"`
	BackdropPath        string              `json:"backdrop_path,omitempty"`
	FirstAirDate        string              `json:"first_air_date,omitempty"`
	Genres              []Genre             `json:"genres,omitempty"`
	Homepage            string              `json:"homepage,omitempty"`
	ID                  int64               `json:"id"`
	InProduction        bool                `json:"in_production,omitempty"`
	Languages           []string            `json:"languages,omitempty"`
	LastAirDate         string              `json:"last_air_date,omitempty"`
	Name                string              `json:"name"`
	Networks            []Network           `json:"networks,omitempty"`
	NumberOfEpisodes    int                 `json:"number_of_episodes,omitempty"`
	NumberOfSeasons     int                 `json:"number_of_seasons,omitempty"`
	OriginCountry       []string            `json:"origin_country,omitempty"`
	OriginalLanguage    string              `json:"original_language,omitempty"`
	OriginalName        string              `json:"original_name,omitempty"`
	Overview            string              `json:"overview,omitempty"`
	Popularity          float64             `json:"popularity,omitempty"`
	PosterPath          string              `json:"poster_path,omitempty"`
	ProductionCompanies []ProductionCompany `json:"production_companies,omitempty"`
	ProductionCountries []ProductionCountry `json:"production_countries,omitempty"`
	Seasons             []Season            `json:"seasons,omitempty"`
	Status              string              `json:"status,omitempty"`
	Tagline             string              `json:"tagline,omitempty"`
	Type                string              `json:"type,omitempty"`
	VoteAverage         float64             `json:"vote_average,omitempty"`
	VoteCount           int64               `json:"vote_count,omitempty"`
}

// Person contains the primary fields returned by the person details endpoint.
type Person struct {
	Adult              bool     `json:"adult,omitempty"`
	AlsoKnownAs        []string `json:"also_known_as,omitempty"`
	Biography          string   `json:"biography,omitempty"`
	Birthday           string   `json:"birthday,omitempty"`
	Deathday           string   `json:"deathday,omitempty"`
	Gender             int      `json:"gender,omitempty"`
	Homepage           string   `json:"homepage,omitempty"`
	ID                 int64    `json:"id"`
	IMDBID             string   `json:"imdb_id,omitempty"`
	KnownForDepartment string   `json:"known_for_department,omitempty"`
	Name               string   `json:"name"`
	PlaceOfBirth       string   `json:"place_of_birth,omitempty"`
	Popularity         float64  `json:"popularity,omitempty"`
	ProfilePath        string   `json:"profile_path,omitempty"`
}

type ImageConfiguration struct {
	BaseURL       string   `json:"base_url"`
	SecureBaseURL string   `json:"secure_base_url"`
	BackdropSizes []string `json:"backdrop_sizes"`
	LogoSizes     []string `json:"logo_sizes"`
	PosterSizes   []string `json:"poster_sizes"`
	ProfileSizes  []string `json:"profile_sizes"`
	StillSizes    []string `json:"still_sizes"`
}

type Configuration struct {
	Images     ImageConfiguration `json:"images"`
	ChangeKeys []string           `json:"change_keys"`
}

type Account struct {
	Avatar struct {
		Gravatar struct {
			Hash string `json:"hash"`
		} `json:"gravatar"`
		TMDB struct {
			AvatarPath string `json:"avatar_path"`
		} `json:"tmdb"`
	} `json:"avatar"`
	ID           int64  `json:"id"`
	Language     string `json:"iso_639_1"`
	Country      string `json:"iso_3166_1"`
	Name         string `json:"name"`
	IncludeAdult bool   `json:"include_adult"`
	Username     string `json:"username"`
}

type RequestToken struct {
	Token     string
	ExpiresAt time.Time
}

type GuestSession struct {
	ID        string
	ExpiresAt time.Time
}

type StatusResponse struct {
	Success       bool   `json:"success,omitempty"`
	StatusCode    int    `json:"status_code"`
	StatusMessage string `json:"status_message"`
}
