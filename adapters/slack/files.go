package slack

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"strings"

	"social-hub/pkg/socialhub"
)

const maxUploadReplyBytes int64 = 1 << 20

type uploadState struct {
	request    FileUploadRequest
	uploadURL  string
	fileID     string
	uploading  bool
	completing bool
	uploaded   bool
}

func (c *Client) BeginUpload(ctx context.Context, input socialhub.BeginUploadRequest, options ...socialhub.CallOption) (*socialhub.UploadSession, error) {
	if input.Category != "" {
		return nil, invalidArgument("begin_upload", "common Slack uploads must be private; use FileWorkflow for channel or thread sharing")
	}
	return c.BeginFileUpload(ctx, FileUploadRequest{
		Filename: input.Filename, Size: input.Size, MIME: input.MIME,
	}, options...)
}

func (c *Client) BeginFileUpload(ctx context.Context, input FileUploadRequest, options ...socialhub.CallOption) (*socialhub.UploadSession, error) {
	if err := c.requireScopes("files.getUploadURLExternal", "files:write"); err != nil {
		return nil, err
	}
	input.Filename = strings.TrimSpace(input.Filename)
	if !validFilename(input.Filename) || input.Size <= 0 {
		return nil, invalidArgument("files.getUploadURLExternal", "filename without path separators and a positive size are required")
	}
	if input.MIME != "" && !validOpaque(input.MIME, 255) {
		return nil, invalidArgument("files.getUploadURLExternal", "MIME type must be bounded and contain no control characters")
	}
	if input.Title != "" && !validText(input.Title, 255) {
		return nil, invalidArgument("files.getUploadURLExternal", "title must contain at most 255 Unicode code points")
	}
	if input.AltText != "" && !validText(input.AltText, 2000) {
		return nil, invalidArgument("files.getUploadURLExternal", "alt text must contain at most 2000 Unicode code points")
	}
	if input.SnippetType != "" && !validOpaque(input.SnippetType, 64) {
		return nil, invalidArgument("files.getUploadURLExternal", "snippet type must be bounded and contain no control characters")
	}
	if input.InitialComment != "" && !validText(input.InitialComment, 40000) {
		return nil, invalidArgument("files.getUploadURLExternal", "initial comment must contain at most 40000 Unicode code points")
	}
	if input.ChannelID != "" && !validSlackID(input.ChannelID, "CGD") {
		return nil, invalidArgument("files.getUploadURLExternal", "channel_id must be a Slack conversation ID")
	}
	if input.ThreadPostID != "" {
		channelID, _, err := parseCompositeID(input.ThreadPostID, "files.getUploadURLExternal")
		if err != nil {
			return nil, err
		}
		if input.ChannelID == "" {
			input.ChannelID = channelID
		} else if input.ChannelID != channelID {
			return nil, invalidArgument("files.getUploadURLExternal", "thread parent must belong to the target conversation")
		}
	}
	var response struct {
		UploadURL string `json:"upload_url"`
		FileID    string `json:"file_id"`
	}
	if err := c.call(ctx, "files.getUploadURLExternal", struct {
		Filename    string `json:"filename"`
		Length      int64  `json:"length"`
		AltText     string `json:"alt_txt,omitempty"`
		SnippetType string `json:"snippet_type,omitempty"`
	}{Filename: input.Filename, Length: input.Size, AltText: input.AltText, SnippetType: input.SnippetType}, &response, options...); err != nil {
		return nil, err
	}
	if !validSlackID(response.FileID, "F") || !validUploadURL(response.UploadURL, c.allowHTTPUploads) {
		return nil, platformError("files.getUploadURLExternal", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	random := make([]byte, 18)
	if _, err := rand.Read(random); err != nil {
		return nil, platformError("files.getUploadURLExternal", socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
	}
	sessionID := "slack-file:" + base64.RawURLEncoding.EncodeToString(random)
	c.uploadMu.Lock()
	c.uploads[sessionID] = &uploadState{request: input, uploadURL: response.UploadURL, fileID: response.FileID}
	c.uploadMu.Unlock()
	return &socialhub.UploadSession{ID: sessionID, MediaID: response.FileID, PartSize: input.Size}, nil
}

func (c *Client) UploadPart(ctx context.Context, sessionID string, partNumber int, reader io.Reader, options ...socialhub.CallOption) (*socialhub.UploadedPart, error) {
	return c.UploadFilePart(ctx, sessionID, partNumber, reader, options...)
}

func (c *Client) UploadFilePart(ctx context.Context, sessionID string, partNumber int, reader io.Reader, options ...socialhub.CallOption) (*socialhub.UploadedPart, error) {
	if strings.TrimSpace(sessionID) == "" || partNumber != 0 || reader == nil {
		return nil, invalidArgument("upload_file_part", "Slack file upload requires session ID, part 0, and a reader")
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
		return nil, platformError("upload_file_part", socialhub.CodeNotFound, socialhub.ClassPermanent, nil)
	}
	if state.uploading || state.completing || state.uploaded {
		c.uploadMu.Unlock()
		return nil, platformError("upload_file_part", socialhub.CodeConflict, socialhub.ClassPermanent, nil)
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
		header.Set("Content-Disposition", `form-data; name="file"; filename="`+safeFilename(state.request.Filename)+`"`)
		contentType := firstNonEmpty(state.request.MIME, "application/octet-stream")
		header.Set("Content-Type", contentType)
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
		return nil, platformError("upload_file_part", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
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
		return nil, platformError("upload_file_part", socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
	}
	defer response.Body.Close()
	body, readErr := io.ReadAll(io.LimitReader(response.Body, maxUploadReplyBytes+1))
	if readErr != nil {
		return nil, platformError("upload_file_part", socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, readErr)
	}
	if writeErr != nil || counting.count != state.request.Size {
		return nil, invalidArgument("upload_file_part", "uploaded byte count does not match declared size")
	}
	if int64(len(body)) > maxUploadReplyBytes || response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, uploadHTTPError(response.StatusCode, response.Header)
	}
	c.uploadMu.Lock()
	state.uploaded = true
	c.uploadMu.Unlock()
	return &socialhub.UploadedPart{Number: 0, ETag: state.fileID, Size: state.request.Size}, nil
}

func (c *Client) CompleteUpload(ctx context.Context, sessionID string, parts []socialhub.UploadedPart, options ...socialhub.CallOption) (*socialhub.Media, error) {
	return c.CompleteFileUpload(ctx, sessionID, parts, options...)
}

func (c *Client) CompleteFileUpload(ctx context.Context, sessionID string, parts []socialhub.UploadedPart, options ...socialhub.CallOption) (*socialhub.Media, error) {
	if strings.TrimSpace(sessionID) == "" || len(parts) != 1 || parts[0].Number != 0 {
		return nil, invalidArgument("files.completeUploadExternal", "exactly upload part 0 is required")
	}
	c.uploadMu.Lock()
	state := c.uploads[sessionID]
	if state == nil {
		c.uploadMu.Unlock()
		return nil, platformError("files.completeUploadExternal", socialhub.CodeNotFound, socialhub.ClassPermanent, nil)
	}
	if state.uploading || state.completing || !state.uploaded || parts[0].ETag != state.fileID || parts[0].Size != state.request.Size {
		c.uploadMu.Unlock()
		return nil, platformError("files.completeUploadExternal", socialhub.CodeConflict, socialhub.ClassPermanent, nil)
	}
	state.completing = true
	request, fileID := state.request, state.fileID
	c.uploadMu.Unlock()
	defer func() {
		c.uploadMu.Lock()
		if current := c.uploads[sessionID]; current == state {
			current.completing = false
		}
		c.uploadMu.Unlock()
	}()
	threadTS := ""
	if request.ThreadPostID != "" {
		_, threadTS, _ = parseCompositeID(request.ThreadPostID, "files.completeUploadExternal")
	}
	var response struct {
		Files []struct {
			ID    string `json:"id"`
			Title string `json:"title"`
		} `json:"files"`
	}
	if err := c.call(ctx, "files.completeUploadExternal", struct {
		Files []struct {
			ID    string `json:"id"`
			Title string `json:"title,omitempty"`
		} `json:"files"`
		ChannelID      string `json:"channel_id,omitempty"`
		ThreadTS       string `json:"thread_ts,omitempty"`
		InitialComment string `json:"initial_comment,omitempty"`
	}{Files: []struct {
		ID    string `json:"id"`
		Title string `json:"title,omitempty"`
	}{{ID: fileID, Title: request.Title}}, ChannelID: request.ChannelID, ThreadTS: threadTS, InitialComment: request.InitialComment}, &response, options...); err != nil {
		return nil, err
	}
	if len(response.Files) != 1 || response.Files[0].ID != fileID {
		return nil, platformError("files.completeUploadExternal", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	media := mapFile(wireFile{ID: fileID, Name: request.Filename, Title: firstNonEmpty(response.Files[0].Title, request.Title), Mimetype: request.MIME, Size: request.Size})
	c.uploadMu.Lock()
	delete(c.uploads, sessionID)
	c.media[fileID] = media
	c.uploadMu.Unlock()
	return &media, nil
}

func (c *Client) MediaStatus(ctx context.Context, mediaID string, options ...socialhub.CallOption) (*socialhub.Media, error) {
	return c.GetFile(ctx, mediaID, options...)
}

func (c *Client) GetFile(ctx context.Context, mediaID string, options ...socialhub.CallOption) (*socialhub.Media, error) {
	mediaID = strings.TrimSpace(mediaID)
	if !validSlackID(mediaID, "F") {
		return nil, invalidArgument("files.info", "file ID must be a Slack file ID")
	}
	c.uploadMu.Lock()
	media, found := c.media[mediaID]
	c.uploadMu.Unlock()
	if found {
		copy := media
		return &copy, nil
	}
	if err := c.requireScopes("files.info", "files:read"); err != nil {
		return nil, err
	}
	var response struct {
		File wireFile `json:"file"`
	}
	if err := c.call(ctx, "files.info", struct {
		File string `json:"file"`
	}{File: mediaID}, &response, options...); err != nil {
		return nil, err
	}
	if response.File.ID != mediaID {
		return nil, platformError("files.info", socialhub.CodeNotFound, socialhub.ClassPermanent, nil)
	}
	mapped := mapFile(response.File)
	return &mapped, nil
}

func uploadHTTPError(status int, header http.Header) error {
	code, class := socialhub.CodePlatformError, socialhub.ClassPermanent
	if status == http.StatusTooManyRequests {
		code, class = socialhub.CodeRateLimited, socialhub.ClassRetryable
	} else if status >= 500 {
		code, class = socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable
	} else if status >= 400 && status < 500 {
		code = socialhub.CodeInvalidArgument
	}
	return &socialhub.Error{
		Code: code, Class: class, Platform: "slack", Product: productName, Op: "upload_file_part",
		HTTPStatus: status, RequestID: slackRequestID(header), RetryAfter: retryAfter(header.Get("Retry-After")),
	}
}

func validUploadURL(value string, allowHTTP bool) bool {
	parsed, err := url.Parse(value)
	if err != nil || parsed.Host == "" || parsed.User != nil {
		return false
	}
	return parsed.Scheme == "https" || (allowHTTP && parsed.Scheme == "http")
}

func validOpaque(value string, maximum int) bool {
	if value == "" || value != strings.TrimSpace(value) || len(value) > maximum {
		return false
	}
	for _, character := range value {
		if character <= 0x20 || character == 0x7f {
			return false
		}
	}
	return true
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

func validText(value string, maximum int) bool {
	value = strings.TrimSpace(value)
	return value != "" && len([]rune(value)) <= maximum && !strings.ContainsRune(value, 0)
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
