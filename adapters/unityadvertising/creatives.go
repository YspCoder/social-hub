package unityadvertising

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"social-hub/pkg/socialhub"
)

const (
	maxImageCreativeBytes    int64 = 5 << 20
	maxVideoCreativeBytes    int64 = 100 << 20
	maxPlayableCreativeBytes int64 = 5 << 20
)

var errCreativeSizeMismatch = errors.New("creative file byte count does not match declared size")

type CreativePreviewFile struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

type Creative struct {
	ID        string                `json:"id"`
	Name      string                `json:"name"`
	Language  CreativeLanguage      `json:"language"`
	Type      CreativeType          `json:"type"`
	Files     []CreativePreviewFile `json:"files"`
	CreatedAt *time.Time            `json:"createdAt"`
	Status    CreativeStatus        `json:"status"`
	Raw       json.RawMessage       `json:"-"`
}

type ListCreativesRequest struct {
	Offset int64
	Limit  int
}

type CreativeUpload interface {
	isCreativeUpload()
}

type SquareEndCardUpload struct {
	Name     string
	Language CreativeLanguage
	Filename string
	Size     int64
	File     io.Reader
}

func (SquareEndCardUpload) isCreativeUpload() {}

type EndCardPairUpload struct {
	Name              string
	Language          CreativeLanguage
	PortraitFilename  string
	PortraitSize      int64
	PortraitFile      io.Reader
	LandscapeFilename string
	LandscapeSize     int64
	LandscapeFile     io.Reader
}

func (EndCardPairUpload) isCreativeUpload() {}

type VideoUpload struct {
	Name     string
	Language CreativeLanguage
	Filename string
	Size     int64
	File     io.Reader
}

func (VideoUpload) isCreativeUpload() {}

type PlayableOrientation string

const (
	PlayableLandscape PlayableOrientation = "landscape"
	PlayablePortrait  PlayableOrientation = "portrait"
	PlayableBoth      PlayableOrientation = "both"
)

type PlayableUpload struct {
	Name        string
	Language    CreativeLanguage
	Filename    string
	Orientation PlayableOrientation
	Size        int64
	File        io.Reader
}

func (PlayableUpload) isCreativeUpload() {}

type CreativesWorkflow interface {
	ListCreatives(context.Context, string, ListCreativesRequest, ...socialhub.CallOption) (Page[Creative], error)
	CreateCreative(context.Context, string, CreativeUpload, ...socialhub.CallOption) (*Creative, error)
	GetCreative(context.Context, string, string, ...socialhub.CallOption) (*Creative, error)
}

func (client *Client) ListCreatives(ctx context.Context, campaignSetID string, input ListCreativesRequest, options ...socialhub.CallOption) (Page[Creative], error) {
	appPath, err := client.appPath("creative_list", campaignSetID)
	if err != nil {
		return Page[Creative]{}, err
	}
	if input.Offset < 0 || input.Limit < 0 || input.Limit > 2000 {
		return Page[Creative]{}, invalidArgument("creative_list", "offset must be non-negative and limit must be between 1 and 2000 when set")
	}
	query := make(url.Values)
	if input.Offset > 0 {
		query.Set("offset", formatInt64(input.Offset))
	}
	if input.Limit > 0 {
		query.Set("limit", formatInt(input.Limit))
	}
	var page Page[Creative]
	if err := client.getJSON(ctx, "creative_list", appPath+"/creatives", query, &page, options...); err != nil {
		return Page[Creative]{}, err
	}
	if !validPage(page, 2000, validCreative) {
		return Page[Creative]{}, platformContractError("creative_list", "Unity returned an invalid creative page")
	}
	return page, nil
}

func (client *Client) GetCreative(ctx context.Context, campaignSetID, creativeID string, options ...socialhub.CallOption) (*Creative, error) {
	path, err := client.creativePath("creative_get", campaignSetID, creativeID)
	if err != nil {
		return nil, err
	}
	var creative Creative
	if err := client.getJSON(ctx, "creative_get", path, nil, &creative, options...); err != nil {
		return nil, err
	}
	if !validCreative(creative) || creative.ID != creativeID {
		return nil, platformContractError("creative_get", "Unity returned a creative that does not match the requested ID")
	}
	return &creative, nil
}

func (client *Client) CreateCreative(ctx context.Context, campaignSetID string, input CreativeUpload, options ...socialhub.CallOption) (*Creative, error) {
	appPath, err := client.appPath("creative_create", campaignSetID)
	if err != nil {
		return nil, err
	}
	metadata, parts, err := creativeUploadParts(input)
	if err != nil {
		return nil, err
	}
	pipeReader, pipeWriter := io.Pipe()
	multipartWriter := multipart.NewWriter(pipeWriter)
	request, err := client.api.NewRequest(ctx, http.MethodPost, appPath+"/creatives", nil, pipeReader, options...)
	if err != nil {
		_ = pipeReader.Close()
		_ = pipeWriter.Close()
		return nil, withOperation(err, "creative_create")
	}
	request.Header.Set("Content-Type", multipartWriter.FormDataContentType())
	done := make(chan error, 1)
	go func() {
		writeErr := writeCreativeMultipart(multipartWriter, metadata, parts)
		if writeErr == nil {
			writeErr = multipartWriter.Close()
		}
		_ = pipeWriter.CloseWithError(writeErr)
		done <- writeErr
	}()
	var creative Creative
	requestErr := withOperation(client.api.Do(request, &creative), "creative_create")
	_ = pipeReader.CloseWithError(requestErr)
	writeErr := <-done
	if errors.Is(writeErr, errCreativeSizeMismatch) {
		return nil, invalidArgument("creative_create", writeErr.Error())
	}
	if requestErr != nil {
		return nil, requestErr
	}
	if writeErr != nil && !errors.Is(writeErr, io.ErrClosedPipe) {
		return nil, platformError("creative_create", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, writeErr)
	}
	if !validCreative(creative) {
		return nil, platformContractError("creative_create", "Unity returned an invalid creative")
	}
	return &creative, nil
}

type creativeUploadPart struct {
	field     string
	filename  string
	mediaType string
	size      int64
	reader    io.Reader
}

func creativeUploadParts(input CreativeUpload) (any, []creativeUploadPart, error) {
	switch typed := input.(type) {
	case SquareEndCardUpload:
		if err := validateCreativeCommon(typed.Name, typed.Language); err != nil || typed.File == nil || typed.Size <= 0 || typed.Size > maxImageCreativeBytes ||
			!validUploadFilename(typed.Filename, ".jpg", ".jpeg", ".png", ".gif") {
			return nil, nil, invalidArgument("creative_create", "square end card metadata or file is invalid")
		}
		metadata := map[string]any{"name": typed.Name, "language": typed.Language, "squareEndCard": map[string]string{"fileName": typed.Filename}}
		return metadata, []creativeUploadPart{{"squareEndCardFile", typed.Filename, mediaTypeForFilename(typed.Filename), typed.Size, typed.File}}, nil
	case *SquareEndCardUpload:
		if typed == nil {
			return nil, nil, invalidArgument("creative_create", "creative upload is required")
		}
		return creativeUploadParts(*typed)
	case EndCardPairUpload:
		if err := validateCreativeCommon(typed.Name, typed.Language); err != nil || typed.PortraitFile == nil || typed.LandscapeFile == nil ||
			typed.PortraitSize <= 0 || typed.PortraitSize > maxImageCreativeBytes || typed.LandscapeSize <= 0 || typed.LandscapeSize > maxImageCreativeBytes ||
			!validUploadFilename(typed.PortraitFilename, ".jpg", ".jpeg", ".png", ".gif") ||
			!validUploadFilename(typed.LandscapeFilename, ".jpg", ".jpeg", ".png", ".gif") {
			return nil, nil, invalidArgument("creative_create", "end card pair metadata or files are invalid")
		}
		metadata := map[string]any{
			"name": typed.Name, "language": typed.Language,
			"portraitEndCard":  map[string]string{"fileName": typed.PortraitFilename},
			"landscapeEndCard": map[string]string{"fileName": typed.LandscapeFilename},
		}
		return metadata, []creativeUploadPart{
			{"portraitEndCardFile", typed.PortraitFilename, mediaTypeForFilename(typed.PortraitFilename), typed.PortraitSize, typed.PortraitFile},
			{"landscapeEndCardFile", typed.LandscapeFilename, mediaTypeForFilename(typed.LandscapeFilename), typed.LandscapeSize, typed.LandscapeFile},
		}, nil
	case *EndCardPairUpload:
		if typed == nil {
			return nil, nil, invalidArgument("creative_create", "creative upload is required")
		}
		return creativeUploadParts(*typed)
	case VideoUpload:
		if err := validateCreativeCommon(typed.Name, typed.Language); err != nil || typed.File == nil || typed.Size <= 0 || typed.Size > maxVideoCreativeBytes ||
			!validUploadFilename(typed.Filename, ".mp4") {
			return nil, nil, invalidArgument("creative_create", "video creative metadata or file is invalid")
		}
		metadata := map[string]any{"name": typed.Name, "language": typed.Language, "video": map[string]string{"fileName": typed.Filename}}
		return metadata, []creativeUploadPart{{"videoFile", typed.Filename, "video/mp4", typed.Size, typed.File}}, nil
	case *VideoUpload:
		if typed == nil {
			return nil, nil, invalidArgument("creative_create", "creative upload is required")
		}
		return creativeUploadParts(*typed)
	case PlayableUpload:
		if err := validateCreativeCommon(typed.Name, typed.Language); err != nil || typed.File == nil || typed.Size <= 0 || typed.Size > maxPlayableCreativeBytes ||
			!validUploadFilename(typed.Filename, ".html", ".htm") || !validPlayableOrientation(typed.Orientation) {
			return nil, nil, invalidArgument("creative_create", "playable creative metadata or file is invalid")
		}
		metadata := map[string]any{
			"name": typed.Name, "language": typed.Language,
			"playable": map[string]any{"fileName": typed.Filename, "orientation": typed.Orientation},
		}
		return metadata, []creativeUploadPart{{"playableFile", typed.Filename, "text/html", typed.Size, typed.File}}, nil
	case *PlayableUpload:
		if typed == nil {
			return nil, nil, invalidArgument("creative_create", "creative upload is required")
		}
		return creativeUploadParts(*typed)
	default:
		return nil, nil, invalidArgument("creative_create", "creative upload type is unsupported")
	}
}

func validateCreativeCommon(name string, language CreativeLanguage) error {
	if !validText(name, 255) || !validCreativeLanguage(language) {
		return invalidArgument("creative_create", "creative name or language is invalid")
	}
	return nil
}

func validPlayableOrientation(value PlayableOrientation) bool {
	return value == PlayableLandscape || value == PlayablePortrait || value == PlayableBoth
}

func writeCreativeMultipart(writer *multipart.Writer, metadata any, parts []creativeUploadPart) error {
	encoded, err := json.Marshal(metadata)
	if err != nil {
		return err
	}
	metadataHeader := make(textproto.MIMEHeader)
	metadataHeader.Set("Content-Disposition", mime.FormatMediaType("form-data", map[string]string{"name": "creativeInfo"}))
	metadataHeader.Set("Content-Type", "application/json")
	metadataPart, err := writer.CreatePart(metadataHeader)
	if err != nil {
		return err
	}
	if _, err := metadataPart.Write(encoded); err != nil {
		return err
	}
	sort.Slice(parts, func(left, right int) bool { return parts[left].field < parts[right].field })
	for _, item := range parts {
		header := make(textproto.MIMEHeader)
		header.Set("Content-Disposition", mime.FormatMediaType("form-data", map[string]string{"name": item.field, "filename": item.filename}))
		header.Set("Content-Type", item.mediaType)
		part, err := writer.CreatePart(header)
		if err != nil {
			return err
		}
		written, err := io.Copy(part, io.LimitReader(item.reader, item.size+1))
		if err != nil {
			return err
		}
		if written != item.size {
			return fmt.Errorf("%w for %s: got %d, want %d", errCreativeSizeMismatch, item.field, written, item.size)
		}
	}
	return nil
}

func mediaTypeForFilename(filename string) string {
	if mediaType := mime.TypeByExtension(strings.ToLower(filepath.Ext(filename))); mediaType != "" {
		return mediaType
	}
	return "application/octet-stream"
}

func validCreative(creative Creative) bool {
	return validMongoID(creative.ID) && (creative.Name == "" || validText(creative.Name, 255))
}

func (creative *Creative) UnmarshalJSON(data []byte) error {
	return captureRaw(data, (*creativeAlias)(creative), &creative.Raw)
}

type creativeAlias Creative
