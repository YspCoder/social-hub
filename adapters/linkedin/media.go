package linkedin

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"social-hub/pkg/socialhub"
)

const maxUploadErrorBytes int64 = 1 << 20

type imageUpload struct {
	URL       string
	MIME      string
	Size      int64
	ExpiresAt *time.Time
	Uploading bool
	Uploaded  bool
}

func (c *Client) BeginUpload(ctx context.Context, input socialhub.BeginUploadRequest, options ...socialhub.CallOption) (*socialhub.UploadSession, error) {
	if input.Type != socialhub.MediaTypeImage || input.Size <= 0 || !strings.HasPrefix(input.MIME, "image/") {
		return nil, invalidArgument("begin_upload", "LinkedIn Images API requires an image type, positive size, and image MIME")
	}
	if err := c.requireScopes("begin_upload", c.socialScope(true)); err != nil {
		return nil, err
	}
	body := struct {
		Initialize struct {
			Owner string `json:"owner"`
		} `json:"initializeUploadRequest"`
	}{}
	body.Initialize.Owner = c.authorURN
	var response imageInitializeResponse
	if err := c.transport.JSON(ctx, http.MethodPost, "/rest/images", url.Values{"action": {"initializeUpload"}}, body, &response, options...); err != nil {
		return nil, err
	}
	if !validURN(response.Value.Image) || !validEndpoint(response.Value.UploadURL) {
		return nil, platformError("begin_upload", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	expiresAt := milliseconds(response.Value.UploadURLExpiresAt)
	c.uploadMu.Lock()
	c.uploads[response.Value.Image] = &imageUpload{URL: response.Value.UploadURL, MIME: input.MIME, Size: input.Size, ExpiresAt: expiresAt}
	c.uploadMu.Unlock()
	return &socialhub.UploadSession{ID: response.Value.Image, MediaID: response.Value.Image, PartSize: input.Size, ExpiresAt: expiresAt}, nil
}

func (c *Client) UploadPart(ctx context.Context, sessionID string, partNumber int, reader io.Reader, options ...socialhub.CallOption) (*socialhub.UploadedPart, error) {
	if !validURN(sessionID) || partNumber != 0 || reader == nil {
		return nil, invalidArgument("upload_part", "image session, part 0, and reader are required")
	}
	callOptions, err := socialhub.ResolveCallOptions(options...)
	if err != nil {
		return nil, err
	}
	if callOptions.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, callOptions.Timeout)
		defer cancel()
	}
	c.uploadMu.Lock()
	state := c.uploads[sessionID]
	if state == nil {
		c.uploadMu.Unlock()
		return nil, platformError("upload_part", socialhub.CodeNotFound, socialhub.ClassPermanent, nil)
	}
	if state.Uploading || state.Uploaded {
		c.uploadMu.Unlock()
		return nil, platformError("upload_part", socialhub.CodeConflict, socialhub.ClassPermanent, nil)
	}
	state.Uploading = true
	uploadURL, mimeType, expectedSize := state.URL, state.MIME, state.Size
	c.uploadMu.Unlock()

	counting := &countingReader{reader: reader}
	request, err := http.NewRequestWithContext(ctx, http.MethodPut, uploadURL, counting)
	if err != nil {
		c.setUploadIdle(sessionID)
		return nil, platformError("upload_part", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	request.Header.Set("Authorization", "Bearer "+c.accessToken)
	request.Header.Set("Content-Type", mimeType)
	if callOptions.RequestID != "" {
		request.Header.Set("X-Request-ID", callOptions.RequestID)
	}
	if callOptions.IdempotencyKey != "" {
		request.Header.Set("Idempotency-Key", callOptions.IdempotencyKey)
	}
	request.ContentLength = expectedSize
	response, err := c.httpClient.Do(request)
	if err != nil {
		c.setUploadIdle(sessionID)
		return nil, platformError("upload_part", socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, sanitizeUploadError(err))
	}
	defer response.Body.Close()
	responseBody, readErr := io.ReadAll(io.LimitReader(response.Body, maxUploadErrorBytes+1))
	if readErr != nil {
		c.setUploadIdle(sessionID)
		return nil, platformError("upload_part", socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, readErr)
	}
	if int64(len(responseBody)) > maxUploadErrorBytes {
		c.setUploadIdle(sessionID)
		return nil, platformError("upload_part", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		c.setUploadIdle(sessionID)
		return nil, decodeHTTPError(response.StatusCode, response.Header, responseBody)
	}
	if counting.count != expectedSize {
		c.setUploadIdle(sessionID)
		return nil, invalidArgument("upload_part", "uploaded byte count does not match declared size")
	}
	c.uploadMu.Lock()
	if current := c.uploads[sessionID]; current != nil {
		current.Uploading, current.Uploaded = false, true
	}
	c.uploadMu.Unlock()
	return &socialhub.UploadedPart{Number: 0, Size: counting.count}, nil
}

func sanitizeUploadError(err error) error {
	var urlError *url.Error
	if errors.As(err, &urlError) && urlError.Err != nil {
		return urlError.Err
	}
	return err
}

func (c *Client) CompleteUpload(_ context.Context, sessionID string, parts []socialhub.UploadedPart, _ ...socialhub.CallOption) (*socialhub.Media, error) {
	if !validURN(sessionID) || len(parts) != 1 || parts[0].Number != 0 {
		return nil, invalidArgument("complete_upload", "exactly image upload part 0 is required")
	}
	c.uploadMu.Lock()
	state := c.uploads[sessionID]
	if state == nil {
		c.uploadMu.Unlock()
		return nil, platformError("complete_upload", socialhub.CodeNotFound, socialhub.ClassPermanent, nil)
	}
	if !state.Uploaded || parts[0].Size != state.Size {
		c.uploadMu.Unlock()
		return nil, platformError("complete_upload", socialhub.CodeConflict, socialhub.ClassPermanent, nil)
	}
	size, mimeType, expiresAt := state.Size, state.MIME, state.ExpiresAt
	c.uploadMu.Unlock()
	return &socialhub.Media{ID: sessionID, MIME: mimeType, Type: socialhub.MediaTypeImage, Size: &size, State: socialhub.MediaStateProcessing, ExpiresAt: expiresAt}, nil
}

func (c *Client) MediaStatus(ctx context.Context, mediaID string, options ...socialhub.CallOption) (*socialhub.Media, error) {
	if !strings.HasPrefix(mediaID, "urn:li:image:") || !validURN(mediaID) {
		return nil, invalidArgument("media_status", "media ID must be a LinkedIn image URN")
	}
	if err := c.requireScopes("media_status", c.socialScope(true)); err != nil {
		return nil, err
	}
	var response linkedInImage
	if err := c.transport.JSON(ctx, http.MethodGet, "/rest/images/"+mediaID, nil, nil, &response, options...); err != nil {
		return nil, err
	}
	if response.ID == "" {
		response.ID = mediaID
	}
	state := socialhub.MediaStateProcessing
	switch response.Status {
	case "WAITING_UPLOAD":
		state = socialhub.MediaStateUploading
	case "AVAILABLE":
		state = socialhub.MediaStateReady
	case "PROCESSING_FAILED":
		state = socialhub.MediaStateFailed
	}
	extension, _ := json.Marshal(struct {
		Status string `json:"status"`
	}{Status: response.Status})
	return &socialhub.Media{
		ID: response.ID, URL: response.DownloadURL, Type: socialhub.MediaTypeImage, State: state,
		ExpiresAt: milliseconds(response.DownloadURLTTL), Extensions: map[string]json.RawMessage{"linkedin.image": extension},
	}, nil
}

func (c *Client) setUploadIdle(sessionID string) {
	c.uploadMu.Lock()
	defer c.uploadMu.Unlock()
	if state := c.uploads[sessionID]; state != nil {
		state.Uploading = false
	}
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
