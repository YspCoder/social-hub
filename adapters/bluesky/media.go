package bluesky

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"social-hub/pkg/socialhub"
)

const (
	maxImageBlobSize int64 = 2_000_000
	maxVideoBlobSize int64 = 100_000_000
)

var errMediaSizeMismatch = errors.New("media reader size does not match declared size")

type uploadSession struct {
	request   socialhub.BeginUploadRequest
	blob      *blobRef
	uploading bool
	uploaded  bool
}

func (c *Client) BeginUpload(_ context.Context, input socialhub.BeginUploadRequest, _ ...socialhub.CallOption) (*socialhub.UploadSession, error) {
	if err := validateUpload(input); err != nil {
		return nil, err
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

func validateUpload(input socialhub.BeginUploadRequest) error {
	if strings.TrimSpace(input.Filename) == "" || strings.TrimSpace(input.MIME) == "" || input.Size <= 0 {
		return invalidArgument("begin_upload", "filename, MIME type, and positive size are required")
	}
	switch input.Type {
	case socialhub.MediaTypeImage, socialhub.MediaTypeAnimation:
		if !strings.HasPrefix(input.MIME, "image/") || input.Size > maxImageBlobSize {
			return invalidArgument("begin_upload", "Bluesky images must use image/* and be at most 2,000,000 bytes")
		}
	case socialhub.MediaTypeVideo:
		if input.MIME != "video/mp4" || input.Size > maxVideoBlobSize {
			return invalidArgument("begin_upload", "Bluesky videos must use video/mp4 and be at most 100,000,000 bytes")
		}
	default:
		return invalidArgument("begin_upload", "Bluesky blob upload supports images and MP4 video")
	}
	return nil
}

func (c *Client) UploadPart(ctx context.Context, sessionID string, partNumber int, source io.Reader, options ...socialhub.CallOption) (*socialhub.UploadedPart, error) {
	if source == nil || partNumber != 1 {
		return nil, invalidArgument("upload_part", "Bluesky uploads require exactly one non-nil part numbered 1")
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

	blob, err := c.uploadBlob(ctx, request, source, options...)
	c.uploadMu.Lock()
	session.uploading = false
	if err == nil {
		session.blob, session.uploaded = blob, true
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
	if session == nil || !session.uploaded || session.blob == nil || parts[0].Size != session.request.Size {
		return nil, invalidArgument("complete_upload", "upload session or part size does not match")
	}
	c.blobs[session.blob.Ref.Link] = *session.blob
	media := mapUploadedBlob(*session.blob, PostMedia{MediaID: session.blob.Ref.Link})
	delete(c.uploads, sessionID)
	return &media, nil
}

func (c *Client) MediaStatus(_ context.Context, mediaID string, _ ...socialhub.CallOption) (*socialhub.Media, error) {
	if strings.TrimSpace(mediaID) == "" {
		return nil, invalidArgument("media_status", "blob CID is required")
	}
	c.uploadMu.Lock()
	blob, found := c.blobs[mediaID]
	c.uploadMu.Unlock()
	if !found {
		return nil, platformError("media_status", socialhub.CodeNotFound, socialhub.ClassPermanent, nil)
	}
	media := mapUploadedBlob(blob, PostMedia{MediaID: mediaID})
	return &media, nil
}

func (c *Client) uploadBlob(ctx context.Context, input socialhub.BeginUploadRequest, source io.Reader, options ...socialhub.CallOption) (*blobRef, error) {
	reader, writer := io.Pipe()
	done := make(chan error, 1)
	go func() {
		err := writeExact(writer, source, input.Size)
		_ = writer.CloseWithError(err)
		done <- err
	}()
	request, err := c.transport.NewRequest(ctx, http.MethodPost, "/xrpc/com.atproto.repo.uploadBlob", nil, reader, options...)
	if err != nil {
		_ = reader.CloseWithError(err)
		<-done
		return nil, err
	}
	request.Header.Set("Content-Type", input.MIME)
	request.ContentLength = input.Size
	var response struct {
		Blob blobRef `json:"blob"`
	}
	_, requestErr := c.transport.DoWithMetadata(request, &response)
	_ = reader.Close()
	streamErr := <-done
	if errors.Is(streamErr, errMediaSizeMismatch) {
		return nil, invalidArgument("upload_part", errMediaSizeMismatch.Error())
	}
	if requestErr != nil {
		return nil, requestErr
	}
	if streamErr != nil {
		return nil, platformError("upload_part", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, streamErr)
	}
	if response.Blob.Ref.Link == "" || response.Blob.MIMEType == "" || response.Blob.Size != input.Size {
		return nil, platformError("upload_part", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return &response.Blob, nil
}

func writeExact(writer io.Writer, source io.Reader, size int64) error {
	written, err := io.CopyN(writer, source, size)
	if err != nil || written != size {
		return errMediaSizeMismatch
	}
	var extra [1]byte
	n, err := io.ReadFull(source, extra[:])
	if n > 0 || err == nil {
		return errMediaSizeMismatch
	}
	if !errors.Is(err, io.EOF) {
		return fmt.Errorf("read media source: %w", err)
	}
	return nil
}

func newUploadID() (string, error) {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return hex.EncodeToString(value), nil
}
