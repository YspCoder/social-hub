package tiktokresearch

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"social-hub/pkg/socialhub"
)

const (
	MaximumPageSize         = 100
	DefaultVideoPageSize    = 20
	DefaultCommentPageSize  = 10
	maxProviderObjectBytes  = 8 << 20
	maximumRequestBodyBytes = 256 << 10
	maximumInt64Value       = uint64(1<<63 - 1)
)

// ID stores TikTok's int64 identifiers as decimal strings, avoiding float64
// precision loss while accepting either JSON numbers or strings from TikTok.
type ID string

func (value ID) String() string { return string(value) }

func (value ID) MarshalJSON() ([]byte, error) {
	if !validID(string(value)) {
		return nil, fmt.Errorf("tiktokresearch: ID is invalid")
	}
	return []byte(value), nil
}

func (value *ID) UnmarshalJSON(data []byte) error {
	var text string
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) > 0 && trimmed[0] == '"' {
		if err := json.Unmarshal(trimmed, &text); err != nil {
			return err
		}
	} else {
		text = string(trimmed)
	}
	if !validID(text) {
		return fmt.Errorf("tiktokresearch: ID must be a non-negative int64 decimal")
	}
	*value = ID(text)
	return nil
}

type QueryField string

const (
	QueryFieldCreateDate   QueryField = "create_date"
	QueryFieldUsername     QueryField = "username"
	QueryFieldRegionCode   QueryField = "region_code"
	QueryFieldVideoID      QueryField = "video_id"
	QueryFieldHashtagName  QueryField = "hashtag_name"
	QueryFieldKeyword      QueryField = "keyword"
	QueryFieldMusicID      QueryField = "music_id"
	QueryFieldEffectID     QueryField = "effect_id"
	QueryFieldVideoLength  QueryField = "video_length"
	QueryFieldViewCount    QueryField = "view_count"
	QueryFieldCommentCount QueryField = "comment_count"
)

type QueryOperator string

const (
	OperatorEqual              QueryOperator = "EQ"
	OperatorIn                 QueryOperator = "IN"
	OperatorGreaterThan        QueryOperator = "GT"
	OperatorGreaterThanOrEqual QueryOperator = "GTE"
	OperatorLessThan           QueryOperator = "LT"
	OperatorLessThanOrEqual    QueryOperator = "LTE"
)

type Condition struct {
	Field       QueryField    `json:"field_name"`
	Operator    QueryOperator `json:"operation"`
	FieldValues []string      `json:"field_values"`
}

// Query is TikTok's boolean query DSL. At least one group must contain a
// condition. Conditions within each group are sent without reinterpretation.
type Query struct {
	And []Condition `json:"and,omitempty"`
	Or  []Condition `json:"or,omitempty"`
	Not []Condition `json:"not,omitempty"`
}

type VideoField string

const (
	VideoFieldID              VideoField = "id"
	VideoFieldDescription     VideoField = "video_description"
	VideoFieldCreateTime      VideoField = "create_time"
	VideoFieldRegionCode      VideoField = "region_code"
	VideoFieldShareCount      VideoField = "share_count"
	VideoFieldViewCount       VideoField = "view_count"
	VideoFieldLikeCount       VideoField = "like_count"
	VideoFieldCommentCount    VideoField = "comment_count"
	VideoFieldMusicID         VideoField = "music_id"
	VideoFieldHashtagNames    VideoField = "hashtag_names"
	VideoFieldUsername        VideoField = "username"
	VideoFieldEffectIDs       VideoField = "effect_ids"
	VideoFieldPlaylistID      VideoField = "playlist_id"
	VideoFieldVoiceToText     VideoField = "voice_to_text"
	VideoFieldStemVerified    VideoField = "is_stem_verified"
	VideoFieldFavoritesCount  VideoField = "favorites_count"
	VideoFieldDuration        VideoField = "video_duration"
	VideoFieldHashtagInfoList VideoField = "hashtag_info_list"
	VideoFieldStickerInfoList VideoField = "sticker_info_list"
	VideoFieldEffectInfoList  VideoField = "effect_info_list"
	VideoFieldMentionList     VideoField = "video_mention_list"
	VideoFieldLabel           VideoField = "video_label"
	VideoFieldTag             VideoField = "video_tag"
)

type UserField string

const (
	UserFieldDisplayName    UserField = "display_name"
	UserFieldBioDescription UserField = "bio_description"
	UserFieldAvatarURL      UserField = "avatar_url"
	UserFieldVerified       UserField = "is_verified"
	UserFieldFollowerCount  UserField = "follower_count"
	UserFieldFollowingCount UserField = "following_count"
	UserFieldLikesCount     UserField = "likes_count"
	UserFieldVideoCount     UserField = "video_count"
	UserFieldBioURL         UserField = "bio_url"
)

type CommentField string

const (
	CommentFieldID              CommentField = "id"
	CommentFieldVideoID         CommentField = "video_id"
	CommentFieldText            CommentField = "text"
	CommentFieldLikeCount       CommentField = "like_count"
	CommentFieldReplyCount      CommentField = "reply_count"
	CommentFieldParentCommentID CommentField = "parent_comment_id"
	CommentFieldCreateTime      CommentField = "create_time"
)

type QueryVideosRequest struct {
	Query     Query
	StartDate string
	EndDate   string
	MaxCount  int
	Cursor    uint64
	SearchID  string
	IsRandom  *bool
	Fields    []VideoField
}

type GetUserInfoRequest struct {
	Username string
	Fields   []UserField
}

type ListCommentsRequest struct {
	VideoID   string
	CommentID string
	MaxCount  int
	Cursor    uint64
	Fields    []CommentField
}

// Video retains complex, evolving Research API subobjects as raw JSON while
// exposing every stable scalar in the official video field mask.
type Video struct {
	ID               ID              `json:"id"`
	Description      *string         `json:"video_description"`
	CreateTime       *int64          `json:"create_time"`
	RegionCode       *string         `json:"region_code"`
	ShareCount       *int64          `json:"share_count"`
	ViewCount        *int64          `json:"view_count"`
	LikeCount        *int64          `json:"like_count"`
	CommentCount     *int64          `json:"comment_count"`
	MusicID          *ID             `json:"music_id"`
	HashtagNames     []string        `json:"hashtag_names"`
	Username         *string         `json:"username"`
	EffectIDs        []ID            `json:"effect_ids"`
	PlaylistID       *ID             `json:"playlist_id"`
	VoiceToText      *string         `json:"voice_to_text"`
	IsStemVerified   *bool           `json:"is_stem_verified"`
	FavoritesCount   *int64          `json:"favorites_count"`
	Duration         *int64          `json:"video_duration"`
	HashtagInfoList  json.RawMessage `json:"hashtag_info_list"`
	StickerInfoList  json.RawMessage `json:"sticker_info_list"`
	EffectInfoList   json.RawMessage `json:"effect_info_list"`
	VideoMentionList json.RawMessage `json:"video_mention_list"`
	VideoLabel       json.RawMessage `json:"video_label"`
	VideoTag         json.RawMessage `json:"video_tag"`
	Raw              json.RawMessage `json:"-"`
}

func (value *Video) UnmarshalJSON(data []byte) error {
	type wire Video
	var decoded wire
	if err := decodeProviderObject(data, &decoded); err != nil {
		return err
	}
	*value = Video(decoded)
	value.Raw = append(json.RawMessage(nil), data...)
	return nil
}

// User contains only fields published for the Research user-info endpoint.
// Username identifies the request and is populated even though it is not a
// selectable response field.
type User struct {
	Username       string          `json:"username,omitempty"`
	DisplayName    *string         `json:"display_name"`
	BioDescription *string         `json:"bio_description"`
	AvatarURL      *string         `json:"avatar_url"`
	IsVerified     *bool           `json:"is_verified"`
	FollowerCount  *int64          `json:"follower_count"`
	FollowingCount *int64          `json:"following_count"`
	LikesCount     *int64          `json:"likes_count"`
	VideoCount     *int64          `json:"video_count"`
	BioURL         *string         `json:"bio_url"`
	Raw            json.RawMessage `json:"-"`
}

func (value *User) UnmarshalJSON(data []byte) error {
	type wire User
	var decoded wire
	if err := decodeProviderObject(data, &decoded); err != nil {
		return err
	}
	*value = User(decoded)
	value.Raw = append(json.RawMessage(nil), data...)
	return nil
}

// Comment is anonymized by TikTok. DisplayName is retained when TikTok sends
// it, but it is intentionally absent from CommentField because the endpoint's
// documented field-mask list does not include it.
type Comment struct {
	ID              ID              `json:"id"`
	VideoID         ID              `json:"video_id"`
	Text            *string         `json:"text"`
	LikeCount       *int64          `json:"like_count"`
	ReplyCount      *int64          `json:"reply_count"`
	ParentCommentID *ID             `json:"parent_comment_id"`
	CreateTime      *int64          `json:"create_time"`
	DisplayName     *string         `json:"display_name"`
	Raw             json.RawMessage `json:"-"`
}

func (value *Comment) UnmarshalJSON(data []byte) error {
	type wire Comment
	var decoded wire
	if err := decodeProviderObject(data, &decoded); err != nil {
		return err
	}
	*value = Comment(decoded)
	value.Raw = append(json.RawMessage(nil), data...)
	return nil
}

type ResponseMeta struct {
	LogID              string
	RetryAfter         string
	RetryAfterDuration time.Duration
}

type VideoPage struct {
	Videos   []Video
	Cursor   uint64
	HasMore  bool
	SearchID string
	Meta     ResponseMeta
	Raw      json.RawMessage
}

type UserResponse struct {
	User User
	Meta ResponseMeta
	Raw  json.RawMessage
}

type CommentPage struct {
	Comments []Comment
	Cursor   uint64
	HasMore  bool
	Meta     ResponseMeta
	Raw      json.RawMessage
}

type ResearchWorkflow interface {
	QueryVideos(context.Context, QueryVideosRequest, ...socialhub.CallOption) (*VideoPage, error)
	GetUserInfo(context.Context, GetUserInfoRequest, ...socialhub.CallOption) (*UserResponse, error)
	ListComments(context.Context, ListCommentsRequest, ...socialhub.CallOption) (*CommentPage, error)
}

func decodeProviderObject(data []byte, target any) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || len(trimmed) > maxProviderObjectBytes || trimmed[0] != '{' || !json.Valid(trimmed) {
		return fmt.Errorf("tiktokresearch: invalid provider object")
	}
	return json.Unmarshal(trimmed, target)
}

func parseInt64ID(value string) (uint64, bool) {
	parsed, err := strconv.ParseUint(value, 10, 63)
	return parsed, err == nil && strconv.FormatUint(parsed, 10) == value
}

var _ ResearchWorkflow = (*Client)(nil)
