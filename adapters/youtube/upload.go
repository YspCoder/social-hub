package youtube

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"social-hub/pkg/socialhub"
)

const maxUploadResponseBytes int64 = 8 << 20

// VideoUploadRequest contains metadata required by videos.insert.
type VideoUploadRequest struct {
	Title                   string
	Description             string
	Tags                    []string
	CategoryID              string
	PrivacyStatus           string
	SelfDeclaredMadeForKids bool
	ContainsSyntheticMedia  bool
	MIME                    string
	Size                    int64
}

// VideoUploadSession identifies one in-process resumable upload session.
type VideoUploadSession struct {
	ID   string
	MIME string
	Size int64
}

// VideoUploadWorkflow exposes YouTube's metadata plus media upload lifecycle.
type VideoUploadWorkflow interface {
	Initialize(context.Context, VideoUploadRequest, ...socialhub.CallOption) (*VideoUploadSession, error)
	Upload(context.Context, string, io.Reader, ...socialhub.CallOption) (*socialhub.Post, error)
	Status(context.Context, string, ...socialhub.CallOption) (*socialhub.Post, error)
	Delete(context.Context, string, ...socialhub.CallOption) error
}

// VideoUploadService implements VideoUploadWorkflow.
type VideoUploadService struct{ client *Client }

type videoUpload struct {
	URL       string
	MIME      string
	Size      int64
	Uploading bool
}

func (s *VideoUploadService) Initialize(ctx context.Context, input VideoUploadRequest, options ...socialhub.CallOption) (*VideoUploadSession, error) {
	if strings.TrimSpace(input.Title) == "" || input.Size <= 0 || !strings.HasPrefix(input.MIME, "video/") {
		return nil, invalidArgument("upload_initialize", "title, positive size, and video MIME are required")
	}
	privacy := input.PrivacyStatus
	if privacy == "" {
		privacy = "private"
	}
	if privacy != "private" && privacy != "public" && privacy != "unlisted" {
		return nil, invalidArgument("upload_initialize", "privacy status must be private, public, or unlisted")
	}
	if err := s.client.requireScope("upload_initialize", "https://www.googleapis.com/auth/youtube.upload"); err != nil {
		return nil, err
	}
	body := struct {
		Snippet struct {
			Title       string   `json:"title"`
			Description string   `json:"description,omitempty"`
			Tags        []string `json:"tags,omitempty"`
			CategoryID  string   `json:"categoryId,omitempty"`
		} `json:"snippet"`
		Status struct {
			PrivacyStatus           string `json:"privacyStatus"`
			SelfDeclaredMadeForKids bool   `json:"selfDeclaredMadeForKids"`
			ContainsSyntheticMedia  bool   `json:"containsSyntheticMedia"`
		} `json:"status"`
	}{}
	body.Snippet.Title, body.Snippet.Description, body.Snippet.Tags, body.Snippet.CategoryID = input.Title, input.Description, append([]string(nil), input.Tags...), input.CategoryID
	body.Status.PrivacyStatus, body.Status.SelfDeclaredMadeForKids, body.Status.ContainsSyntheticMedia = privacy, input.SelfDeclaredMadeForKids, input.ContainsSyntheticMedia
	encoded, err := json.Marshal(body)
	if err != nil {
		return nil, platformError("upload_initialize", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	query := url.Values{"uploadType": {"resumable"}, "part": {"snippet,status"}}
	request, err := s.client.uploadTransport.NewRequest(ctx, http.MethodPost, "/videos", query, bytes.NewReader(encoded), options...)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/json; charset=UTF-8")
	request.Header.Set("X-Upload-Content-Length", strconvFormatInt(input.Size))
	request.Header.Set("X-Upload-Content-Type", input.MIME)
	metadata, err := s.client.uploadTransport.DoWithMetadata(request, nil)
	if err != nil {
		return nil, err
	}
	location := metadata.Header.Get("Location")
	if !validEndpoint(location) {
		return nil, platformError("upload_initialize", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	digest := sha256.Sum256([]byte(location))
	sessionID := hex.EncodeToString(digest[:])
	s.client.uploadMu.Lock()
	s.client.uploads[sessionID] = &videoUpload{URL: location, MIME: input.MIME, Size: input.Size}
	s.client.uploadMu.Unlock()
	return &VideoUploadSession{ID: sessionID, MIME: input.MIME, Size: input.Size}, nil
}

func (s *VideoUploadService) Upload(ctx context.Context, sessionID string, reader io.Reader, options ...socialhub.CallOption) (*socialhub.Post, error) {
	if sessionID == "" || reader == nil {
		return nil, invalidArgument("video_upload", "session ID and reader are required")
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
		return nil, platformError("video_upload", socialhub.CodeNotFound, socialhub.ClassPermanent, nil)
	}
	if state.Uploading {
		s.client.uploadMu.Unlock()
		return nil, platformError("video_upload", socialhub.CodeConflict, socialhub.ClassPermanent, nil)
	}
	state.Uploading = true
	uploadURL, mimeType, expectedSize := state.URL, state.MIME, state.Size
	s.client.uploadMu.Unlock()
	counting := &countingReader{reader: reader}
	request, err := http.NewRequestWithContext(ctx, http.MethodPut, uploadURL, counting)
	if err != nil {
		s.setUploadIdle(sessionID)
		return nil, platformError("video_upload", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	request.Header.Set("Authorization", "Bearer "+s.client.accessToken)
	request.Header.Set("Content-Type", mimeType)
	if callOptions.RequestID != "" {
		request.Header.Set("X-Request-ID", callOptions.RequestID)
	}
	request.ContentLength = expectedSize
	response, err := s.client.httpClient.Do(request)
	if err != nil {
		s.setUploadIdle(sessionID)
		return nil, platformError("video_upload", socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, sanitizeUploadError(err))
	}
	defer response.Body.Close()
	body, readErr := io.ReadAll(io.LimitReader(response.Body, maxUploadResponseBytes+1))
	if readErr != nil {
		s.setUploadIdle(sessionID)
		return nil, platformError("video_upload", socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, readErr)
	}
	if int64(len(body)) > maxUploadResponseBytes {
		s.setUploadIdle(sessionID)
		return nil, platformError("video_upload", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		s.setUploadIdle(sessionID)
		return nil, decodeHTTPError(response.StatusCode, response.Header, body)
	}
	if counting.count != expectedSize {
		s.setUploadIdle(sessionID)
		return nil, invalidArgument("video_upload", "uploaded byte count does not match declared size")
	}
	var video youtubeVideo
	if err := json.Unmarshal(body, &video); err != nil || video.ID == "" {
		s.setUploadIdle(sessionID)
		return nil, platformError("video_upload", socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	s.client.uploadMu.Lock()
	delete(s.client.uploads, sessionID)
	s.client.uploadMu.Unlock()
	return mapVideo(s.client.accountID, video, s.client.clock.Now()), nil
}

func (s *VideoUploadService) Status(ctx context.Context, videoID string, options ...socialhub.CallOption) (*socialhub.Post, error) {
	return s.client.GetPost(ctx, videoID, options...)
}

func (s *VideoUploadService) Delete(ctx context.Context, videoID string, options ...socialhub.CallOption) error {
	if videoID == "" {
		return invalidArgument("video_delete", "video ID is required")
	}
	if err := s.client.requireScope("video_delete", "https://www.googleapis.com/auth/youtube.force-ssl"); err != nil {
		return err
	}
	return s.client.transport.JSON(ctx, http.MethodDelete, "/videos", url.Values{"id": {videoID}}, nil, nil, options...)
}

func (s *VideoUploadService) setUploadIdle(sessionID string) {
	s.client.uploadMu.Lock()
	defer s.client.uploadMu.Unlock()
	if state := s.client.uploads[sessionID]; state != nil {
		state.Uploading = false
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

func strconvFormatInt(value int64) string {
	return strconv.FormatInt(value, 10)
}

var _ VideoUploadWorkflow = (*VideoUploadService)(nil)
