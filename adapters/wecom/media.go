package wecom

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"strings"
	"time"

	"social-hub/pkg/socialhub"
)

const (
	minimumMediaBytes int64 = 6
	maxImageBytes           = 10 << 20
	maxVoiceBytes           = 2 << 20
	maxVideoBytes           = 10 << 20
	maxFileBytes            = 20 << 20
)

type uploadState struct {
	request   socialhub.BeginUploadRequest
	media     *socialhub.Media
	uploading bool
}

func (c *Client) BeginUpload(_ context.Context, input socialhub.BeginUploadRequest, _ ...socialhub.CallOption) (*socialhub.UploadSession, error) {
	if err := validateMediaDescriptor(input); err != nil {
		return nil, err
	}
	random := make([]byte, 18)
	if _, err := rand.Read(random); err != nil {
		return nil, platformError("begin_upload", socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
	}
	sessionID := "direct:" + base64.RawURLEncoding.EncodeToString(random)
	c.uploadMu.Lock()
	c.uploads[sessionID] = &uploadState{request: input}
	c.uploadMu.Unlock()
	return &socialhub.UploadSession{ID: sessionID, PartSize: input.Size}, nil
}

func (c *Client) UploadPart(ctx context.Context, sessionID string, partNumber int, reader io.Reader, options ...socialhub.CallOption) (*socialhub.UploadedPart, error) {
	if sessionID == "" || partNumber != 0 || reader == nil {
		return nil, invalidArgument("upload_part", "temporary media upload requires session ID, part 0, and reader")
	}
	c.uploadMu.Lock()
	state := c.uploads[sessionID]
	if state == nil {
		c.uploadMu.Unlock()
		return nil, platformError("upload_part", socialhub.CodeNotFound, socialhub.ClassPermanent, nil)
	}
	if state.uploading || state.media != nil {
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
	counting := &countingReader{reader: io.LimitReader(reader, state.request.Size+1)}
	media, err := c.uploadTemporary(ctx, state.request, counting, options...)
	if err != nil {
		return nil, err
	}
	if counting.count != state.request.Size {
		return nil, invalidArgument("upload_part", "uploaded byte count does not match declared size")
	}
	c.uploadMu.Lock()
	state.media = media
	c.uploadMu.Unlock()
	return &socialhub.UploadedPart{Number: 0, ETag: media.ID, Size: state.request.Size}, nil
}

func (c *Client) CompleteUpload(_ context.Context, sessionID string, parts []socialhub.UploadedPart, _ ...socialhub.CallOption) (*socialhub.Media, error) {
	if sessionID == "" || len(parts) != 1 || parts[0].Number != 0 {
		return nil, invalidArgument("complete_upload", "exactly upload part 0 is required")
	}
	c.uploadMu.Lock()
	defer c.uploadMu.Unlock()
	state := c.uploads[sessionID]
	if state == nil {
		return nil, platformError("complete_upload", socialhub.CodeNotFound, socialhub.ClassPermanent, nil)
	}
	if state.media == nil || parts[0].ETag != state.media.ID || parts[0].Size != state.request.Size {
		return nil, platformError("complete_upload", socialhub.CodeConflict, socialhub.ClassPermanent, nil)
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
		return nil, platformError("media_status", socialhub.CodeNotFound, socialhub.ClassPermanent, nil)
	}
	copy := *state.media
	return &copy, nil
}

func (c *Client) uploadTemporary(ctx context.Context, descriptor socialhub.BeginUploadRequest, reader io.Reader, options ...socialhub.CallOption) (*socialhub.Media, error) {
	typeName, _, err := mediaContract(descriptor.Type, descriptor.MIME)
	if err != nil {
		return nil, err
	}
	pipeReader, pipeWriter := io.Pipe()
	writer := multipart.NewWriter(pipeWriter)
	done := make(chan error, 1)
	go func() {
		header := make(textproto.MIMEHeader)
		header.Set("Content-Disposition", fmt.Sprintf(`form-data; name="media"; filename="%s"`, safeFilename(descriptor.Filename)))
		header.Set("Content-Type", descriptor.MIME)
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
	request, err := c.api.NewRequest(ctx, http.MethodPost, "/cgi-bin/media/upload", url.Values{"type": {typeName}}, pipeReader, options...)
	if err != nil {
		_ = pipeReader.Close()
		_ = <-done
		return nil, err
	}
	request.Header.Set("Content-Type", writer.FormDataContentType())
	var response struct {
		APIResponse
		Type      string `json:"type"`
		MediaID   string `json:"media_id"`
		CreatedAt int64  `json:"created_at"`
	}
	requestErr := c.api.Do(request, &response)
	writeErr := <-done
	if requestErr != nil {
		return nil, requestErr
	}
	if writeErr != nil {
		return nil, platformError("upload_part", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, writeErr)
	}
	if err := c.responseError(ctx, "upload_part", response.APIResponse); err != nil {
		return nil, err
	}
	if response.MediaID == "" {
		return nil, platformError("upload_part", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	createdAt := c.clock.Now()
	if response.CreatedAt > 0 {
		createdAt = time.Unix(response.CreatedAt, 0)
	}
	expiresAt := createdAt.Add(3 * 24 * time.Hour)
	return &socialhub.Media{
		ID: response.MediaID, MIME: descriptor.MIME, Type: descriptor.Type, Size: &descriptor.Size,
		State: socialhub.MediaStateReady, ExpiresAt: &expiresAt,
	}, nil
}

type countingReader struct {
	reader io.Reader
	count  int64
}

func (reader *countingReader) Read(buffer []byte) (int, error) {
	count, err := reader.reader.Read(buffer)
	reader.count += int64(count)
	return count, err
}

func validateMediaDescriptor(input socialhub.BeginUploadRequest) error {
	if input.Filename == "" || strings.TrimSpace(input.Filename) != input.Filename || strings.ContainsAny(input.Filename, "\r\n") || input.MIME == "" {
		return invalidArgument("begin_upload", "filename and MIME type are required")
	}
	_, maximum, err := mediaContract(input.Type, input.MIME)
	if err != nil {
		return err
	}
	if input.Size < minimumMediaBytes || input.Size > maximum {
		return invalidArgument("begin_upload", "media size is outside the documented range for its type")
	}
	return nil
}

func mediaContract(mediaType socialhub.MediaType, mimeType string) (string, int64, error) {
	switch mediaType {
	case socialhub.MediaTypeImage:
		if mimeType != "image/jpeg" && mimeType != "image/png" {
			return "", 0, invalidArgument("media_type", "image media must be JPG or PNG")
		}
		return "image", maxImageBytes, nil
	case socialhub.MediaTypeAudio:
		if mimeType != "audio/amr" && mimeType != "audio/x-amr" {
			return "", 0, invalidArgument("media_type", "voice media must be AMR")
		}
		return "voice", maxVoiceBytes, nil
	case socialhub.MediaTypeVideo:
		if mimeType != "video/mp4" {
			return "", 0, invalidArgument("media_type", "video media must be MP4")
		}
		return "video", maxVideoBytes, nil
	case socialhub.MediaTypeDocument:
		return "file", maxFileBytes, nil
	default:
		return "", 0, unsupported("media_type", "WeCom temporary media supports image, voice, video, and file")
	}
}

func safeFilename(value string) string {
	value = strings.ReplaceAll(value, "\\", "\\\\")
	return strings.ReplaceAll(value, "\"", "\\\"")
}
