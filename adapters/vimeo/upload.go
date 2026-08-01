package vimeo

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"social-hub/pkg/socialhub"
)

const (
	defaultTUSPartSize    int64 = 256 << 20
	maxVimeoVideoSize     int64 = 300_000_000_000
	maxUploadResponseSize int64 = 8 << 20
)

// VideoUploadRequest contains metadata for a new Vimeo video and TUS upload.
type VideoUploadRequest struct {
	Name        string
	Description string
	Size        int64
	PrivacyView string
}

// VideoUpdateRequest contains mutable Vimeo video metadata.
type VideoUpdateRequest struct {
	Name        *string
	Description *string
	PrivacyView *string
}

// VideoUploadSession identifies one in-process TUS upload.
type VideoUploadSession struct {
	ID       string
	VideoID  string
	Size     int64
	Offset   int64
	PartSize int64
}

// VideoUploadWorkflow exposes Vimeo's TUS video lifecycle.
type VideoUploadWorkflow interface {
	Initialize(context.Context, VideoUploadRequest, ...socialhub.CallOption) (*VideoUploadSession, error)
	UploadPart(context.Context, string, int, io.Reader, ...socialhub.CallOption) (*socialhub.UploadedPart, error)
	Complete(context.Context, string, []socialhub.UploadedPart, ...socialhub.CallOption) (*socialhub.Post, error)
	Status(context.Context, string, ...socialhub.CallOption) (*socialhub.Post, error)
	Update(context.Context, string, VideoUpdateRequest, ...socialhub.CallOption) (*socialhub.Post, error)
	Delete(context.Context, string, ...socialhub.CallOption) error
}

// VideoUploadService implements VideoUploadWorkflow.
type VideoUploadService struct{ client *Client }

type videoUpload struct {
	VideoID    string
	URL        string
	Size       int64
	PartSize   int64
	Offset     int64
	NextPart   int
	Parts      []socialhub.UploadedPart
	Uploading  bool
	Completing bool
}

func (s *VideoUploadService) Initialize(ctx context.Context, input VideoUploadRequest, options ...socialhub.CallOption) (*VideoUploadSession, error) {
	if strings.TrimSpace(input.Name) == "" || input.Size <= 0 || input.Size > maxVimeoVideoSize {
		return nil, invalidArgument("upload_initialize", "name and size between 1 byte and 300 GB are required")
	}
	if input.PrivacyView != "" && !validPrivacy(input.PrivacyView) {
		return nil, invalidArgument("upload_initialize", "privacy view is invalid")
	}
	if err := s.client.requireScopes("upload_initialize", "upload", "edit"); err != nil {
		return nil, err
	}
	body := struct {
		Name        string `json:"name"`
		Description string `json:"description,omitempty"`
		Upload      struct {
			Approach string `json:"approach"`
			Size     int64  `json:"size"`
		} `json:"upload"`
		Privacy *struct {
			View string `json:"view"`
		} `json:"privacy,omitempty"`
	}{Name: input.Name, Description: input.Description}
	body.Upload.Approach, body.Upload.Size = "tus", input.Size
	if input.PrivacyView != "" {
		body.Privacy = &struct {
			View string `json:"view"`
		}{View: input.PrivacyView}
	}
	var response vimeoVideo
	if err := s.client.requestJSON(ctx, http.MethodPost, "/me/videos", nil, body, &response, options...); err != nil {
		return nil, err
	}
	videoID, err := resourceID(response.URI, "videos")
	if err != nil || response.Upload.Approach != "tus" || !validTUSUploadURL(response.Upload.UploadLink, s.client.apiBaseURL) {
		return nil, platformError("upload_initialize", socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	partSize := min(input.Size, defaultTUSPartSize)
	digest := sha256.Sum256([]byte(response.Upload.UploadLink))
	sessionID := hex.EncodeToString(digest[:])
	state := &videoUpload{VideoID: videoID, URL: response.Upload.UploadLink, Size: input.Size, PartSize: partSize}
	s.client.uploadMu.Lock()
	s.client.uploads[sessionID] = state
	s.client.uploadMu.Unlock()
	return &VideoUploadSession{ID: sessionID, VideoID: videoID, Size: input.Size, PartSize: partSize}, nil
}

func (s *VideoUploadService) UploadPart(ctx context.Context, sessionID string, partNumber int, reader io.Reader, options ...socialhub.CallOption) (*socialhub.UploadedPart, error) {
	if sessionID == "" || partNumber < 0 || reader == nil {
		return nil, invalidArgument("upload_part", "session ID, non-negative part number, and reader are required")
	}
	callOptions, err := socialhub.ResolveCallOptions(options...)
	if err != nil {
		return nil, err
	}
	if callOptions.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, callOptions.Timeout)
		defer cancel()
	}
	s.client.uploadMu.Lock()
	state := s.client.uploads[sessionID]
	if state == nil {
		s.client.uploadMu.Unlock()
		return nil, platformError("upload_part", socialhub.CodeNotFound, socialhub.ClassPermanent, nil)
	}
	if state.Uploading || state.Completing {
		s.client.uploadMu.Unlock()
		return nil, platformError("upload_part", socialhub.CodeConflict, socialhub.ClassPermanent, nil)
	}
	if partNumber != state.NextPart {
		s.client.uploadMu.Unlock()
		return nil, invalidArgument("upload_part", "part number is not the next expected part")
	}
	if state.Offset >= state.Size {
		s.client.uploadMu.Unlock()
		return nil, platformError("upload_part", socialhub.CodeConflict, socialhub.ClassPermanent, nil)
	}
	state.Uploading = true
	uploadURL, oldOffset := state.URL, state.Offset
	maximum := min(state.PartSize, state.Size-state.Offset)
	s.client.uploadMu.Unlock()

	counting := &countingReader{reader: io.LimitReader(reader, maximum)}
	request, err := http.NewRequestWithContext(ctx, http.MethodPatch, uploadURL, counting)
	if err != nil {
		s.setUploadIdle(sessionID)
		return nil, platformError("upload_part", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	request.Header.Set("Tus-Resumable", "1.0.0")
	request.Header.Set("Upload-Offset", strconv.FormatInt(oldOffset, 10))
	request.Header.Set("Content-Type", "application/offset+octet-stream")
	if callOptions.RequestID != "" {
		request.Header.Set("X-Request-ID", callOptions.RequestID)
	}
	if length := readerLength(reader, maximum); length >= 0 {
		request.ContentLength = length
	}
	uploadClient := *s.client.httpClient
	uploadClient.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	response, err := uploadClient.Do(request)
	if err != nil {
		s.setUploadIdle(sessionID)
		return nil, platformError("upload_part", socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, sanitizeUploadError(err))
	}
	defer response.Body.Close()
	body, readErr := io.ReadAll(io.LimitReader(response.Body, maxUploadResponseSize+1))
	if readErr != nil {
		s.setUploadIdle(sessionID)
		return nil, platformError("upload_part", socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, readErr)
	}
	if int64(len(body)) > maxUploadResponseSize {
		s.setUploadIdle(sessionID)
		return nil, platformError("upload_part", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		s.setUploadIdle(sessionID)
		return nil, decodeHTTPError(response.StatusCode, response.Header, body)
	}
	if counting.count <= 0 {
		s.setUploadIdle(sessionID)
		return nil, invalidArgument("upload_part", "upload part must contain bytes")
	}
	newOffset, err := strconv.ParseInt(response.Header.Get("Upload-Offset"), 10, 64)
	if err != nil || newOffset != oldOffset+counting.count || newOffset > oldOffset+maximum {
		s.setUploadIdle(sessionID)
		return nil, platformError("upload_part", socialhub.CodePlatformError, socialhub.ClassPermanent, errors.New("invalid TUS Upload-Offset response"))
	}
	part := socialhub.UploadedPart{Number: partNumber, ETag: firstNonEmpty(response.Header.Get("ETag"), strconv.FormatInt(newOffset, 10)), Size: counting.count}
	s.client.uploadMu.Lock()
	state = s.client.uploads[sessionID]
	if state == nil || !state.Uploading || state.Offset != oldOffset {
		s.client.uploadMu.Unlock()
		return nil, platformError("upload_part", socialhub.CodeConflict, socialhub.ClassPermanent, nil)
	}
	state.Offset, state.NextPart, state.Uploading = newOffset, state.NextPart+1, false
	state.Parts = append(state.Parts, part)
	s.client.uploadMu.Unlock()
	return &part, nil
}

func (s *VideoUploadService) Complete(ctx context.Context, sessionID string, parts []socialhub.UploadedPart, options ...socialhub.CallOption) (*socialhub.Post, error) {
	if sessionID == "" {
		return nil, invalidArgument("upload_complete", "session ID is required")
	}
	s.client.uploadMu.Lock()
	state := s.client.uploads[sessionID]
	if state == nil {
		s.client.uploadMu.Unlock()
		return nil, platformError("upload_complete", socialhub.CodeNotFound, socialhub.ClassPermanent, nil)
	}
	if state.Uploading || state.Completing {
		s.client.uploadMu.Unlock()
		return nil, platformError("upload_complete", socialhub.CodeConflict, socialhub.ClassPermanent, nil)
	}
	if state.Offset != state.Size || !slices.Equal(parts, state.Parts) {
		s.client.uploadMu.Unlock()
		return nil, invalidArgument("upload_complete", "all accepted parts must be supplied after the declared size is uploaded")
	}
	state.Completing = true
	videoID := state.VideoID
	s.client.uploadMu.Unlock()
	post, err := s.client.GetPost(ctx, videoID, options...)
	if err != nil {
		s.setCompleteIdle(sessionID)
		return nil, err
	}
	s.client.uploadMu.Lock()
	delete(s.client.uploads, sessionID)
	s.client.uploadMu.Unlock()
	return post, nil
}

func (s *VideoUploadService) Status(ctx context.Context, videoID string, options ...socialhub.CallOption) (*socialhub.Post, error) {
	return s.client.GetPost(ctx, videoID, options...)
}

func (s *VideoUploadService) Update(ctx context.Context, videoID string, input VideoUpdateRequest, options ...socialhub.CallOption) (*socialhub.Post, error) {
	if !validResourceID(videoID) {
		return nil, invalidArgument("video_update", "video ID is required and must be valid")
	}
	if input.Name == nil && input.Description == nil && input.PrivacyView == nil {
		return nil, invalidArgument("video_update", "at least one mutable field is required")
	}
	if input.Name != nil && strings.TrimSpace(*input.Name) == "" {
		return nil, invalidArgument("video_update", "name must not be empty")
	}
	if input.PrivacyView != nil && !validPrivacy(*input.PrivacyView) {
		return nil, invalidArgument("video_update", "privacy view is invalid")
	}
	if err := s.client.requireScopes("video_update", "edit"); err != nil {
		return nil, err
	}
	body := struct {
		Name        *string `json:"name,omitempty"`
		Description *string `json:"description,omitempty"`
		Privacy     *struct {
			View string `json:"view"`
		} `json:"privacy,omitempty"`
	}{Name: input.Name, Description: input.Description}
	if input.PrivacyView != nil {
		body.Privacy = &struct {
			View string `json:"view"`
		}{View: *input.PrivacyView}
	}
	var response vimeoVideo
	if err := s.client.requestJSON(ctx, http.MethodPatch, "/videos/"+escapedID(videoID), nil, body, &response, options...); err != nil {
		return nil, err
	}
	post, err := s.client.mapVideo(response)
	if err != nil {
		return nil, err
	}
	if post.ID != videoID {
		return nil, platformError("video_update", socialhub.CodePlatformError, socialhub.ClassPermanent, errors.New("response video ID mismatch"))
	}
	return post, nil
}

func (s *VideoUploadService) Delete(ctx context.Context, videoID string, options ...socialhub.CallOption) error {
	if !validResourceID(videoID) {
		return invalidArgument("video_delete", "video ID is required and must be valid")
	}
	if err := s.client.requireScopes("video_delete", "delete"); err != nil {
		return err
	}
	return s.client.requestJSON(ctx, http.MethodDelete, "/videos/"+escapedID(videoID), nil, nil, nil, options...)
}

func validTUSUploadURL(value string, apiBase *url.URL) bool {
	parsed, err := url.Parse(value)
	if err != nil || parsed.Host == "" || parsed.User != nil || parsed.Fragment != "" {
		return false
	}
	hostname := strings.ToLower(parsed.Hostname())
	if hostname == "vimeo.com" || strings.HasSuffix(hostname, ".vimeo.com") {
		return parsed.Scheme == "https"
	}
	return apiBase != nil && parsed.Scheme == apiBase.Scheme && parsed.Host == apiBase.Host &&
		(parsed.Scheme == "http" || parsed.Scheme == "https")
}

func validPrivacy(value string) bool {
	switch value {
	case "anybody", "contacts", "disable", "nobody", "password", "unlisted", "users":
		return true
	default:
		return false
	}
}

func readerLength(reader io.Reader, maximum int64) int64 {
	type lenReader interface{ Len() int }
	if value, ok := reader.(lenReader); ok {
		return min(int64(value.Len()), maximum)
	}
	return -1
}

func (s *VideoUploadService) setUploadIdle(sessionID string) {
	s.client.uploadMu.Lock()
	defer s.client.uploadMu.Unlock()
	if state := s.client.uploads[sessionID]; state != nil {
		state.Uploading = false
	}
}

func (s *VideoUploadService) setCompleteIdle(sessionID string) {
	s.client.uploadMu.Lock()
	defer s.client.uploadMu.Unlock()
	if state := s.client.uploads[sessionID]; state != nil {
		state.Completing = false
	}
}

func sanitizeUploadError(err error) error {
	var urlError *url.Error
	if errors.As(err, &urlError) && urlError.Err != nil {
		return urlError.Err
	}
	return err
}

type countingReader struct {
	reader io.Reader
	count  int64
}

func (r *countingReader) Read(buffer []byte) (int, error) {
	n, err := r.reader.Read(buffer)
	r.count += int64(n)
	return n, err
}

var _ VideoUploadWorkflow = (*VideoUploadService)(nil)
