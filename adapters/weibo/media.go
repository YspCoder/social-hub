package weibo

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"strings"

	"social-hub/pkg/socialhub"
)

const maxImageBytes int64 = 10 << 20

type uploadState struct {
	request   socialhub.BeginUploadRequest
	media     *socialhub.Media
	uploading bool
}

type pictureUploadResponse struct {
	APIError
	PicID        string `json:"pic_id"`
	ThumbnailPic string `json:"thumbnail_pic"`
	BmiddlePic   string `json:"bmiddle_pic"`
	OriginalPic  string `json:"original_pic"`
}

func (c *Client) BeginUpload(_ context.Context, input socialhub.BeginUploadRequest, _ ...socialhub.CallOption) (*socialhub.UploadSession, error) {
	if input.Filename == "" || input.Size <= 0 || input.MIME == "" {
		return nil, invalidArgument("begin_upload", "filename, positive size, and MIME type are required")
	}
	if input.Type != socialhub.MediaTypeImage {
		return nil, &socialhub.Error{Code: socialhub.CodeUnsupported, Class: socialhub.ClassPermanent, Platform: "weibo", Product: "open-api", Op: "begin_upload", PlatformMessage: "upload_pic supports images only"}
	}
	if input.Size >= maxImageBytes {
		return nil, invalidArgument("begin_upload", "image must be smaller than 10 MiB")
	}
	switch input.MIME {
	case "image/jpeg", "image/png", "image/gif":
	default:
		return nil, invalidArgument("begin_upload", "image MIME type must be JPEG, PNG, or GIF")
	}
	random := make([]byte, 18)
	if _, err := rand.Read(random); err != nil {
		return nil, wrapError("begin_upload", socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
	}
	sessionID := "picture:" + base64.RawURLEncoding.EncodeToString(random)
	c.uploadMu.Lock()
	c.uploads[sessionID] = &uploadState{request: input}
	c.uploadMu.Unlock()
	return &socialhub.UploadSession{ID: sessionID, PartSize: input.Size}, nil
}

func (c *Client) UploadPart(ctx context.Context, sessionID string, partNumber int, reader io.Reader, options ...socialhub.CallOption) (*socialhub.UploadedPart, error) {
	if sessionID == "" || partNumber != 0 || reader == nil {
		return nil, invalidArgument("upload_part", "image upload requires session ID, part 0, and reader")
	}
	c.uploadMu.Lock()
	state := c.uploads[sessionID]
	if state == nil {
		c.uploadMu.Unlock()
		return nil, wrapError("upload_part", socialhub.CodeNotFound, socialhub.ClassPermanent, nil)
	}
	if state.uploading || state.media != nil {
		c.uploadMu.Unlock()
		return nil, wrapError("upload_part", socialhub.CodeConflict, socialhub.ClassPermanent, nil)
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
	done := make(chan error, 1)
	counting := &countingReader{reader: reader}
	go func() {
		header := make(textproto.MIMEHeader)
		header.Set("Content-Disposition", `form-data; name="pic"; filename="`+safeFilename(state.request.Filename)+`"`)
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
	request, err := c.transport.NewRequest(ctx, http.MethodPost, "/2/statuses/upload_pic.json", nil, pipeReader, options...)
	if err != nil {
		_ = pipeReader.Close()
		_ = <-done
		return nil, err
	}
	request.Header.Set("Content-Type", writer.FormDataContentType())
	var response pictureUploadResponse
	err = c.transport.Do(request, &response)
	writeErr := <-done
	if err != nil {
		return nil, err
	}
	if writeErr != nil {
		return nil, wrapError("upload_part", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, writeErr)
	}
	if err := response.APIError.Err("upload_part", http.StatusOK, nil); err != nil {
		return nil, err
	}
	if response.PicID == "" {
		return nil, wrapError("upload_part", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	if counting.count != state.request.Size {
		return nil, invalidArgument("upload_part", "uploaded byte count does not match declared size")
	}
	size := state.request.Size
	media := &socialhub.Media{ID: response.PicID, URL: firstNonEmpty(response.OriginalPic, response.BmiddlePic, response.ThumbnailPic), MIME: state.request.MIME, Type: socialhub.MediaTypeImage, Size: &size, State: socialhub.MediaStateReady}
	c.uploadMu.Lock()
	state.media = media
	c.uploadMu.Unlock()
	return &socialhub.UploadedPart{Number: 0, ETag: response.PicID, Size: size}, nil
}

func (c *Client) CompleteUpload(_ context.Context, sessionID string, parts []socialhub.UploadedPart, _ ...socialhub.CallOption) (*socialhub.Media, error) {
	if sessionID == "" || len(parts) != 1 || parts[0].Number != 0 {
		return nil, invalidArgument("complete_upload", "exactly upload part 0 is required")
	}
	c.uploadMu.Lock()
	defer c.uploadMu.Unlock()
	state := c.uploads[sessionID]
	if state == nil {
		return nil, wrapError("complete_upload", socialhub.CodeNotFound, socialhub.ClassPermanent, nil)
	}
	if state.media == nil || parts[0].ETag != state.media.ID {
		return nil, wrapError("complete_upload", socialhub.CodeConflict, socialhub.ClassPermanent, nil)
	}
	media := *state.media
	delete(c.uploads, sessionID)
	c.uploads[media.ID] = &uploadState{request: state.request, media: &media}
	return &media, nil
}

func (c *Client) MediaStatus(_ context.Context, mediaID string, _ ...socialhub.CallOption) (*socialhub.Media, error) {
	if mediaID == "" {
		return nil, invalidArgument("media_status", "media ID is required")
	}
	c.uploadMu.Lock()
	defer c.uploadMu.Unlock()
	state := c.uploads[mediaID]
	if state == nil || state.media == nil {
		return nil, wrapError("media_status", socialhub.CodeNotFound, socialhub.ClassPermanent, nil)
	}
	media := *state.media
	return &media, nil
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
