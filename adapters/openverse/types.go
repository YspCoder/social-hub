package openverse

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"social-hub/pkg/socialhub"
)

const (
	AnonymousMaximumPageSize     = 20
	AuthenticatedMaximumPageSize = 50
	MaximumSearchDepth           = 240
	maxProviderObjectBytes       = 8 << 20
)

type License string

const (
	LicenseBY         License = "by"
	LicenseBYSA       License = "by-sa"
	LicenseBYND       License = "by-nd"
	LicenseBYNC       License = "by-nc"
	LicenseBYNCSA     License = "by-nc-sa"
	LicenseBYNCND     License = "by-nc-nd"
	LicenseCC0        License = "cc0"
	LicensePDM        License = "pdm"
	LicenseSampling   License = "sampling+"
	LicenseNCSampling License = "nc-sampling+"
)

type LicenseType string

const (
	LicenseTypeAll          LicenseType = "all"
	LicenseTypeAllCC        LicenseType = "all-cc"
	LicenseTypeCommercial   LicenseType = "commercial"
	LicenseTypeModification LicenseType = "modification"
)

type ImageCategory string

const (
	ImageCategoryDigitizedArtwork ImageCategory = "digitized_artwork"
	ImageCategoryIllustration     ImageCategory = "illustration"
	ImageCategoryPhotograph       ImageCategory = "photograph"
)

type ImageAspectRatio string

const (
	ImageAspectTall   ImageAspectRatio = "tall"
	ImageAspectWide   ImageAspectRatio = "wide"
	ImageAspectSquare ImageAspectRatio = "square"
)

type ImageSize string

const (
	ImageSizeSmall  ImageSize = "small"
	ImageSizeMedium ImageSize = "medium"
	ImageSizeLarge  ImageSize = "large"
)

type AudioCategory string

const (
	AudioCategoryAudiobook     AudioCategory = "audiobook"
	AudioCategoryMusic         AudioCategory = "music"
	AudioCategoryNews          AudioCategory = "news"
	AudioCategoryPodcast       AudioCategory = "podcast"
	AudioCategoryPronunciation AudioCategory = "pronunciation"
	AudioCategorySoundEffect   AudioCategory = "sound_effect"
)

type AudioLength string

const (
	AudioLengthShortest AudioLength = "shortest"
	AudioLengthShort    AudioLength = "short"
	AudioLengthMedium   AudioLength = "medium"
	AudioLengthLong     AudioLength = "long"
)

// SearchRequest contains stable filters shared by image and audio search.
// At least one of Query, Creator, Tags, or Title must be supplied.
type SearchRequest struct {
	Query           string
	Sources         []string
	ExcludedSources []string
	Licenses        []License
	LicenseTypes    []LicenseType
	Creator         string
	Tags            string
	Title           string
	Extension       string
	Mature          *bool
	Page            int
	PageSize        int
}

type ImageSearchRequest struct {
	SearchRequest
	Category    ImageCategory
	AspectRatio ImageAspectRatio
	Size        ImageSize
}

type AudioSearchRequest struct {
	SearchRequest
	Category AudioCategory
	Length   AudioLength
}

type ImagesWorkflow interface {
	SearchImages(context.Context, ImageSearchRequest, ...socialhub.CallOption) (ImageSearchResponse, error)
	GetImage(context.Context, string, ...socialhub.CallOption) (Image, error)
}

type AudioWorkflow interface {
	SearchAudio(context.Context, AudioSearchRequest, ...socialhub.CallOption) (AudioSearchResponse, error)
	GetAudio(context.Context, string, ...socialhub.CallOption) (Audio, error)
}

// ResponseMeta preserves the documented request and scoped rate-limit headers.
// Values remain strings so provider changes are not silently coerced.
type ResponseMeta struct {
	RequestID  string
	RateLimits map[string]RateLimit
}

type RateLimit struct {
	Limit     string
	Available string
}

type Tag struct {
	Name             string   `json:"name"`
	Accuracy         *float64 `json:"accuracy"`
	UnstableProvider string   `json:"unstable__provider"`
}

// Media contains fields shared by Openverse image and audio records.
type Media struct {
	ID                  string          `json:"id"`
	Title               string          `json:"title"`
	IndexedOn           string          `json:"indexed_on"`
	ForeignLandingURL   string          `json:"foreign_landing_url"`
	URL                 string          `json:"url"`
	Creator             string          `json:"creator"`
	CreatorURL          string          `json:"creator_url"`
	License             License         `json:"license"`
	LicenseVersion      string          `json:"license_version"`
	LicenseURL          string          `json:"license_url"`
	Provider            string          `json:"provider"`
	Source              string          `json:"source"`
	Category            string          `json:"category"`
	Filesize            *int64          `json:"filesize"`
	Filetype            string          `json:"filetype"`
	Tags                []Tag           `json:"tags"`
	Attribution         string          `json:"attribution"`
	FieldsMatched       []string        `json:"fields_matched"`
	Mature              bool            `json:"mature"`
	UnstableSensitivity json.RawMessage `json:"unstable__sensitivity"`
	Thumbnail           string          `json:"thumbnail"`
	DetailURL           string          `json:"detail_url"`
	RelatedURL          string          `json:"related_url"`
}

type Image struct {
	Media
	Height *int64          `json:"height"`
	Width  *int64          `json:"width"`
	Meta   ResponseMeta    `json:"-"`
	Raw    json.RawMessage `json:"-"`
}

func (value *Image) UnmarshalJSON(data []byte) error {
	type wire Image
	var decoded wire
	if err := decodeProviderObject(data, &decoded); err != nil {
		return err
	}
	*value = Image(decoded)
	value.Raw = append(value.Raw[:0], data...)
	return nil
}

type AudioFile struct {
	URL        string `json:"url"`
	BitRate    *int64 `json:"bit_rate"`
	Filesize   *int64 `json:"filesize"`
	Filetype   string `json:"filetype"`
	SampleRate *int64 `json:"sample_rate"`
}

type AudioSet struct {
	Title             string `json:"title"`
	ForeignLandingURL string `json:"foreign_landing_url"`
	Creator           string `json:"creator"`
	CreatorURL        string `json:"creator_url"`
	URL               string `json:"url"`
	Filesize          *int64 `json:"filesize"`
	Filetype          string `json:"filetype"`
}

type Audio struct {
	Media
	Genres     []string        `json:"genres"`
	AltFiles   []AudioFile     `json:"alt_files"`
	AudioSet   *AudioSet       `json:"audio_set"`
	Duration   *int64          `json:"duration"`
	BitRate    *int64          `json:"bit_rate"`
	SampleRate *int64          `json:"sample_rate"`
	Waveform   string          `json:"waveform"`
	Meta       ResponseMeta    `json:"-"`
	Raw        json.RawMessage `json:"-"`
}

func (value *Audio) UnmarshalJSON(data []byte) error {
	type wire Audio
	var decoded wire
	if err := decodeProviderObject(data, &decoded); err != nil {
		return err
	}
	*value = Audio(decoded)
	value.Raw = append(value.Raw[:0], data...)
	return nil
}

type ImageSearchResponse struct {
	Warnings    []json.RawMessage `json:"warnings"`
	ResultCount int64             `json:"result_count"`
	PageCount   int               `json:"page_count"`
	PageSize    int               `json:"page_size"`
	Page        int               `json:"page"`
	Results     []Image           `json:"results"`
	Meta        ResponseMeta      `json:"-"`
	Raw         json.RawMessage   `json:"-"`
}

func (value *ImageSearchResponse) UnmarshalJSON(data []byte) error {
	type wire ImageSearchResponse
	var decoded wire
	if err := decodeProviderObject(data, &decoded); err != nil {
		return err
	}
	*value = ImageSearchResponse(decoded)
	value.Raw = append(value.Raw[:0], data...)
	return nil
}

type AudioSearchResponse struct {
	Warnings    []json.RawMessage `json:"warnings"`
	ResultCount int64             `json:"result_count"`
	PageCount   int               `json:"page_count"`
	PageSize    int               `json:"page_size"`
	Page        int               `json:"page"`
	Results     []Audio           `json:"results"`
	Meta        ResponseMeta      `json:"-"`
	Raw         json.RawMessage   `json:"-"`
}

func (value *AudioSearchResponse) UnmarshalJSON(data []byte) error {
	type wire AudioSearchResponse
	var decoded wire
	if err := decodeProviderObject(data, &decoded); err != nil {
		return err
	}
	*value = AudioSearchResponse(decoded)
	value.Raw = append(value.Raw[:0], data...)
	return nil
}

func decodeProviderObject(data []byte, target any) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || len(trimmed) > maxProviderObjectBytes || trimmed[0] != '{' || !json.Valid(trimmed) {
		return fmt.Errorf("openverse: invalid provider object")
	}
	return json.Unmarshal(trimmed, target)
}
