package musicbrainz

type Area struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	SortName       string   `json:"sort-name,omitempty"`
	Type           string   `json:"type,omitempty"`
	TypeID         string   `json:"type-id,omitempty"`
	Disambiguation string   `json:"disambiguation,omitempty"`
	ISOCodes       []string `json:"iso-3166-1-codes,omitempty"`
}

type LifeSpan struct {
	Begin string `json:"begin,omitempty"`
	End   string `json:"end,omitempty"`
	Ended bool   `json:"ended"`
}

type Alias struct {
	Name      string  `json:"name"`
	SortName  string  `json:"sort-name,omitempty"`
	Locale    *string `json:"locale"`
	Type      *string `json:"type"`
	TypeID    *string `json:"type-id"`
	Primary   *bool   `json:"primary"`
	BeginDate *string `json:"begin-date"`
	EndDate   *string `json:"end-date"`
}

type Genre struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Disambiguation string `json:"disambiguation,omitempty"`
	Count          int    `json:"count,omitempty"`
}

type Tag struct {
	Name  string `json:"name"`
	Count int    `json:"count,omitempty"`
}

type Artist struct {
	ID             string   `json:"id"`
	Type           string   `json:"type,omitempty"`
	TypeID         string   `json:"type-id,omitempty"`
	Gender         string   `json:"gender,omitempty"`
	GenderID       string   `json:"gender-id,omitempty"`
	Name           string   `json:"name"`
	SortName       string   `json:"sort-name,omitempty"`
	Disambiguation string   `json:"disambiguation,omitempty"`
	Country        string   `json:"country,omitempty"`
	Area           *Area    `json:"area,omitempty"`
	BeginArea      *Area    `json:"begin-area,omitempty"`
	EndArea        *Area    `json:"end-area,omitempty"`
	LifeSpan       LifeSpan `json:"life-span"`
	ISNIs          []string `json:"isnis,omitempty"`
	IPIs           []string `json:"ipis,omitempty"`
	Aliases        []Alias  `json:"aliases,omitempty"`
	Genres         []Genre  `json:"genres,omitempty"`
	Tags           []Tag    `json:"tags,omitempty"`
	Score          int      `json:"score,omitempty"`
}

type ArtistCredit struct {
	Name       string `json:"name"`
	JoinPhrase string `json:"joinphrase,omitempty"`
	Artist     Artist `json:"artist"`
}

type ReleaseGroup struct {
	ID               string         `json:"id"`
	Title            string         `json:"title"`
	PrimaryType      string         `json:"primary-type,omitempty"`
	PrimaryTypeID    string         `json:"primary-type-id,omitempty"`
	SecondaryTypes   []string       `json:"secondary-types,omitempty"`
	SecondaryTypeIDs []string       `json:"secondary-type-ids,omitempty"`
	FirstReleaseDate string         `json:"first-release-date,omitempty"`
	Disambiguation   string         `json:"disambiguation,omitempty"`
	ArtistCredit     []ArtistCredit `json:"artist-credit,omitempty"`
	Releases         []Release      `json:"releases,omitempty"`
	Genres           []Genre        `json:"genres,omitempty"`
	Tags             []Tag          `json:"tags,omitempty"`
	Score            int            `json:"score,omitempty"`
}

type TextRepresentation struct {
	Language string `json:"language,omitempty"`
	Script   string `json:"script,omitempty"`
}

type ReleaseEvent struct {
	Date string `json:"date,omitempty"`
	Area *Area  `json:"area,omitempty"`
}

type Label struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	SortName       string `json:"sort-name,omitempty"`
	Type           string `json:"type,omitempty"`
	TypeID         string `json:"type-id,omitempty"`
	Disambiguation string `json:"disambiguation,omitempty"`
	Country        string `json:"country,omitempty"`
}

type LabelInfo struct {
	CatalogNumber string `json:"catalog-number,omitempty"`
	Label         *Label `json:"label,omitempty"`
}

type CoverArtArchive struct {
	Artwork  bool `json:"artwork"`
	Count    int  `json:"count"`
	Front    bool `json:"front"`
	Back     bool `json:"back"`
	Darkened bool `json:"darkened"`
}

type Medium struct {
	Position    int     `json:"position"`
	Format      string  `json:"format,omitempty"`
	FormatID    string  `json:"format-id,omitempty"`
	Title       string  `json:"title,omitempty"`
	TrackCount  int     `json:"track-count"`
	TrackOffset int     `json:"track-offset,omitempty"`
	Tracks      []Track `json:"tracks,omitempty"`
}

type Track struct {
	ID           string         `json:"id"`
	Position     int            `json:"position"`
	Number       string         `json:"number,omitempty"`
	Title        string         `json:"title"`
	Length       *int64         `json:"length"`
	ArtistCredit []ArtistCredit `json:"artist-credit,omitempty"`
	Recording    Recording      `json:"recording"`
}

type Release struct {
	ID                 string             `json:"id"`
	Title              string             `json:"title"`
	Status             string             `json:"status,omitempty"`
	StatusID           string             `json:"status-id,omitempty"`
	Quality            string             `json:"quality,omitempty"`
	Packaging          string             `json:"packaging,omitempty"`
	PackagingID        string             `json:"packaging-id,omitempty"`
	Date               string             `json:"date,omitempty"`
	Country            string             `json:"country,omitempty"`
	Barcode            *string            `json:"barcode"`
	ASIN               *string            `json:"asin"`
	Disambiguation     string             `json:"disambiguation,omitempty"`
	TextRepresentation TextRepresentation `json:"text-representation"`
	ArtistCredit       []ArtistCredit     `json:"artist-credit,omitempty"`
	ReleaseGroup       *ReleaseGroup      `json:"release-group,omitempty"`
	ReleaseEvents      []ReleaseEvent     `json:"release-events,omitempty"`
	LabelInfo          []LabelInfo        `json:"label-info,omitempty"`
	Media              []Medium           `json:"media,omitempty"`
	CoverArtArchive    CoverArtArchive    `json:"cover-art-archive"`
	Genres             []Genre            `json:"genres,omitempty"`
	Score              int                `json:"score,omitempty"`
}

type Recording struct {
	ID               string         `json:"id"`
	Title            string         `json:"title"`
	Length           *int64         `json:"length"`
	Video            bool           `json:"video"`
	Disambiguation   string         `json:"disambiguation,omitempty"`
	FirstReleaseDate string         `json:"first-release-date,omitempty"`
	ArtistCredit     []ArtistCredit `json:"artist-credit,omitempty"`
	ISRCs            []string       `json:"isrcs,omitempty"`
	Releases         []Release      `json:"releases,omitempty"`
	Genres           []Genre        `json:"genres,omitempty"`
	Tags             []Tag          `json:"tags,omitempty"`
	Score            int            `json:"score,omitempty"`
}

type Relation struct {
	Type            string            `json:"type"`
	TypeID          string            `json:"type-id,omitempty"`
	TargetType      string            `json:"target-type"`
	Direction       string            `json:"direction,omitempty"`
	Begin           *string           `json:"begin"`
	End             *string           `json:"end"`
	Ended           bool              `json:"ended"`
	SourceCredit    string            `json:"source-credit,omitempty"`
	TargetCredit    string            `json:"target-credit,omitempty"`
	Attributes      []string          `json:"attributes,omitempty"`
	AttributeIDs    map[string]string `json:"attribute-ids,omitempty"`
	AttributeValues map[string]string `json:"attribute-values,omitempty"`
	Artist          *Artist           `json:"artist,omitempty"`
	Recording       *Recording        `json:"recording,omitempty"`
}

type WorkAttribute struct {
	Type    string `json:"type"`
	TypeID  string `json:"type-id,omitempty"`
	Value   string `json:"value"`
	ValueID string `json:"value-id,omitempty"`
}

type Work struct {
	ID             string          `json:"id"`
	Title          string          `json:"title"`
	Type           string          `json:"type,omitempty"`
	TypeID         string          `json:"type-id,omitempty"`
	Language       string          `json:"language,omitempty"`
	Languages      []string        `json:"languages,omitempty"`
	ISWCs          []string        `json:"iswcs,omitempty"`
	Disambiguation string          `json:"disambiguation,omitempty"`
	Aliases        []Alias         `json:"aliases,omitempty"`
	Attributes     []WorkAttribute `json:"attributes,omitempty"`
	Genres         []Genre         `json:"genres,omitempty"`
	Tags           []Tag           `json:"tags,omitempty"`
	Relations      []Relation      `json:"relations,omitempty"`
	Score          int             `json:"score,omitempty"`
}

type artistSearchEnvelope struct {
	Count   int      `json:"count"`
	Offset  int      `json:"offset"`
	Artists []Artist `json:"artists"`
}

type releaseGroupSearchEnvelope struct {
	Count         int            `json:"count"`
	Offset        int            `json:"offset"`
	ReleaseGroups []ReleaseGroup `json:"release-groups"`
}

type releaseSearchEnvelope struct {
	Count    int       `json:"count"`
	Offset   int       `json:"offset"`
	Releases []Release `json:"releases"`
}

type recordingSearchEnvelope struct {
	Count      int         `json:"count"`
	Offset     int         `json:"offset"`
	Recordings []Recording `json:"recordings"`
}

type workSearchEnvelope struct {
	Count  int    `json:"count"`
	Offset int    `json:"offset"`
	Works  []Work `json:"works"`
}

type artistReleaseGroupsEnvelope struct {
	Count         int            `json:"release-group-count"`
	Offset        int            `json:"release-group-offset"`
	ReleaseGroups []ReleaseGroup `json:"release-groups"`
}

type artistRecordingsEnvelope struct {
	Count      int         `json:"recording-count"`
	Offset     int         `json:"recording-offset"`
	Recordings []Recording `json:"recordings"`
}
