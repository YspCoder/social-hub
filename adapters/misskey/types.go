package misskey

import (
	"context"
	"encoding/json"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

type NoteVisibility string

const (
	VisibilityPublic    NoteVisibility = "public"
	VisibilityHome      NoteVisibility = "home"
	VisibilityFollowers NoteVisibility = "followers"
	VisibilitySpecified NoteVisibility = "specified"
)

type ReactionAcceptance string

const (
	ReactionLikeOnly                           ReactionAcceptance = "likeOnly"
	ReactionLikeOnlyForRemote                  ReactionAcceptance = "likeOnlyForRemote"
	ReactionNonSensitiveOnly                   ReactionAcceptance = "nonSensitiveOnly"
	ReactionNonSensitiveLocalLikeOnlyForRemote ReactionAcceptance = "nonSensitiveOnlyForLocalLikeOnlyForRemote"
)

type Poll struct {
	Choices     []string
	Multiple    bool
	ExpiresAt   *time.Time
	ExpireAfter time.Duration
}

// CreateNoteRequest preserves Misskey fields not representable by the common
// CreatePostRequest.
type CreateNoteRequest struct {
	Text               *string
	FileIDs            []string
	ReplyID            *string
	RenoteID           *string
	Visibility         NoteVisibility
	VisibleUserIDs     []string
	ContentWarning     *string
	LocalOnly          bool
	ReactionAcceptance *ReactionAcceptance
	ChannelID          *string
	Poll               *Poll
}

type TimelineRequest struct {
	Cursor      string
	MaxResults  int
	StartTime   *time.Time
	EndTime     *time.Time
	WithFiles   bool
	WithRenotes *bool
}

type DriveUploadRequest struct {
	Upload    socialhub.BeginUploadRequest
	FolderID  string
	Comment   string
	Sensitive bool
	Force     bool
}

type InstanceInfo struct {
	Name                  string
	ShortName             string
	Version               string
	Description           string
	URI                   string
	DisableLocalTimeline  bool
	DisableGlobalTimeline bool
	MediaProxy            string
}

// MiAuthRequest contains the browser authorization parameters for one fresh
// MiAuth session.
type MiAuthRequest struct {
	Session     string
	Name        string
	IconURL     string
	CallbackURL string
	Permissions []string
}

// MiAuthResult reports whether the user approved the one-time session.
type MiAuthResult struct {
	OK          bool
	AccessToken string
	User        *socialhub.User
}

type NoteWorkflow interface {
	CreateNote(context.Context, CreateNoteRequest, ...socialhub.CallOption) (*socialhub.Post, error)
	ReactWithEmoji(context.Context, string, string, ...socialhub.CallOption) error
}

type TimelineWorkflow interface {
	HomeTimeline(context.Context, TimelineRequest, ...socialhub.CallOption) (socialhub.Page[socialhub.Post], error)
}

type DriveWorkflow interface {
	BeginDriveUpload(context.Context, DriveUploadRequest, ...socialhub.CallOption) (*socialhub.UploadSession, error)
}

type InstanceWorkflow interface {
	Instance(context.Context, ...socialhub.CallOption) (*InstanceInfo, error)
}

type misskeyUser struct {
	ID             string     `json:"id"`
	Name           *string    `json:"name"`
	Username       string     `json:"username"`
	Host           *string    `json:"host"`
	AvatarURL      string     `json:"avatarUrl"`
	URL            *string    `json:"url"`
	URI            *string    `json:"uri"`
	CreatedAt      *time.Time `json:"createdAt"`
	Description    *string    `json:"description"`
	Location       *string    `json:"location"`
	Lang           *string    `json:"lang"`
	BannerURL      *string    `json:"bannerUrl"`
	IsBot          bool       `json:"isBot"`
	IsCat          bool       `json:"isCat"`
	IsLocked       bool       `json:"isLocked"`
	IsSilenced     bool       `json:"isSilenced"`
	IsSuspended    bool       `json:"isSuspended"`
	OnlineStatus   string     `json:"onlineStatus"`
	FollowersCount int64      `json:"followersCount"`
	FollowingCount int64      `json:"followingCount"`
	NotesCount     int64      `json:"notesCount"`
}

type misskeyNote struct {
	ID                 string             `json:"id"`
	CreatedAt          *time.Time         `json:"createdAt"`
	Text               *string            `json:"text"`
	ContentWarning     *string            `json:"cw"`
	UserID             string             `json:"userId"`
	User               misskeyUser        `json:"user"`
	ReplyID            *string            `json:"replyId"`
	RenoteID           *string            `json:"renoteId"`
	Renote             *misskeyNote       `json:"renote"`
	Visibility         string             `json:"visibility"`
	VisibleUserIDs     []string           `json:"visibleUserIds"`
	Files              []misskeyDriveFile `json:"files"`
	Tags               []string           `json:"tags"`
	Poll               json.RawMessage    `json:"poll"`
	ChannelID          *string            `json:"channelId"`
	LocalOnly          bool               `json:"localOnly"`
	ReactionAcceptance *string            `json:"reactionAcceptance"`
	Reactions          map[string]int64   `json:"reactions"`
	ReactionCount      int64              `json:"reactionCount"`
	RenoteCount        int64              `json:"renoteCount"`
	RepliesCount       int64              `json:"repliesCount"`
	URI                string             `json:"uri"`
	URL                string             `json:"url"`
	MyReaction         *string            `json:"myReaction"`
}

type misskeyDriveFile struct {
	ID         string     `json:"id"`
	CreatedAt  *time.Time `json:"createdAt"`
	Name       string     `json:"name"`
	Type       string     `json:"type"`
	MD5        string     `json:"md5"`
	Size       int64      `json:"size"`
	Sensitive  bool       `json:"isSensitive"`
	BlurHash   *string    `json:"blurhash"`
	Properties struct {
		Width  int `json:"width"`
		Height int `json:"height"`
	} `json:"properties"`
	URL          string  `json:"url"`
	ThumbnailURL *string `json:"thumbnailUrl"`
	Comment      *string `json:"comment"`
	FolderID     *string `json:"folderId"`
	UserID       *string `json:"userId"`
}

type misskeyMeta struct {
	Name                  string `json:"name"`
	ShortName             string `json:"shortName"`
	Version               string `json:"version"`
	Description           string `json:"description"`
	URI                   string `json:"uri"`
	DisableLocalTimeline  bool   `json:"disableLocalTimeline"`
	DisableGlobalTimeline bool   `json:"disableGlobalTimeline"`
	MediaProxy            string `json:"mediaProxy"`
}

func validVisibility(value NoteVisibility) bool {
	switch value {
	case VisibilityPublic, VisibilityHome, VisibilityFollowers, VisibilitySpecified:
		return true
	default:
		return false
	}
}

func validReactionAcceptance(value ReactionAcceptance) bool {
	switch value {
	case ReactionLikeOnly, ReactionLikeOnlyForRemote, ReactionNonSensitiveOnly, ReactionNonSensitiveLocalLikeOnlyForRemote:
		return true
	default:
		return false
	}
}

func validID(value string) bool { return validBoundedString(value, 512) }

func validBoundedString(value string, maximum int) bool {
	if value == "" || strings.TrimSpace(value) != value || len(value) > maximum || !utf8.ValidString(value) {
		return false
	}
	for _, character := range value {
		if character < 0x20 || character == 0x7f {
			return false
		}
	}
	return true
}

func validOptionalString(value string, maximum int) bool {
	if len(value) > maximum || !utf8.ValidString(value) {
		return false
	}
	for _, character := range value {
		if character < 0x20 || character == 0x7f {
			return false
		}
	}
	return true
}

func validContentString(value string, maximum int) bool {
	if value == "" || len(value) > maximum || !utf8.ValidString(value) {
		return false
	}
	for _, character := range value {
		if (character < 0x20 && character != '\n' && character != '\r' && character != '\t') || character == 0x7f {
			return false
		}
	}
	return true
}

func validHTTPURL(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && (parsed.Scheme == "https" || parsed.Scheme == "http") &&
		parsed.Host != "" && parsed.User == nil && parsed.Fragment == ""
}
