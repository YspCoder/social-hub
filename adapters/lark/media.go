package lark

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"strings"

	"social-hub/pkg/socialhub"
)

const (
	maxImageBytes int64 = 10 << 20
	maxFileBytes  int64 = 30 << 20
)

type uploadState struct {
	request   socialhub.BeginUploadRequest
	uploading bool
	uploaded  bool
	key       string
	media     socialhub.Media
}

func (c *Client) BeginUpload(_ context.Context, input socialhub.BeginUploadRequest, options ...socialhub.CallOption) (*socialhub.UploadSession, error) {
	if c.tokenKind != TokenTenant {
		return nil, unsupported("im.resource.begin", "IM resource upload requires tenant_access_token")
	}
	if err := c.requireAnyScope("im.resource.begin", "im:resource", "im:resource:upload"); err != nil {
		return nil, err
	}
	resolved, err := socialhub.ResolveCallOptions(options...)
	if err != nil {
		return nil, err
	}
	if resolved.IdempotencyKey != "" || len(resolved.Fields) > 0 {
		return nil, unsupported("im.resource.begin", "resource allocation has no remote request or idempotency/field-selection contract")
	}
	input.Filename = strings.TrimSpace(input.Filename)
	if !validFilename(input.Filename) || input.Size <= 0 {
		return nil, invalidArgument("im.resource.begin", "filename and positive size are required")
	}
	if input.MIME != "" && !validMIME(input.MIME) {
		return nil, invalidArgument("im.resource.begin", "MIME type must be bounded and contain no control characters")
	}
	limit := maxFileBytes
	if input.Type == socialhub.MediaTypeImage || input.Type == socialhub.MediaTypeAnimation {
		limit = maxImageBytes
		if input.Category != "" && input.Category != "message" {
			return nil, invalidArgument("im.resource.begin", "image category must be empty or message")
		}
	} else if _, err := larkFileType(input); err != nil {
		return nil, err
	}
	if input.Size > limit {
		return nil, invalidArgument("im.resource.begin", "resource exceeds the Open Platform size limit")
	}
	random := make([]byte, 18)
	if _, err := rand.Read(random); err != nil {
		return nil, platformError("im.resource.begin", socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
	}
	sessionID := "lark-resource:" + base64.RawURLEncoding.EncodeToString(random)
	c.uploadMu.Lock()
	c.uploads[sessionID] = &uploadState{request: input}
	c.uploadMu.Unlock()
	return &socialhub.UploadSession{ID: sessionID, PartSize: input.Size}, nil
}

func (c *Client) UploadPart(ctx context.Context, sessionID string, partNumber int, reader io.Reader, options ...socialhub.CallOption) (*socialhub.UploadedPart, error) {
	if strings.TrimSpace(sessionID) == "" || partNumber != 0 || reader == nil {
		return nil, invalidArgument("im.resource.upload", "session ID, part 0, and a reader are required")
	}
	resolved, err := socialhub.ResolveCallOptions(options...)
	if err != nil {
		return nil, err
	}
	if resolved.IdempotencyKey != "" || len(resolved.Fields) > 0 {
		return nil, unsupported("im.resource.upload", "IM resource upload does not document idempotency or field selection")
	}
	c.uploadMu.Lock()
	state := c.uploads[sessionID]
	if state == nil {
		c.uploadMu.Unlock()
		return nil, platformError("im.resource.upload", socialhub.CodeNotFound, socialhub.ClassPermanent, nil)
	}
	if state.uploading || state.uploaded {
		c.uploadMu.Unlock()
		return nil, platformError("im.resource.upload", socialhub.CodeConflict, socialhub.ClassPermanent, nil)
	}
	state.uploading = true
	request := state.request
	c.uploadMu.Unlock()
	defer func() {
		c.uploadMu.Lock()
		state.uploading = false
		c.uploadMu.Unlock()
	}()

	content, err := io.ReadAll(io.LimitReader(reader, request.Size+1))
	if err != nil {
		return nil, platformError("im.resource.upload", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	if int64(len(content)) != request.Size {
		return nil, invalidArgument("im.resource.upload", "uploaded byte count does not match declared size")
	}
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	path, keyField := "/open-apis/im/v1/files", "file_key"
	if request.Type == socialhub.MediaTypeImage || request.Type == socialhub.MediaTypeAnimation {
		path, keyField = "/open-apis/im/v1/images", "image_key"
		_ = writer.WriteField("image_type", "message")
	} else {
		fileType, _ := larkFileType(request)
		_ = writer.WriteField("file_type", fileType)
		_ = writer.WriteField("file_name", request.Filename)
	}
	header := make(textproto.MIMEHeader)
	field := "file"
	if keyField == "image_key" {
		field = "image"
	}
	header.Set("Content-Disposition", `form-data; name="`+field+`"; filename="`+safeFilename(request.Filename)+`"`)
	header.Set("Content-Type", firstNonEmpty(request.MIME, "application/octet-stream"))
	part, err := writer.CreatePart(header)
	if err == nil {
		_, err = part.Write(content)
	}
	if closeErr := writer.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return nil, platformError("im.resource.upload", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	httpRequest, err := c.api.NewRequest(ctx, http.MethodPost, path, nil, bytes.NewReader(body.Bytes()), cleanCallOptions(resolved)...)
	if err != nil {
		return nil, err
	}
	httpRequest.Header.Set("Content-Type", writer.FormDataContentType())
	httpRequest.Header.Del("Idempotency-Key")
	var raw json.RawMessage
	metadata, err := c.api.DoWithMetadata(httpRequest, &raw)
	if err != nil {
		return nil, operationError(err, "im.resource.upload")
	}
	var envelope apiEnvelope
	if len(raw) == 0 || json.Unmarshal(raw, &envelope) != nil || envelope.Code == nil {
		return nil, platformError("im.resource.upload", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	if *envelope.Code != 0 {
		return nil, apiResponseError("im.resource.upload", metadata.StatusCode, metadata.Header, envelope)
	}
	var response struct {
		Data map[string]string `json:"data"`
	}
	if json.Unmarshal(raw, &response) != nil || !validOpaqueID(response.Data[keyField], 512) {
		return nil, platformError("im.resource.upload", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	key := response.Data[keyField]
	media := socialhub.Media{ID: key, MIME: request.MIME, Type: request.Type, Size: int64Pointer(request.Size), State: socialhub.MediaStateReady}
	extension, _ := json.Marshal(map[string]string{
		"message_type": messageTypeForMedia(request.Type), "filename": request.Filename, "resource_key": key,
	})
	media.Extensions = map[string]json.RawMessage{"lark.resource": extension}
	c.uploadMu.Lock()
	state.uploaded, state.key, state.media = true, key, media
	c.uploadMu.Unlock()
	return &socialhub.UploadedPart{Number: 0, ETag: key, Size: request.Size}, nil
}

func (c *Client) CompleteUpload(_ context.Context, sessionID string, parts []socialhub.UploadedPart, options ...socialhub.CallOption) (*socialhub.Media, error) {
	resolved, err := socialhub.ResolveCallOptions(options...)
	if err != nil {
		return nil, err
	}
	if resolved.IdempotencyKey != "" || len(resolved.Fields) > 0 {
		return nil, unsupported("im.resource.complete", "resource completion is local and accepts no idempotency or field selection")
	}
	if strings.TrimSpace(sessionID) == "" || len(parts) != 1 || parts[0].Number != 0 {
		return nil, invalidArgument("im.resource.complete", "exactly upload part 0 is required")
	}
	c.uploadMu.Lock()
	defer c.uploadMu.Unlock()
	state := c.uploads[sessionID]
	if state == nil {
		return nil, platformError("im.resource.complete", socialhub.CodeNotFound, socialhub.ClassPermanent, nil)
	}
	if state.uploading || !state.uploaded || parts[0].ETag != state.key || parts[0].Size != state.request.Size {
		return nil, platformError("im.resource.complete", socialhub.CodeConflict, socialhub.ClassPermanent, nil)
	}
	media := state.media
	delete(c.uploads, sessionID)
	c.media[media.ID] = media
	return &media, nil
}

func (c *Client) MediaStatus(_ context.Context, mediaID string, options ...socialhub.CallOption) (*socialhub.Media, error) {
	resolved, err := socialhub.ResolveCallOptions(options...)
	if err != nil {
		return nil, err
	}
	if resolved.IdempotencyKey != "" || len(resolved.Fields) > 0 {
		return nil, unsupported("im.resource.status", "local resource status accepts no idempotency or field selection")
	}
	mediaID = strings.TrimSpace(mediaID)
	if !validOpaqueID(mediaID, 512) {
		return nil, invalidArgument("im.resource.status", "media_id must be a bounded resource key")
	}
	c.uploadMu.Lock()
	media, found := c.media[mediaID]
	c.uploadMu.Unlock()
	if !found {
		return nil, platformError("im.resource.status", socialhub.CodeNotFound, socialhub.ClassPermanent, nil)
	}
	copy := media
	return &copy, nil
}

func larkFileType(input socialhub.BeginUploadRequest) (string, error) {
	if input.Category != "" {
		switch input.Category {
		case "opus", "mp4", "pdf", "doc", "xls", "ppt", "stream":
			return input.Category, nil
		default:
			return "", invalidArgument("im.resource.begin", "file category must be opus, mp4, pdf, doc, xls, ppt, or stream")
		}
	}
	switch {
	case input.MIME == "audio/ogg" || input.MIME == "audio/opus":
		return "opus", nil
	case input.MIME == "video/mp4":
		return "mp4", nil
	case input.MIME == "application/pdf":
		return "pdf", nil
	case strings.Contains(input.MIME, "word"):
		return "doc", nil
	case strings.Contains(input.MIME, "sheet") || strings.Contains(input.MIME, "excel"):
		return "xls", nil
	case strings.Contains(input.MIME, "presentation") || strings.Contains(input.MIME, "powerpoint"):
		return "ppt", nil
	default:
		return "stream", nil
	}
}

func messageTypeForMedia(mediaType socialhub.MediaType) string {
	switch mediaType {
	case socialhub.MediaTypeImage, socialhub.MediaTypeAnimation:
		return "image"
	case socialhub.MediaTypeAudio:
		return "audio"
	case socialhub.MediaTypeVideo:
		return "media"
	default:
		return "file"
	}
}

func validFilename(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" || len([]rune(value)) > 255 || strings.ContainsAny(value, "\\/") {
		return false
	}
	for _, character := range value {
		if character < 0x20 || character == 0x7f {
			return false
		}
	}
	return true
}

func validMIME(value string) bool {
	if value != strings.TrimSpace(value) || value == "" || len(value) > 255 {
		return false
	}
	for _, character := range value {
		if character <= 0x20 || character == 0x7f {
			return false
		}
	}
	return true
}

func safeFilename(value string) string {
	value = strings.ReplaceAll(value, "\\", "\\\\")
	return strings.ReplaceAll(value, `"`, `\"`)
}

func int64Pointer(value int64) *int64 { return &value }
