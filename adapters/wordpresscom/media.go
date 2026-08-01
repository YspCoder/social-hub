package wordpresscom

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"strings"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

var errUploadSizeMismatch = errors.New("wordpress.com: media reader size does not match declared size")

func (client *Client) BeginUpload(_ context.Context, input socialhub.BeginUploadRequest, _ ...socialhub.CallOption) (*socialhub.UploadSession, error) {
	if _, err := client.requireUser("begin_upload"); err != nil {
		return nil, err
	}
	if err := client.requireScopes("begin_upload", "media"); err != nil {
		return nil, err
	}
	if !validUploadRequest(input) {
		return nil, invalidArgument("begin_upload", "filename, MIME type, supported media type, and positive size are required")
	}
	sessionID, err := newUploadID()
	if err != nil {
		return nil, platformError("begin_upload", socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	client.uploadMu.Lock()
	client.uploads[sessionID] = &uploadState{request: input}
	client.uploadMu.Unlock()
	return &socialhub.UploadSession{ID: sessionID, PartSize: input.Size}, nil
}

func (client *Client) UploadPart(ctx context.Context, sessionID string, partNumber int, source io.Reader, options ...socialhub.CallOption) (*socialhub.UploadedPart, error) {
	if source == nil || partNumber != 0 {
		return nil, invalidArgument("upload_part", "WordPress.com uploads require exactly one non-nil part numbered 0")
	}
	api, err := client.requireUser("upload_part")
	if err != nil {
		return nil, err
	}
	if err := client.requireScopes("upload_part", "media"); err != nil {
		return nil, err
	}

	client.uploadMu.Lock()
	session := client.uploads[sessionID]
	if session == nil || session.uploading || session.media != nil {
		client.uploadMu.Unlock()
		return nil, invalidArgument("upload_part", "unknown or already-used upload session")
	}
	session.uploading = true
	input := session.request
	client.uploadMu.Unlock()

	media, err := client.uploadMedia(ctx, api, input, source, options...)
	client.uploadMu.Lock()
	session.uploading = false
	if err == nil {
		session.media = media
	}
	client.uploadMu.Unlock()
	if err != nil {
		return nil, err
	}
	return &socialhub.UploadedPart{Number: 0, Size: input.Size}, nil
}

func (client *Client) CompleteUpload(_ context.Context, sessionID string, parts []socialhub.UploadedPart, _ ...socialhub.CallOption) (*socialhub.Media, error) {
	if len(parts) != 1 || parts[0].Number != 0 {
		return nil, invalidArgument("complete_upload", "exactly one uploaded part numbered 0 is required")
	}
	client.uploadMu.Lock()
	defer client.uploadMu.Unlock()
	session := client.uploads[sessionID]
	if session == nil || session.uploading || session.media == nil || parts[0].Size != session.request.Size {
		return nil, invalidArgument("complete_upload", "upload session or part size does not match")
	}
	media := *session.media
	delete(client.uploads, sessionID)
	return &media, nil
}

func (client *Client) MediaStatus(ctx context.Context, mediaID string, options ...socialhub.CallOption) (*socialhub.Media, error) {
	if !validID(mediaID) {
		return nil, invalidArgument("media_status", "media ID must be a positive integer")
	}
	api, err := client.requireUser("media_status")
	if err != nil {
		return nil, err
	}
	if err := client.requireScopes("media_status", "media"); err != nil {
		return nil, err
	}
	var response wpMedia
	if err := api.JSON(ctx, http.MethodGet, client.sitePath("media", mediaID), nil, nil, &response, options...); err != nil {
		return nil, err
	}
	if response.ID != mustID(mediaID) {
		return nil, platformError("media_status", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	media := mapMedia(response)
	return &media, nil
}

func (client *Client) DeleteMedia(ctx context.Context, mediaID string, options ...socialhub.CallOption) error {
	if !validID(mediaID) {
		return invalidArgument("delete_media", "media ID must be a positive integer")
	}
	api, err := client.requireUser("delete_media")
	if err != nil {
		return err
	}
	if err := client.requireScopes("delete_media", "media"); err != nil {
		return err
	}
	var response wpMedia
	if err := client.form(ctx, api, client.sitePath("media", mediaID, "delete"), nil, &response, options...); err != nil {
		return err
	}
	if response.ID != mustID(mediaID) {
		return platformError("delete_media", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return nil
}

func (client *Client) uploadMedia(ctx context.Context, api *transport.Client, input socialhub.BeginUploadRequest, source io.Reader, options ...socialhub.CallOption) (*socialhub.Media, error) {
	reader, writer := io.Pipe()
	multipartWriter := multipart.NewWriter(writer)
	request, err := api.NewRequest(ctx, http.MethodPost, client.sitePath("media", "new"), nil, reader, options...)
	if err != nil {
		_ = reader.Close()
		_ = writer.CloseWithError(err)
		return nil, err
	}
	request.Header.Set("Content-Type", multipartWriter.FormDataContentType())
	written := make(chan error, 1)
	go func() { written <- writeUpload(writer, multipartWriter, input, source) }()

	var response mediaUploadResponse
	requestErr := api.Do(request, &response)
	_ = reader.Close()
	writeErr := <-written
	if errors.Is(writeErr, errUploadSizeMismatch) {
		return nil, platformError("upload_part", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, writeErr)
	}
	if requestErr != nil {
		return nil, requestErr
	}
	if writeErr != nil {
		return nil, platformError("upload_part", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, writeErr)
	}
	if len(response.Errors) > 0 {
		return nil, &socialhub.Error{
			Code: socialhub.CodePlatformError, Class: socialhub.ClassPermanent, Platform: "wordpress.com", Product: productName,
			Op: "upload_part", PlatformCode: boundedMessage(response.Errors[0].Error, 256), PlatformMessage: boundedMessage(response.Errors[0].Message, 512),
		}
	}
	if len(response.Media) != 1 || response.Media[0].ID <= 0 || firstNonEmpty(response.Media[0].URL, response.Media[0].GUID) == "" {
		return nil, platformError("upload_part", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	media := mapMedia(response.Media[0])
	size := input.Size
	media.Size = &size
	return &media, nil
}

func validUploadRequest(input socialhub.BeginUploadRequest) bool {
	if strings.TrimSpace(input.Filename) == "" || strings.ContainsAny(input.Filename, "\x00\r\n") || input.Size <= 0 {
		return false
	}
	mediaType, _, err := mime.ParseMediaType(input.MIME)
	if err != nil || !strings.Contains(mediaType, "/") {
		return false
	}
	switch input.Type {
	case socialhub.MediaTypeImage, socialhub.MediaTypeAnimation:
		return strings.HasPrefix(mediaType, "image/")
	case socialhub.MediaTypeVideo:
		return strings.HasPrefix(mediaType, "video/")
	case socialhub.MediaTypeAudio:
		return strings.HasPrefix(mediaType, "audio/")
	case socialhub.MediaTypeDocument:
		return !strings.HasPrefix(mediaType, "image/") && !strings.HasPrefix(mediaType, "video/") && !strings.HasPrefix(mediaType, "audio/")
	default:
		return false
	}
}

func writeUpload(pipe *io.PipeWriter, writer *multipart.Writer, input socialhub.BeginUploadRequest, source io.Reader) error {
	disposition := mime.FormatMediaType("form-data", map[string]string{"name": "media[]", "filename": input.Filename})
	header := textproto.MIMEHeader{"Content-Disposition": {disposition}, "Content-Type": {input.MIME}}
	part, err := writer.CreatePart(header)
	if err == nil {
		var written int64
		written, err = io.Copy(part, io.LimitReader(source, input.Size+1))
		if err == nil && written != input.Size {
			err = errUploadSizeMismatch
		}
	}
	if err == nil {
		err = writer.Close()
	}
	if err != nil {
		_ = pipe.CloseWithError(err)
		return err
	}
	return pipe.Close()
}

func newUploadID() (string, error) {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return hex.EncodeToString(value), nil
}
