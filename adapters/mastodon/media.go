package mastodon

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"strings"

	"social-hub/pkg/socialhub"
)

type uploadSession struct {
	request    socialhub.BeginUploadRequest
	attachment *mastodonAttachment
	uploading  bool
	uploaded   bool
}

func (c *Client) BeginUpload(_ context.Context, input socialhub.BeginUploadRequest, _ ...socialhub.CallOption) (*socialhub.UploadSession, error) {
	if err := c.requireScopes("begin_upload", "write:media"); err != nil {
		return nil, err
	}
	if strings.TrimSpace(input.Filename) == "" || strings.TrimSpace(input.MIME) == "" || input.Size <= 0 {
		return nil, invalidArgument("begin_upload", "filename, MIME type, and positive size are required")
	}
	switch input.Type {
	case socialhub.MediaTypeImage, socialhub.MediaTypeVideo, socialhub.MediaTypeAnimation, socialhub.MediaTypeAudio:
	default:
		return nil, invalidArgument("begin_upload", "unsupported Mastodon media type")
	}
	sessionID, err := newUploadID()
	if err != nil {
		return nil, platformError("begin_upload", socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	c.uploadMu.Lock()
	c.uploads[sessionID] = &uploadSession{request: input}
	c.uploadMu.Unlock()
	return &socialhub.UploadSession{ID: sessionID, PartSize: input.Size}, nil
}

func (c *Client) UploadPart(ctx context.Context, sessionID string, partNumber int, reader io.Reader, options ...socialhub.CallOption) (*socialhub.UploadedPart, error) {
	if reader == nil || partNumber != 1 {
		return nil, invalidArgument("upload_part", "Mastodon uploads require exactly one non-nil part numbered 1")
	}
	c.uploadMu.Lock()
	session := c.uploads[sessionID]
	if session == nil || session.uploading || session.uploaded {
		c.uploadMu.Unlock()
		return nil, invalidArgument("upload_part", "unknown or already-used upload session")
	}
	session.uploading = true
	request := session.request
	c.uploadMu.Unlock()

	attachment, err := c.uploadAttachment(ctx, request, reader, options...)
	c.uploadMu.Lock()
	session.uploading = false
	if err == nil {
		session.attachment, session.uploaded = attachment, true
	}
	c.uploadMu.Unlock()
	if err != nil {
		return nil, err
	}
	return &socialhub.UploadedPart{Number: 1, Size: request.Size}, nil
}

func (c *Client) CompleteUpload(_ context.Context, sessionID string, parts []socialhub.UploadedPart, _ ...socialhub.CallOption) (*socialhub.Media, error) {
	if len(parts) != 1 || parts[0].Number != 1 {
		return nil, invalidArgument("complete_upload", "exactly one uploaded part numbered 1 is required")
	}
	c.uploadMu.Lock()
	defer c.uploadMu.Unlock()
	session := c.uploads[sessionID]
	if session == nil || !session.uploaded || session.attachment == nil || parts[0].Size != session.request.Size {
		return nil, invalidArgument("complete_upload", "upload session or part size does not match")
	}
	media := mapAttachment(*session.attachment)
	delete(c.uploads, sessionID)
	return &media, nil
}

func (c *Client) MediaStatus(ctx context.Context, mediaID string, options ...socialhub.CallOption) (*socialhub.Media, error) {
	if strings.TrimSpace(mediaID) == "" {
		return nil, invalidArgument("media_status", "media ID is required")
	}
	if err := c.requireScopes("media_status", "write:media"); err != nil {
		return nil, err
	}
	var response mastodonAttachment
	if err := c.transport.JSON(ctx, http.MethodGet, "/api/v1/media/"+url.PathEscape(mediaID), nil, nil, &response, options...); err != nil {
		return nil, err
	}
	if response.ID != mediaID {
		return nil, platformError("media_status", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	media := mapAttachment(response)
	return &media, nil
}

func (c *Client) uploadAttachment(ctx context.Context, input socialhub.BeginUploadRequest, source io.Reader, options ...socialhub.CallOption) (*mastodonAttachment, error) {
	reader, writer := io.Pipe()
	multipartWriter := multipart.NewWriter(writer)
	request, err := c.transport.NewRequest(ctx, http.MethodPost, "/api/v2/media", nil, reader, options...)
	if err != nil {
		_ = reader.Close()
		_ = writer.CloseWithError(err)
		return nil, err
	}
	request.Header.Set("Content-Type", multipartWriter.FormDataContentType())
	go writeMediaPart(writer, multipartWriter, input, source)
	defer reader.Close()
	var response mastodonAttachment
	if _, err := c.transport.DoWithMetadata(request, &response); err != nil {
		return nil, err
	}
	if response.ID == "" {
		return nil, platformError("upload_part", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return &response, nil
}

func writeMediaPart(pipe *io.PipeWriter, writer *multipart.Writer, input socialhub.BeginUploadRequest, source io.Reader) {
	disposition := mime.FormatMediaType("form-data", map[string]string{"name": "file", "filename": input.Filename})
	header := textproto.MIMEHeader{"Content-Disposition": {disposition}, "Content-Type": {input.MIME}}
	part, err := writer.CreatePart(header)
	if err != nil {
		_ = pipe.CloseWithError(err)
		return
	}
	written, err := io.Copy(part, io.LimitReader(source, input.Size+1))
	if err == nil && written != input.Size {
		err = fmt.Errorf("mastodon: media reader size does not match declared size")
	}
	if err != nil {
		_ = pipe.CloseWithError(err)
		return
	}
	if err := writer.Close(); err != nil {
		_ = pipe.CloseWithError(err)
		return
	}
	_ = pipe.Close()
}

func newUploadID() (string, error) {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return hex.EncodeToString(value), nil
}
