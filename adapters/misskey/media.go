package misskey

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
	"strconv"
	"strings"

	"social-hub/pkg/socialhub"
)

type uploadSession struct {
	request   DriveUploadRequest
	file      *misskeyDriveFile
	uploading bool
	uploaded  bool
}

func (c *Client) BeginUpload(ctx context.Context, input socialhub.BeginUploadRequest, options ...socialhub.CallOption) (*socialhub.UploadSession, error) {
	return c.BeginDriveUpload(ctx, DriveUploadRequest{Upload: input}, options...)
}

func (c *Client) BeginDriveUpload(_ context.Context, input DriveUploadRequest, _ ...socialhub.CallOption) (*socialhub.UploadSession, error) {
	if err := c.requirePermissions("begin_upload", "write:drive"); err != nil {
		return nil, err
	}
	if err := validateDriveUpload(input); err != nil {
		return nil, err
	}
	sessionID, err := newUploadID()
	if err != nil {
		return nil, platformError("begin_upload", socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	c.uploadMu.Lock()
	c.uploads[sessionID] = &uploadSession{request: input}
	c.uploadMu.Unlock()
	return &socialhub.UploadSession{ID: sessionID, PartSize: input.Upload.Size}, nil
}

func validateDriveUpload(input DriveUploadRequest) error {
	request := input.Upload
	if !validBoundedString(request.Filename, 4096) || request.Size <= 0 {
		return invalidArgument("begin_upload", "filename and positive size are required")
	}
	mediaType, _, err := mime.ParseMediaType(request.MIME)
	if err != nil || !strings.Contains(mediaType, "/") {
		return invalidArgument("begin_upload", "valid MIME type is required")
	}
	switch request.Type {
	case socialhub.MediaTypeImage, socialhub.MediaTypeVideo, socialhub.MediaTypeAudio,
		socialhub.MediaTypeDocument, socialhub.MediaTypeAnimation:
	default:
		return invalidArgument("begin_upload", "media type is invalid")
	}
	if input.FolderID != "" && !validID(input.FolderID) {
		return invalidArgument("begin_upload", "folder ID is invalid")
	}
	if input.Comment != "" && !validContentString(input.Comment, 4096) {
		return invalidArgument("begin_upload", "comment is invalid")
	}
	return nil
}

func (c *Client) UploadPart(ctx context.Context, sessionID string, partNumber int, source io.Reader, options ...socialhub.CallOption) (*socialhub.UploadedPart, error) {
	if source == nil || partNumber != 1 {
		return nil, invalidArgument("upload_part", "Misskey Drive uploads require one non-nil part numbered 1")
	}
	c.uploadMu.Lock()
	session := c.uploads[sessionID]
	if session == nil || session.uploading || session.uploaded {
		c.uploadMu.Unlock()
		return nil, invalidArgument("upload_part", "unknown or already-used upload session")
	}
	session.uploading = true
	input := session.request
	c.uploadMu.Unlock()

	file, err := c.uploadDriveFile(ctx, input, source, options...)
	c.uploadMu.Lock()
	session.uploading = false
	if err == nil {
		session.file, session.uploaded = file, true
	}
	c.uploadMu.Unlock()
	if err != nil {
		return nil, err
	}
	return &socialhub.UploadedPart{Number: 1, Size: input.Upload.Size}, nil
}

func (c *Client) uploadDriveFile(ctx context.Context, input DriveUploadRequest, source io.Reader, options ...socialhub.CallOption) (*misskeyDriveFile, error) {
	reader, writer := io.Pipe()
	multipartWriter := multipart.NewWriter(writer)
	request, err := c.api.NewRequest(ctx, http.MethodPost, "/drive/files/create", nil, reader, options...)
	if err != nil {
		_ = reader.Close()
		_ = writer.CloseWithError(err)
		return nil, err
	}
	request.Header.Set("Content-Type", multipartWriter.FormDataContentType())
	go writeDriveMultipart(writer, multipartWriter, input, source)
	defer reader.Close()
	var response misskeyDriveFile
	if err := c.api.Do(request, &response); err != nil {
		return nil, err
	}
	if response.ID == "" {
		return nil, platformError("upload_part", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return &response, nil
}

func writeDriveMultipart(pipe *io.PipeWriter, writer *multipart.Writer, input DriveUploadRequest, source io.Reader) {
	fields := map[string]string{
		"name": input.Upload.Filename, "isSensitive": strconv.FormatBool(input.Sensitive),
		"force": strconv.FormatBool(input.Force),
	}
	if input.FolderID != "" {
		fields["folderId"] = input.FolderID
	}
	if input.Comment != "" {
		fields["comment"] = input.Comment
	}
	for name, value := range fields {
		if err := writer.WriteField(name, value); err != nil {
			_ = pipe.CloseWithError(err)
			return
		}
	}
	disposition := mime.FormatMediaType("form-data", map[string]string{"name": "file", "filename": input.Upload.Filename})
	header := textproto.MIMEHeader{"Content-Disposition": {disposition}, "Content-Type": {input.Upload.MIME}}
	part, err := writer.CreatePart(header)
	if err != nil {
		_ = pipe.CloseWithError(err)
		return
	}
	written, err := io.Copy(part, io.LimitReader(source, input.Upload.Size+1))
	if err == nil && written != input.Upload.Size {
		err = fmt.Errorf("misskey: media reader size does not match declared size")
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

func (c *Client) CompleteUpload(_ context.Context, sessionID string, parts []socialhub.UploadedPart, _ ...socialhub.CallOption) (*socialhub.Media, error) {
	if len(parts) != 1 || parts[0].Number != 1 {
		return nil, invalidArgument("complete_upload", "exactly one uploaded part numbered 1 is required")
	}
	c.uploadMu.Lock()
	defer c.uploadMu.Unlock()
	session := c.uploads[sessionID]
	if session == nil || !session.uploaded || session.file == nil || parts[0].Size != session.request.Upload.Size {
		return nil, invalidArgument("complete_upload", "upload session or part size does not match")
	}
	media, err := mapDriveFile(*session.file)
	if err != nil {
		return nil, err
	}
	delete(c.uploads, sessionID)
	return media, nil
}

func (c *Client) MediaStatus(ctx context.Context, mediaID string, options ...socialhub.CallOption) (*socialhub.Media, error) {
	if !validID(mediaID) {
		return nil, invalidArgument("media_status", "Drive file ID is invalid")
	}
	if err := c.requirePermissions("media_status", "read:drive"); err != nil {
		return nil, err
	}
	var response misskeyDriveFile
	if err := c.post(ctx, "drive/files/show", struct {
		FileID string `json:"fileId"`
	}{FileID: mediaID}, &response, options...); err != nil {
		return nil, err
	}
	if response.ID != mediaID {
		return nil, platformError("media_status", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return mapDriveFile(response)
}

func newUploadID() (string, error) {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return hex.EncodeToString(value), nil
}
