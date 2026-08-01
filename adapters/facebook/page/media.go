package page

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

	"social-hub/pkg/socialhub"
)

type uploadState struct {
	request   socialhub.BeginUploadRequest
	media     *socialhub.Media
	uploading bool
}

func (c *Client) BeginUpload(_ context.Context, input socialhub.BeginUploadRequest, _ ...socialhub.CallOption) (*socialhub.UploadSession, error) {
	if input.Type != socialhub.MediaTypeImage {
		return nil, &socialhub.Error{Code: socialhub.CodeUnsupported, Class: socialhub.ClassPermanent, Platform: "facebook", Product: "page", Op: "begin_upload", PlatformMessage: "initial Page adapter supports image uploads only"}
	}
	if input.Filename == "" || input.Size <= 0 || input.MIME == "" {
		return nil, invalidArgument("begin_upload", "filename, positive size, and MIME type are required")
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
		return nil, invalidArgument("upload_part", "Facebook Page photo upload requires session ID, part 0, and reader")
	}
	c.uploadMu.Lock()
	state, found := c.uploads[sessionID]
	if !found {
		c.uploadMu.Unlock()
		return nil, platformError("upload_part", socialhub.CodeNotFound, socialhub.ClassPermanent, nil)
	}
	if state.media != nil {
		c.uploadMu.Unlock()
		return nil, platformError("upload_part", socialhub.CodeConflict, socialhub.ClassPermanent, nil)
	}
	if state.uploading {
		c.uploadMu.Unlock()
		return nil, platformError("upload_part", socialhub.CodeConflict, socialhub.ClassPermanent, nil)
	}
	state.uploading = true
	requestMetadata := state.request
	c.uploadMu.Unlock()

	pipeReader, pipeWriter := io.Pipe()
	writer := multipart.NewWriter(pipeWriter)
	done := make(chan uploadWriteResult, 1)
	go func() {
		result := uploadWriteResult{}
		if err := writer.WriteField("published", "false"); err != nil {
			result.err = err
		} else {
			header := make(textproto.MIMEHeader)
			header.Set("Content-Disposition", fmt.Sprintf(`form-data; name="source"; filename="%s"`, escapeQuotes(requestMetadata.Filename)))
			header.Set("Content-Type", requestMetadata.MIME)
			part, err := writer.CreatePart(header)
			if err != nil {
				result.err = err
			} else {
				result.size, result.err = io.Copy(part, reader)
			}
		}
		if closeErr := writer.Close(); result.err == nil {
			result.err = closeErr
		}
		_ = pipeWriter.CloseWithError(result.err)
		done <- result
	}()

	request, err := c.transport.NewRequest(ctx, http.MethodPost, "/"+url.PathEscape(c.pageID)+"/photos", nil, pipeReader, options...)
	if err != nil {
		_ = pipeReader.Close()
		_ = <-done
		c.setUploadIdle(sessionID)
		return nil, err
	}
	request.Header.Set("Content-Type", writer.FormDataContentType())
	var response idResponse
	err = c.transport.Do(request, &response)
	writeResult := <-done
	if err != nil {
		c.setUploadIdle(sessionID)
		return nil, err
	}
	if writeResult.err != nil {
		c.setUploadIdle(sessionID)
		return nil, platformError("upload_part", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, writeResult.err)
	}
	if writeResult.size != requestMetadata.Size {
		c.setUploadIdle(sessionID)
		return nil, invalidArgument("upload_part", "uploaded byte count does not match declared size")
	}
	media := &socialhub.Media{ID: response.ID, MIME: requestMetadata.MIME, Type: socialhub.MediaTypeImage, Size: &writeResult.size, State: socialhub.MediaStateReady}
	c.uploadMu.Lock()
	state.media = media
	state.uploading = false
	c.uploadMu.Unlock()
	return &socialhub.UploadedPart{Number: 0, ETag: response.ID, Size: writeResult.size}, nil
}

func (c *Client) CompleteUpload(_ context.Context, sessionID string, parts []socialhub.UploadedPart, _ ...socialhub.CallOption) (*socialhub.Media, error) {
	if sessionID == "" || len(parts) != 1 || parts[0].Number != 0 {
		return nil, invalidArgument("complete_upload", "exactly upload part 0 is required")
	}
	c.uploadMu.Lock()
	state, found := c.uploads[sessionID]
	if !found {
		c.uploadMu.Unlock()
		return nil, platformError("complete_upload", socialhub.CodeNotFound, socialhub.ClassPermanent, nil)
	}
	if state.media == nil {
		c.uploadMu.Unlock()
		return nil, platformError("complete_upload", socialhub.CodeConflict, socialhub.ClassPermanent, nil)
	}
	media := *state.media
	delete(c.uploads, sessionID)
	c.uploads[media.ID] = &uploadState{request: state.request, media: &media}
	c.uploadMu.Unlock()
	return &media, nil
}

func (c *Client) MediaStatus(_ context.Context, mediaID string, _ ...socialhub.CallOption) (*socialhub.Media, error) {
	if mediaID == "" {
		return nil, invalidArgument("media_status", "media ID is required")
	}
	c.uploadMu.Lock()
	defer c.uploadMu.Unlock()
	state, found := c.uploads[mediaID]
	if !found || state.media == nil {
		return nil, platformError("media_status", socialhub.CodeNotFound, socialhub.ClassPermanent, nil)
	}
	copy := *state.media
	return &copy, nil
}

type uploadWriteResult struct {
	size int64
	err  error
}

func escapeQuotes(value string) string {
	value = strings.ReplaceAll(value, "\r", "_")
	value = strings.ReplaceAll(value, "\n", "_")
	value = strings.ReplaceAll(value, "\\", "\\\\")
	return strings.ReplaceAll(value, "\"", "\\\"")
}

func (c *Client) setUploadIdle(sessionID string) {
	c.uploadMu.Lock()
	defer c.uploadMu.Unlock()
	if state := c.uploads[sessionID]; state != nil {
		state.uploading = false
	}
}
