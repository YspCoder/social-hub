package imgur

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"

	"social-hub/pkg/socialhub"
)

// ID accepts the string and integer identifiers used by different Imgur models.
type ID string

func (identifier *ID) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, []byte("null")) {
		*identifier = ""
		return nil
	}
	var text string
	if len(data) > 0 && data[0] == '"' {
		if err := json.Unmarshal(data, &text); err != nil {
			return err
		}
		*identifier = ID(text)
		return nil
	}
	var number json.Number
	if err := json.Unmarshal(data, &number); err != nil {
		return err
	}
	if _, err := number.Int64(); err != nil {
		return fmt.Errorf("imgur: invalid numeric identifier")
	}
	*identifier = ID(number.String())
	return nil
}

// Account is the stable subset of an Imgur account response.
type Account struct {
	ID             ID     `json:"id"`
	URL            string `json:"url"`
	Bio            string `json:"bio"`
	Avatar         string `json:"avatar"`
	Cover          string `json:"cover"`
	Reputation     int64  `json:"reputation"`
	ReputationName string `json:"reputation_name"`
	Created        int64  `json:"created"`
	ProExpiration  *int64 `json:"pro_expiration"`
}

// Image is the stable subset shared by image, account-image, and upload responses.
type Image struct {
	ID            string `json:"id"`
	DeleteHash    string `json:"deletehash"`
	AccountID     ID     `json:"account_id"`
	AccountURL    string `json:"account_url"`
	Title         string `json:"title"`
	Description   string `json:"description"`
	Name          string `json:"name"`
	Datetime      int64  `json:"datetime"`
	MIME          string `json:"type"`
	Animated      bool   `json:"animated"`
	Width         int    `json:"width"`
	Height        int    `json:"height"`
	Size          int64  `json:"size"`
	Views         int64  `json:"views"`
	Bandwidth     int64  `json:"bandwidth"`
	Vote          string `json:"vote"`
	Favorite      bool   `json:"favorite"`
	NSFW          *bool  `json:"nsfw"`
	Section       string `json:"section"`
	InGallery     bool   `json:"in_gallery"`
	InMostViral   bool   `json:"in_most_viral"`
	HasSound      bool   `json:"has_sound"`
	Link          string `json:"link"`
	MP4           string `json:"mp4"`
	GIFV          string `json:"gifv"`
	HLS           string `json:"hls"`
	MP4Size       int64  `json:"mp4_size"`
	CommentCount  int64  `json:"comment_count"`
	FavoriteCount int64  `json:"favorite_count"`
	Ups           int64  `json:"ups"`
	Downs         int64  `json:"downs"`
	Points        int64  `json:"points"`
	Score         int64  `json:"score"`
}

// Album is the stable subset of an Imgur album response.
type Album struct {
	ID          string  `json:"id"`
	DeleteHash  string  `json:"deletehash"`
	Title       string  `json:"title"`
	Description string  `json:"description"`
	Datetime    int64   `json:"datetime"`
	Cover       string  `json:"cover"`
	AccountID   ID      `json:"account_id"`
	AccountURL  string  `json:"account_url"`
	Privacy     string  `json:"privacy"`
	Views       int64   `json:"views"`
	Link        string  `json:"link"`
	Favorite    bool    `json:"favorite"`
	NSFW        *bool   `json:"nsfw"`
	Section     string  `json:"section"`
	ImagesCount int     `json:"images_count"`
	Images      []Image `json:"images"`
	InGallery   bool    `json:"in_gallery"`
}

// Comment is an Imgur Gallery comment and its nested replies.
type Comment struct {
	ID         ID        `json:"id"`
	ImageID    string    `json:"image_id"`
	Text       string    `json:"comment"`
	Author     string    `json:"author"`
	AuthorID   ID        `json:"author_id"`
	OnAlbum    bool      `json:"on_album"`
	AlbumCover string    `json:"album_cover"`
	Vote       string    `json:"vote"`
	Ups        int64     `json:"ups"`
	Downs      int64     `json:"downs"`
	Points     int64     `json:"points"`
	Datetime   int64     `json:"datetime"`
	ParentID   ID        `json:"parent_id"`
	Deleted    bool      `json:"deleted"`
	Children   []Comment `json:"children"`
}

// Credits reports the current application and user credit allocation.
type Credits struct {
	UserLimit       int64 `json:"UserLimit"`
	UserRemaining   int64 `json:"UserRemaining"`
	UserReset       int64 `json:"UserReset"`
	ClientLimit     int64 `json:"ClientLimit"`
	ClientRemaining int64 `json:"ClientRemaining"`
}

// UploadRequest describes a streaming Imgur upload.
type UploadRequest struct {
	Filename     string
	MIME         string
	Size         int64
	Album        string
	Name         string
	Title        string
	Description  string
	DisableAudio *bool
}

// ImageUpdateRequest changes optional image metadata.
type ImageUpdateRequest struct {
	Title       *string
	Description *string
}

// AlbumRequest creates or updates an album without deprecated layout controls.
type AlbumRequest struct {
	ImageIDs     []string
	DeleteHashes []string
	Title        *string
	Description  *string
	Cover        *string
}

// AlbumReference identifies a created account or anonymous album.
type AlbumReference struct {
	ID         string `json:"id"`
	DeleteHash string `json:"deletehash"`
}

// GalleryShareRequest publishes one uploaded image to the public Gallery.
type GalleryShareRequest struct {
	ImageID string
	Title   string
	Topic   string
	Mature  bool
	Tags    []string
}

// GalleryVote is an Imgur Gallery vote command.
type GalleryVote string

const (
	GalleryVoteUp   GalleryVote = "up"
	GalleryVoteDown GalleryVote = "down"
	GalleryVoteVeto GalleryVote = "veto"
)

// ImageWorkflow exposes image reads, uploads, metadata, deletion, and favorite toggling.
type ImageWorkflow interface {
	GetImage(context.Context, string, ...socialhub.CallOption) (*Image, error)
	ListAccountImages(context.Context, string, string, int, ...socialhub.CallOption) (socialhub.Page[Image], error)
	Upload(context.Context, UploadRequest, io.Reader, ...socialhub.CallOption) (*Image, error)
	UpdateImage(context.Context, string, ImageUpdateRequest, ...socialhub.CallOption) error
	DeleteImage(context.Context, string, ...socialhub.CallOption) error
	ToggleFavorite(context.Context, string, ...socialhub.CallOption) (string, error)
}

// AlbumWorkflow exposes Imgur album lifecycle operations.
type AlbumWorkflow interface {
	GetAlbum(context.Context, string, ...socialhub.CallOption) (*Album, error)
	ListAlbumImages(context.Context, string, ...socialhub.CallOption) ([]Image, error)
	CreateAlbum(context.Context, AlbumRequest, ...socialhub.CallOption) (*AlbumReference, error)
	UpdateAlbum(context.Context, string, AlbumRequest, ...socialhub.CallOption) error
	DeleteAlbum(context.Context, string, ...socialhub.CallOption) error
}

// GalleryWorkflow exposes Gallery publication, removal, and voting.
type GalleryWorkflow interface {
	ShareImage(context.Context, GalleryShareRequest, ...socialhub.CallOption) error
	RemoveFromGallery(context.Context, string, ...socialhub.CallOption) error
	Vote(context.Context, string, GalleryVote, ...socialhub.CallOption) error
}

// CreditWorkflow exposes Imgur rate-credit counters.
type CreditWorkflow interface {
	Credits(context.Context, ...socialhub.CallOption) (*Credits, error)
}

type idResponse struct {
	ID ID `json:"id"`
}
