package vk

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"strconv"
	"strings"

	"social-hub/pkg/socialhub"
)

const (
	maxWallPhotoBytes   int64 = 50 << 20
	maxUploadReplyBytes int64 = 1 << 20
)

type wallUploadResponse struct {
	Server int64  `json:"server"`
	Photo  string `json:"photo"`
	Hash   string `json:"hash"`
}

type uploadState struct {
	request    socialhub.BeginUploadRequest
	uploadURL  string
	uploading  bool
	completing bool
	uploaded   *wallUploadResponse
}

func (c *Client) BeginUpload(ctx context.Context, input socialhub.BeginUploadRequest, options ...socialhub.CallOption) (*socialhub.UploadSession, error) {
	if c.tokenKind != TokenUser {
		return nil, tokenPermission("photos.getWallUploadServer", "VK wall photo upload requires a user access token")
	}
	if strings.TrimSpace(input.Filename) == "" || input.Size <= 0 || input.Size > maxWallPhotoBytes {
		return nil, invalidArgument("begin_upload", "filename and a positive size no greater than 50 MiB are required")
	}
	if input.Type != socialhub.MediaTypeImage {
		return nil, unsupported("begin_upload", "the common VK uploader supports wall photos only")
	}
	switch input.MIME {
	case "image/jpeg", "image/png", "image/gif":
	default:
		return nil, invalidArgument("begin_upload", "wall photo MIME type must be JPEG, PNG, or GIF")
	}
	if input.Category != "" && input.Category != "wall" {
		return nil, invalidArgument("begin_upload", "media category must be empty or wall")
	}
	values := make(url.Values)
	if c.ownerID < 0 {
		values.Set("group_id", strconv.FormatInt(-c.ownerID, 10))
	}
	var response struct {
		UploadURL string `json:"upload_url"`
	}
	if err := c.method(ctx, "photos.getWallUploadServer", values, &response, options...); err != nil {
		return nil, err
	}
	if !validUploadURL(response.UploadURL, c.allowHTTPUploads) {
		return nil, platformError("photos.getWallUploadServer", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	random := make([]byte, 18)
	if _, err := rand.Read(random); err != nil {
		return nil, platformError("begin_upload", socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
	}
	sessionID := "wall-photo:" + base64.RawURLEncoding.EncodeToString(random)
	c.uploadMu.Lock()
	c.uploads[sessionID] = &uploadState{request: input, uploadURL: response.UploadURL}
	c.uploadMu.Unlock()
	return &socialhub.UploadSession{ID: sessionID, PartSize: input.Size}, nil
}

func (c *Client) UploadPart(ctx context.Context, sessionID string, partNumber int, reader io.Reader, options ...socialhub.CallOption) (*socialhub.UploadedPart, error) {
	if strings.TrimSpace(sessionID) == "" || partNumber != 0 || reader == nil {
		return nil, invalidArgument("upload_part", "wall photo upload requires session ID, part 0, and a reader")
	}
	resolved, err := socialhub.ResolveCallOptions(options...)
	if err != nil {
		return nil, err
	}
	if resolved.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, resolved.Timeout)
		defer cancel()
	}
	c.uploadMu.Lock()
	state := c.uploads[sessionID]
	if state == nil {
		c.uploadMu.Unlock()
		return nil, platformError("upload_part", socialhub.CodeNotFound, socialhub.ClassPermanent, nil)
	}
	if state.uploading || state.completing || state.uploaded != nil {
		c.uploadMu.Unlock()
		return nil, platformError("upload_part", socialhub.CodeConflict, socialhub.ClassPermanent, nil)
	}
	state.uploading = true
	c.uploadMu.Unlock()
	defer func() {
		c.uploadMu.Lock()
		state.uploading = false
		c.uploadMu.Unlock()
	}()

	pipeReader, pipeWriter := io.Pipe()
	writer := multipart.NewWriter(pipeWriter)
	counting := &countingReader{reader: io.LimitReader(reader, state.request.Size+1)}
	done := make(chan error, 1)
	go func() {
		header := make(textproto.MIMEHeader)
		header.Set("Content-Disposition", `form-data; name="photo"; filename="`+safeFilename(state.request.Filename)+`"`)
		header.Set("Content-Type", state.request.MIME)
		part, writeErr := writer.CreatePart(header)
		if writeErr == nil {
			_, writeErr = io.Copy(part, counting)
		}
		if closeErr := writer.Close(); writeErr == nil {
			writeErr = closeErr
		}
		_ = pipeWriter.CloseWithError(writeErr)
		done <- writeErr
	}()
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, state.uploadURL, pipeReader)
	if err != nil {
		_ = pipeReader.Close()
		_ = <-done
		return nil, platformError("upload_part", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	request.Header.Set("Content-Type", writer.FormDataContentType())
	if resolved.RequestID != "" {
		request.Header.Set("X-Request-ID", resolved.RequestID)
	}
	response, err := c.httpClient.Do(request)
	if err != nil {
		_ = pipeReader.CloseWithError(err)
	}
	writeErr := <-done
	if err != nil {
		return nil, platformError("upload_part", socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
	}
	defer response.Body.Close()
	body, readErr := io.ReadAll(io.LimitReader(response.Body, maxUploadReplyBytes+1))
	if readErr != nil {
		return nil, platformError("upload_part", socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, readErr)
	}
	if writeErr != nil {
		return nil, platformError("upload_part", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, writeErr)
	}
	if counting.count != state.request.Size {
		return nil, invalidArgument("upload_part", "uploaded byte count does not match declared size")
	}
	if int64(len(body)) > maxUploadReplyBytes || response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, platformError("upload_part", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	var uploaded wallUploadResponse
	if json.Unmarshal(body, &uploaded) != nil || uploaded.Server == 0 || !validOpaque(uploaded.Photo, 1<<20) || !validOpaque(uploaded.Hash, 512) {
		return nil, platformError("upload_part", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	c.uploadMu.Lock()
	state.uploaded = &uploaded
	c.uploadMu.Unlock()
	return &socialhub.UploadedPart{Number: 0, ETag: uploaded.Hash, Size: state.request.Size}, nil
}

func (c *Client) CompleteUpload(ctx context.Context, sessionID string, parts []socialhub.UploadedPart, options ...socialhub.CallOption) (*socialhub.Media, error) {
	if strings.TrimSpace(sessionID) == "" || len(parts) != 1 || parts[0].Number != 0 {
		return nil, invalidArgument("complete_upload", "exactly upload part 0 is required")
	}
	c.uploadMu.Lock()
	state := c.uploads[sessionID]
	if state == nil {
		c.uploadMu.Unlock()
		return nil, platformError("complete_upload", socialhub.CodeNotFound, socialhub.ClassPermanent, nil)
	}
	if state.uploading || state.completing || state.uploaded == nil || parts[0].ETag != state.uploaded.Hash || parts[0].Size != state.request.Size {
		c.uploadMu.Unlock()
		return nil, platformError("complete_upload", socialhub.CodeConflict, socialhub.ClassPermanent, nil)
	}
	state.completing = true
	uploaded, uploadRequest := *state.uploaded, state.request
	c.uploadMu.Unlock()
	defer func() {
		c.uploadMu.Lock()
		if current := c.uploads[sessionID]; current == state {
			current.completing = false
		}
		c.uploadMu.Unlock()
	}()
	values := url.Values{
		"server": {strconv.FormatInt(uploaded.Server, 10)}, "photo": {uploaded.Photo}, "hash": {uploaded.Hash},
	}
	if c.ownerID < 0 {
		values.Set("group_id", strconv.FormatInt(-c.ownerID, 10))
	} else {
		values.Set("user_id", strconv.FormatInt(c.ownerID, 10))
	}
	var response []wirePhoto
	if err := c.method(ctx, "photos.saveWallPhoto", values, &response, options...); err != nil {
		return nil, err
	}
	if len(response) != 1 || response[0].ID <= 0 || response[0].OwnerID == 0 {
		return nil, platformError("photos.saveWallPhoto", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	media, _ := mapAttachment(wireAttachment{Type: "photo", Photo: response[0]})
	media.MIME, media.Size = uploadRequest.MIME, &uploadRequest.Size
	c.uploadMu.Lock()
	delete(c.uploads, sessionID)
	c.media[media.ID] = media
	c.uploadMu.Unlock()
	return &media, nil
}

func (c *Client) MediaStatus(_ context.Context, mediaID string, _ ...socialhub.CallOption) (*socialhub.Media, error) {
	if strings.TrimSpace(mediaID) == "" {
		return nil, invalidArgument("media_status", "media ID is required")
	}
	c.uploadMu.Lock()
	defer c.uploadMu.Unlock()
	media, found := c.media[mediaID]
	if !found {
		return nil, platformError("media_status", socialhub.CodeNotFound, socialhub.ClassPermanent, nil)
	}
	copy := media
	return &copy, nil
}

func validUploadURL(value string, allowHTTP bool) bool {
	parsed, err := url.Parse(value)
	if err != nil || parsed.Host == "" || parsed.User != nil {
		return false
	}
	return parsed.Scheme == "https" || (allowHTTP && parsed.Scheme == "http")
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
