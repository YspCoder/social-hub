package wikimedia

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"social-hub/pkg/socialhub"
)

const maxProviderObjectBytes = 8 << 20

type ResponseMeta struct {
	RequestID    string
	CacheControl string
}

type SearchPagesRequest struct {
	Query string
	Limit int
}

type GetPageRequest struct {
	FollowRedirects *bool
}

// KnowledgeWorkflow is the bounded anonymous MediaWiki REST v1 surface.
// Selected collection endpoints return a fixed result set and define no
// continuation token.
type KnowledgeWorkflow interface {
	SearchPages(context.Context, SearchPagesRequest, ...socialhub.CallOption) (SearchResponse, error)
	GetPage(context.Context, string, GetPageRequest, ...socialhub.CallOption) (Page, error)
	ListPageMedia(context.Context, string, ...socialhub.CallOption) (PageMediaResponse, error)
	GetFile(context.Context, string, ...socialhub.CallOption) (File, error)
	ListFileThumbnails(context.Context, string, ...socialhub.CallOption) (FileThumbnailsResponse, error)
}

type SearchResponse struct {
	Pages []SearchResult  `json:"pages"`
	Meta  ResponseMeta    `json:"-"`
	Raw   json.RawMessage `json:"-"`
}

type SearchResult struct {
	ID           int64            `json:"id"`
	Key          string           `json:"key"`
	Title        string           `json:"title"`
	Excerpt      *string          `json:"excerpt"`
	MatchedTitle *string          `json:"matched_title"`
	Anchor       *string          `json:"anchor"`
	Description  *string          `json:"description"`
	Thumbnail    *SearchThumbnail `json:"thumbnail"`
	Raw          json.RawMessage  `json:"-"`
}

func (value *SearchResult) UnmarshalJSON(data []byte) error {
	type wire SearchResult
	var decoded wire
	if err := decodeProviderObject(data, &decoded); err != nil {
		return err
	}
	*value = SearchResult(decoded)
	value.Raw = append(value.Raw[:0], data...)
	return nil
}

type SearchThumbnail struct {
	MIMEType string   `json:"mimetype"`
	Width    *int64   `json:"width"`
	Height   *int64   `json:"height"`
	Duration *float64 `json:"duration"`
	URL      string   `json:"url"`
}

type Revision struct {
	ID        int64     `json:"id"`
	Timestamp time.Time `json:"timestamp"`
}

type License struct {
	URL   string `json:"url"`
	Title string `json:"title"`
}

type Page struct {
	ID           int64           `json:"id"`
	Key          string          `json:"key"`
	Title        string          `json:"title"`
	Latest       Revision        `json:"latest"`
	ContentModel string          `json:"content_model"`
	License      License         `json:"license"`
	HTMLURL      string          `json:"html_url"`
	Meta         ResponseMeta    `json:"-"`
	Raw          json.RawMessage `json:"-"`
}

func (value *Page) UnmarshalJSON(data []byte) error {
	type wire Page
	var decoded wire
	if err := decodeProviderObject(data, &decoded); err != nil {
		return err
	}
	*value = Page(decoded)
	value.Raw = append(value.Raw[:0], data...)
	return nil
}

type MediaType string

const (
	MediaBitmap     MediaType = "BITMAP"
	MediaDrawing    MediaType = "DRAWING"
	MediaAudio      MediaType = "AUDIO"
	MediaVideo      MediaType = "VIDEO"
	MediaMultimedia MediaType = "MULTIMEDIA"
	MediaUnknown    MediaType = "UNKNOWN"
	MediaOffice     MediaType = "OFFICE"
	MediaText       MediaType = "TEXT"
	MediaExecutable MediaType = "EXECUTABLE"
	MediaArchive    MediaType = "ARCHIVE"
	Media3D         MediaType = "3D"
)

type FileUser struct {
	ID   *int64  `json:"id"`
	Name *string `json:"name"`
}

type FileRevision struct {
	Timestamp time.Time `json:"timestamp"`
	User      FileUser  `json:"user"`
}

type FileRepresentation struct {
	MediaType MediaType `json:"mediatype"`
	Size      *int64    `json:"size"`
	Width     *int64    `json:"width"`
	Height    *int64    `json:"height"`
	Duration  *float64  `json:"duration"`
	URL       string    `json:"url"`
}

type File struct {
	Title              string              `json:"title"`
	FileDescriptionURL string              `json:"file_description_url"`
	Latest             *FileRevision       `json:"latest"`
	Preferred          *FileRepresentation `json:"preferred"`
	Original           *FileRepresentation `json:"original"`
	Thumbnail          *FileRepresentation `json:"thumbnail,omitempty"`
	Meta               ResponseMeta        `json:"-"`
	Raw                json.RawMessage     `json:"-"`
}

func (value *File) UnmarshalJSON(data []byte) error {
	type wire File
	var decoded wire
	if err := decodeProviderObject(data, &decoded); err != nil {
		return err
	}
	*value = File(decoded)
	value.Raw = append(value.Raw[:0], data...)
	return nil
}

type PageMediaResponse struct {
	Files []File          `json:"files"`
	Meta  ResponseMeta    `json:"-"`
	Raw   json.RawMessage `json:"-"`
}

type ThumbnailOriginal struct {
	MediaType MediaType `json:"mediatype"`
	Width     *int64    `json:"width"`
	Height    *int64    `json:"height"`
	URL       string    `json:"url"`
}

type FileThumbnail struct {
	Width          int64             `json:"width"`
	Height         int64             `json:"height"`
	URL            string            `json:"url"`
	MIMEType       string            `json:"mime"`
	ResponsiveURLs map[string]string `json:"responsive_urls"`
}

type FileThumbnailsResponse struct {
	Title      string            `json:"title"`
	Original   ThumbnailOriginal `json:"original"`
	Thumbnails []FileThumbnail   `json:"thumbnails"`
	Meta       ResponseMeta      `json:"-"`
	Raw        json.RawMessage   `json:"-"`
}

func decodeProviderObject(data []byte, target any) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || len(trimmed) > maxProviderObjectBytes || trimmed[0] != '{' || !json.Valid(trimmed) {
		return fmt.Errorf("wikimedia: invalid provider object")
	}
	return json.Unmarshal(trimmed, target)
}
