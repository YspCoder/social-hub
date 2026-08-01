package bilibili

import (
	"bytes"
	"context"
	"crypto/md5"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"strings"

	"social-hub/extensions/video"
	"social-hub/pkg/socialhub"
)

const (
	maximumSingleVideoBytes int64 = 100_000_000
	maximumCoverBytes       int64 = 5_000_000
	maximumUploadResponse   int64 = 1 << 20
)

type uploadState struct {
	request    socialhub.BeginUploadRequest
	data       []byte
	media      *socialhub.Media
	part       *socialhub.UploadedPart
	uploading  bool
	completing bool
	completed  bool
}

type cancelContextKey struct{}

func (c *Client) BeginUpload(ctx context.Context, input socialhub.BeginUploadRequest, options ...socialhub.CallOption) (*socialhub.UploadSession, error) {
	if err := c.requireScope("begin_upload", "ARC_BASE"); err != nil {
		return nil, err
	}
	if input.Filename == "" || input.Size <= 0 || input.MIME == "" {
		return nil, invalidArgument("begin_upload", "filename, positive size, and MIME type are required")
	}
	state := &uploadState{request: input}
	var sessionID string
	switch input.Type {
	case socialhub.MediaTypeVideo:
		if !strings.HasPrefix(input.MIME, "video/") {
			return nil, invalidArgument("begin_upload", "video MIME type must be a video")
		}
		if input.Size > maximumSingleVideoBytes {
			return nil, &socialhub.Error{Code: socialhub.CodeUnsupported, Class: socialhub.ClassPermanent, Platform: "bilibili", Product: "open-platform", Op: "begin_upload", PlatformMessage: "files above 100 MB require Bilibili's separate fragment upload workflow, which is not included in the initial adapter"}
		}
		var response responseEnvelope[struct {
			UploadToken string `json:"upload_token"`
		}]
		body := map[string]string{"name": input.Filename, "utype": "1"}
		if err := c.transport.JSON(ctx, http.MethodPost, "/arcopen/fn/archive/video/init", nil, body, &response, options...); err != nil {
			return nil, err
		}
		if err := response.Err("begin_upload", http.StatusOK, nil); err != nil {
			return nil, err
		}
		if response.Data.UploadToken == "" {
			return nil, wrapError("begin_upload", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
		}
		sessionID = response.Data.UploadToken
	case socialhub.MediaTypeImage:
		if input.MIME != "image/jpeg" && input.MIME != "image/png" {
			return nil, invalidArgument("begin_upload", "cover must be JPEG or PNG")
		}
		if input.Size > maximumCoverBytes {
			return nil, invalidArgument("begin_upload", "cover must not exceed 5 MB")
		}
		generated, err := randomID("cover:")
		if err != nil {
			return nil, err
		}
		sessionID = generated
	default:
		return nil, &socialhub.Error{Code: socialhub.CodeUnsupported, Class: socialhub.ClassPermanent, Platform: "bilibili", Product: "open-platform", Op: "begin_upload", PlatformMessage: "archive publication accepts video and cover image media only"}
	}
	c.uploadMu.Lock()
	defer c.uploadMu.Unlock()
	if _, exists := c.uploads[sessionID]; exists {
		return nil, wrapError("begin_upload", socialhub.CodeConflict, socialhub.ClassPermanent, nil)
	}
	c.uploads[sessionID] = state
	return &socialhub.UploadSession{ID: sessionID, PartSize: input.Size}, nil
}

func (c *Client) UploadPart(ctx context.Context, sessionID string, partNumber int, reader io.Reader, options ...socialhub.CallOption) (*socialhub.UploadedPart, error) {
	if sessionID == "" || partNumber != 0 || reader == nil {
		return nil, invalidArgument("upload_part", "session ID, part number 0, and reader are required")
	}
	c.uploadMu.Lock()
	state := c.uploads[sessionID]
	if state == nil {
		c.uploadMu.Unlock()
		return nil, wrapError("upload_part", socialhub.CodeNotFound, socialhub.ClassPermanent, nil)
	}
	if state.uploading || state.completing || state.completed || state.part != nil {
		c.uploadMu.Unlock()
		return nil, wrapError("upload_part", socialhub.CodeConflict, socialhub.ClassPermanent, nil)
	}
	state.uploading = true
	requestInfo := state.request
	c.uploadMu.Unlock()
	defer func() {
		c.uploadMu.Lock()
		state.uploading = false
		c.uploadMu.Unlock()
	}()

	if requestInfo.Type == socialhub.MediaTypeImage {
		data, err := io.ReadAll(io.LimitReader(reader, requestInfo.Size+1))
		if err != nil {
			return nil, wrapError("upload_part", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
		if int64(len(data)) != requestInfo.Size {
			return nil, invalidArgument("upload_part", "cover byte count does not match declared size")
		}
		part := socialhub.UploadedPart{Number: 0, ETag: sessionID, Size: int64(len(data))}
		c.uploadMu.Lock()
		state.data = data
		state.part = &part
		c.uploadMu.Unlock()
		return &part, nil
	}

	counting := &countingReader{reader: io.LimitReader(reader, requestInfo.Size+1)}
	query := url.Values{"upload_token": {sessionID}}
	var response responseEnvelope[jsonEmpty]
	if err := c.rawUpload(ctx, "/video/v2/upload", query, "application/octet-stream", counting, &requestInfo.Size, &response, options...); err != nil {
		return nil, err
	}
	if err := response.Err("upload_part", http.StatusOK, nil); err != nil {
		return nil, err
	}
	if counting.count != requestInfo.Size {
		return nil, invalidArgument("upload_part", "video byte count does not match declared size")
	}
	part := socialhub.UploadedPart{Number: 0, ETag: sessionID, Size: counting.count}
	c.uploadMu.Lock()
	state.part = &part
	c.uploadMu.Unlock()
	return &part, nil
}

func (c *Client) CompleteUpload(ctx context.Context, sessionID string, parts []socialhub.UploadedPart, options ...socialhub.CallOption) (*socialhub.Media, error) {
	if sessionID == "" || len(parts) != 1 || parts[0].Number != 0 {
		return nil, invalidArgument("complete_upload", "session ID and exactly one part numbered 0 are required")
	}
	c.uploadMu.Lock()
	state := c.uploads[sessionID]
	if state == nil {
		c.uploadMu.Unlock()
		return nil, wrapError("complete_upload", socialhub.CodeNotFound, socialhub.ClassPermanent, nil)
	}
	if state.uploading || state.completing || state.completed || state.part == nil || *state.part != parts[0] || parts[0].Size != state.request.Size {
		c.uploadMu.Unlock()
		return nil, wrapError("complete_upload", socialhub.CodeConflict, socialhub.ClassPermanent, nil)
	}
	state.completing = true
	requestInfo := state.request
	data := append([]byte(nil), state.data...)
	c.uploadMu.Unlock()

	media := &socialhub.Media{ID: sessionID, MIME: requestInfo.MIME, Type: requestInfo.Type, Size: &requestInfo.Size, State: socialhub.MediaStateReady}
	if requestInfo.Type == socialhub.MediaTypeImage {
		coverURL, err := c.uploadCover(ctx, requestInfo.Filename, requestInfo.MIME, data, options...)
		if err != nil {
			c.setCompleting(state, false)
			return nil, err
		}
		media.URL = coverURL
	}
	c.uploadMu.Lock()
	state.completing = false
	state.completed = true
	state.media = media
	c.uploadMu.Unlock()
	copy := *media
	return &copy, nil
}

func (c *Client) MediaStatus(_ context.Context, mediaID string, _ ...socialhub.CallOption) (*socialhub.Media, error) {
	if mediaID == "" {
		return nil, invalidArgument("media_status", "media ID is required")
	}
	c.uploadMu.Lock()
	defer c.uploadMu.Unlock()
	state := c.uploads[mediaID]
	if state == nil {
		return nil, wrapError("media_status", socialhub.CodeNotFound, socialhub.ClassPermanent, nil)
	}
	if state.media == nil {
		return &socialhub.Media{ID: mediaID, MIME: state.request.MIME, Type: state.request.Type, State: socialhub.MediaStateUploading}, nil
	}
	copy := *state.media
	return &copy, nil
}

func (c *Client) uploadCover(ctx context.Context, filename, mimeType string, data []byte, options ...socialhub.CallOption) (string, error) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	header := make(textproto.MIMEHeader)
	header.Set("Content-Disposition", `form-data; name="file"; filename="`+safeFilename(filename)+`"`)
	header.Set("Content-Type", mimeType)
	part, err := writer.CreatePart(header)
	if err == nil {
		_, err = part.Write(data)
	}
	if closeErr := writer.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return "", wrapError("cover_upload", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	request, err := c.rawSignedRequest(ctx, http.MethodPost, "/arcopen/fn/archive/cover/upload", nil, bytes.NewReader(body.Bytes()), options...)
	if err != nil {
		return "", err
	}
	request.Header.Set("Content-Type", writer.FormDataContentType())
	request.Header.Set("Accept", "application/json")
	// The v2 signing contract excludes file bytes; this request has no non-file fields.
	empty := md5.Sum(nil)
	if err := c.signer.sign(request, c.token, hex.EncodeToString(empty[:])); err != nil {
		return "", wrapError("cover_upload", socialhub.CodeUnauthenticated, socialhub.ClassUserAction, err)
	}
	var response responseEnvelope[struct {
		URL string `json:"url"`
	}]
	if err := c.doRaw(request, &response); err != nil {
		return "", err
	}
	if err := response.Err("cover_upload", http.StatusOK, nil); err != nil {
		return "", err
	}
	if response.Data.URL == "" {
		return "", wrapError("cover_upload", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return response.Data.URL, nil
}

func (c *Client) rawUpload(ctx context.Context, path string, query url.Values, contentType string, body io.Reader, contentLength *int64, output any, options ...socialhub.CallOption) error {
	request, err := c.rawRequest(ctx, http.MethodPost, c.uploadBaseURL, path, query, body, options...)
	if err != nil {
		return err
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Content-Type", contentType)
	if contentLength != nil {
		request.ContentLength = *contentLength
	}
	return c.doRaw(request, output)
}

func (c *Client) rawSignedRequest(ctx context.Context, method, path string, query url.Values, body io.Reader, options ...socialhub.CallOption) (*http.Request, error) {
	return c.rawRequest(ctx, method, c.baseURL, path, query, body, options...)
}

func (c *Client) rawRequest(ctx context.Context, method, baseURL, path string, query url.Values, body io.Reader, options ...socialhub.CallOption) (*http.Request, error) {
	callOptions, err := socialhub.ResolveCallOptions(options...)
	if err != nil {
		return nil, err
	}
	if callOptions.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, callOptions.Timeout)
		ctx = context.WithValue(ctx, cancelContextKey{}, cancel)
	}
	endpoint, err := url.Parse(strings.TrimRight(baseURL, "/") + "/" + strings.TrimLeft(path, "/"))
	if err != nil || endpoint.Scheme == "" || endpoint.Host == "" {
		return nil, invalidArgument("raw_request", "invalid configured endpoint")
	}
	endpoint.RawQuery = query.Encode()
	request, err := http.NewRequestWithContext(ctx, method, endpoint.String(), body)
	if err != nil {
		return nil, wrapError("raw_request", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	if callOptions.RequestID != "" {
		request.Header.Set("X-Request-ID", callOptions.RequestID)
	}
	return request, nil
}

func (c *Client) doRaw(request *http.Request, output any) error {
	if cancel, ok := request.Context().Value(cancelContextKey{}).(context.CancelFunc); ok {
		defer cancel()
	}
	response, err := c.httpClient.Do(request)
	if err != nil {
		return wrapError("raw_request", socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, maximumUploadResponse+1))
	if err != nil {
		return wrapError("raw_request", socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
	}
	if int64(len(body)) > maximumUploadResponse {
		return wrapError("raw_request", socialhub.CodePlatformError, socialhub.ClassPermanent, fmt.Errorf("response exceeded size limit"))
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return decodeHTTPError(response.StatusCode, response.Header, body)
	}
	if output == nil || len(body) == 0 {
		return nil
	}
	if err := json.Unmarshal(body, output); err != nil {
		return wrapError("raw_request", socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	return nil
}

func (c *Client) publicationMedia(mediaIDs []string) (string, string, error) {
	if len(mediaIDs) < 1 || len(mediaIDs) > 2 {
		return "", "", invalidArgument("publish", "one completed video and at most one completed cover are required")
	}
	c.uploadMu.Lock()
	defer c.uploadMu.Unlock()
	var videoID, coverID string
	for _, mediaID := range mediaIDs {
		state := c.uploads[mediaID]
		if state == nil || !state.completed || state.media == nil {
			return "", "", wrapError("publish", socialhub.CodeConflict, socialhub.ClassPermanent, nil)
		}
		if state.request.Type == socialhub.MediaTypeVideo {
			if videoID != "" {
				return "", "", invalidArgument("publish", "only one video is allowed")
			}
			videoID = mediaID
		} else if state.request.Type == socialhub.MediaTypeImage {
			if coverID != "" {
				return "", "", invalidArgument("publish", "only one cover is allowed")
			}
			coverID = mediaID
		}
	}
	if videoID == "" {
		return "", "", invalidArgument("publish", "a completed video is required")
	}
	return videoID, coverID, nil
}

func (c *Client) submissionMedia(videoID, coverID string) (*uploadState, *uploadState, error) {
	c.uploadMu.Lock()
	defer c.uploadMu.Unlock()
	videoState := c.uploads[videoID]
	if videoState == nil || !videoState.completed || videoState.media == nil || videoState.request.Type != socialhub.MediaTypeVideo {
		return nil, nil, wrapError("submission_publish", socialhub.CodeConflict, socialhub.ClassPermanent, nil)
	}
	var coverState *uploadState
	if coverID != "" {
		coverState = c.uploads[coverID]
		if coverState == nil || !coverState.completed || coverState.media == nil || coverState.request.Type != socialhub.MediaTypeImage {
			return nil, nil, wrapError("submission_publish", socialhub.CodeConflict, socialhub.ClassPermanent, nil)
		}
	}
	return videoState, coverState, nil
}

func (c *Client) setCompleting(state *uploadState, value bool) {
	c.uploadMu.Lock()
	state.completing = value
	c.uploadMu.Unlock()
}

func randomID(prefix string) (string, error) {
	random := make([]byte, 18)
	if _, err := rand.Read(random); err != nil {
		return "", wrapError("begin_upload", socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
	}
	return prefix + base64.RawURLEncoding.EncodeToString(random), nil
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

func safeFilename(value string) string {
	value = strings.ReplaceAll(value, "\r", "_")
	value = strings.ReplaceAll(value, "\n", "_")
	value = strings.ReplaceAll(value, "\\", "\\\\")
	return strings.ReplaceAll(value, `"`, `\"`)
}

// VideoService adapts account defaults to the common short-video workflow.
type VideoService struct{ client *Client }

func (s *VideoService) Create(ctx context.Context, input video.CreateRequest) (*video.Session, error) {
	session, err := s.client.BeginUpload(ctx, socialhub.BeginUploadRequest{Filename: input.Filename, MIME: input.MIME, Type: socialhub.MediaTypeVideo, Size: input.Size})
	if err != nil {
		return nil, err
	}
	return &video.Session{ID: session.ID, State: video.StateCreated, PartSize: session.PartSize}, nil
}

func (s *VideoService) Upload(ctx context.Context, sessionID string, reader io.Reader, size int64) error {
	if sessionID == "" || reader == nil || size <= 0 {
		return invalidArgument("video_workflow_upload", "session ID, reader, and positive size are required")
	}
	s.client.uploadMu.Lock()
	state := s.client.uploads[sessionID]
	if state == nil || state.request.Type != socialhub.MediaTypeVideo {
		s.client.uploadMu.Unlock()
		return wrapError("video_workflow_upload", socialhub.CodeNotFound, socialhub.ClassPermanent, nil)
	}
	if state.request.Size != size {
		s.client.uploadMu.Unlock()
		return invalidArgument("video_workflow_upload", "size does not match the created session")
	}
	s.client.uploadMu.Unlock()
	part, err := s.client.UploadPart(ctx, sessionID, 0, io.LimitReader(reader, size))
	if err != nil {
		return err
	}
	if part.Size != size {
		return invalidArgument("video_workflow_upload", "reader ended before declared size")
	}
	return nil
}

func (s *VideoService) Complete(ctx context.Context, sessionID string) error {
	s.client.uploadMu.Lock()
	state := s.client.uploads[sessionID]
	if state == nil || state.part == nil {
		s.client.uploadMu.Unlock()
		return wrapError("video_workflow_complete", socialhub.CodeNotFound, socialhub.ClassPermanent, nil)
	}
	part := *state.part
	s.client.uploadMu.Unlock()
	_, err := s.client.CompleteUpload(ctx, sessionID, []socialhub.UploadedPart{part})
	return err
}

func (s *VideoService) Publish(ctx context.Context, sessionID string, input video.PublishRequest) (*video.Job, error) {
	if s.client.defaults.DefaultTID <= 0 || len(s.client.defaults.DefaultTags) == 0 {
		return nil, invalidArgument("video_workflow_publish", "account default_tid and default_tags are required; otherwise use SubmissionWorkflow")
	}
	title := strings.TrimSpace(input.Title)
	if title == "" {
		title = strings.TrimSpace(input.Description)
	}
	post, err := s.client.submissions.Publish(ctx, SubmissionRequest{
		UploadToken: sessionID, CoverID: input.CoverID, Title: title, Description: input.Description,
		TID: s.client.defaults.DefaultTID, Tags: append([]string(nil), s.client.defaults.DefaultTags...),
		Copyright: s.client.defaults.DefaultCopyright, Source: s.client.defaults.DefaultSource, NoReprint: s.client.defaults.NoReprint,
	})
	if err != nil {
		return nil, err
	}
	return &video.Job{ID: post.ID, PostID: post.ID, State: video.StatePublishPending, Message: post.Status.Message}, nil
}

func (s *VideoService) Status(ctx context.Context, jobID string) (*video.Job, error) {
	status, err := s.client.PublishStatus(ctx, jobID)
	if err != nil {
		return nil, err
	}
	state := video.StatePublishPending
	if status.State == socialhub.PublishStatePublished {
		state = video.StatePublished
	} else if status.State == socialhub.PublishStateFailed {
		state = video.StateFailed
	}
	return &video.Job{ID: jobID, PostID: jobID, State: state, Message: status.Message, UpdatedAt: status.UpdatedAt}, nil
}

func (s *VideoService) Abort(_ context.Context, sessionID string) error {
	s.client.uploadMu.Lock()
	defer s.client.uploadMu.Unlock()
	state := s.client.uploads[sessionID]
	if state == nil {
		return wrapError("video_workflow_abort", socialhub.CodeNotFound, socialhub.ClassPermanent, nil)
	}
	if state.uploading || state.completing {
		return wrapError("video_workflow_abort", socialhub.CodeConflict, socialhub.ClassPermanent, nil)
	}
	delete(s.client.uploads, sessionID)
	return nil
}
