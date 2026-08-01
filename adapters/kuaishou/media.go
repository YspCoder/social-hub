package kuaishou

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"social-hub/extensions/video"
	"social-hub/pkg/socialhub"
)

const (
	directUploadThreshold int64 = 10 << 20
	defaultPartSize       int64 = 10 << 20
	maximumCoverBytes     int64 = (10 << 20) - 1
	maximumUploadResponse int64 = 1 << 20
)

type uploadState struct {
	request    socialhub.BeginUploadRequest
	endpoint   *url.URL
	chunked    bool
	media      *socialhub.Media
	coverData  []byte
	parts      map[int]socialhub.UploadedPart
	uploading  map[int]bool
	completing bool
	completed  bool
}

type startUploadEnvelope struct {
	resultEnvelope
	UploadToken string `json:"upload_token"`
	Endpoint    string `json:"endpoint"`
}

type uploadEnvelope struct {
	resultEnvelope
	Checksum string `json:"checksum"`
	Size     int64  `json:"size"`
}

func (c *Client) BeginUpload(ctx context.Context, input socialhub.BeginUploadRequest, options ...socialhub.CallOption) (*socialhub.UploadSession, error) {
	if err := c.requireScope("begin_upload", "user_video_publish"); err != nil {
		return nil, err
	}
	if input.Filename == "" || input.Size <= 0 || input.MIME == "" {
		return nil, invalidArgument("begin_upload", "filename, positive size, and MIME type are required")
	}
	state := &uploadState{request: input, parts: make(map[int]socialhub.UploadedPart), uploading: make(map[int]bool)}
	var sessionID string
	partSize := input.Size
	switch input.Type {
	case socialhub.MediaTypeImage:
		if !strings.HasPrefix(input.MIME, "image/") {
			return nil, invalidArgument("begin_upload", "cover MIME type must be an image")
		}
		if input.Size > maximumCoverBytes {
			return nil, invalidArgument("begin_upload", "cover must be smaller than 10 MiB")
		}
		generated, err := randomID("cover:")
		if err != nil {
			return nil, err
		}
		sessionID = generated
	case socialhub.MediaTypeVideo:
		if !strings.HasPrefix(input.MIME, "video/") {
			return nil, invalidArgument("begin_upload", "video MIME type must be a video")
		}
		var response startUploadEnvelope
		if err := c.transport.JSON(ctx, http.MethodPost, "/openapi/photo/start_upload", c.appQuery(), nil, &response, options...); err != nil {
			return nil, err
		}
		if err := resultError(response.Result, response.ErrorMessage, "begin_upload", http.StatusOK, nil); err != nil {
			return nil, err
		}
		if response.UploadToken == "" || response.Endpoint == "" {
			return nil, wrapError("begin_upload", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
		}
		endpoint, err := c.validateUploadEndpoint(response.Endpoint)
		if err != nil {
			return nil, err
		}
		sessionID = response.UploadToken
		state.endpoint = endpoint
		state.chunked = input.Size >= directUploadThreshold
		if state.chunked {
			partSize = defaultPartSize
		}
	default:
		return nil, &socialhub.Error{Code: socialhub.CodeUnsupported, Class: socialhub.ClassPermanent, Platform: "kuaishou", Product: "openapi", Op: "begin_upload", PlatformMessage: "Kuaishou publication accepts video and cover image media only"}
	}
	c.uploadMu.Lock()
	defer c.uploadMu.Unlock()
	if _, exists := c.uploads[sessionID]; exists {
		return nil, wrapError("begin_upload", socialhub.CodeConflict, socialhub.ClassPermanent, nil)
	}
	c.uploads[sessionID] = state
	return &socialhub.UploadSession{ID: sessionID, PartSize: partSize}, nil
}

func (c *Client) UploadPart(ctx context.Context, sessionID string, partNumber int, reader io.Reader, options ...socialhub.CallOption) (*socialhub.UploadedPart, error) {
	if sessionID == "" || partNumber < 0 || reader == nil {
		return nil, invalidArgument("upload_part", "session ID, non-negative part number, and reader are required")
	}
	c.uploadMu.Lock()
	state := c.uploads[sessionID]
	if state == nil {
		c.uploadMu.Unlock()
		return nil, wrapError("upload_part", socialhub.CodeNotFound, socialhub.ClassPermanent, nil)
	}
	if state.completed || state.completing || state.uploading[partNumber] {
		c.uploadMu.Unlock()
		return nil, wrapError("upload_part", socialhub.CodeConflict, socialhub.ClassPermanent, nil)
	}
	if (!state.chunked || state.request.Type == socialhub.MediaTypeImage) && partNumber != 0 {
		c.uploadMu.Unlock()
		return nil, invalidArgument("upload_part", "direct and cover uploads accept only part 0")
	}
	if _, exists := state.parts[partNumber]; exists {
		c.uploadMu.Unlock()
		return nil, wrapError("upload_part", socialhub.CodeConflict, socialhub.ClassPermanent, nil)
	}
	state.uploading[partNumber] = true
	requestInfo := state.request
	chunked := state.chunked
	c.uploadMu.Unlock()
	defer func() {
		c.uploadMu.Lock()
		delete(state.uploading, partNumber)
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
		state.coverData = data
		state.parts[0] = part
		c.uploadMu.Unlock()
		return &part, nil
	}

	path := "/api/upload"
	query := url.Values{"upload_token": {sessionID}}
	limit := requestInfo.Size
	if chunked {
		path = "/api/upload/fragment"
		query.Set("fragment_id", strconv.Itoa(partNumber))
		limit = defaultPartSize
	}
	counting := &countingReader{reader: io.LimitReader(reader, limit+1)}
	var response uploadEnvelope
	if err := c.uploadRequest(ctx, state.endpoint, path, query, requestInfo.MIME, counting, &response, options...); err != nil {
		return nil, err
	}
	if err := resultError(response.Result, response.ErrorMessage, "upload_part", http.StatusOK, nil); err != nil {
		return nil, err
	}
	if counting.count <= 0 || counting.count > limit {
		return nil, invalidArgument("upload_part", "uploaded part is empty or exceeds the allowed part size")
	}
	if !chunked && counting.count != requestInfo.Size {
		return nil, invalidArgument("upload_part", "direct upload byte count does not match declared size")
	}
	if response.Size > 0 && response.Size != counting.count {
		return nil, wrapError("upload_part", socialhub.CodePlatformError, socialhub.ClassPermanent, fmt.Errorf("platform size does not match uploaded bytes"))
	}
	part := socialhub.UploadedPart{Number: partNumber, ETag: response.Checksum, Size: counting.count}
	c.uploadMu.Lock()
	state.parts[partNumber] = part
	c.uploadMu.Unlock()
	return &part, nil
}

func (c *Client) CompleteUpload(ctx context.Context, sessionID string, parts []socialhub.UploadedPart, options ...socialhub.CallOption) (*socialhub.Media, error) {
	if sessionID == "" || len(parts) == 0 {
		return nil, invalidArgument("complete_upload", "session ID and uploaded parts are required")
	}
	c.uploadMu.Lock()
	state := c.uploads[sessionID]
	if state == nil {
		c.uploadMu.Unlock()
		return nil, wrapError("complete_upload", socialhub.CodeNotFound, socialhub.ClassPermanent, nil)
	}
	if state.completed || state.completing || len(state.uploading) > 0 {
		c.uploadMu.Unlock()
		return nil, wrapError("complete_upload", socialhub.CodeConflict, socialhub.ClassPermanent, nil)
	}
	total, valid := validateUploadedParts(parts, state.parts)
	if !valid || total != state.request.Size || (state.chunked && !validChunkLayout(parts)) {
		c.uploadMu.Unlock()
		return nil, invalidArgument("complete_upload", "parts must match uploaded parts and declared total size")
	}
	state.completing = true
	requestInfo := state.request
	endpoint := state.endpoint
	chunked := state.chunked
	c.uploadMu.Unlock()

	if chunked {
		query := url.Values{"upload_token": {sessionID}, "fragment_count": {strconv.Itoa(len(parts))}}
		var response uploadEnvelope
		if err := c.uploadRequest(ctx, endpoint, "/api/upload/complete", query, requestInfo.MIME, nil, &response, options...); err != nil {
			c.setCompleting(state, false)
			return nil, err
		}
		if err := resultError(response.Result, response.ErrorMessage, "complete_upload", http.StatusOK, nil); err != nil {
			c.setCompleting(state, false)
			return nil, err
		}
	}
	size := requestInfo.Size
	media := &socialhub.Media{ID: sessionID, MIME: requestInfo.MIME, Type: requestInfo.Type, Size: &size, State: socialhub.MediaStateReady}
	c.uploadMu.Lock()
	state.completing = false
	state.completed = true
	state.media = media
	c.uploads[media.ID] = state
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

func (c *Client) validateUploadEndpoint(value string) (*url.URL, error) {
	if strings.Contains(value, "://") || strings.ContainsAny(value, "/?#@") {
		return nil, invalidArgument("begin_upload", "platform upload endpoint must be a host and optional port only")
	}
	parsed, err := url.Parse(c.uploadScheme + "://" + value)
	if err != nil || parsed.Hostname() == "" {
		return nil, invalidArgument("begin_upload", "platform returned an invalid upload endpoint")
	}
	host := strings.ToLower(parsed.Hostname())
	allowed := false
	for _, configured := range c.allowedUploadHosts {
		if strings.EqualFold(host, configured) {
			allowed = true
			break
		}
	}
	if c.uploadScheme == "https" && (strings.HasSuffix(host, ".gifshow.com") || strings.HasSuffix(host, ".kuaishou.com")) {
		allowed = true
	}
	if !allowed {
		return nil, &socialhub.Error{Code: socialhub.CodePermissionDenied, Class: socialhub.ClassPermanent, Platform: "kuaishou", Product: "openapi", Op: "begin_upload", PlatformMessage: "platform upload endpoint is outside the allowed Kuaishou hosts"}
	}
	return parsed, nil
}

func (c *Client) uploadRequest(ctx context.Context, endpoint *url.URL, path string, query url.Values, contentType string, body io.Reader, output any, options ...socialhub.CallOption) error {
	callOptions, err := socialhub.ResolveCallOptions(options...)
	if err != nil {
		return err
	}
	if callOptions.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, callOptions.Timeout)
		defer cancel()
	}
	requestURL := *endpoint
	requestURL.Path = path
	requestURL.RawQuery = query.Encode()
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, requestURL.String(), body)
	if err != nil {
		return wrapError("upload_request", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	request.Header.Set("Accept", "application/json")
	if contentType != "" {
		request.Header.Set("Content-Type", contentType)
	}
	if callOptions.RequestID != "" {
		request.Header.Set("X-Request-ID", callOptions.RequestID)
	}
	response, err := c.httpClient.Do(request)
	if err != nil {
		return wrapError("upload_request", socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
	}
	defer response.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(response.Body, maximumUploadResponse+1))
	if err != nil {
		return wrapError("upload_request", socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
	}
	if int64(len(responseBody)) > maximumUploadResponse {
		return wrapError("upload_request", socialhub.CodePlatformError, socialhub.ClassPermanent, fmt.Errorf("response exceeded size limit"))
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return decodeHTTPError(response.StatusCode, response.Header, responseBody)
	}
	if err := json.Unmarshal(responseBody, output); err != nil {
		return wrapError("upload_request", socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	return nil
}

func (c *Client) publicationMedia(mediaIDs []string) (*uploadState, *uploadState, error) {
	c.uploadMu.Lock()
	defer c.uploadMu.Unlock()
	var videoState, coverState *uploadState
	for _, mediaID := range mediaIDs {
		state := c.uploads[mediaID]
		if state == nil || !state.completed || state.media == nil {
			return nil, nil, wrapError("publish", socialhub.CodeConflict, socialhub.ClassPermanent, nil)
		}
		switch state.request.Type {
		case socialhub.MediaTypeVideo:
			if videoState != nil {
				return nil, nil, invalidArgument("publish", "only one video is allowed")
			}
			videoState = state
		case socialhub.MediaTypeImage:
			if coverState != nil {
				return nil, nil, invalidArgument("publish", "only one cover is allowed")
			}
			coverState = state
		}
	}
	if videoState == nil || coverState == nil || len(coverState.coverData) == 0 {
		return nil, nil, invalidArgument("publish", "one completed video and one completed cover image are required")
	}
	return videoState, coverState, nil
}

func (c *Client) multipartPublish(ctx context.Context, query url.Values, caption string, cover *uploadState, output any, options ...socialhub.CallOption) error {
	pipeReader, pipeWriter := io.Pipe()
	writer := multipart.NewWriter(pipeWriter)
	done := make(chan error, 1)
	coverData := append([]byte(nil), cover.coverData...)
	go func() {
		writeErr := writer.WriteField("caption", caption)
		if writeErr == nil {
			header := make(textproto.MIMEHeader)
			header.Set("Content-Disposition", `form-data; name="cover"; filename="`+safeFilename(cover.request.Filename)+`"`)
			header.Set("Content-Type", cover.request.MIME)
			var part io.Writer
			part, writeErr = writer.CreatePart(header)
			if writeErr == nil {
				_, writeErr = io.Copy(part, bytes.NewReader(coverData))
			}
		}
		if closeErr := writer.Close(); writeErr == nil {
			writeErr = closeErr
		}
		_ = pipeWriter.CloseWithError(writeErr)
		done <- writeErr
	}()
	request, err := c.transport.NewRequest(ctx, http.MethodPost, "/openapi/photo/publish", query, pipeReader, options...)
	if err != nil {
		_ = pipeReader.Close()
		_ = <-done
		return err
	}
	request.Header.Set("Content-Type", writer.FormDataContentType())
	err = c.transport.Do(request, output)
	writeErr := <-done
	if err != nil {
		return err
	}
	if writeErr != nil {
		return wrapError("publish", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, writeErr)
	}
	return nil
}

func validateUploadedParts(parts []socialhub.UploadedPart, recorded map[int]socialhub.UploadedPart) (int64, bool) {
	seen := make(map[int]struct{}, len(parts))
	var total int64
	for _, part := range parts {
		if _, duplicate := seen[part.Number]; duplicate {
			return 0, false
		}
		record, exists := recorded[part.Number]
		if !exists || record.Size != part.Size || record.ETag != part.ETag {
			return 0, false
		}
		seen[part.Number] = struct{}{}
		total += part.Size
	}
	return total, len(seen) == len(recorded)
}

func validChunkLayout(parts []socialhub.UploadedPart) bool {
	ordered := append([]socialhub.UploadedPart(nil), parts...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].Number < ordered[j].Number })
	for index, part := range ordered {
		if part.Number != index || part.Size <= 0 || part.Size > defaultPartSize {
			return false
		}
	}
	return true
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

// VideoService implements the typed short-video workflow.
type VideoService struct{ client *Client }

func (s *VideoService) Create(ctx context.Context, input video.CreateRequest) (*video.Session, error) {
	session, err := s.client.BeginUpload(ctx, socialhub.BeginUploadRequest{Filename: input.Filename, MIME: input.MIME, Type: socialhub.MediaTypeVideo, Size: input.Size})
	if err != nil {
		return nil, err
	}
	return &video.Session{ID: session.ID, State: video.StateCreated, PartSize: session.PartSize, ExpiresAt: session.ExpiresAt}, nil
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
	if size != state.request.Size {
		s.client.uploadMu.Unlock()
		return invalidArgument("video_workflow_upload", "size does not match the created session")
	}
	partSize := state.request.Size
	if state.chunked {
		partSize = defaultPartSize
	}
	s.client.uploadMu.Unlock()

	remaining := size
	for partNumber := 0; remaining > 0; partNumber++ {
		currentSize := min(remaining, partSize)
		part, err := s.client.UploadPart(ctx, sessionID, partNumber, io.LimitReader(reader, currentSize))
		if err != nil {
			return err
		}
		if part.Size != currentSize {
			return invalidArgument("video_workflow_upload", "reader ended before declared size")
		}
		remaining -= currentSize
	}
	return nil
}

func (s *VideoService) Complete(ctx context.Context, sessionID string) error {
	s.client.uploadMu.Lock()
	state := s.client.uploads[sessionID]
	if state == nil {
		s.client.uploadMu.Unlock()
		return wrapError("video_workflow_complete", socialhub.CodeNotFound, socialhub.ClassPermanent, nil)
	}
	parts := make([]socialhub.UploadedPart, 0, len(state.parts))
	for _, part := range state.parts {
		parts = append(parts, part)
	}
	s.client.uploadMu.Unlock()
	sort.Slice(parts, func(i, j int) bool { return parts[i].Number < parts[j].Number })
	_, err := s.client.CompleteUpload(ctx, sessionID, parts)
	return err
}

func (s *VideoService) Publish(ctx context.Context, sessionID string, input video.PublishRequest) (*video.Job, error) {
	if input.CoverID == "" {
		return nil, invalidArgument("video_workflow_publish", "a completed cover ID is required")
	}
	caption := strings.TrimSpace(input.Description)
	if caption == "" {
		caption = strings.TrimSpace(input.Title)
	}
	if caption == "" {
		return nil, invalidArgument("video_workflow_publish", "title or description is required")
	}
	post, err := s.client.Publish(ctx, socialhub.CreatePostRequest{Text: &caption, MediaIDs: []string{sessionID, input.CoverID}})
	if err != nil {
		return nil, err
	}
	state := video.StatePublished
	if post.Status != nil && post.Status.State == socialhub.PublishStatePending {
		state = video.StatePublishPending
	}
	return &video.Job{ID: post.ID, PostID: post.ID, State: state, Message: post.Status.Message, UpdatedAt: post.Status.UpdatedAt}, nil
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
	if len(state.uploading) > 0 || state.completing {
		return wrapError("video_workflow_abort", socialhub.CodeConflict, socialhub.ClassPermanent, nil)
	}
	delete(s.client.uploads, sessionID)
	return nil
}
