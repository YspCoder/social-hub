package lemmy

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"strings"

	"social-hub/pkg/socialhub"
)

var errUploadSizeMismatch = errors.New("lemmy: upload reader size does not match declared size")

type uploadState struct {
	request   socialhub.BeginUploadRequest
	uploading bool
	media     *socialhub.Media
}

type pictrsUploadResponse struct {
	Message string       `json:"msg"`
	Files   []pictrsFile `json:"files"`
}

type pictrsFile struct {
	File        string `json:"file"`
	DeleteToken string `json:"delete_token"`
}

func (client *Client) BeginUpload(_ context.Context, input socialhub.BeginUploadRequest, _ ...socialhub.CallOption) (*socialhub.UploadSession, error) {
	if !validImageUpload(input) {
		return nil, invalidArgument("begin_upload", "filename, image MIME type, supported media type, and positive size are required")
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
		return nil, invalidArgument("upload_part", "Lemmy image uploads require exactly one non-nil part numbered 0")
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

	media, err := client.uploadImage(ctx, input, source, options...)
	client.uploadMu.Lock()
	session.uploading = false
	if err == nil {
		session.media = media
	}
	client.uploadMu.Unlock()
	if err != nil {
		return nil, err
	}
	return &socialhub.UploadedPart{Number: 0, ETag: media.ID, Size: input.Size}, nil
}

func (client *Client) CompleteUpload(_ context.Context, sessionID string, parts []socialhub.UploadedPart, _ ...socialhub.CallOption) (*socialhub.Media, error) {
	if len(parts) != 1 || parts[0].Number != 0 {
		return nil, invalidArgument("complete_upload", "exactly one uploaded part numbered 0 is required")
	}
	client.uploadMu.Lock()
	defer client.uploadMu.Unlock()
	session := client.uploads[sessionID]
	if session == nil || session.uploading || session.media == nil || parts[0].Size != session.request.Size ||
		(parts[0].ETag != "" && parts[0].ETag != session.media.ID) {
		return nil, invalidArgument("complete_upload", "upload session or part metadata does not match")
	}
	media := *session.media
	client.media[media.ID] = media
	delete(client.uploads, sessionID)
	return &media, nil
}

func (client *Client) MediaStatus(_ context.Context, mediaID string, _ ...socialhub.CallOption) (*socialhub.Media, error) {
	if !validMediaID(mediaID) {
		return nil, invalidArgument("media_status", "media ID is invalid")
	}
	client.uploadMu.Lock()
	media, found := client.media[mediaID]
	client.uploadMu.Unlock()
	if !found {
		return nil, unsupported("media_status", "Lemmy API v3 has no Pictrs upload lookup endpoint; only uploads completed by this client are available")
	}
	copy := media
	return &copy, nil
}

func (client *Client) uploadImage(ctx context.Context, input socialhub.BeginUploadRequest, source io.Reader, options ...socialhub.CallOption) (*socialhub.Media, error) {
	reader, pipe := io.Pipe()
	writer := multipart.NewWriter(pipe)
	request, err := client.api.NewRequest(ctx, http.MethodPost, "/pictrs/image", nil, reader, options...)
	if err != nil {
		_ = reader.Close()
		_ = pipe.CloseWithError(err)
		return nil, err
	}
	request.Header.Set("Content-Type", writer.FormDataContentType())
	request.Header.Set("User-Agent", userAgent)
	written := make(chan error, 1)
	go func() { written <- writeImagePart(pipe, writer, input, source) }()

	var response pictrsUploadResponse
	requestErr := client.api.Do(request, &response)
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
	if response.Message == "too_large" {
		return nil, invalidArgument("upload_part", "image exceeds the instance upload limit")
	}
	if response.Message != "ok" || len(response.Files) != 1 || !validMediaID(response.Files[0].File) {
		return nil, platformError("upload_part", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	file := response.Files[0]
	raw, _ := json.Marshal(file)
	size := input.Size
	return &socialhub.Media{
		ID: file.File, URL: client.baseURL + "/pictrs/image/" + url.PathEscape(file.File), MIME: input.MIME,
		Type: input.Type, Size: &size, State: socialhub.MediaStateReady,
		Extensions: map[string]json.RawMessage{"lemmy.pictrs_file": raw},
	}, nil
}

func validImageUpload(input socialhub.BeginUploadRequest) bool {
	if strings.TrimSpace(input.Filename) == "" || strings.ContainsAny(input.Filename, "\x00\r\n") || input.Size <= 0 || input.Category != "" {
		return false
	}
	mediaType, _, err := mime.ParseMediaType(input.MIME)
	if err != nil || !strings.HasPrefix(mediaType, "image/") {
		return false
	}
	return input.Type == socialhub.MediaTypeImage || input.Type == socialhub.MediaTypeAnimation
}

func validMediaID(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > 4096 {
		return false
	}
	for _, character := range value {
		if character < 0x20 || character == 0x7f {
			return false
		}
	}
	return true
}

func writeImagePart(pipe *io.PipeWriter, writer *multipart.Writer, input socialhub.BeginUploadRequest, source io.Reader) error {
	disposition := mime.FormatMediaType("form-data", map[string]string{"name": "images[]", "filename": input.Filename})
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
