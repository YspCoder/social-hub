package douyin

import (
	"context"
	"crypto/rand"
	"encoding/base64"
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
	directUploadThreshold int64 = 50 << 20
	maximumVideoBytes     int64 = 4 << 30
	defaultPartSize       int64 = 20 << 20
	minimumPartSize       int64 = 5 << 20
)

type uploadState struct {
	request    socialhub.BeginUploadRequest
	chunked    bool
	media      *socialhub.Media
	parts      map[int]socialhub.UploadedPart
	uploading  map[int]bool
	completing bool
	completed  bool
}

type uploadInitEnvelope struct {
	Data struct {
		APIResponse
		UploadID string `json:"upload_id"`
	} `json:"data"`
	Extra responseExtra `json:"extra"`
}

type uploadVideo struct {
	VideoID string `json:"video_id"`
	Width   int    `json:"width"`
	Height  int    `json:"height"`
}

type videoUploadEnvelope struct {
	Data struct {
		APIResponse
		Video uploadVideo `json:"video"`
	} `json:"data"`
	Extra responseExtra `json:"extra"`
}

func (c *Client) BeginUpload(ctx context.Context, input socialhub.BeginUploadRequest, options ...socialhub.CallOption) (*socialhub.UploadSession, error) {
	if err := c.requireScope("begin_upload", "video.create"); err != nil {
		return nil, err
	}
	if input.Filename == "" || input.Size <= 0 || input.MIME == "" {
		return nil, invalidArgument("begin_upload", "filename, positive size, and MIME type are required")
	}
	if input.Type != socialhub.MediaTypeVideo || !strings.HasPrefix(input.MIME, "video/") {
		return nil, &socialhub.Error{Code: socialhub.CodeUnsupported, Class: socialhub.ClassPermanent, Platform: "douyin", Product: "openapi", Op: "begin_upload", PlatformMessage: "Douyin upload supports video media only"}
	}
	if input.Size > maximumVideoBytes {
		return nil, invalidArgument("begin_upload", "video exceeds the 4 GiB multipart limit")
	}
	chunked := input.Size > directUploadThreshold
	sessionID := ""
	if chunked {
		var response uploadInitEnvelope
		if err := c.transport.JSON(ctx, http.MethodPost, "/video/part/init/", c.openIDQuery(), map[string]any{}, &response, options...); err != nil {
			return nil, err
		}
		if err := responseError(response.Data.APIResponse, response.Extra, "begin_upload", http.StatusOK, nil); err != nil {
			return nil, err
		}
		sessionID = response.Data.UploadID
		if sessionID == "" {
			return nil, wrapError("begin_upload", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
		}
	} else {
		random := make([]byte, 18)
		if _, err := rand.Read(random); err != nil {
			return nil, wrapError("begin_upload", socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
		}
		sessionID = "direct:" + base64.RawURLEncoding.EncodeToString(random)
	}
	c.uploadMu.Lock()
	if _, exists := c.uploads[sessionID]; exists {
		c.uploadMu.Unlock()
		return nil, wrapError("begin_upload", socialhub.CodeConflict, socialhub.ClassPermanent, nil)
	}
	c.uploads[sessionID] = &uploadState{request: input, chunked: chunked, parts: make(map[int]socialhub.UploadedPart), uploading: make(map[int]bool)}
	c.uploadMu.Unlock()
	partSize := input.Size
	if chunked {
		partSize = defaultPartSize
	}
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
	if !state.chunked && partNumber != 0 {
		c.uploadMu.Unlock()
		return nil, invalidArgument("upload_part", "direct upload accepts only part 0")
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

	query := c.openIDQuery()
	path := "/video/upload/"
	if chunked {
		path = "/video/part/upload/"
		query.Set("upload_id", sessionID)
		query.Set("part_number", strconv.Itoa(partNumber+1))
	}
	counting := &countingReader{reader: reader}
	var response videoUploadEnvelope
	if err := c.multipartVideo(ctx, path, query, requestInfo.Filename, requestInfo.MIME, counting, &response, options...); err != nil {
		return nil, err
	}
	if err := responseError(response.Data.APIResponse, response.Extra, "upload_part", http.StatusOK, nil); err != nil {
		return nil, err
	}
	if counting.count <= 0 {
		return nil, invalidArgument("upload_part", "uploaded part must not be empty")
	}
	if chunked && counting.count > defaultPartSize {
		return nil, invalidArgument("upload_part", "uploaded part exceeds the 20 MiB part size")
	}
	if !chunked && counting.count != requestInfo.Size {
		return nil, invalidArgument("upload_part", "direct upload byte count does not match declared size")
	}
	part := socialhub.UploadedPart{Number: partNumber, ETag: response.Data.Video.VideoID, Size: counting.count}
	c.uploadMu.Lock()
	state.parts[partNumber] = part
	if !chunked {
		if response.Data.Video.VideoID == "" {
			c.uploadMu.Unlock()
			return nil, wrapError("upload_part", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
		}
		size := requestInfo.Size
		state.media = &socialhub.Media{ID: response.Data.Video.VideoID, MIME: requestInfo.MIME, Type: socialhub.MediaTypeVideo, Size: &size, Width: intPointer(response.Data.Video.Width), Height: intPointer(response.Data.Video.Height), State: socialhub.MediaStateReady}
		part.ETag = response.Data.Video.VideoID
		state.parts[partNumber] = part
	}
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
	if !state.chunked {
		if state.media == nil || len(parts) != 1 || parts[0].Number != 0 || parts[0].ETag != state.media.ID {
			c.uploadMu.Unlock()
			return nil, wrapError("complete_upload", socialhub.CodeConflict, socialhub.ClassPermanent, nil)
		}
		media := *state.media
		state.completed = true
		c.uploads[media.ID] = state
		c.uploadMu.Unlock()
		return &media, nil
	}
	state.completing = true
	requestInfo := state.request
	c.uploadMu.Unlock()

	query := c.openIDQuery()
	query.Set("upload_id", sessionID)
	var response videoUploadEnvelope
	err := c.transport.JSON(ctx, http.MethodPost, "/video/part/complete/", query, map[string]any{}, &response, options...)
	c.uploadMu.Lock()
	state.completing = false
	defer c.uploadMu.Unlock()
	if err != nil {
		return nil, err
	}
	if err := responseError(response.Data.APIResponse, response.Extra, "complete_upload", http.StatusOK, nil); err != nil {
		return nil, err
	}
	if response.Data.Video.VideoID == "" {
		return nil, wrapError("complete_upload", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	size := requestInfo.Size
	state.media = &socialhub.Media{ID: response.Data.Video.VideoID, MIME: requestInfo.MIME, Type: socialhub.MediaTypeVideo, Size: &size, Width: intPointer(response.Data.Video.Width), Height: intPointer(response.Data.Video.Height), State: socialhub.MediaStateReady}
	state.completed = true
	c.uploads[state.media.ID] = state
	media := *state.media
	return &media, nil
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
		return &socialhub.Media{ID: mediaID, MIME: state.request.MIME, Type: socialhub.MediaTypeVideo, State: socialhub.MediaStateUploading}, nil
	}
	media := *state.media
	return &media, nil
}

func (c *Client) multipartVideo(ctx context.Context, path string, query url.Values, filename, mime string, reader io.Reader, output any, options ...socialhub.CallOption) error {
	pipeReader, pipeWriter := io.Pipe()
	writer := multipart.NewWriter(pipeWriter)
	done := make(chan error, 1)
	go func() {
		header := make(textproto.MIMEHeader)
		header.Set("Content-Disposition", `form-data; name="video"; filename="`+safeFilename(filename)+`"`)
		header.Set("Content-Type", mime)
		part, writeErr := writer.CreatePart(header)
		if writeErr == nil {
			_, writeErr = io.Copy(part, reader)
		}
		if closeErr := writer.Close(); writeErr == nil {
			writeErr = closeErr
		}
		_ = pipeWriter.CloseWithError(writeErr)
		done <- writeErr
	}()
	request, err := c.transport.NewRequest(ctx, http.MethodPost, path, query, pipeReader, options...)
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
		return wrapError("video_upload", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, writeErr)
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
		if index < len(ordered)-1 && part.Size < minimumPartSize {
			return false
		}
	}
	return true
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

func intPointer(value int) *int {
	if value == 0 {
		return nil
	}
	copy := value
	return &copy
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
	if state == nil {
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
		limited := io.LimitReader(reader, currentSize)
		part, err := s.client.UploadPart(ctx, sessionID, partNumber, limited)
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
	if input.CoverID != "" {
		return nil, &socialhub.Error{Code: socialhub.CodeUnsupported, Class: socialhub.ClassPermanent, Platform: "douyin", Product: "openapi", Op: "video_workflow_publish", PlatformMessage: "cover selection is not exposed by the initial video.create integration"}
	}
	s.client.uploadMu.Lock()
	state := s.client.uploads[sessionID]
	if state == nil || state.media == nil || !state.completed {
		s.client.uploadMu.Unlock()
		return nil, wrapError("video_workflow_publish", socialhub.CodeConflict, socialhub.ClassPermanent, nil)
	}
	mediaID := state.media.ID
	s.client.uploadMu.Unlock()
	text := input.Description
	if text == "" {
		text = input.Title
	}
	post, err := s.client.Publish(ctx, socialhub.CreatePostRequest{Text: stringPointer(text), MediaIDs: []string{mediaID}})
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
	if len(state.uploading) > 0 || state.completing {
		return wrapError("video_workflow_abort", socialhub.CodeConflict, socialhub.ClassPermanent, nil)
	}
	delete(s.client.uploads, sessionID)
	if state.media != nil {
		delete(s.client.uploads, state.media.ID)
	}
	return nil
}
