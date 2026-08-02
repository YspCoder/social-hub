package tvmaze

import "time"

type Country struct {
	Name     string `json:"name"`
	Code     string `json:"code"`
	Timezone string `json:"timezone"`
}

type Network struct {
	ID      int64    `json:"id"`
	Name    string   `json:"name"`
	Country *Country `json:"country"`
	URL     string   `json:"officialSite,omitempty"`
}

type WebChannel struct {
	ID      int64    `json:"id"`
	Name    string   `json:"name"`
	Country *Country `json:"country"`
	URL     string   `json:"officialSite,omitempty"`
}

type Image struct {
	Medium   string `json:"medium,omitempty"`
	Original string `json:"original,omitempty"`
}

type Rating struct {
	Average *float64 `json:"average"`
}

type AirSchedule struct {
	Time string   `json:"time"`
	Days []string `json:"days"`
}

type Externals struct {
	TVRage *int64  `json:"tvrage"`
	TVDB   *int64  `json:"thetvdb"`
	IMDB   *string `json:"imdb"`
}

type Link struct {
	Href string `json:"href"`
}

type Links struct {
	Self            Link  `json:"self"`
	PreviousEpisode *Link `json:"previousepisode,omitempty"`
	NextEpisode     *Link `json:"nextepisode,omitempty"`
	Show            *Link `json:"show,omitempty"`
	Character       *Link `json:"character,omitempty"`
}

// Show is the full TVmaze show representation returned by public reads.
type Show struct {
	ID             int64       `json:"id"`
	URL            string      `json:"url"`
	Name           string      `json:"name"`
	Type           string      `json:"type"`
	Language       *string     `json:"language"`
	Genres         []string    `json:"genres"`
	Status         string      `json:"status"`
	Runtime        *int        `json:"runtime"`
	AverageRuntime *int        `json:"averageRuntime"`
	Premiered      *string     `json:"premiered"`
	Ended          *string     `json:"ended"`
	OfficialSite   *string     `json:"officialSite"`
	Schedule       AirSchedule `json:"schedule"`
	Rating         Rating      `json:"rating"`
	Weight         int         `json:"weight"`
	Network        *Network    `json:"network"`
	WebChannel     *WebChannel `json:"webChannel"`
	DVDCountry     *Country    `json:"dvdCountry"`
	Externals      Externals   `json:"externals"`
	Image          *Image      `json:"image"`
	Summary        *string     `json:"summary"`
	Updated        int64       `json:"updated"`
	Links          Links       `json:"_links"`
}

type ShowSearchResult struct {
	Score float64 `json:"score"`
	Show  Show    `json:"show"`
}

// Episode is also used by schedule endpoints, where Show is embedded.
type Episode struct {
	ID       int64        `json:"id"`
	URL      string       `json:"url"`
	Name     string       `json:"name"`
	Season   *int         `json:"season"`
	Number   *int         `json:"number"`
	Type     string       `json:"type"`
	Airdate  *string      `json:"airdate"`
	Airtime  *string      `json:"airtime"`
	Airstamp *string      `json:"airstamp"`
	Runtime  *int         `json:"runtime"`
	Rating   Rating       `json:"rating"`
	Image    *Image       `json:"image"`
	Summary  *string      `json:"summary"`
	Links    Links        `json:"_links"`
	Embedded EpisodeEmbed `json:"_embedded"`
}

type EpisodeEmbed struct {
	Show *Show `json:"show,omitempty"`
}

type Season struct {
	ID           int64       `json:"id"`
	URL          string      `json:"url"`
	Number       int         `json:"number"`
	Name         string      `json:"name"`
	EpisodeOrder *int        `json:"episodeOrder"`
	PremiereDate *string     `json:"premiereDate"`
	EndDate      *string     `json:"endDate"`
	Network      *Network    `json:"network"`
	WebChannel   *WebChannel `json:"webChannel"`
	Image        *Image      `json:"image"`
	Summary      *string     `json:"summary"`
	Links        Links       `json:"_links"`
}

type Person struct {
	ID       int64    `json:"id"`
	URL      string   `json:"url"`
	Name     string   `json:"name"`
	Country  *Country `json:"country"`
	Birthday *string  `json:"birthday"`
	Deathday *string  `json:"deathday"`
	Gender   *string  `json:"gender"`
	Image    *Image   `json:"image"`
	Updated  int64    `json:"updated"`
	Links    Links    `json:"_links"`
}

type PersonSearchResult struct {
	Score  float64 `json:"score"`
	Person Person  `json:"person"`
}

type Character struct {
	ID    int64  `json:"id"`
	URL   string `json:"url"`
	Name  string `json:"name"`
	Image *Image `json:"image"`
	Links Links  `json:"_links"`
}

type CastMember struct {
	Person    Person    `json:"person"`
	Character Character `json:"character"`
	Self      bool      `json:"self"`
	Voice     bool      `json:"voice"`
}

type CrewMember struct {
	Type   string `json:"type"`
	Person Person `json:"person"`
}

type CreditEmbed struct {
	Show *Show `json:"show,omitempty"`
}

type CastCredit struct {
	Self     bool        `json:"self"`
	Voice    bool        `json:"voice"`
	Links    Links       `json:"_links"`
	Embedded CreditEmbed `json:"_embedded"`
}

type CrewCredit struct {
	Type     string      `json:"type"`
	Links    Links       `json:"_links"`
	Embedded CreditEmbed `json:"_embedded"`
}

// LookupShowRequest identifies a show by exactly one external database ID.
type LookupShowRequest struct {
	IMDB   string
	TVDB   int64
	TVRage int64
}

type ScheduleRequest struct {
	Country string
	Date    *time.Time
}

// WebScheduleRequest uses a country pointer to preserve TVmaze's tri-state API:
// nil omits country, pointer to "" selects global channels, and a code selects local channels.
type WebScheduleRequest struct {
	Country *string
	Date    *time.Time
}

type UpdatePeriod string

const (
	UpdateDay   UpdatePeriod = "day"
	UpdateWeek  UpdatePeriod = "week"
	UpdateMonth UpdatePeriod = "month"
)

type Update struct {
	ID        int64
	UpdatedAt time.Time
}
