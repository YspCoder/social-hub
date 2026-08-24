package ximalaya

import (
	"context"
	"encoding/json"
	"time"

	"social-hub/pkg/socialhub"
)

const maxProviderObjectBytes = 8 << 20

type ID int64

type DeviceIDType string

const (
	DeviceOAID       DeviceIDType = "OAID"
	DeviceOAIDMD5    DeviceIDType = "OAID_MD5"
	DeviceAndroidID  DeviceIDType = "Android_ID"
	DeviceAndroidMD5 DeviceIDType = "Android_ID_MD5"
	DeviceIDFA       DeviceIDType = "IDFA"
	DeviceIDFAMD5    DeviceIDType = "IDFA_MD5"
	DeviceUUID       DeviceIDType = "UUID"
)

type ResponseMeta struct {
	StatusCode         int
	RetryAfter         string
	RetryAfterDuration time.Duration
}

type CategoryList struct {
	Categories []Category      `json:"-"`
	Meta       ResponseMeta    `json:"-"`
	Raw        json.RawMessage `json:"-"`
}

type Category struct {
	ID             ID              `json:"id"`
	Kind           string          `json:"kind"`
	Name           string          `json:"category_name"`
	CoverURLSmall  string          `json:"cover_url_small"`
	CoverURLMiddle string          `json:"cover_url_middle"`
	CoverURLLarge  string          `json:"cover_url_large"`
	OrderNumber    int             `json:"order_num"`
	Raw            json.RawMessage `json:"-"`
}

func (value *Category) UnmarshalJSON(data []byte) error {
	type wire Category
	var decoded wire
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*value = Category(decoded)
	value.Raw = append(json.RawMessage(nil), data...)
	return nil
}

type Announcer struct {
	ID          ID     `json:"id"`
	Kind        string `json:"kind"`
	Nickname    string `json:"nickname"`
	AvatarURL   string `json:"avatar_url"`
	IsVerified  bool   `json:"is_verified"`
	AnchorGrade int    `json:"anchor_grade"`
}

type LastUpTrack struct {
	TrackID   ID      `json:"track_id"`
	Title     string  `json:"track_title"`
	Duration  float64 `json:"duration"`
	CreatedAt int64   `json:"created_at"`
	UpdatedAt int64   `json:"updated_at"`
}

type PriceDetail struct {
	PriceType       int    `json:"price_type"`
	Price           string `json:"price"`
	DiscountedPrice string `json:"discounted_price"`
	PriceUnit       string `json:"price_unit"`
}

type Album struct {
	ID                   ID              `json:"id"`
	Kind                 string          `json:"kind"`
	Title                string          `json:"album_title"`
	TracksNaturalOrdered bool            `json:"tracks_natural_ordered"`
	CategoryID           ID              `json:"category_id"`
	Tags                 string          `json:"album_tags"`
	Intro                string          `json:"album_intro"`
	CoverURL             string          `json:"cover_url"`
	CoverURLSmall        string          `json:"cover_url_small"`
	CoverURLMiddle       string          `json:"cover_url_middle"`
	CoverURLLarge        string          `json:"cover_url_large"`
	Announcer            *Announcer      `json:"announcer"`
	PlayCount            int64           `json:"play_count"`
	FavoriteCount        int64           `json:"favorite_count"`
	ShareCount           int64           `json:"share_count"`
	SubscribeCount       int64           `json:"subscribe_count"`
	IncludeTrackCount    int64           `json:"include_track_count"`
	LastUpTrack          *LastUpTrack    `json:"last_uptrack"`
	IsFinished           int             `json:"is_finished"`
	CanDownload          bool            `json:"can_download"`
	CopyrightSource      int             `json:"copyright_source"`
	CreatedAt            int64           `json:"created_at"`
	UpdatedAt            int64           `json:"updated_at"`
	Meta                 string          `json:"meta"`
	RecommendReason      string          `json:"recommend_reason"`
	ShortRichIntro       string          `json:"short_rich_intro"`
	QualityTags          string          `json:"quality_tags"`
	QualityScore         string          `json:"quality_score"`
	IsPaid               bool            `json:"is_paid"`
	AlbumScore           string          `json:"album_score"`
	ShortIntro           string          `json:"short_intro"`
	EstimatedTrackCount  int64           `json:"estimated_track_count"`
	FreeTrackCount       int64           `json:"free_track_count"`
	FreeTrackIDs         string          `json:"free_track_ids"`
	DetailBannerURL      string          `json:"detail_banner_url"`
	TargetCloud          string          `json:"target_cloud"`
	Outline              string          `json:"outline"`
	AlbumRichIntro       string          `json:"album_rich_intro"`
	SpeakerIntro         string          `json:"speaker_intro"`
	SaleIntro            string          `json:"sale_intro"`
	ExpectedRevenue      string          `json:"expected_revenue"`
	BuyNotes             string          `json:"buy_notes"`
	SpeakerTitle         string          `json:"speaker_title"`
	SpeakerContent       string          `json:"speaker_content"`
	HasSample            bool            `json:"has_sample"`
	ComposedPriceType    int             `json:"composed_price_type"`
	PriceTypeDetail      string          `json:"price_type_detail"`
	PriceTypeInfo        []PriceDetail   `json:"price_type_info"`
	SellingPoint         string          `json:"selling_point"`
	IsVIPFree            bool            `json:"is_vipfree"`
	IsVIPExclusive       bool            `json:"is_vip_exclusive"`
	IsFreeListen         bool            `json:"is_free_listen"`
	FreeListenStart      int64           `json:"free_listen_start"`
	FreeListenEnd        int64           `json:"free_listen_end"`
	IsActivity           bool            `json:"is_activity"`
	IsGradientActivity   bool            `json:"is_gradient_activity"`
	Raw                  json.RawMessage `json:"-"`
}

func (value *Album) UnmarshalJSON(data []byte) error {
	type wire Album
	var decoded wire
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*value = Album(decoded)
	value.Raw = append(json.RawMessage(nil), data...)
	return nil
}

type SubordinatedAlbum struct {
	ID             ID     `json:"id"`
	Title          string `json:"album_title"`
	CoverURLSmall  string `json:"cover_url_small"`
	CoverURLMiddle string `json:"cover_url_middle"`
	CoverURLLarge  string `json:"cover_url_large"`
}

type Track struct {
	ID                ID                 `json:"id"`
	Kind              string             `json:"kind"`
	CategoryID        ID                 `json:"category_id"`
	Title             string             `json:"track_title"`
	CleanTitle        string             `json:"track_clean_title"`
	Tags              string             `json:"track_tags"`
	Intro             string             `json:"track_intro"`
	RichIntro         string             `json:"track_rich_intro"`
	CoverURLSmall     string             `json:"cover_url_small"`
	CoverURLMiddle    string             `json:"cover_url_middle"`
	CoverURLLarge     string             `json:"cover_url_large"`
	Announcer         *Announcer         `json:"announcer"`
	Duration          float64            `json:"duration"`
	PlayCount         int64              `json:"play_count"`
	FavoriteCount     int64              `json:"favorite_count"`
	CommentCount      int64              `json:"comment_count"`
	VIPFirstStatus    int                `json:"vip_first_status"`
	CanDownload       bool               `json:"can_download"`
	DownloadSize      int64              `json:"download_size"`
	DownloadCount     int64              `json:"download_count"`
	OrderNumber       int                `json:"order_num"`
	Position          int                `json:"position"`
	SubordinatedAlbum *SubordinatedAlbum `json:"subordinated_album"`
	Source            int                `json:"source"`
	CreatedAt         int64              `json:"created_at"`
	UpdatedAt         int64              `json:"updated_at"`
	IsPaid            bool               `json:"is_paid"`
	IsFree            bool               `json:"is_free"`
	IsTrailer         bool               `json:"is_trailer"`
	HasSample         bool               `json:"has_sample"`
	SampleDuration    float64            `json:"sample_duration"`
	IsBought          *bool              `json:"is_bought"`
	ContainVideo      bool               `json:"contain_video"`
	Is22Kbps          bool               `json:"is_22kbps"`
	PlayURL32         string             `json:"play_url_32"`
	PlaySize32        int64              `json:"play_size_32"`
	PlayURL64         string             `json:"play_url_64"`
	PlaySize64        int64              `json:"play_size_64"`
	PlayURL24M4A      string             `json:"play_url_24_m4a"`
	PlaySize24M4A     int64              `json:"play_size_24_m4a"`
	PlayURL64M4A      string             `json:"play_url_64_m4a"`
	PlaySize64M4A     int64              `json:"play_size_64_m4a"`
	PlayURLAMR        string             `json:"play_url_amr"`
	PlaySizeAMR       int64              `json:"play_size_amr"`
	Raw               json.RawMessage    `json:"-"`
}

func (value *Track) UnmarshalJSON(data []byte) error {
	type wire Track
	var decoded wire
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*value = Track(decoded)
	value.Raw = append(json.RawMessage(nil), data...)
	return nil
}

type AlbumPage struct {
	TotalPages  int             `json:"total_page"`
	TotalCount  int64           `json:"total_count"`
	CurrentPage int             `json:"current_page"`
	CategoryID  ID              `json:"category_id"`
	TagName     string          `json:"tag_name"`
	Albums      []Album         `json:"albums"`
	Meta        ResponseMeta    `json:"-"`
	Raw         json.RawMessage `json:"-"`
}

type AlbumTracksPage struct {
	TotalPages       int             `json:"total_page"`
	TotalCount       int64           `json:"total_count"`
	CurrentPage      int             `json:"current_page"`
	AlbumID          ID              `json:"album_id"`
	AlbumTitle       string          `json:"album_title"`
	AlbumIntro       string          `json:"album_intro"`
	CategoryID       ID              `json:"category_id"`
	CoverURL         string          `json:"cover_url"`
	CoverURLSmall    string          `json:"cover_url_small"`
	CoverURLMiddle   string          `json:"cover_url_middle"`
	CoverURLLarge    string          `json:"cover_url_large"`
	CanDownload      bool            `json:"can_download"`
	Tracks           []Track         `json:"tracks"`
	SellingPoint     string          `json:"selling_point"`
	RecommendReason  string          `json:"recommend_reason"`
	ShortRichIntro   string          `json:"short_rich_intro"`
	ChannelPlayCount int64           `json:"channel_play_count"`
	Meta             ResponseMeta    `json:"-"`
	Raw              json.RawMessage `json:"-"`
}

type TrackPage struct {
	TotalPages  int             `json:"total_page"`
	TotalCount  int64           `json:"total_count"`
	CurrentPage int             `json:"current_page"`
	Tracks      []Track         `json:"tracks"`
	Meta        ResponseMeta    `json:"-"`
	Raw         json.RawMessage `json:"-"`
}

type AlbumListDimension int

const (
	AlbumsHot        AlbumListDimension = 1
	AlbumsNewest     AlbumListDimension = 2
	AlbumsMostPlayed AlbumListDimension = 3
)

type ListAlbumsRequest struct {
	CategoryID   ID
	TagName      string
	Dimension    AlbumListDimension
	Page         int
	Count        int
	ContainsPaid *bool
}

type TrackSort string

const (
	TrackSortXimalayaAscending  TrackSort = "asc"
	TrackSortXimalayaDescending TrackSort = "desc"
	TrackSortTimeAscending      TrackSort = "time_asc"
	TrackSortTimeDescending     TrackSort = "time_desc"
)

type BrowseAlbumRequest struct {
	AlbumID ID
	Sort    TrackSort
	Page    int
	Count   int
}

type AlbumSearchSort string

const (
	AlbumSortCreated         AlbumSearchSort = "created_at"
	AlbumSortUpdated         AlbumSearchSort = "updated_at"
	AlbumSortDiscountedPrice AlbumSearchSort = "discountedPrice"
	AlbumSortPlayCount       AlbumSearchSort = "play_count"
	AlbumSortWeekScore       AlbumSearchSort = "week_score_plus"
)

type SearchAlbumsRequest struct {
	ID           ID
	Title        string
	AnnouncerID  ID
	Nickname     string
	Tags         string
	Paid         *bool
	PriceType    int
	CategoryID   ID
	CategoryName string
	SortBy       AlbumSearchSort
	Descending   *bool
	Page         int
	Count        int
}

type TrackSearchSort string

const (
	TrackSearchCreated TrackSearchSort = "created_at"
	TrackSearchUpdated TrackSearchSort = "updated_at"
)

type SearchTracksRequest struct {
	ID           ID
	Title        string
	AlbumID      ID
	AlbumTitle   string
	AnnouncerID  ID
	Nickname     string
	Tags         string
	Paid         *bool
	CategoryID   ID
	CategoryName string
	SortBy       TrackSearchSort
	Descending   *bool
	Page         int
	Count        int
}

type ReadWorkflow interface {
	ListCategories(context.Context, ...socialhub.CallOption) (CategoryList, error)
	ListAlbums(context.Context, ListAlbumsRequest, ...socialhub.CallOption) (AlbumPage, error)
	BrowseAlbum(context.Context, BrowseAlbumRequest, ...socialhub.CallOption) (AlbumTracksPage, error)
	SearchAlbums(context.Context, SearchAlbumsRequest, ...socialhub.CallOption) (AlbumPage, error)
	SearchTracks(context.Context, SearchTracksRequest, ...socialhub.CallOption) (TrackPage, error)
}

var _ ReadWorkflow = (*Client)(nil)
