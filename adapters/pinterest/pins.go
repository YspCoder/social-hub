package pinterest

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"sort"
	"strings"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

const maxUploadResponseBytes int64 = 1 << 20

// MediaSourceType is a supported Pinterest Pin media-source discriminator.
type MediaSourceType string

const (
	MediaSourceImageURL MediaSourceType = "image_url"
	MediaSourceVideoID  MediaSourceType = "video_id"
)

// PinMediaSource describes one stable organic Pin media source.
type PinMediaSource struct {
	SourceType             MediaSourceType `json:"source_type"`
	URL                    string          `json:"url,omitempty"`
	MediaID                string          `json:"media_id,omitempty"`
	CoverImageURL          string          `json:"cover_image_url,omitempty"`
	CoverImageContentType  string          `json:"cover_image_content_type,omitempty"`
	CoverImageData         string          `json:"cover_image_data,omitempty"`
	CoverImageKeyFrameTime *int            `json:"cover_image_key_frame_time,omitempty"`
	IsStandard             *bool           `json:"is_standard,omitempty"`
}

// PinCreateRequest contains the board and media fields that the common post
// request cannot express.
type PinCreateRequest struct {
	BoardID        string         `json:"board_id"`
	BoardSectionID string         `json:"board_section_id,omitempty"`
	Title          string         `json:"title,omitempty"`
	Description    string         `json:"description,omitempty"`
	AltText        string         `json:"alt_text,omitempty"`
	Link           string         `json:"link,omitempty"`
	DominantColor  string         `json:"dominant_color,omitempty"`
	MediaSource    PinMediaSource `json:"media_source"`
}

// VideoUploadStatus reports Pinterest's registered media lifecycle.
type VideoUploadStatus struct {
	MediaID   string               `json:"media_id"`
	MediaType string               `json:"media_type"`
	Status    string               `json:"status"`
	State     socialhub.MediaState `json:"state"`
}

// PinWorkflow exposes Pinterest's board-aware Pin and signed video upload flow.
type PinWorkflow interface {
	Create(context.Context, PinCreateRequest, ...socialhub.CallOption) (*socialhub.Post, error)
	Delete(context.Context, string, ...socialhub.CallOption) error
	RegisterVideo(context.Context, ...socialhub.CallOption) (*VideoUploadStatus, error)
	UploadVideo(context.Context, string, string, io.Reader, ...socialhub.CallOption) error
	MediaStatus(context.Context, string, ...socialhub.CallOption) (*VideoUploadStatus, error)
}

// PinService implements PinWorkflow.
type PinService struct{ client *Client }

func (s *PinService) Create(ctx context.Context, input PinCreateRequest, options ...socialhub.CallOption) (*socialhub.Post, error) {
	if err := s.client.requireScopes("pin_create", "boards:read", "boards:write", "pins:read", "pins:write"); err != nil {
		return nil, err
	}
	if err := validatePinCreate(input); err != nil {
		return nil, err
	}
	var response pinterestPin
	if err := s.client.transport.JSON(ctx, http.MethodPost, "/pins", nil, input, &response, options...); err != nil {
		return nil, err
	}
	if response.ID == "" {
		return nil, platformError("pin_create", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return mapPin(s.client.accountID, s.client.userID, response, s.client.clock.Now()), nil
}

func validatePinCreate(input PinCreateRequest) error {
	if !digitsOnly(input.BoardID) || (input.BoardSectionID != "" && !digitsOnly(input.BoardSectionID)) {
		return invalidArgument("pin_create", "numeric board_id is required and board_section_id must be numeric")
	}
	if utf8.RuneCountInString(input.Title) > 100 || utf8.RuneCountInString(input.Description) > 800 || utf8.RuneCountInString(input.AltText) > 500 {
		return invalidArgument("pin_create", "title, description, or alt text exceeds Pinterest limits")
	}
	if input.Link != "" && (!validHTTPURL(input.Link) || len(input.Link) > 2048) {
		return invalidArgument("pin_create", "link must be an absolute HTTP(S) URL no longer than 2048 bytes")
	}
	switch input.MediaSource.SourceType {
	case MediaSourceImageURL:
		if !validHTTPURL(input.MediaSource.URL) || input.MediaSource.MediaID != "" || input.MediaSource.CoverImageURL != "" || input.MediaSource.CoverImageData != "" || input.MediaSource.CoverImageContentType != "" || input.MediaSource.CoverImageKeyFrameTime != nil {
			return invalidArgument("pin_create", "image_url requires only an absolute image URL")
		}
	case MediaSourceVideoID:
		if !digitsOnly(input.MediaSource.MediaID) || input.MediaSource.URL != "" {
			return invalidArgument("pin_create", "video_id requires a numeric media ID and no image URL")
		}
		if input.MediaSource.CoverImageURL != "" && !validHTTPURL(input.MediaSource.CoverImageURL) {
			return invalidArgument("pin_create", "cover image URL must be absolute HTTP(S)")
		}
		if input.MediaSource.CoverImageKeyFrameTime != nil && *input.MediaSource.CoverImageKeyFrameTime < 0 {
			return invalidArgument("pin_create", "cover image key frame time cannot be negative")
		}
		if input.MediaSource.CoverImageData != "" && input.MediaSource.CoverImageContentType == "" {
			return invalidArgument("pin_create", "base64 cover image data requires a content type")
		}
	default:
		return invalidArgument("pin_create", "source_type must be image_url or video_id")
	}
	return nil
}

func (s *PinService) Delete(ctx context.Context, pinID string, options ...socialhub.CallOption) error {
	if !digitsOnly(pinID) {
		return invalidArgument("pin_delete", "numeric Pin ID is required")
	}
	if err := s.client.requireScopes("pin_delete", "pins:write"); err != nil {
		return err
	}
	return s.client.transport.JSON(ctx, http.MethodDelete, "/pins/"+url.PathEscape(pinID), nil, nil, nil, options...)
}

func (s *PinService) RegisterVideo(ctx context.Context, options ...socialhub.CallOption) (*VideoUploadStatus, error) {
	if err := s.client.requireScopes("video_register", "pins:write"); err != nil {
		return nil, err
	}
	var response mediaRegistration
	if err := s.client.transport.JSON(ctx, http.MethodPost, "/media", nil, map[string]string{"media_type": "video"}, &response, options...); err != nil {
		return nil, err
	}
	if !digitsOnly(response.MediaID) || response.MediaType != "video" || !validEndpoint(response.UploadURL) || len(response.UploadParameters) == 0 {
		return nil, platformError("video_register", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	s.client.uploadMu.Lock()
	s.client.uploads[response.MediaID] = &videoUpload{URL: response.UploadURL, Parameters: cloneStrings(response.UploadParameters)}
	s.client.uploadMu.Unlock()
	return &VideoUploadStatus{MediaID: response.MediaID, MediaType: response.MediaType, Status: "registered", State: socialhub.MediaStateCreated}, nil
}

func (s *PinService) UploadVideo(ctx context.Context, mediaID, filename string, content io.Reader, options ...socialhub.CallOption) error {
	if !digitsOnly(mediaID) || strings.TrimSpace(filename) == "" || strings.ContainsAny(filename, "\r\n") || content == nil {
		return invalidArgument("video_upload", "media ID, safe filename, and content are required")
	}
	callOptions, err := socialhub.ResolveCallOptions(options...)
	if err != nil {
		return err
	}
	if callOptions.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, callOptions.Timeout)
		defer cancel()
	}
	s.client.uploadMu.Lock()
	state := s.client.uploads[mediaID]
	if state == nil {
		s.client.uploadMu.Unlock()
		return platformError("video_upload", socialhub.CodeNotFound, socialhub.ClassPermanent, nil)
	}
	if state.Uploading || state.Used {
		s.client.uploadMu.Unlock()
		return platformError("video_upload", socialhub.CodeConflict, socialhub.ClassPermanent, nil)
	}
	state.Uploading = true
	uploadURL, parameters := state.URL, cloneStrings(state.Parameters)
	s.client.uploadMu.Unlock()

	pipeReader, pipeWriter := io.Pipe()
	writer := multipart.NewWriter(pipeWriter)
	writeDone := make(chan error, 1)
	go func() {
		writeDone <- writeUploadForm(writer, pipeWriter, parameters, filename, content)
	}()
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, uploadURL, pipeReader)
	if err != nil {
		_ = pipeReader.Close()
		s.setUploadResult(mediaID, false)
		return platformError("video_upload", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	request.Header.Set("Content-Type", writer.FormDataContentType())
	if callOptions.RequestID != "" {
		request.Header.Set("X-Request-ID", callOptions.RequestID)
	}
	response, err := s.client.httpClient.Do(request)
	if err != nil {
		_ = pipeReader.Close()
		<-writeDone
		s.setUploadResult(mediaID, false)
		return platformError("video_upload", socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, sanitizeUploadError(err))
	}
	defer response.Body.Close()
	body, readErr := io.ReadAll(io.LimitReader(response.Body, maxUploadResponseBytes+1))
	writeErr := <-writeDone
	if writeErr != nil || readErr != nil {
		s.setUploadResult(mediaID, false)
		return platformError("video_upload", socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, firstError(writeErr, readErr))
	}
	if int64(len(body)) > maxUploadResponseBytes {
		s.setUploadResult(mediaID, false)
		return platformError("video_upload", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		s.setUploadResult(mediaID, false)
		return decodeHTTPError(response.StatusCode, response.Header, body)
	}
	s.setUploadResult(mediaID, true)
	return nil
}

func writeUploadForm(writer *multipart.Writer, pipeWriter *io.PipeWriter, parameters map[string]string, filename string, content io.Reader) error {
	defer pipeWriter.Close()
	keys := make([]string, 0, len(parameters))
	for key := range parameters {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if err := writer.WriteField(key, parameters[key]); err != nil {
			_ = pipeWriter.CloseWithError(err)
			return err
		}
	}
	header := make(textproto.MIMEHeader)
	disposition := mime.FormatMediaType("form-data", map[string]string{"name": "file", "filename": filename})
	if disposition == "" {
		err := fmt.Errorf("pinterest: invalid upload filename")
		_ = pipeWriter.CloseWithError(err)
		return err
	}
	header.Set("Content-Disposition", disposition)
	header.Set("Content-Type", "application/octet-stream")
	part, err := writer.CreatePart(header)
	if err == nil {
		_, err = io.Copy(part, content)
	}
	if err == nil {
		err = writer.Close()
	}
	if err != nil {
		_ = pipeWriter.CloseWithError(err)
	}
	return err
}

func (s *PinService) setUploadResult(mediaID string, used bool) {
	s.client.uploadMu.Lock()
	defer s.client.uploadMu.Unlock()
	if state := s.client.uploads[mediaID]; state != nil {
		state.Uploading = false
		state.Used = used
		if used {
			state.URL = ""
			state.Parameters = nil
		}
	}
}

func (s *PinService) MediaStatus(ctx context.Context, mediaID string, options ...socialhub.CallOption) (*VideoUploadStatus, error) {
	if !digitsOnly(mediaID) {
		return nil, invalidArgument("media_status", "numeric media ID is required")
	}
	if err := s.client.requireScopes("media_status", "pins:read"); err != nil {
		return nil, err
	}
	var response mediaStatusResponse
	if err := s.client.transport.JSON(ctx, http.MethodGet, "/media/"+url.PathEscape(mediaID), nil, nil, &response, options...); err != nil {
		return nil, err
	}
	if response.MediaID != mediaID || response.MediaType != "video" {
		return nil, platformError("media_status", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	state := socialhub.MediaStateProcessing
	switch response.Status {
	case "registered":
		state = socialhub.MediaStateCreated
	case "succeeded":
		state = socialhub.MediaStateReady
	case "failed":
		state = socialhub.MediaStateFailed
	}
	return &VideoUploadStatus{MediaID: response.MediaID, MediaType: response.MediaType, Status: response.Status, State: state}, nil
}

func validHTTPURL(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host != "" && parsed.User == nil
}

func digitsOnly(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func cloneStrings(input map[string]string) map[string]string {
	copy := make(map[string]string, len(input))
	for key, value := range input {
		copy[key] = value
	}
	return copy
}

func sanitizeUploadError(err error) error {
	var urlError *url.Error
	if errors.As(err, &urlError) && urlError.Err != nil {
		return urlError.Err
	}
	return err
}

func firstError(values ...error) error {
	for _, value := range values {
		if value != nil {
			return value
		}
	}
	return nil
}

var _ PinWorkflow = (*PinService)(nil)
