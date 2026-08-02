package letterboxd

import "time"

type ImageSize struct {
	Width  int    `json:"width,omitempty"`
	Height int    `json:"height,omitempty"`
	URL    string `json:"url,omitempty"`
}

type Image struct {
	Sizes []ImageSize `json:"sizes,omitempty"`
}

type Link struct {
	Type     string `json:"type,omitempty"`
	ID       string `json:"id,omitempty"`
	URL      string `json:"url,omitempty"`
	Label    string `json:"label,omitempty"`
	CheckURL string `json:"checkUrl,omitempty"`
}

// Film contains third-party-visible fields from the legacy Film schema. Fields
// marked FIRST PARTY in the official specification are intentionally omitted.
type Film struct {
	ID               string  `json:"id"`
	Name             string  `json:"name"`
	FullDisplayName  string  `json:"fullDisplayName,omitempty"`
	Link             string  `json:"link,omitempty"`
	ReleaseYear      int     `json:"releaseYear,omitempty"`
	ReleaseDate      string  `json:"releaseDate,omitempty"`
	RunTime          int     `json:"runTime,omitempty"`
	Rating           float64 `json:"rating,omitempty"`
	Poster           *Image  `json:"poster,omitempty"`
	Backdrop         *Image  `json:"backdrop,omitempty"`
	Description      string  `json:"description,omitempty"`
	TopFilmsPosition int     `json:"topFilmsPosition,omitempty"`
}

type FilmSummary struct {
	ID               string `json:"id"`
	Name             string `json:"name,omitempty"`
	FullDisplayName  string `json:"fullDisplayName,omitempty"`
	ReleaseYear      int    `json:"releaseYear,omitempty"`
	Link             string `json:"link,omitempty"`
	Poster           *Image `json:"poster,omitempty"`
	ContextualPoster *Image `json:"contextualPoster,omitempty"`
}

type MemberSummary struct {
	ID            string `json:"id"`
	Username      string `json:"username"`
	GivenName     string `json:"givenName,omitempty"`
	FamilyName    string `json:"familyName,omitempty"`
	DisplayName   string `json:"displayName,omitempty"`
	ShortName     string `json:"shortName,omitempty"`
	Avatar        *Image `json:"avatar,omitempty"`
	MemberStatus  string `json:"memberStatus,omitempty"`
	AccountStatus string `json:"accountStatus,omitempty"`
}

type Member struct {
	MemberSummary
	Website          string        `json:"website,omitempty"`
	Location         string        `json:"location,omitempty"`
	Bio              string        `json:"bio,omitempty"`
	Links            []Link        `json:"links,omitempty"`
	FavoriteFilms    []FilmSummary `json:"favoriteFilms,omitempty"`
	PrivateWatchlist bool          `json:"privateWatchlist,omitempty"`
}

type MemberAccount struct {
	Member
	EmailAddress                   string `json:"emailAddress,omitempty"`
	TwoFactorAuthenticationEnabled bool   `json:"twoFactorAuthenticationEnabled,omitempty"`
	EmailAddressValidated          bool   `json:"emailAddressValidated,omitempty"`
	PrivateAccount                 bool   `json:"privateAccount,omitempty"`
}

type SearchItem struct {
	Type   string         `json:"type"`
	Score  float64        `json:"score"`
	Film   *FilmSummary   `json:"film,omitempty"`
	Member *MemberSummary `json:"member,omitempty"`
	Review *LogEntry      `json:"review,omitempty"`
	Name   string         `json:"name,omitempty"`
}

type ActivityItem struct {
	Type        string         `json:"type"`
	Member      MemberSummary  `json:"member"`
	WhenCreated time.Time      `json:"whenCreated"`
	Film        *FilmSummary   `json:"film,omitempty"`
	LogEntry    *LogEntry      `json:"logEntry,omitempty"`
	Owner       *MemberSummary `json:"owner,omitempty"`
}

type DiaryDetails struct {
	DiaryDate string `json:"diaryDate"`
	Rewatch   bool   `json:"rewatch,omitempty"`
}

type Review struct {
	ContainsSpoilers bool      `json:"containsSpoilers,omitempty"`
	WhenReviewed     time.Time `json:"whenReviewed,omitempty"`
	Text             string    `json:"text,omitempty"`
	LBML             string    `json:"lbml,omitempty"`
	OriginalLBML     string    `json:"originalLbml,omitempty"`
}

type Tag struct {
	Code       string `json:"code,omitempty"`
	DisplayTag string `json:"displayTag,omitempty"`
}

type LogEntry struct {
	ID            string        `json:"id"`
	Film          *FilmSummary  `json:"film,omitempty"`
	Production    *FilmSummary  `json:"production,omitempty"`
	Name          string        `json:"name,omitempty"`
	Owner         MemberSummary `json:"owner"`
	DiaryDetails  *DiaryDetails `json:"diaryDetails,omitempty"`
	Review        *Review       `json:"review,omitempty"`
	Tags          []string      `json:"tags,omitempty"`
	Tags2         []Tag         `json:"tags2,omitempty"`
	WhenCreated   time.Time     `json:"whenCreated,omitempty"`
	WhenUpdated   time.Time     `json:"whenUpdated,omitempty"`
	Rating        *float64      `json:"rating,omitempty"`
	Like          bool          `json:"like,omitempty"`
	Commentable   bool          `json:"commentable,omitempty"`
	CommentPolicy string        `json:"commentPolicy,omitempty"`
	PrivacyPolicy string        `json:"privacyPolicy,omitempty"`
	Links         []Link        `json:"links,omitempty"`
}

type ReviewComment struct {
	ID                    string        `json:"id"`
	Member                MemberSummary `json:"member"`
	WhenCreated           time.Time     `json:"whenCreated"`
	WhenUpdated           time.Time     `json:"whenUpdated"`
	Comment               string        `json:"comment,omitempty"`
	CommentLBML           string        `json:"commentLbml,omitempty"`
	RemovedByAdmin        bool          `json:"removedByAdmin,omitempty"`
	RemovedByContentOwner bool          `json:"removedByContentOwner,omitempty"`
	Deleted               bool          `json:"deleted,omitempty"`
	Blocked               bool          `json:"blocked,omitempty"`
	BlockedByOwner        bool          `json:"blockedByOwner,omitempty"`
}

type LogEntryCreationDiaryDetails struct {
	DiaryDate string `json:"diaryDate"`
	Rewatch   bool   `json:"rewatch,omitempty"`
}

type LogEntryCreationReview struct {
	Text             string `json:"text"`
	ContainsSpoilers bool   `json:"containsSpoilers,omitempty"`
}

type LogEntryCreationRequest struct {
	FilmID        string                        `json:"filmId"`
	DiaryDetails  *LogEntryCreationDiaryDetails `json:"diaryDetails,omitempty"`
	Review        *LogEntryCreationReview       `json:"review,omitempty"`
	Tags          []string                      `json:"tags,omitempty"`
	Rating        *float64                      `json:"rating,omitempty"`
	Like          *bool                         `json:"like,omitempty"`
	CommentPolicy string                        `json:"commentPolicy,omitempty"`
	PrivacyPolicy string                        `json:"privacyPolicy,omitempty"`
}

type LogEntryUpdateRequest struct {
	DiaryDetails  *LogEntryCreationDiaryDetails `json:"diaryDetails,omitempty"`
	Review        *LogEntryCreationReview       `json:"review,omitempty"`
	Tags          []string                      `json:"tags,omitempty"`
	Rating        *float64                      `json:"rating,omitempty"`
	Like          *bool                         `json:"like,omitempty"`
	CommentPolicy string                        `json:"commentPolicy,omitempty"`
	PrivacyPolicy string                        `json:"privacyPolicy,omitempty"`
}

type UpdateMessage struct {
	Type  string `json:"type"`
	Code  string `json:"code"`
	Title string `json:"title"`
}

type LogEntryUpdateResponse struct {
	Data     LogEntry        `json:"data"`
	Messages []UpdateMessage `json:"messages"`
}
